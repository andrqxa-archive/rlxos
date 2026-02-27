package canvas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeSVGPureEvenOddFillRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evenodd.svg")
	svg := `<svg viewBox="0 0 10 10" fill-rule="evenodd" xmlns="http://www.w3.org/2000/svg">
<path d="M1 1 H9 V9 H1 Z M3 3 H7 V7 H3 Z" fill="#ff0000"/>
</svg>`
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		t.Fatalf("write temp svg: %v", err)
	}

	buf, err := DecodeSVGToBuffer(path, 10)
	if err != nil {
		t.Fatalf("DecodeSVGToBuffer(): %v", err)
	}
	if buf == nil {
		t.Fatalf("DecodeSVGToBuffer() returned nil buffer")
	}

	ring := buf.GetPixel(2, 5)
	if ring.A == 0 {
		t.Fatalf("expected ring pixel to be filled, got alpha=0")
	}

	hole := buf.GetPixel(5, 5)
	if hole.A != 0 {
		t.Fatalf("expected hole center to be transparent, got alpha=%d color=%+v", hole.A, hole)
	}
}
