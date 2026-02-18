# AvyOS Project Summary

> This document helps developers and AI assistants understand the project quickly without reading the full codebase.

## What is AvyOS?

AvyOS is a **Linux-based operating system** with userspace written entirely in **pure Go** — no external packages, no CGO, no POSIX compatibility. It reimagines the traditional Linux userspace from scratch.

## Key Characteristics

| Property              | Value                   |
| --------------------- | ----------------------- |
| Language              | Pure Go (CGO_ENABLED=0) |
| Build Tags            | `-tags netgo`           |
| Go Version            | 1.21+                   |
| Module Path           | `avyos.dev`             |
| External Dependencies | None (stdlib only)      |
| Target Architectures  | arm64, amd64            |
| License               | GPL v3.0                |

## Project Philosophy

1. **Pure Go** — Single language, zero C dependencies
2. **Simple** — Minimal design, no bloat
3. **Auditable** — Small, readable codebase
4. **Secure** — Immutable core, memory-safe, capability-based
5. **No Legacy** — Non-POSIX, clean modern interfaces
6. **Identity-First** — Permissions tied to user identity

## Directory Structure

```
avyos/
├── cmd/              # System commands (23 binaries)
├── apps/             # User applications (4 apps)
├── services/         # System services (3 services)
├── pkg/              # Shared packages
│   ├── parse/        # Command-line parsing framework
│   ├── format/       # Output formatting (colors, tables)
│   ├── fs/           # Filesystem utilities
│   ├── ui/           # TUI framework (16 widgets)
│   └── ...
├── config/           # Default system configuration
├── data/             # Static data (icons, fonts, themes)
├── scripts/          # Build scripts
├── external/         # Architecture-specific files (kernel, firmware)
├── web/              # Project website
└── docs/             # Documentation
```

## Filesystem Hierarchy (Runtime)

```
/
├── avyos/            # Immutable system (read-only squashfs)
│   ├── cmd/          # Commands
│   ├── config/       # Default configs
│   ├── apps/         # Applications
│   └── services/     # Services
├── config/           # Mutable config overrides
├── users/            # Home directories
├── cache/
│   ├── kernel/       # procfs, sysfs, devtmpfs
│   └── runtime/      # Runtime state (like /run)
└── linux/            # POSIX compatibility containers
    ├── alpine/
    ├── debian/
    └── fedora/
```

## Commands (cmd/)

| Command    | Description                                         |
| ---------- | --------------------------------------------------- |
| `init`     | Init system (PID 1), service manager                |
| `shell`    | Interactive shell with builtins                     |
| `list`     | List directory contents                             |
| `read`     | Read files (subcommands: top, last, pattern, count) |
| `write`    | Write to files                                      |
| `copy`     | Copy files/directories                              |
| `move`     | Move/rename files                                   |
| `delete`   | Delete files                                        |
| `mkdir`    | Create directories                                  |
| `link`     | Create symbolic links                               |
| `find`     | Search for files                                    |
| `tree`     | Directory tree view                                 |
| `info`     | File/system information                             |
| `mount`    | Mount filesystems                                   |
| `net`      | Network config (interfaces, address, route)         |
| `request`  | HTTP/DNS (ping, fetch, dns, listen, connect)        |
| `process`  | Process management                                  |
| `power`    | Shutdown/reboot                                     |
| `identity` | User/identity management                            |
| `system`   | System utilities                                    |

## Apps (apps/)

| App         | Description                            |
| ----------- | -------------------------------------- |
| `browser`   | HTTP browser with basic HTML rendering |
| `notepad`   | Text editor (nano-style)               |
| `welcome`   | First-boot setup wizard                |
| `installer` | Package/system installer               |

## Services (services/)

| Service   | Description                                       |
| --------- | ------------------------------------------------- |
| `sutra`   | IPC message bus                                   |
| `login`   | Authentication/login manager                      |
| `desktop` | Tiling window manager (BSP, grid, spiral layouts) |

## Key Packages

### Command Parsing (`flag`)
```go
var flagVerbose bool

func init() {
    flag.BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
}

func main() {
    flag.Parse()
    args := flag.Args()
    _ = args
}
```

### `pkg/format` — Output Formatting
- `format.Color(style, text)` — Colored output
- `format.NewTable(headers...)` — Table output
- `format.Size(bytes)` — Human-readable sizes
- `format.Error()`, `format.Success()`, `format.Warning()`

Colors: `format.Red`, `format.Green`, `format.Blue`, `format.Yellow`, `format.Cyan`, `format.Bold`

### `pkg/ui` — TUI Framework
```go
type MyWidget struct {
    ui.BaseWidget
}

func (w *MyWidget) HandleEvent(event ui.Event) bool { }
func (w *MyWidget) Render(area ui.Drawable, ctx *ui.Context) { }

// Run app
ui.Run(widget, backend.NewTerm())
```

Widgets: Box, Button, Checkbox, Input, Label, List, Table, Flex, Grid, Progress, etc.

### `pkg/fs` — Filesystem
- `fs.ListDir(path)` — List directory
- `fs.ReadFile(path)` — Read file
- `fs.WriteFile(path, data)` — Write file
- `fs.Stat(path)` — File info
- `fs.PermString(mode)` — Permission string

## Build System

```bash
# Build disk image
make GOARCH=arm64

# Run in QEMU
make GOARCH=arm64 run

# Clean
make clean
```

Build output: `_cache/<arch>/system/` → `system.img` (squashfs)

## Adding New Commands

1. Create `cmd/<name>/main.go`
2. Use the `flag` package pattern
3. Add to `COMMANDS` in Makefile

```go
package main

import (
    "flag"
    "fmt"
    "os"

    "avyos.dev/pkg/format"
)

var flagOption bool

func init() {
    flag.BoolVar(&flagOption, "option", false, "description")
    flag.Usage = func() {
        fmt.Fprintln(os.Stderr, "Usage: name [options] <args>")
        flag.PrintDefaults()
    }
}

func main() {
    flag.Parse()

    if err := run(flag.Args()); err != nil {
        format.Error("%s", err)
        os.Exit(1)
    }
}

func run(args []string) error {
    // implementation
    return nil
}
```

## Adding New Apps

1. Create `apps/<name>/main.go`
2. Implement widget with `ui.BaseWidget`
3. Add to `APPS` in Makefile

```go
package main

import (
    "avyos.dev/pkg/ui"
    "avyos.dev/pkg/ui/backend"
)

type MyApp struct {
    ui.BaseWidget
}

func (a *MyApp) HandleEvent(event ui.Event) bool { return false }
func (a *MyApp) Render(area ui.Drawable, ctx *ui.Context) { }

func main() {
    ui.Run(&MyApp{}, backend.NewTerm())
}
```

## Networking

- **Low-level**: `cmd/net/` uses Netlink sockets for interface config
- **HTTP/DNS**: `cmd/request/` uses stdlib `net` and `net/http`
- **Browser**: `apps/browser/` uses raw TCP with `net.Dial()`

Currently HTTP only (no TLS), static IPv4.

## IPC (Sutra)

Message-based IPC with code generation. Services communicate through `/cache/runtime/sutra.sock`.

## Security Model

- **No root/sudo** — Capability-based permissions
- **Identity-based** — Permissions tied to user identity
- **Immutable core** — `/avyos/` is read-only squashfs
- **Mount restrictions** — `nosuid`, `nodev`, `noexec` where appropriate

## Current Status

| Component       | Status                   |
| --------------- | ------------------------ |
| Boot & Init     | Done                     |
| Shell           | Done                     |
| Sutra IPC       | Done                     |
| TUI Toolkit     | Done (16 widgets)        |
| Desktop/WM      | Done (BSP, grid, spiral) |
| First Boot      | Done                     |
| Networking      | Basic (static IPv4)      |
| Core Utils      | Minimal (23 commands)    |
| Framebuffer     | TODO                     |
| Audio           | TODO                     |
| Package Manager | TODO                     |

## Common Patterns

### Error Handling
```go
if err != nil {
    return fmt.Errorf("context: %w", err)
}
```

### Flag Access
```go
flagSet := flag.NewFlagSet("subcommand", flag.ContinueOnError)
showAll := flagSet.Bool("all", false, "Show all")
output := flagSet.String("output", "", "Output file")
count := flagSet.Int("count", 0, "Count")
_ = []any{showAll, output, count}
```

### Colored Output
```go
format.Success("Done: %s", path)
format.Error("Failed: %s", err)
format.Color(format.Blue+format.Bold, "text")
```

### File Operations
```go
infos, err := fs.ListDir(path)
data, err := fs.ReadFile(path)
err := fs.WriteFile(path, data, 0644)
```

## Testing

No external test framework. Run:
```bash
go build ./...
```

## Notes for Contributors

- No external dependencies allowed
- All code must build with `CGO_ENABLED=0`
- Follow existing patterns for consistency
- Keep commands simple and focused
- Use `pkg/format` for all user output
- Use the standard `flag` package for command-line parsing
