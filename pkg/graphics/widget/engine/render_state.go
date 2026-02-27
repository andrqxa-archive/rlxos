package engine

import (
	"strings"

	graphics "avyos.dev/pkg/graphics/input"
)

func isImageElement(e *Element) bool {
	return strings.EqualFold(e.Tag(), "image") || e.Attr("src", "") != ""
}

func effectiveBackground(e *Element) graphics.Color {
	base := e.AttrColor("background", graphics.Color{})
	if overlay := stateBackgroundOverlay(e); overlay.A > 0 {
		return overlay.Blend(base)
	}
	return base
}

func stateBackgroundOverlay(e *Element) graphics.Color {
	if e.pressed {
		if c := e.AttrColor("pressedBackground", graphics.Color{}); c.A > 0 {
			return c
		}
	}
	if e.hovered {
		if c := e.AttrColor("hoverBackground", graphics.Color{}); c.A > 0 {
			return c
		}
	}
	if e.focused {
		if c := e.AttrColor("focusedBackground", graphics.Color{}); c.A > 0 {
			return c
		}
	}
	return graphics.Color{}
}

func effectiveBorderColor(e *Element) graphics.Color {
	base := e.AttrColor("borderColor", graphics.Color{})
	if base.A > 0 {
		if e.focused && e.AttrBool("focusBorderReplace", false) {
			if c := e.AttrColor("focusedBorderColor", graphics.Color{}); c.A > 0 {
				return c
			}
		}
		return base
	}
	if e.focused {
		if c := e.AttrColor("focusedBorderColor", graphics.Color{}); c.A > 0 {
			return c
		}
	}
	return base
}
