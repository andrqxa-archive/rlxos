# pkg/dl — Pure-Go ELF Dynamic Loader

A runtime ELF dynamic linker implemented entirely in Go, with no cgo or
external dependencies.  Targets **Linux amd64 (x86-64)** and **Linux arm64
(AArch64)**.

## Security Warning

**This package loads and executes native machine code from disk.**  Loading a
shared object (`.so`) is equivalent to executing arbitrary code in the current
process.  No sandboxing is provided or claimed.  Treat all `.so` files as
untrusted unless you have independently verified them.

## What This Is

A "mini ld.so" for userspace Go programs.  It can:

- Load ELF shared objects (ET_DYN) from the filesystem
- Recursively resolve dependencies (DT_NEEDED)
- Perform relocations (RELATIVE, GLOB_DAT, JUMP_SLOT, ABS64, TLS, TLSDESC, IRELATIVE)
- Resolve symbols using GNU hash and SysV hash tables
- Support symbol versioning (VERSYM/VERNEED/VERDEF)
- Run constructors/destructors (DT_INIT/DT_INIT_ARRAY, DT_FINI/DT_FINI_ARRAY)
- Apply GNU_RELRO protections
- Handle TLS relocations (DTPMOD, DTPOFF, TPOFF, TLSDESC)
- Call exported functions with integer/pointer arguments

## Supported Platforms

| Architecture | Build Tag | Relocations |
|---|---|---|
| x86-64 | `GOARCH=amd64` | RELATIVE, GLOB_DAT, JUMP_SLOT, 64, DTPMOD64, DTPOFF64, TPOFF64, TLSDESC, IRELATIVE |
| AArch64 | `GOARCH=arm64` | RELATIVE, GLOB_DAT, JUMP_SLOT, ABS64, TLS_DTPMOD64, TLS_DTPOFF64, TLS_TPREL64, TLSDESC, IRELATIVE |

## Limitations

- **No floating-point arguments**: The `Call*` helpers use `syscall.Syscall`
  which only passes integer registers.  Functions like `cos(double)` cannot be
  called correctly.
- **TLS access from Go**: TLS relocations are applied for structural
  correctness, but Go manages its own thread-local storage (FS/TPIDR_EL0).
  Calling C functions that access TLS variables from Go goroutines may not work.
- **Lazy binding**: PLT lazy binding is implemented as eager resolution because
  we cannot inject a resolver trampoline into the loaded code.  The `RTLD_LAZY`
  flag is accepted but behaves identically to `RTLD_NOW`.
- **RTLD_DEEPBIND**: Implemented for symbol lookup ordering but may not match
  glibc behavior in all edge cases.
- **Init/Fini**: Constructors and destructors are called via `syscall.Syscall`,
  which works for simple init functions but may fail for complex constructors
  that depend on full C runtime state.

## Building

```bash
# Native build (amd64)
GOOS=linux GOARCH=amd64 go build ./pkg/dl/...
GOOS=linux GOARCH=amd64 go build ./cmd/dl/...

# Cross-compile for arm64
GOOS=linux GOARCH=arm64 go build ./pkg/dl/...
GOOS=linux GOARCH=arm64 go build ./cmd/dl/...
```

## Running Examples

All examples use the single `cmd/dltest` binary with subcommands:

### Using system libraries (default root `/`)

```bash
go run ./cmd/dltest puts
go run ./cmd/dltest info libc.so.6
go run ./cmd/dltest selfcheck
```

### Using an alternate sysroot (e.g. `/linux`)

```bash
go run ./cmd/dltest -root /linux puts
go run ./cmd/dltest -root /linux info libc.so.6
go run ./cmd/dltest -root /linux selfcheck
```

## Debugging

Set the environment variable `LOADER_DEBUG=1` or use `dl.WithDebug()`:

```bash
LOADER_DEBUG=1 go run ./cmd/dltest puts
```

Or programmatically:

```go
ld := dl.New(dl.WithDebug())
```

## API Overview

```go
// Create a loader with options.
ld := dl.New(
    dl.WithRoot("/linux"),
    dl.WithSearchPaths("/opt/lib"),
    dl.WithDebug(),
)

// Load a shared object.
h, err := ld.Open("libc.so.6", dl.RTLD_NOW|dl.RTLD_GLOBAL)
defer h.Close()

// Resolve a symbol.
addr, err := h.Sym("puts")

// Resolve a versioned symbol.
addr, err = h.SymVersion("puts", "GLIBC_2.2.5")

// Call the function.
buf, ptr := dl.CString("hello")
_ = buf // keep alive
dl.Call1(addr, ptr)

// Check stats.
stats := ld.Stats()
```

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│                  Loader                      │
│  - Object cache (by path + soname)           │
│  - Global scope (RTLD_GLOBAL objects)        │
│  - TLS module allocator                      │
│  - Search path configuration                 │
├─────────────────────────────────────────────┤
│                  Object                      │
│  - Mapped PT_LOAD segments                   │
│  - Parsed dynamic section (DT_*)             │
│  - Symbol table + string table               │
│  - GNU hash / SysV hash                      │
│  - Version tables (VERSYM/VERDEF/VERNEED)    │
│  - Relocation tables (RELA/REL/JMPREL)       │
│  - TLS metadata                              │
│  - Init/fini arrays                          │
│  - Refcount + state flags                    │
├─────────────────────────────────────────────┤
│                  Handle                      │
│  - Primary object pointer                    │
│  - Local scope (self + transitive deps)      │
│  - Sym() / SymVersion() / Close()            │
└─────────────────────────────────────────────┘
```

### Loading flow

1. **Search**: Resolve soname to filesystem path (RUNPATH → LD_LIBRARY_PATH → RPATH → defaults)
2. **Parse**: Read ELF header + program headers, validate e_machine
3. **Map**: Reserve address range, mmap PT_LOAD segments with correct protections
4. **Dynamic**: Parse PT_DYNAMIC for tables, strings, symbols, hashes, versions
5. **Dependencies**: BFS-load all DT_NEEDED libraries
6. **Relocate**: Apply RELA/REL/JMPREL entries (deps first, then primary)
7. **RELRO**: mprotect GNU_RELRO region as read-only
8. **Init**: Call DT_INIT then DT_INIT_ARRAY (deps first)
9. **Return**: Handle with local scope for symbol lookup

### Symbol resolution order

1. DEEPBIND → search self first
2. Local scope (handle's objects in load order)
3. Global scope (all RTLD_GLOBAL objects in load order)
4. Weak symbols accepted as fallback

## File Layout

| File | Purpose |
|---|---|
| `dl.go` | Public API: Loader, Handle, Flags, Options, Stats |
| `elf.go` | ELF64 constants and structures |
| `errors.go` | Typed error types |
| `object.go` | Object struct, ELF header/phdr parsing, symbol access |
| `loader.go` | Open/Close, library search, dependency loading |
| `dynamic.go` | PT_DYNAMIC parsing and rebasing |
| `map.go` | mmap, protections, BSS, RELRO |
| `sym.go` | GNU hash, SysV hash, versioning, scoped lookup |
| `reloc.go` | Relocation engine (table iteration) |
| `reloc_amd64.go` | x86-64 relocation dispatch |
| `reloc_arm64.go` | AArch64 relocation dispatch |
| `tls.go` | TLS module/offset allocation |
| `tls_amd64.go` | x86-64 TLS variant II notes |
| `tls_arm64.go` | AArch64 TLS variant I notes |
| `initfini.go` | Constructor/destructor calling |
| `call.go` | CString helpers |
| `call_amd64.go` | x86-64 Call0..Call6, IFUNC resolver |
| `call_arm64.go` | AArch64 Call0..Call6, IFUNC resolver |
| `arch_amd64.go` | x86-64 machine validation + default paths |
| `arch_arm64.go` | AArch64 machine validation + default paths |
