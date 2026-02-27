// Command dltest exercises the pure-Go ELF dynamic loader (pkg/dl).
//
// Subcommands:
//
//	dltest puts [message]       — load libc and call puts(3)
//	dltest info <library>       — print loaded objects, deps, symbols, stats
//	dltest selfcheck            — run validation checks against system libs
//
// Flags:
//
//	-root  string   filesystem root for library search (default "/")
//	-debug          enable debug logging
//
// SECURITY WARNING: This program loads and executes native machine code.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"avyos.dev/pkg/dl"
)

func main() {
	root := flag.String("root", "/", "filesystem root for library search")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: dltest [-root ROOT] [-debug] <puts|info|selfcheck> [args...]\n")
		os.Exit(1)
	}

	opts := []dl.Option{dl.WithRoot(*root)}
	if *debug {
		opts = append(opts, dl.WithDebug())
	}

	switch flag.Arg(0) {
	case "puts":
		cmdPuts(opts, flag.Args()[1:])
	case "info":
		cmdInfo(opts, flag.Args()[1:])
	case "selfcheck":
		cmdSelfcheck(opts)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", flag.Arg(0))
		os.Exit(1)
	}
}

func cmdPuts(opts []dl.Option, args []string) {
	ld := dl.New(opts...)

	msg := "Hello from pure-Go ELF loader!"
	if len(args) > 0 {
		msg = args[0]
	}

	h, err := openLibc(ld)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load libc: %v\n", err)
		os.Exit(1)
	}
	defer h.Close()

	putsAddr, err := h.Sym("puts")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve puts: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("libc loaded, puts @ 0x%x\n", putsAddr)

	buf, ptr := dl.CString(msg)
	_ = buf // keep alive for GC
	ret := dl.Call1(putsAddr, ptr)
	fmt.Printf("puts returned: %d\n", ret)

	stats := ld.Stats()
	fmt.Printf("stats: %d objects loaded, %d relocations, %d symbol lookups\n",
		stats.LoadedObjects, stats.TotalRelocs, stats.SymbolLookups)
}

func cmdInfo(opts []dl.Option, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: dltest info <library>\n")
		os.Exit(1)
	}
	libName := args[0]

	ld := dl.New(opts...)
	h, err := ld.Open(libName, dl.RTLD_NOW|dl.RTLD_GLOBAL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer h.Close()

	fmt.Println("=== Loaded Objects ===")
	fmt.Println()
	for _, obj := range ld.Objects() {
		fmt.Printf("  Path:     %s\n", obj.Path)
		if obj.Soname != "" {
			fmt.Printf("  Soname:   %s\n", obj.Soname)
		}
		fmt.Printf("  Base:     0x%x\n", obj.Base)
		fmt.Printf("  MapSize:  0x%x (%d KB)\n", obj.MapSize, obj.MapSize/1024)
		fmt.Printf("  Segments: %d phdrs\n", len(obj.Phdrs))

		if len(obj.Needed) > 0 {
			fmt.Printf("  Needed:   %v\n", obj.Needed)
		}
		if obj.Rpath != "" {
			fmt.Printf("  RPATH:    %s\n", obj.Rpath)
		}
		if obj.Runpath != "" {
			fmt.Printf("  RUNPATH:  %s\n", obj.Runpath)
		}

		for _, sym := range []string{"printf", "malloc", "free", "strlen", "puts"} {
			addr, err := h.Sym(sym)
			if err == nil {
				fmt.Printf("  sym %-12s = 0x%x\n", sym, addr)
			}
		}
		fmt.Println()
	}

	stats := ld.Stats()
	fmt.Println("=== Statistics ===")
	fmt.Printf("  Loaded objects:  %d\n", stats.LoadedObjects)
	fmt.Printf("  Total relocs:    %d\n", stats.TotalRelocs)
	fmt.Printf("  Symbol lookups:  %d\n", stats.SymbolLookups)
}

var (
	passed int
	failed int
)

func check(name string, ok bool, detail string) {
	if ok {
		fmt.Printf("  PASS  %s\n", name)
		passed++
	} else {
		fmt.Printf("  FAIL  %s: %s\n", name, detail)
		failed++
	}
}

func cmdSelfcheck(opts []dl.Option) {
	fmt.Printf("dl selfcheck (arch=%s)\n\n", runtime.GOARCH)

	ld := dl.New(opts...)

	// Load libc.
	fmt.Println("[libc loading]")
	h, err := openLibc(ld)
	check("load libc", err == nil, fmt.Sprint(err))
	if err != nil {
		fmt.Printf("\nCannot continue without libc. Results: %d passed, %d failed\n", passed, failed)
		os.Exit(1)
	}
	defer h.Close()

	// Known symbols.
	fmt.Println("\n[symbol resolution]")
	for _, sym := range []string{"puts", "printf", "malloc", "free", "strlen", "memcpy", "write", "read", "open", "close"} {
		addr, err := h.Sym(sym)
		check(fmt.Sprintf("resolve %s", sym), err == nil && addr != 0,
			fmt.Sprintf("err=%v addr=0x%x", err, addr))
	}

	// Versioned symbol.
	fmt.Println("\n[versioned symbols]")
	var verSym, verStr string
	switch runtime.GOARCH {
	case "amd64":
		verSym, verStr = "puts", "GLIBC_2.2.5"
	case "arm64":
		verSym, verStr = "puts", "GLIBC_2.17"
	}
	if verSym != "" {
		addr, err := h.SymVersion(verSym, verStr)
		check(fmt.Sprintf("versioned %s@@%s", verSym, verStr), err == nil && addr != 0,
			fmt.Sprintf("err=%v addr=0x%x", err, addr))
	}

	// Stats.
	fmt.Println("\n[statistics]")
	stats := ld.Stats()
	check("loaded objects > 0", stats.LoadedObjects > 0, fmt.Sprintf("got %d", stats.LoadedObjects))
	check("total relocs > 0", stats.TotalRelocs > 0, fmt.Sprintf("got %d", stats.TotalRelocs))
	check("symbol lookups > 0", stats.SymbolLookups > 0, fmt.Sprintf("got %d", stats.SymbolLookups))

	// Load libm.
	fmt.Println("\n[dependency loading]")
	hm, merr := openLib(ld, "libm.so.6", "libm.so")
	if hm != nil {
		check("load libm", true, "")
		addr, err := hm.Sym("ceil")
		check("resolve ceil", err == nil && addr != 0, fmt.Sprintf("err=%v addr=0x%x", err, addr))
		hm.Close()
	} else {
		check("load libm", false, fmt.Sprint(merr))
	}

	// Double open / refcount.
	fmt.Println("\n[refcounting]")
	h2, err := openLibc(ld)
	check("double open libc", err == nil, fmt.Sprint(err))
	if h2 != nil {
		addr1, _ := h.Sym("puts")
		addr2, _ := h2.Sym("puts")
		check("same address on double open", addr1 == addr2,
			fmt.Sprintf("0x%x vs 0x%x", addr1, addr2))
		h2.Close()
	}

	// Summary.
	fmt.Printf("\n=== Results: %d passed, %d failed ===\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func openLibc(ld *dl.Loader) (*dl.Handle, error) {
	return openLib(ld, "libc.so.6", "libc.so")
}

func openLib(ld *dl.Loader, names ...string) (*dl.Handle, error) {
	var lastErr error
	for _, name := range names {
		h, err := ld.Open(name, dl.RTLD_NOW|dl.RTLD_GLOBAL)
		if err == nil {
			return h, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
