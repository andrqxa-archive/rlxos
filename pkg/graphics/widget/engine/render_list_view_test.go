package engine

import (
	"testing"

	core "avyos.dev/pkg/graphics/pixmap"
	gfxtheme "avyos.dev/pkg/graphics/themes"
)

func TestDrawListViewSelectedGradient(t *testing.T) {
	e := NewElement("list")
	e.SetAttribute("listView", true)
	e.SetAttribute("items", "One")
	e.SetAttribute("selected", 0)
	e.SetAttribute("rowHeight", 48)
	e.SetAttribute("rowRadius", 0)
	e.SetAttribute("selectedIndicatorWidth", 0)
	e.SetAttribute("selectedBackground", "transparent")
	e.SetAttribute("selectedGradientTop", "#123456")
	e.SetAttribute("selectedGradientBottom", "#123456")
	e.bounds = core.RectXYWH(0, 0, 60, 60)

	buf := core.NewBuffer(60, 60)
	drawListView(e, buf)

	got := buf.GetPixel(40, 24)
	want := core.NewColorHex(0x123456)
	if got != want {
		t.Fatalf("selected gradient color mismatch: got=%+v want=%+v", got, want)
	}
}

func TestDrawListViewHoverOverRowBase(t *testing.T) {
	e := NewElement("list")
	e.SetAttribute("listView", true)
	e.SetAttribute("items", "One")
	e.SetAttribute("selected", -1)
	e.SetAttribute("hoveredIndex", 0)
	e.SetAttribute("rowHeight", 48)
	e.SetAttribute("rowRadius", 0)
	e.SetAttribute("hoverBackground", "#0D63F31F")
	e.bounds = core.RectXYWH(0, 0, 60, 60)

	buf := core.NewBuffer(60, 60)
	drawListView(e, buf)

	base := gfxtheme.DefaultTheme.SurfaceGlass
	overlay := core.NewColorHex(0x0D63F31F)
	want := core.Blend(overlay, base)
	got := buf.GetPixel(40, 24)
	if got != want {
		t.Fatalf("hover overlay mismatch: got=%+v want=%+v", got, want)
	}
}
