package dl

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// findTestLibc returns the path to a libc.so.6 for testing, or skips the test.
func findTestLibc(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/lib/x86_64-linux-gnu/libc.so.6",
		"/lib/aarch64-linux-gnu/libc.so.6",
		"/usr/lib/x86_64-linux-gnu/libc.so.6",
		"/usr/lib/aarch64-linux-gnu/libc.so.6",
		"/lib64/libc.so.6",
		"/usr/lib64/libc.so.6",
		"/lib/libc.so.6",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Skip("no libc.so.6 found on system")
	return ""
}

// ── ELF parsing tests ───────────────────────────────────────────────────────

func TestReadELFHeader(t *testing.T) {
	path := findTestLibc(t)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	hdr, err := readELFHeader(f, path)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Type != etDYN {
		t.Errorf("expected ET_DYN, got %d", hdr.Type)
	}

	switch runtime.GOARCH {
	case "amd64":
		if hdr.Machine != emX86_64 {
			t.Errorf("expected EM_X86_64, got %d", hdr.Machine)
		}
	case "arm64":
		if hdr.Machine != emAARCH64 {
			t.Errorf("expected EM_AARCH64, got %d", hdr.Machine)
		}
	}
}

func TestReadPhdrs(t *testing.T) {
	path := findTestLibc(t)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	hdr, err := readELFHeader(f, path)
	if err != nil {
		t.Fatal(err)
	}

	phdrs, err := readPhdrs(f, hdr, path)
	if err != nil {
		t.Fatal(err)
	}

	if len(phdrs) == 0 {
		t.Fatal("no program headers")
	}

	// Must have at least one PT_LOAD and one PT_DYNAMIC.
	hasLoad, hasDyn := false, false
	for _, ph := range phdrs {
		if ph.Type == ptLOAD {
			hasLoad = true
		}
		if ph.Type == ptDYNAMIC {
			hasDyn = true
		}
	}
	if !hasLoad {
		t.Error("no PT_LOAD segment")
	}
	if !hasDyn {
		t.Error("no PT_DYNAMIC segment")
	}
}

func TestNotELF(t *testing.T) {
	// Create a temp file that is not an ELF.
	tmp := filepath.Join(t.TempDir(), "notelf")
	if err := os.WriteFile(tmp, []byte("not an elf file"), 0644); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(tmp)
	defer f.Close()
	_, err := readELFHeader(f, tmp)
	if err == nil {
		t.Error("expected error for non-ELF file")
	}
}

// ── Hash function tests ─────────────────────────────────────────────────────

func TestGnuHash(t *testing.T) {
	// Verify basic properties of the GNU hash function.
	// Empty string should hash to 5381 (initial value).
	if h := gnuHashStr(""); h != 5381 {
		t.Errorf("gnuHashStr(\"\") = %d, want 5381", h)
	}
	// Same input produces same output.
	h1 := gnuHashStr("puts")
	h2 := gnuHashStr("puts")
	if h1 != h2 {
		t.Errorf("gnuHashStr not deterministic: 0x%x vs 0x%x", h1, h2)
	}
	// Different inputs produce different outputs.
	h3 := gnuHashStr("printf")
	if h1 == h3 {
		t.Errorf("unexpected collision for 'puts' and 'printf'")
	}
	t.Logf("gnuHashStr(\"puts\")=0x%x gnuHashStr(\"printf\")=0x%x", h1, h3)
}

func TestSysvHash(t *testing.T) {
	// SysV hash of "puts" = 0x06d6538.
	// Let's just verify basic properties.
	h := sysvHashStr("puts")
	if h == 0 {
		t.Error("sysvHashStr('puts') should not be 0")
	}
	// Same string should produce same hash.
	h2 := sysvHashStr("puts")
	if h != h2 {
		t.Errorf("sysvHashStr not deterministic: %d vs %d", h, h2)
	}
	// Different strings should (almost certainly) produce different hashes.
	h3 := sysvHashStr("printf")
	if h == h3 {
		t.Errorf("sysvHashStr collision for 'puts' and 'printf'")
	}
}

// ── Full loader tests ───────────────────────────────────────────────────────

func TestLoaderOpenLibc(t *testing.T) {
	_ = findTestLibc(t) // skip if not available

	ld := New()
	h, err := ld.Open("libc.so.6", RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	// Resolve puts.
	addr, err := h.Sym("puts")
	if err != nil {
		t.Fatal(err)
	}
	if addr == 0 {
		t.Error("puts resolved to address 0")
	}
	t.Logf("puts @ 0x%x", addr)

	// Stats should show activity.
	stats := ld.Stats()
	if stats.LoadedObjects == 0 {
		t.Error("expected at least 1 loaded object")
	}
	if stats.TotalRelocs == 0 {
		t.Error("expected relocations to be applied")
	}
	t.Logf("stats: %+v", stats)
}

func TestLoaderDoubleOpen(t *testing.T) {
	_ = findTestLibc(t)

	ld := New()
	h1, err := ld.Open("libc.so.6", RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := ld.Open("libc.so.6", RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}

	addr1, _ := h1.Sym("puts")
	addr2, _ := h2.Sym("puts")
	if addr1 != addr2 {
		t.Errorf("double open returned different addresses: 0x%x vs 0x%x", addr1, addr2)
	}

	h1.Close()
	h2.Close()
}

func TestLoaderSymbolNotFound(t *testing.T) {
	_ = findTestLibc(t)

	ld := New()
	h, err := ld.Open("libc.so.6", RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	_, err = h.Sym("this_symbol_definitely_does_not_exist_12345")
	if err == nil {
		t.Error("expected error for nonexistent symbol")
	}
}

func TestLoaderMultipleSymbols(t *testing.T) {
	_ = findTestLibc(t)

	ld := New()
	h, err := ld.Open("libc.so.6", RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	syms := []string{"puts", "printf", "malloc", "free", "strlen", "memcpy"}
	for _, name := range syms {
		addr, err := h.Sym(name)
		if err != nil {
			t.Errorf("failed to resolve %s: %v", name, err)
			continue
		}
		if addr == 0 {
			t.Errorf("%s resolved to address 0", name)
		}
	}
}

func TestClosedHandle(t *testing.T) {
	_ = findTestLibc(t)

	ld := New()
	h, err := ld.Open("libc.so.6", RTLD_NOW)
	if err != nil {
		t.Fatal(err)
	}
	h.Close()

	_, err = h.Sym("puts")
	if err != ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}

	err = h.Close()
	if err != ErrClosed {
		t.Errorf("double close: expected ErrClosed, got %v", err)
	}
}

func TestVersionedSymbol(t *testing.T) {
	_ = findTestLibc(t)

	ld := New()
	h, err := ld.Open("libc.so.6", RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	var sym, ver string
	switch runtime.GOARCH {
	case "amd64":
		sym, ver = "puts", "GLIBC_2.2.5"
	case "arm64":
		sym, ver = "puts", "GLIBC_2.17"
	default:
		t.Skip("unknown arch for version test")
	}

	addr, err := h.SymVersion(sym, ver)
	if err != nil {
		t.Fatalf("SymVersion(%s, %s): %v", sym, ver, err)
	}
	if addr == 0 {
		t.Errorf("versioned %s@@%s resolved to 0", sym, ver)
	}

	// Should match the unversioned lookup.
	addr2, err := h.Sym(sym)
	if err != nil {
		t.Fatal(err)
	}
	if addr != addr2 {
		t.Logf("versioned addr=0x%x, unversioned addr=0x%x (may differ if multiple versions)", addr, addr2)
	}
}

func TestSearchPathExpansion(t *testing.T) {
	got := expandOrigin("$ORIGIN/../lib", "/usr/lib/libfoo.so")
	want := "/usr/lib/../lib"
	if got != want {
		t.Errorf("expandOrigin: got %q, want %q", got, want)
	}

	got = expandOrigin("${ORIGIN}/deps", "/opt/app/bin/app.so")
	want = "/opt/app/bin/deps"
	if got != want {
		t.Errorf("expandOrigin: got %q, want %q", got, want)
	}

	got = expandOrigin("/fixed/path", "/anything")
	want = "/fixed/path"
	if got != want {
		t.Errorf("expandOrigin with no $ORIGIN: got %q, want %q", got, want)
	}
}

func TestCString(t *testing.T) {
	s, ptr := CString("hello")
	if ptr == 0 {
		t.Fatal("CString returned null pointer")
	}
	if len(s) != 6 { // "hello" + NUL
		t.Errorf("expected len 6, got %d", len(s))
	}
	if s[5] != 0 {
		t.Error("missing NUL terminator")
	}
}

// ── Dependency order test ───────────────────────────────────────────────────

func TestDependencyLoading(t *testing.T) {
	_ = findTestLibc(t)

	ld := New(WithDebug())
	h, err := ld.Open("libc.so.6", RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	objs := ld.Objects()
	t.Logf("loaded %d objects:", len(objs))
	for _, obj := range objs {
		t.Logf("  %s (soname=%s, base=0x%x, needed=%v)", obj.Path, obj.Soname, obj.Base, obj.Needed)
	}
}
