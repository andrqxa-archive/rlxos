package engine

import (
	"testing"

	core "avyos.dev/pkg/graphics/pixmap"
)

func TestEffectiveBackgroundBlendsStateOverlay(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("background", "#223344")
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.hovered = true

	got := effectiveBackground(e)
	want := core.Blend(core.NewColorHex(0x0D63F31F), core.NewColorHex(0x223344))
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
	if got, want := stateBackgroundOverlay(e), core.NewColorHex(0x99AABBCC); got != want {
		t.Fatalf("pressed overlay mismatch: got=%+v want=%+v", got, want)
	}

	e.pressed = false
	if got, want := stateBackgroundOverlay(e), core.NewColorHex(0x55667788); got != want {
		t.Fatalf("hover overlay mismatch: got=%+v want=%+v", got, want)
	}

	e.hovered = false
	if got, want := stateBackgroundOverlay(e), core.NewColorHex(0x11223344); got != want {
		t.Fatalf("focused overlay mismatch: got=%+v want=%+v", got, want)
	}
}

func TestDrawElementBackgroundBlendsOverlayIntoGradient(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("gradientTop", "#204060")
	e.SetAttribute("gradientBottom", "#406080")
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.hovered = true

	buf := core.NewBuffer(1, 2)
	drawElementBackground(e, buf, core.RectXYWH(0, 0, 1, 2), 0)

	overlay := core.NewColorHex(0x0D63F31F)
	wantTop := core.Blend(overlay, core.NewColorHex(0x204060))
	wantBottom := core.Blend(overlay, core.NewColorHex(0x406080))

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

	buf := core.NewBuffer(1, 1)
	under := core.NewColorHex(0x345678)
	buf.SetPixel(0, 0, under)
	drawElementBackground(e, buf, core.RectXYWH(0, 0, 1, 1), 0)

	want := core.Blend(core.NewColorHex(0x0D63F31F), under)
	if got := buf.GetPixel(0, 0); got != want {
		t.Fatalf("overlay over existing mismatch: got=%+v want=%+v", got, want)
	}
}

func TestDrawElementBackgroundAppliesOverlayOverBaseFill(t *testing.T) {
	e := NewElement("button")
	e.SetAttribute("background", "#223344")
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.hovered = true

	buf := core.NewBuffer(1, 1)
	buf.SetPixel(0, 0, core.NewColorHex(0x8899AA))
	drawElementBackground(e, buf, core.RectXYWH(0, 0, 1, 1), 0)

	base := core.NewColorHex(0x223344)
	want := core.Blend(core.NewColorHex(0x0D63F31F), base)
	if got := buf.GetPixel(0, 0); got != want {
		t.Fatalf("overlay over base mismatch: got=%+v want=%+v", got, want)
	}
}
