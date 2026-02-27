// Package dl implements a pure-Go ELF dynamic loader / runtime linker for
// Linux amd64 and arm64.  It provides dlopen/dlsym semantics: loading shared
// objects (.so files) at runtime, resolving symbols, performing relocations,
// and calling exported functions — all without cgo.
//
// SECURITY WARNING: Loading a shared object executes arbitrary native machine
// code in the current process.  No sandboxing is provided or claimed.  Treat
// all .so files as untrusted unless you have independently verified them.
//
// # Quick start
//
//	ld := dl.New(dl.WithRoot("/linux"))
//	h, err := ld.Open("libc.so.6", dl.RTLD_NOW|dl.RTLD_GLOBAL)
//	if err != nil { log.Fatal(err) }
//	defer h.Close()
//	addr, err := h.Sym("puts")
//	if err != nil { log.Fatal(err) }
//	dl.Call1(addr, dl.CString("hello world"))
package dl

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// ──────────────────────────────────────────────────────────────────────────────
// Flags — mirrors RTLD_* semantics
// ──────────────────────────────────────────────────────────────────────────────

// Flags control how a shared object is loaded and bound.
type Flags uint32

const (
	// RTLD_LAZY defers function binding until first call (PLT lazy binding).
	RTLD_LAZY Flags = 0x00001

	// RTLD_NOW resolves all symbols before Open returns.
	RTLD_NOW Flags = 0x00002

	// RTLD_GLOBAL makes the object's symbols available for subsequent loads.
	RTLD_GLOBAL Flags = 0x00100

	// RTLD_LOCAL restricts symbol scope to the handle (default).
	RTLD_LOCAL Flags = 0x00000

	// RTLD_DEEPBIND causes the loaded object to prefer its own symbols over
	// those in the global scope when resolving references.
	RTLD_DEEPBIND Flags = 0x00008

	// RTLD_NODELETE prevents unloading even when refcount reaches zero.
	RTLD_NODELETE Flags = 0x01000

	// RTLD_NOLOAD does not load the object; returns a handle only if already
	// loaded (useful for promoting to RTLD_GLOBAL or getting stats).
	RTLD_NOLOAD Flags = 0x00004
)

// ──────────────────────────────────────────────────────────────────────────────
// Stats
// ──────────────────────────────────────────────────────────────────────────────

// Stats contains diagnostic counters for a Loader.
type Stats struct {
	LoadedObjects int    // number of currently loaded ELF objects
	TotalRelocs   uint64 // total relocations applied across all objects
	SymbolLookups uint64 // total symbol lookup attempts
	CacheHits     uint64 // symbol cache hits
}

// ──────────────────────────────────────────────────────────────────────────────
// Handle — returned by Open, provides Sym and Close
// ──────────────────────────────────────────────────────────────────────────────

// Handle represents a loaded shared object (or group of objects for transitive
// dependencies).  Use Sym to resolve exported symbols and Close to decrement
// the reference count.
type Handle struct {
	loader *Loader
	obj    *Object   // primary object
	local  []*Object // local scope (self + deps in load order)
	closed bool
	mu     sync.Mutex
}

// Sym resolves a symbol by name and returns its absolute virtual address.
// The returned uintptr can be passed to Call0..Call6 helpers.
func (h *Handle) Sym(name string) (uintptr, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return 0, ErrClosed
	}
	return h.loader.lookupSymbol(name, "", h.obj, h.local)
}

// SymVersion resolves a versioned symbol.  Version should match the version
// string from the library's version definition (e.g. "GLIBC_2.17").
func (h *Handle) SymVersion(name, version string) (uintptr, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return 0, ErrClosed
	}
	return h.loader.lookupSymbol(name, version, h.obj, h.local)
}

// Close decrements the reference count of all objects in this handle's scope.
// When an object's count reaches zero, its destructors (DT_FINI_ARRAY, DT_FINI)
// are called and its mappings are released — in reverse dependency order.
func (h *Handle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return ErrClosed
	}
	h.closed = true
	return h.loader.closeHandle(h)
}

// ──────────────────────────────────────────────────────────────────────────────
// Option — functional options for New
// ──────────────────────────────────────────────────────────────────────────────

// Option configures a Loader.
type Option func(*Loader)

// WithRoot sets an alternate filesystem root.  Library search paths like /lib,
// /usr/lib become <root>/lib, <root>/usr/lib.  Default is "/".
func WithRoot(root string) Option {
	return func(ld *Loader) { ld.root = root }
}

// WithSearchPaths adds extra directories to the default library search list.
func WithSearchPaths(paths ...string) Option {
	return func(ld *Loader) { ld.extraPaths = append(ld.extraPaths, paths...) }
}

// WithDebug enables verbose debug logging to stderr.
func WithDebug() Option {
	return func(ld *Loader) { ld.debug = true }
}

// WithLogger sets a custom logger for debug output.
func WithLogger(l *log.Logger) Option {
	return func(ld *Loader) { ld.logger = l; ld.debug = true }
}

// WithLDLibraryPath enables reading LD_LIBRARY_PATH from the environment and
// prepending those directories to the search list.
func WithLDLibraryPath() Option {
	return func(ld *Loader) { ld.useLDPath = true }
}

// ──────────────────────────────────────────────────────────────────────────────
// Loader — the main entry point
// ──────────────────────────────────────────────────────────────────────────────

// Loader holds global state for the dynamic linker: loaded objects, search
// paths, the global symbol scope, and caching.  It is safe for concurrent use.
type Loader struct {
	mu sync.Mutex // guards all mutable state

	root       string   // filesystem root (default "/")
	extraPaths []string // additional search paths
	useLDPath  bool     // honour LD_LIBRARY_PATH?

	// All loaded objects keyed by resolved path.
	objects map[string]*Object

	// Global scope: objects opened with RTLD_GLOBAL, in load order.
	global []*Object

	// TLS state: monotonically increasing module ID and offset allocator.
	nextTLSModuleID uint64
	tlsOffset       uint64 // total static TLS size allocated so far

	// Statistics.
	stats Stats

	// Debug / logging.
	debug  bool
	logger *log.Logger
}

// New creates a Loader with the given options.
func New(opts ...Option) *Loader {
	ld := &Loader{
		root:    "/",
		objects: make(map[string]*Object),
		logger:  log.New(os.Stderr, "dl: ", log.Lmicroseconds),
	}
	for _, o := range opts {
		o(ld)
	}
	// Check LOADER_DEBUG env if debug not already enabled.
	if !ld.debug {
		if v := os.Getenv("LOADER_DEBUG"); v == "1" || v == "true" {
			ld.debug = true
		}
	}
	return ld
}

// AddSearchPath appends a directory to the library search list.
func (ld *Loader) AddSearchPath(path string) {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	ld.extraPaths = append(ld.extraPaths, path)
}

// SetRoot changes the filesystem root for library search.
func (ld *Loader) SetRoot(root string) {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	ld.root = root
}

// Stats returns a snapshot of loader statistics.
func (ld *Loader) Stats() Stats {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	s := ld.stats
	// Count unique objects (map may have both path and soname keys).
	seen := make(map[*Object]bool, len(ld.objects))
	for _, o := range ld.objects {
		seen[o] = true
	}
	s.LoadedObjects = len(seen)
	return s
}

// Objects returns a deduplicated snapshot of all loaded objects for debugging.
// The map may contain both path and soname keys pointing to the same object;
// this method deduplicates by pointer identity.
func (ld *Loader) Objects() []*Object {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	seen := make(map[*Object]bool, len(ld.objects))
	out := make([]*Object, 0, len(ld.objects))
	for _, o := range ld.objects {
		if !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	return out
}

// logf logs a debug message if debug mode is enabled.
func (ld *Loader) logf(format string, args ...interface{}) {
	if ld.debug {
		ld.logger.Output(2, fmt.Sprintf(format, args...))
	}
}
