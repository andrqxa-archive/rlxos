package graphics

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestLoadTTFFontDrawsSmoothGlyphs(t *testing.T) {
	f, err := LoadTTFFont(goregular.TTF, nil)
	if err != nil {
		t.Fatalf("LoadTTFFont() error = %v", err)
	}
	defer func() { _ = f.Close() }()

	buf := NewBuffer(240, 80)
	f.DrawText(buf, "Smooth AV", 8, 8, ColorWhite, ColorTransparent)

	var sawPartial bool
	for i := 3; i < len(buf.Data); i += 4 {
		a := buf.Data[i]
		if a > 0 && a < 255 {
			sawPartial = true
			break
		}
	}
	if !sawPartial {
		t.Fatalf("expected anti-aliased coverage pixels")
	}
}

func TestTTFTextWidthUsesShapedAdvances(t *testing.T) {
	f, err := LoadTTFFont(goregular.TTF, nil)
	if err != nil {
		t.Fatalf("LoadTTFFont() error = %v", err)
	}
	defer func() { _ = f.Close() }()

	wNarrow := f.TextWidth("ii")
	wWide := f.TextWidth("WW")
	if wNarrow >= wWide {
		t.Fatalf("expected proportional shaping, got ii=%d WW=%d", wNarrow, wWide)
	}
}
