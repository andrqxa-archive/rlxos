// Package dl provides a pure-Go ELF dynamic loader / runtime linker.
//
// SECURITY WARNING: This package loads and executes native machine code from
// disk. Treat all .so files as untrusted unless independently verified. Loading
// a shared object is equivalent to executing arbitrary native code in the
// current process. No sandboxing is provided or claimed.
package dl

import "fmt"

// LoadError is returned when an ELF object cannot be loaded.
type LoadError struct {
	Path string
	Err  error
}

func (e *LoadError) Error() string { return fmt.Sprintf("dl: load %q: %v", e.Path, e.Err) }
func (e *LoadError) Unwrap() error { return e.Err }

// SymbolError is returned when a symbol cannot be resolved.
type SymbolError struct {
	Name    string
	Version string // empty if unversioned
	Object  string // object that references the symbol
	Err     error
}

func (e *SymbolError) Error() string {
	if e.Version != "" {
		return fmt.Sprintf("dl: symbol %q@@%s in %q: %v", e.Name, e.Version, e.Object, e.Err)
	}
	return fmt.Sprintf("dl: symbol %q in %q: %v", e.Name, e.Object, e.Err)
}
func (e *SymbolError) Unwrap() error { return e.Err }

// RelocError is returned when a relocation cannot be applied.
type RelocError struct {
	Type   uint32 // R_* relocation type number
	Symbol string // symbol name, if any
	Object string // object containing the relocation
	Err    error
}

func (e *RelocError) Error() string {
	return fmt.Sprintf("dl: reloc type %d for symbol %q in %q: %v", e.Type, e.Symbol, e.Object, e.Err)
}
func (e *RelocError) Unwrap() error { return e.Err }

// FormatError is returned when an ELF file is malformed.
type FormatError struct {
	Path   string
	Detail string
}

func (e *FormatError) Error() string {
	return fmt.Sprintf("dl: %q: bad ELF: %s", e.Path, e.Detail)
}

// Common sentinel errors.
var (
	ErrNotFound       = fmt.Errorf("dl: library not found")
	ErrNotELF         = fmt.Errorf("dl: not an ELF file")
	ErrWrongArch      = fmt.Errorf("dl: wrong ELF machine type for this architecture")
	ErrNotShared      = fmt.Errorf("dl: ELF type is not ET_DYN (shared object / PIE)")
	ErrSymbolNotFound = fmt.Errorf("dl: symbol not found")
	ErrClosed         = fmt.Errorf("dl: handle is closed")
	ErrUnsupported    = fmt.Errorf("dl: unsupported")
)
