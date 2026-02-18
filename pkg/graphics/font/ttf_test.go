package font

import (
	"image"
	"image/color"
	"testing"
)

func TestMeasureUsesGlyphAdvances(t *testing.T) {
	f, err := NewDefault(nil)
	if err != nil {
		t.Fatalf("NewDefault() error = %v", err)
	}
	defer func() { _ = f.Close() }()

	wNarrow, _ := f.Measure("ii")
	wWide, _ := f.Measure("WW")
	if wNarrow >= wWide {
		t.Fatalf("expected proportional shaping, got ii=%d WW=%d", wNarrow, wWide)
	}
}

func TestDrawTextProducesAntialiasing(t *testing.T) {
	f, err := NewDefault(nil)
	if err != nil {
		t.Fatalf("NewDefault() error = %v", err)
	}
	defer func() { _ = f.Close() }()

	dst := image.NewNRGBA(image.Rect(0, 0, 200, 80))
	f.DrawText(dst, "Smooth", 8, 8, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, nil)

	var sawPartial bool
	for i := 3; i < len(dst.Pix); i += 4 {
		a := dst.Pix[i]
		if a > 0 && a < 255 {
			sawPartial = true
			break
		}
	}
	if !sawPartial {
		t.Fatalf("expected anti-aliased coverage (partial alpha pixels)")
	}
}

func TestMeasureMultilineHeight(t *testing.T) {
	f, err := NewDefault(nil)
	if err != nil {
		t.Fatalf("NewDefault() error = %v", err)
	}
	defer func() { _ = f.Close() }()

	_, h := f.Measure("a\nb\nc")
	if want := f.LineHeight() * 3; h != want {
		t.Fatalf("Measure height = %d, want %d", h, want)
	}
}
