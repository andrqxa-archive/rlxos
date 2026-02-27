package engine

import (
	"testing"

	graphics "avyos.dev/pkg/graphics/input"
)

func TestEffectiveBackgroundBlendsStateOverlay(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("background", "#223344")
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.hovered = true

	got := effectiveBackground(e)
	want := graphics.NewColorHex(0x0D63F31F).Blend(graphics.NewColorHex(0x223344))
	if got != want {
		t.Fatalf("effectiveBackground mismatch: got=%+v want=%+v", got, want)
	}
}

func TestStateBackgroundOverlayPriority(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("focusedBackground", "#11223344")
	e.SetAttribute("hoverBackground", "#55667788")
	e.SetAttribute("pressedBackground", "#99AABBCC")

	e.focused = true
	e.hovered = true
	e.pressed = true
	if got, want := stateBackgroundOverlay(e), graphics.NewColorHex(0x99AABBCC); got != want {
		t.Fatalf("pressed overlay mismatch: got=%+v want=%+v", got, want)
	}

	e.pressed = false
	if got, want := stateBackgroundOverlay(e), graphics.NewColorHex(0x55667788); got != want {
		t.Fatalf("hover overlay mismatch: got=%+v want=%+v", got, want)
	}

	e.hovered = false
	if got, want := stateBackgroundOverlay(e), graphics.NewColorHex(0x11223344); got != want {
		t.Fatalf("focused overlay mismatch: got=%+v want=%+v", got, want)
	}
}

func TestDrawElementBackgroundBlendsOverlayIntoGradient(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("gradientTop", "#204060")
	e.SetAttribute("gradientBottom", "#406080")
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.hovered = true

	buf := graphics.NewBuffer(1, 2)
	drawElementBackground(e, buf, graphics.Rect{X: 0, Y: 0, W: 1, H: 2}, 0)

	overlay := graphics.NewColorHex(0x0D63F31F)
	wantTop := overlay.Blend(graphics.NewColorHex(0x204060))
	wantBottom := overlay.Blend(graphics.NewColorHex(0x406080))

	if got := buf.GetPixel(0, 0); got != wantTop {
		t.Fatalf("top gradient mismatch: got=%+v want=%+v", got, wantTop)
	}
	if got := buf.GetPixel(0, 1); got != wantBottom {
		t.Fatalf("bottom gradient mismatch: got=%+v want=%+v", got, wantBottom)
	}
}

func TestDrawElementBackgroundBlendsOverlayOverExistingWhenBaseTransparent(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("background", "transparent")
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.hovered = true

	buf := graphics.NewBuffer(1, 1)
	under := graphics.NewColorHex(0x345678)
	buf.SetPixel(0, 0, under)
	drawElementBackground(e, buf, graphics.Rect{X: 0, Y: 0, W: 1, H: 1}, 0)

	want := graphics.NewColorHex(0x0D63F31F).Blend(under)
	if got := buf.GetPixel(0, 0); got != want {
		t.Fatalf("overlay over existing mismatch: got=%+v want=%+v", got, want)
	}
}

func TestDrawElementBackgroundAppliesOverlayOverBaseFill(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("background", "#223344")
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.hovered = true

	buf := graphics.NewBuffer(1, 1)
	buf.SetPixel(0, 0, graphics.NewColorHex(0x8899AA))
	drawElementBackground(e, buf, graphics.Rect{X: 0, Y: 0, W: 1, H: 1}, 0)

	base := graphics.NewColorHex(0x223344)
	want := graphics.NewColorHex(0x0D63F31F).Blend(base)
	if got := buf.GetPixel(0, 0); got != want {
		t.Fatalf("overlay over base mismatch: got=%+v want=%+v", got, want)
	}
}
