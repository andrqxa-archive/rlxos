package engine

import (
	"image/color"
	"strings"

	core "avyos.dev/pkg/graphics/pixmap"
)

func isImageElement(e *Element) bool {
	return strings.EqualFold(e.Tag(), "image") || e.Attr("src", "") != ""
}

func effectiveBackground(e *Element) color.NRGBA {
	base := e.AttrColor("background", color.NRGBA{})
	if overlay := stateBackgroundOverlay(e); overlay.A > 0 {
		return core.Blend(overlay, base)
	}
	return base
}

func stateBackgroundOverlay(e *Element) color.NRGBA {
	if e.pressed {
		if c := e.AttrColor("pressedBackground", color.NRGBA{}); c.A > 0 {
			return c
		}
	}
	if e.hovered {
		if c := e.AttrColor("hoverBackground", color.NRGBA{}); c.A > 0 {
			return c
		}
	}
	if e.focused {
		if c := e.AttrColor("focusedBackground", color.NRGBA{}); c.A > 0 {
			return c
		}
	}
	return color.NRGBA{}
}

func effectiveBorderColor(e *Element) color.NRGBA {
	base := e.AttrColor("borderColor", color.NRGBA{})
	if base.A > 0 {
		if e.focused && e.AttrBool("focusBorderReplace", false) {
			if c := e.AttrColor("focusedBorderColor", color.NRGBA{}); c.A > 0 {
				return c
			}
		}
		return base
	}
	if e.focused {
		if c := e.AttrColor("focusedBorderColor", color.NRGBA{}); c.A > 0 {
			return c
		}
	}
	return base
}
