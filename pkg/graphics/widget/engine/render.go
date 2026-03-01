package engine

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"strings"
	"time"

	gfxfont "avyos.dev/pkg/graphics/fonts"
	core "avyos.dev/pkg/graphics/pixmap"
	gfxrenderer "avyos.dev/pkg/graphics/renderer"
	gfxtheme "avyos.dev/pkg/graphics/themes"
)

var tableMultiSpaceSplit = regexp.MustCompile(`\s{2,}`)

// drawElement renders an element to the buffer based on its attributes.
func drawElement(e *Element, buf *core.Buffer) {
	bounds := e.bounds
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}

	radius := e.AttrInt("borderRadius", 0)

	// 0. Shadow
	drawElementShadow(e, buf, bounds, radius)

	// 1. Background (state-aware)
	drawElementBackground(e, buf, bounds, radius)

	// 2. Focus ring
	drawElementFocusRing(e, buf, bounds, radius)

	// 3. Border (with frame title support)
	title := e.Attr("title", "")
	if title != "" && !e.AttrBool("hideTitle", false) {
		drawFrameTitle(e, buf, title, radius)
	} else {
		borderColor := effectiveBorderColor(e)
		if borderColor.A > 0 {
			if radius > 0 {
				buf.DrawRoundedRect(bounds, radius, borderColor)
			} else {
				buf.DrawRect(bounds, borderColor)
			}
		}
	}

	restoreSelfClip := false
	selfPrevClip := image.Rectangle{}
	selfPrevClipOn := false
	if e.overflowClipped() {
		selfPrevClip, selfPrevClipOn = buf.Clip()
		clipRect := bounds
		if selfPrevClipOn {
			clipRect = clipRect.Intersect(selfPrevClip)
		}
		buf.SetClip(clipRect)
		restoreSelfClip = true
	}

	// 4. Widget-specific rendering based on attributes
	if isImageElement(e) {
		drawImage(e, buf)
	} else if e.AttrBool("toggleable", false) {
		drawToggle(e, buf)
	} else if e.AttrBool("checkable", false) {
		drawCheckable(e, buf)
	} else if e.AttrBool("listView", false) {
		drawListView(e, buf)
	} else if e.AttrBool("slidable", false) {
		drawSlidable(e, buf)
	} else if e.AttrBool("showProgress", false) {
		drawProgress(e, buf)
	}

	// 5. Text content
	if isImageElement(e) {
		// image widgets don't render text content
	} else if e.AttrBool("editable", false) {
		if e.AttrBool("multiline", false) {
			drawTextArea(e, buf)
		} else {
			drawTextInput(e, buf)
		}
	} else if e.AttrBool("checkable", false) || e.AttrBool("toggleable", false) || e.AttrBool("listView", false) {
		// custom widgets render their own text content
	} else if text := e.Attr("text", ""); text != "" {
		drawText(e, buf, text)
	}

	// 6. Children
	restoreChildrenClip := false
	childrenPrevClip := image.Rectangle{}
	childrenPrevClipOn := false
	if e.overflowClipped() && len(e.children) > 0 {
		childrenPrevClip, childrenPrevClipOn = buf.Clip()
		clipRect := e.contentArea()
		if childrenPrevClipOn {
			clipRect = clipRect.Intersect(childrenPrevClip)
		}
		buf.SetClip(clipRect)
		restoreChildrenClip = true
	}

	for _, child := range e.children {
		if child.visible {
			child.Draw(buf)
		}
		child.MarkClean()
	}

	drawOverflowScrollbar(e, buf)

	if restoreChildrenClip {
		if childrenPrevClipOn {
			buf.SetClip(childrenPrevClip)
		} else {
			buf.ClearClip()
		}
	}

	if restoreSelfClip {
		if selfPrevClipOn {
			buf.SetClip(selfPrevClip)
		} else {
			buf.ClearClip()
		}
	}
}

func drawOverflowScrollbar(e *Element, buf *core.Buffer) {
	if !e.overflowScrollable() || e.isRow() {
		return
	}

	content := e.contentArea()
	if content.Dx() < 10 || content.Dy() < 16 {
		return
	}

	maxScroll := e.maxScrollableY()
	if maxScroll <= 0 {
		return
	}

	scrollY := e.AttrInt("scrollY", 0)
	if scrollY < 0 {
		scrollY = 0
	}
	if scrollY > maxScroll {
		scrollY = maxScroll
	}

	barW := e.AttrInt("scrollbarWidth", 6)
	if barW < 4 {
		barW = 4
	}
	if barW > 14 {
		barW = 14
	}
	margin := e.AttrInt("scrollbarMargin", 2)
	if margin < 0 {
		margin = 0
	}

	trackRect := core.RectXYWH(content.Min.X+content.Dx()-barW-margin, content.Min.Y+margin, barW, content.Dy()-margin*2)
	if trackRect.Dy() < 8 || trackRect.Dx() < 2 {
		return
	}

	contentTotal := content.Dy() + maxScroll
	thumbH := 0
	if contentTotal > 0 {
		thumbH = trackRect.Dy() * content.Dy() / contentTotal
	}
	minThumb := e.AttrInt("scrollbarThumbMinHeight", 22)
	if minThumb < 10 {
		minThumb = 10
	}
	if thumbH < minThumb {
		thumbH = minThumb
	}
	if thumbH > trackRect.Dy() {
		thumbH = trackRect.Dy()
	}

	travel := trackRect.Dy() - thumbH
	thumbY := trackRect.Min.Y
	if travel > 0 {
		thumbY += scrollY * travel / maxScroll
	}
	thumbRect := core.RectXYWH(trackRect.Min.X, thumbY, trackRect.Dx(), thumbH)

	trackColor := e.AttrColor("scrollbarTrackColor", core.NewColor(16, 24, 40, 28))
	thumbColor := e.AttrColor("scrollbarThumbColor", core.NewColor(70, 96, 148, 136))
	radius := barW / 2

	if trackColor.A > 0 {
		buf.FillRoundedRect(trackRect, radius, trackColor)
	}
	if thumbColor.A > 0 {
		buf.FillRoundedRect(thumbRect, radius, thumbColor)
	}
}

func drawElementShadow(e *Element, buf *core.Buffer, bounds image.Rectangle, radius int) {
	if !e.AttrBool("shadow", false) && e.Attr("shadowColor", "") == "" {
		return
	}
	if e.AttrBool("shadowOnlyOnHover", false) && !(e.hovered || e.pressed || e.focused) {
		return
	}
	bg := effectiveBackground(e)
	if bg.A == 0 && !e.AttrBool("allowShadowWithoutBackground", false) {
		// Avoid shadow-on-outline artifacts for elements with no fill.
		return
	}
	shadowBase := bg
	if shadowBase.A == 0 {
		if c := e.AttrColor("gradientTop", color.NRGBA{}); c.A > 0 {
			shadowBase = c
		}
		if c := e.AttrColor("gradientBottom", color.NRGBA{}); c.A > 0 {
			if shadowBase.A == 0 {
				shadowBase = c
			} else {
				shadowBase = gfxrenderer.MixColors(shadowBase, c, 0.5)
			}
		}
	}
	if shadowBase.A > 0 {
		shadowBase.A = 255
	}
	c := e.AttrColor("shadowColor", core.NewColor(16, 24, 40, 14))
	if c.A == 0 {
		return
	}
	spread := e.AttrInt("shadowSpread", 6)
	if spread < 1 {
		spread = 1
	}
	gap := e.AttrInt("shadowGap", 0)
	if gap < 0 {
		gap = 0
	}
	if gap >= spread {
		gap = spread - 1
	}
	ox := e.AttrInt("shadowOffsetX", 0)
	oy := e.AttrInt("shadowOffsetY", 3)

	// Make interactive controls feel responsive on hover/press.
	if e.AttrBool("interactive", false) {
		if e.pressed {
			if spread > 2 {
				spread -= 2
			}
			if oy > 0 {
				oy--
			}
			c.A = uint8((uint16(c.A) * 3) / 4)
		} else if e.hovered {
			spread++
			if c.A > 220 {
				c.A = 255
			} else {
				c.A += 24
			}
		}
	}

	shadowRect := core.RectXYWH(bounds.Min.X+ox, bounds.Min.Y+oy, bounds.Dx(), bounds.Dy())
	gfxrenderer.DrawRoundedShadow(buf, shadowRect, radius, c, shadowBase, spread, gap)
}

func drawElementBackground(e *Element, buf *core.Buffer, bounds image.Rectangle, radius int) {
	bg := e.AttrColor("background", color.NRGBA{})
	gradTop := e.AttrColor("gradientTop", color.NRGBA{})
	gradBottom := e.AttrColor("gradientBottom", color.NRGBA{})
	overlay := stateBackgroundOverlay(e)

	if gradTop.A == 0 && gradBottom.A == 0 {
		if bg.A > 0 {
			if radius > 0 {
				buf.FillRoundedRect(bounds, radius, bg)
			} else {
				buf.FillRect(bounds, bg)
			}
		}
		if overlay.A > 0 {
			gfxrenderer.DrawVerticalGradient(buf, bounds, radius, overlay, overlay)
		}
		return
	}

	if gradTop.A == 0 {
		gradTop = bg
	}
	if gradBottom.A == 0 {
		gradBottom = bg
	}
	gfxrenderer.DrawVerticalGradient(buf, bounds, radius, gradTop, gradBottom)
	if overlay.A > 0 {
		gfxrenderer.DrawVerticalGradient(buf, bounds, radius, overlay, overlay)
	}
}

func drawElementFocusRing(e *Element, buf *core.Buffer, bounds image.Rectangle, radius int) {
	if !e.focused {
		return
	}

	defaultFocusRing := e.AttrBool("focusable", false) ||
		e.AttrBool("interactive", false) ||
		e.AttrBool("editable", false) ||
		e.AttrBool("checkable", false) ||
		e.AttrBool("toggleable", false) ||
		e.AttrBool("listView", false) ||
		e.AttrBool("slidable", false)
	if !e.AttrBool("focusRing", defaultFocusRing) {
		return
	}

	ringColor := e.AttrColor("focusRingColor", e.AttrColor("focusedBorderColor", gfxtheme.DefaultTheme.BorderFocused))
	if ringColor.A == 0 {
		return
	}

	ringWidth := e.AttrInt("focusRingWidth", 2)
	if ringWidth < 1 {
		ringWidth = 1
	}
	if ringWidth > 4 {
		ringWidth = 4
	}

	ringOffset := e.AttrInt("focusRingOffset", 1)
	if ringOffset < 0 {
		ringOffset = 0
	}
	if ringOffset > 4 {
		ringOffset = 4
	}
	gfxrenderer.DrawFocusRing(buf, bounds, radius, ringColor, ringWidth, ringOffset)
}

func drawText(e *Element, buf *core.Buffer, text string) {
	font := elementFont(e, gfxfont.UIFontParagraph)
	textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.Foreground)
	align := e.Attr("textAlign", "left")

	content := e.contentArea()
	if e.AttrBool("clipText", false) && content.Dx() > 0 {
		text = gfxrenderer.TrimTextToWidth(font, text, content.Dx(), align)
	}
	if text == "" {
		return
	}
	textW := font.TextWidth(text)
	textH := font.TextHeight(text)

	x := content.Min.X
	switch align {
	case "center":
		x = content.Min.X + (content.Dx()-textW)/2
	case "right":
		x = content.Min.X + content.Dx() - textW
	}
	y := content.Min.Y + (content.Dy()-textH)/2

	font.DrawText(buf, text, x, y, textColor, core.ColorTransparent)
}

// --- Frame title ---

func drawFrameTitle(e *Element, buf *core.Buffer, title string, radius int) {
	bounds := e.Bounds()
	font := elementTitleFont(e)
	borderColor := effectiveBorderColor(e)
	titleColor := e.AttrColor("titleColor", gfxtheme.DefaultTheme.Foreground)

	titleH := font.Height / 2
	titleGap := 4

	// Draw border offset down by titleH
	borderRect := core.RectXYWH(bounds.Min.X, bounds.Min.Y+titleH, bounds.Dx(), bounds.Dy()-titleH)
	if borderColor.A > 0 {
		if radius > 0 {
			buf.DrawRoundedRect(borderRect, radius, borderColor)
		} else {
			buf.DrawRect(borderRect, borderColor)
		}
	}

	// Draw title text on the border
	titleX := bounds.Min.X + 1 + titleGap
	titleY := bounds.Min.Y

	// Clear the border behind the title
	textW := font.TextWidth(title)
	bg := e.AttrColor("background", color.NRGBA{})
	clearRect := core.RectXYWH(titleX-2, bounds.Min.Y+titleH, textW+4, 1)
	buf.FillRect(clearRect, bg)

	font.DrawText(buf, title, titleX, titleY, titleColor, core.ColorTransparent)
}

// --- Checkbox ---

func drawCheckable(e *Element, buf *core.Buffer) {
	bounds := e.Bounds()
	font := elementFont(e, gfxfont.UIFontParagraph)
	boxSize := font.Height + 4
	if boxSize > bounds.Dy()-2 {
		boxSize = bounds.Dy() - 2
	}
	if boxSize < font.Height {
		boxSize = font.Height
	}
	gap := 8
	isRadio := e.AttrBool("radio", false)

	boxY := bounds.Min.Y + (bounds.Dy()-boxSize)/2
	boxRect := core.RectXYWH(bounds.Min.X, boxY, boxSize, boxSize)
	radius := gfxtheme.DefaultTheme.BorderRadius / 2
	if isRadio {
		radius = boxSize / 2
	}

	borderColor := e.AttrColor("boxBorderColor", e.AttrColor("borderColor", gfxtheme.DefaultTheme.Border))
	if e.focused {
		borderColor = e.AttrColor("focusedBoxBorderColor", e.AttrColor("focusedBorderColor", gfxtheme.DefaultTheme.BorderFocused))
	}
	fillColor := e.AttrColor("boxBackground", e.AttrColor("background", gfxtheme.DefaultTheme.InputBackground))
	if fillColor.A > 0 {
		buf.FillRoundedRect(boxRect, radius, fillColor)
	}
	buf.DrawRoundedRect(boxRect, radius, borderColor)

	checkColor := e.AttrColor("checkColor", gfxtheme.DefaultTheme.Primary)

	if e.hovered {
		highlight := core.NewColor(checkColor.R, checkColor.G, checkColor.B, 30)
		buf.FillRoundedRect(core.RectXYWH(boxRect.Min.X+1, boxRect.Min.Y+1, boxRect.Dx()-2, boxRect.Dy()-2), radius, highlight)
	}

	if e.AttrBool("checked", false) {
		if isRadio {
			dotD := (boxSize * 9) / 20
			dotRect := core.RectXYWH(boxRect.Min.X+(boxSize-dotD)/2, boxRect.Min.Y+(boxSize-dotD)/2, dotD, dotD)
			buf.FillRoundedRect(dotRect, dotD/2, checkColor)
		} else {
			buf.FillRoundedRect(core.RectXYWH(boxRect.Min.X+1, boxRect.Min.Y+1, boxRect.Dx()-2, boxRect.Dy()-2), radius, checkColor)
			bx, by := float64(boxRect.Min.X), float64(boxRect.Min.Y)
			sz := float64(boxSize)
			tickColor := core.NewColor(255, 255, 255, 246)
			p1x, p1y := bx+0.24*sz, by+0.54*sz
			p2x, p2y := bx+0.44*sz, by+0.74*sz
			p3x, p3y := bx+0.78*sz, by+0.30*sz
			gfxrenderer.DrawAALine(buf, p1x, p1y, p2x, p2y, 2.2, tickColor)
			gfxrenderer.DrawAALine(buf, p2x, p2y, p3x, p3y, 2.2, tickColor)
		}
	}

	text := e.Attr("text", "")
	if text != "" {
		textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.Foreground)
		textX := bounds.Min.X + boxSize + gap
		textH := font.TextHeight(text)
		textY := bounds.Min.Y + (bounds.Dy()-textH)/2
		font.DrawText(buf, text, textX, textY, textColor, core.ColorTransparent)
	}
}

func drawToggle(e *Element, buf *core.Buffer) {
	bounds := e.Bounds()
	font := elementFont(e, gfxfont.UIFontParagraph)
	text := e.Attr("text", "")

	trackW := 42
	trackH := 24
	r := trackH / 2
	trackX := bounds.Min.X
	trackY := bounds.Min.Y + (bounds.Dy()-trackH)/2
	trackRect := core.RectXYWH(trackX, trackY, trackW, trackH)

	offColor := e.AttrColor("offTrackColor", core.NewColorHex(0x1B2A4A24))
	onColor := e.AttrColor("onTrackColor", core.NewColorHex(0x2F6BBF))
	borderColor := e.AttrColor("trackBorderColor", e.AttrColor("borderColor", core.NewColorHex(0x1B2A4A24)))
	if e.focused {
		borderColor = e.AttrColor("focusedTrackBorderColor", e.AttrColor("focusedBorderColor", borderColor))
	}

	position := 0.0
	if e.AttrBool("checked", false) {
		position = 1.0
	}

	trackColor := gfxrenderer.MixColors(offColor, onColor, position)
	if e.hovered {
		trackColor = gfxrenderer.MixColors(trackColor, core.ColorWhite, 0.06)
	}
	if e.pressed {
		trackColor = gfxrenderer.MixColors(trackColor, core.ColorBlack, 0.08)
	}
	buf.FillRoundedRect(trackRect, r, trackColor)
	buf.DrawRoundedRect(trackRect, r, borderColor)

	thumbD := 18
	thumbMinX := trackX + 3
	thumbMaxX := trackX + trackW - thumbD - 3
	thumbX := thumbMinX
	if thumbMaxX > thumbMinX {
		thumbX += int(float64(thumbMaxX-thumbMinX)*position + 0.5)
	}
	thumbY := trackY + (trackH-thumbD)/2
	thumbRect := core.RectXYWH(thumbX, thumbY, thumbD, thumbD)
	gfxrenderer.DrawVerticalGradient(
		buf,
		thumbRect,
		thumbD/2,
		core.NewColor(255, 255, 255, 255),
		core.NewColor(237, 244, 255, 252),
	)
	outline := core.NewColorHex(0x1B2A4A1C)
	if e.AttrBool("checked", false) {
		outline = core.NewColorHex(0x1B2A4A26)
	}
	buf.DrawRoundedRect(thumbRect, thumbD/2, outline)

	if text != "" {
		textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.Foreground)
		tx := trackRect.Min.X + trackRect.Dx() + 8
		ty := bounds.Min.Y + (bounds.Dy()-font.Height)/2
		font.DrawText(buf, text, tx, ty, textColor, core.ColorTransparent)
	}
}

func drawListView(e *Element, buf *core.Buffer) {
	bounds := e.Bounds()
	font := elementFont(e, gfxfont.UIFontParagraph)
	itemsRaw := e.Attr("items", "")
	if itemsRaw == "" {
		return
	}
	items := strings.Split(itemsRaw, "|")
	rowH := e.AttrInt("rowHeight", 34)
	if rowH < font.Height+8 {
		rowH = font.Height + 8
	}
	visibleRows := bounds.Dy() / rowH
	if visibleRows < 1 {
		visibleRows = 1
	}
	maxScroll := len(items) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	scrollIndex := e.AttrInt("scrollIndex", 0)
	if scrollIndex < 0 {
		scrollIndex = 0
	}
	if scrollIndex > maxScroll {
		scrollIndex = maxScroll
		e.attrs["scrollIndex"] = fmt.Sprintf("%d", scrollIndex)
		e.dirty = true
	}
	selected := e.AttrInt("selected", -1)
	if selected >= len(items) {
		selected = len(items) - 1
		e.attrs["selected"] = fmt.Sprintf("%d", selected)
		e.dirty = true
	}
	if selected < -1 {
		selected = -1
		e.attrs["selected"] = "-1"
		e.dirty = true
	}
	hoverColor := e.AttrColor("hoverBackground", core.NewColorHex(0x2F6BFF24))
	selectColor := e.AttrColor("selectedBackground", core.NewColorHex(0x2F6BFF24))
	baseColor := gfxtheme.DefaultTheme.SurfaceGlass
	hoverGradTop := e.AttrColor("hoverGradientTop", color.NRGBA{})
	hoverGradBottom := e.AttrColor("hoverGradientBottom", color.NRGBA{})
	selectGradTop := e.AttrColor("selectedGradientTop", color.NRGBA{})
	selectGradBottom := e.AttrColor("selectedGradientBottom", color.NRGBA{})
	indicatorColor := e.AttrColor("selectedIndicatorColor", gfxtheme.DefaultTheme.Primary)
	indicatorGradTop := e.AttrColor("selectedIndicatorGradientTop", color.NRGBA{})
	indicatorGradBottom := e.AttrColor("selectedIndicatorGradientBottom", color.NRGBA{})
	indicatorW := e.AttrInt("selectedIndicatorWidth", 3)
	if indicatorW < 0 {
		indicatorW = 0
	}
	textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.Foreground)
	borderColor := e.AttrColor("borderColor", core.NewColorHex(0x1B2A4A24))
	dividerColor := e.AttrColor("dividerColor", gfxtheme.DefaultTheme.StrokeDivider)
	rowRadius := e.AttrInt("rowRadius", 8)

	for i := 0; i < len(items); i++ {
		itemIdx := scrollIndex + i
		if itemIdx >= len(items) {
			break
		}
		y := bounds.Min.Y + i*rowH
		if y+rowH > bounds.Min.Y+bounds.Dy() {
			break
		}
		row := core.RectXYWH(bounds.Min.X+1, y, bounds.Dx()-2, rowH)
		hovered := itemIdx == e.AttrInt("hoveredIndex", -1)
		gfxrenderer.DrawListRowFill(buf, row, rowRadius, baseColor, color.NRGBA{}, color.NRGBA{})
		if hovered && itemIdx != selected {
			gfxrenderer.DrawListRowFill(buf, row, rowRadius, hoverColor, hoverGradTop, hoverGradBottom)
		}
		textInset := 12
		if itemIdx == selected {
			gfxrenderer.DrawListRowFill(buf, row, rowRadius, selectColor, selectGradTop, selectGradBottom)
			if indicatorW > 0 {
				iw := indicatorW
				if iw > row.Dx()-4 {
					iw = row.Dx() - 4
				}
				if iw > 0 {
					indicatorRect := core.RectXYWH(row.Min.X+2, row.Min.Y+4, iw, row.Dy()-8)
					if indicatorRect.Dy() < 2 {
						indicatorRect = core.RectXYWH(row.Min.X+2, row.Min.Y+1, iw, row.Dy()-2)
					}
					if indicatorRect.Dy() > 0 {
						gfxrenderer.DrawListRowFill(buf, indicatorRect, iw, indicatorColor, indicatorGradTop, indicatorGradBottom)
					}
					textInset += iw + 4
				}
			}
		}
		if i > 0 && dividerColor.A > 0 {
			dy := y
			for x := bounds.Min.X + 8; x < bounds.Min.X+bounds.Dx()-8; x++ {
				bg := buf.GetPixel(x, dy)
				buf.SetPixel(x, dy, core.Blend(dividerColor, bg))
			}
		}
		tx := row.Min.X + textInset
		ty := row.Min.Y + (row.Dy()-font.Height)/2
		font.DrawText(buf, strings.TrimSpace(items[itemIdx]), tx, ty, textColor, core.ColorTransparent)
	}

	if borderColor.A > 0 {
		buf.DrawRoundedRect(bounds, e.AttrInt("borderRadius", 10), borderColor)
	}
}

// --- Slider ---

func drawSlidable(e *Element, buf *core.Buffer) {
	bounds := e.Bounds()
	st := e.getSliderState()

	orientation := e.Attr("orientation", "horizontal")
	if orientation == "vertical" {
		drawSliderVertical(e, buf, bounds, st)
	} else {
		drawSliderHorizontal(e, buf, bounds, st)
	}
}

func drawSliderHorizontal(e *Element, buf *core.Buffer, bounds image.Rectangle, st *sliderState) {
	radius := gfxtheme.DefaultTheme.BorderRadius / 2
	trackColor := e.AttrColor("trackColor", gfxtheme.DefaultTheme.Secondary)
	thumbColor := e.AttrColor("thumbColor", gfxtheme.DefaultTheme.Primary)

	trackY := bounds.Min.Y + (bounds.Dy()-sliderTrackH)/2
	trackRect := core.RectXYWH(bounds.Min.X+sliderThumbW/2, trackY, bounds.Dx()-sliderThumbW, sliderTrackH)
	buf.FillRoundedRect(trackRect, sliderTrackH/2, trackColor)

	thumbX := e.sliderValueToPixelH(bounds)
	filledRect := core.RectXYWH(trackRect.Min.X, trackY, thumbX-trackRect.Min.X+sliderThumbW/2, sliderTrackH)
	if filledRect.Dx() > 0 {
		buf.FillRoundedRect(filledRect, sliderTrackH/2, thumbColor)
	}

	thumbRect := core.RectXYWH(thumbX, bounds.Min.Y+(bounds.Dy()-sliderThumbH)/2, sliderThumbW, sliderThumbH)
	tc := thumbColor
	if st.dragging {
		tc = e.AttrColor("thumbActiveColor", gfxtheme.DefaultTheme.PrimaryActive)
	} else if e.hovered {
		tc = e.AttrColor("thumbHoverColor", gfxtheme.DefaultTheme.PrimaryHover)
	}
	buf.FillRoundedRect(thumbRect, radius, tc)

	borderColor := e.AttrColor("borderColor", gfxtheme.DefaultTheme.Border)
	if e.focused {
		borderColor = e.AttrColor("focusedBorderColor", gfxtheme.DefaultTheme.BorderFocused)
	}
	buf.DrawRoundedRect(thumbRect, radius, borderColor)
}

func drawSliderVertical(e *Element, buf *core.Buffer, bounds image.Rectangle, st *sliderState) {
	radius := gfxtheme.DefaultTheme.BorderRadius / 2
	trackColor := e.AttrColor("trackColor", gfxtheme.DefaultTheme.Secondary)
	thumbColor := e.AttrColor("thumbColor", gfxtheme.DefaultTheme.Primary)

	trackX := bounds.Min.X + (bounds.Dx()-sliderTrackH)/2
	trackRect := core.RectXYWH(trackX, bounds.Min.Y+sliderThumbW/2, sliderTrackH, bounds.Dy()-sliderThumbW)
	buf.FillRoundedRect(trackRect, sliderTrackH/2, trackColor)

	thumbY := e.sliderValueToPixelV(bounds)
	filledRect := core.RectXYWH(trackX, thumbY+sliderThumbW/2, sliderTrackH, trackRect.Min.Y+trackRect.Dy()-thumbY-sliderThumbW/2)
	if filledRect.Dy() > 0 {
		buf.FillRoundedRect(filledRect, sliderTrackH/2, thumbColor)
	}

	thumbRect := core.RectXYWH(bounds.Min.X+(bounds.Dx()-sliderThumbH)/2, thumbY, sliderThumbH, sliderThumbW)
	tc := thumbColor
	if st.dragging {
		tc = e.AttrColor("thumbActiveColor", gfxtheme.DefaultTheme.PrimaryActive)
	} else if e.hovered {
		tc = e.AttrColor("thumbHoverColor", gfxtheme.DefaultTheme.PrimaryHover)
	}
	buf.FillRoundedRect(thumbRect, radius, tc)

	borderColor := e.AttrColor("borderColor", gfxtheme.DefaultTheme.Border)
	if e.focused {
		borderColor = e.AttrColor("focusedBorderColor", gfxtheme.DefaultTheme.BorderFocused)
	}
	buf.DrawRoundedRect(thumbRect, radius, borderColor)
}

// --- Progress bar ---

func drawProgress(e *Element, buf *core.Buffer) {
	bounds := e.Bounds()
	value := e.AttrFloat("value", 0)
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	fillColor := e.AttrColor("fillColor", gfxtheme.DefaultTheme.Primary)
	fillTop := e.AttrColor("fillGradientTop", color.NRGBA{})
	fillBottom := e.AttrColor("fillGradientBottom", color.NRGBA{})
	radius := e.AttrInt("borderRadius", gfxtheme.DefaultTheme.BorderRadius)

	innerW := bounds.Dx() - 2
	if innerW > 0 {
		filledW := int(float64(innerW) * value)
		if filledW > 0 {
			fillRect := core.RectXYWH(bounds.Min.X+1, bounds.Min.Y+1, filledW, bounds.Dy()-2)
			if fillTop.A > 0 || fillBottom.A > 0 {
				if fillTop.A == 0 {
					fillTop = fillColor
				}
				if fillBottom.A == 0 {
					fillBottom = fillColor
				}
				gfxrenderer.DrawVerticalGradient(buf, fillRect, radius, fillTop, fillBottom)
			} else {
				buf.FillRoundedRect(fillRect, radius, fillColor)
			}
		}
	}

	if e.AttrBool("showText", true) {
		font := elementFont(e, gfxfont.UIFontParagraph)
		textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.Foreground)
		text := fmt.Sprintf("%d%%", int(value*100))
		textW := font.TextWidth(text)
		textH := font.TextHeight(text)
		x := bounds.Min.X + (bounds.Dx()-textW)/2
		y := bounds.Min.Y + (bounds.Dy()-textH)/2
		font.DrawText(buf, text, x, y, textColor, core.ColorTransparent)
	}
}

// --- Text input ---

func drawTextInput(e *Element, buf *core.Buffer) {
	font := elementFont(e, gfxfont.UIFontParagraph)
	st := e.getTextInputState()
	textArea := e.contentArea()
	if textArea.Dx() <= 0 || textArea.Dy() <= 0 {
		return
	}

	text := st.text
	isPassword := e.AttrBool("password", false)
	display := textInputDisplayRunes(st, isPassword)

	startCol, endCol := textInputVisibleRange(font, display, st.scrollOffset, textArea.Dx())
	textY := textArea.Min.Y + (textArea.Dy()-font.Height)/2

	// Placeholder
	if len(text) == 0 && !e.focused {
		placeholder := e.Attr("placeholder", "")
		if placeholder != "" {
			placeholderColor := e.AttrColor("placeholderColor", core.ColorGray)
			font.DrawText(buf, placeholder, textArea.Min.X, textY,
				placeholderColor, core.ColorTransparent)
		}
		return
	}

	// Display text
	visible := string(display[startCol:endCol])
	textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.InputForeground)
	font.DrawText(buf, visible, textArea.Min.X, textY,
		textColor, core.ColorTransparent)

	// Cursor
	if e.focused && !e.AttrBool("readOnly", false) {
		elapsed := time.Since(st.lastBlink)
		if elapsed > 500*time.Millisecond {
			st.cursorBlink = !st.cursorBlink
			st.lastBlink = time.Now()
		}
		if st.cursorBlink {
			if st.cursorPos >= startCol && st.cursorPos <= endCol {
				cx := textArea.Min.X + textInputSliceWidth(font, display, startCol, st.cursorPos)
				cy := textY
				cursorColor := e.AttrColor("cursorColor", gfxtheme.DefaultTheme.Foreground)
				for dy := 0; dy < font.Height; dy++ {
					buf.SetPixel(cx, cy+dy, cursorColor)
				}
			}
		}
	}
}

// --- Text area ---

func drawTextArea(e *Element, buf *core.Buffer) {
	if e.AttrBool("table", false) {
		drawTableArea(e, buf)
		return
	}

	font := elementFont(e, gfxfont.UIFontParagraph)
	st := e.getTextAreaState()
	textArea := e.contentArea()
	if textArea.Dx() <= 0 || textArea.Dy() <= 0 {
		return
	}
	visRows := textArea.Dy() / font.Height
	if visRows <= 0 {
		return
	}

	if e.AttrBool("wrap", false) {
		textW := e.textAreaTextWidth(textArea)
		if textW <= 0 {
			return
		}
		segments := textAreaWrappedSegments(st.lines, font, textW)
		if len(segments) == 0 {
			return
		}

		maxScroll := len(segments) - visRows
		if maxScroll < 0 {
			maxScroll = 0
		}
		if st.scrollRow < 0 {
			st.scrollRow = 0
		}
		if st.scrollRow > maxScroll {
			st.scrollRow = maxScroll
		}
		st.scrollCol = 0

		drawScrollbar := e.AttrBool("showScrollbar", false) && maxScroll > 0
		var trackRect image.Rectangle
		var thumbRect image.Rectangle
		if drawScrollbar {
			barW := e.AttrInt("scrollbarWidth", 6)
			if barW < 4 {
				barW = 4
			}
			if barW > 14 {
				barW = 14
			}
			margin := e.AttrInt("scrollbarMargin", 2)
			if margin < 0 {
				margin = 0
			}
			trackRect = core.RectXYWH(textArea.Min.X+textArea.Dx()-barW-margin, textArea.Min.Y+margin, barW, textArea.Dy()-margin*2)
			if trackRect.Min.X <= textArea.Min.X || trackRect.Dy() < 8 {
				drawScrollbar = false
			} else {
				thumbH := trackRect.Dy() * visRows / len(segments)
				minThumb := e.AttrInt("scrollbarThumbMinHeight", 16)
				if minThumb < 8 {
					minThumb = 8
				}
				if thumbH < minThumb {
					thumbH = minThumb
				}
				if thumbH > trackRect.Dy() {
					thumbH = trackRect.Dy()
				}

				travel := trackRect.Dy() - thumbH
				thumbY := trackRect.Min.Y
				if travel > 0 && maxScroll > 0 {
					thumbY += st.scrollRow * travel / maxScroll
				}
				thumbRect = core.RectXYWH(trackRect.Min.X, thumbY, trackRect.Dx(), thumbH)
			}
		}

		textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.InputForeground)
		textX := textArea.Min.X
		for i := 0; i < visRows; i++ {
			visualIdx := st.scrollRow + i
			if visualIdx >= len(segments) {
				break
			}
			seg := segments[visualIdx]
			line := st.lines[seg.row]
			if seg.start > len(line) {
				continue
			}
			if seg.end > len(line) {
				seg.end = len(line)
			}
			y := textArea.Min.Y + i*font.Height
			font.DrawText(buf, string(line[seg.start:seg.end]), textX, y,
				textColor, core.ColorTransparent)
		}

		if e.focused && !e.AttrBool("readOnly", false) {
			elapsed := time.Since(st.lastBlink)
			if elapsed > 500*time.Millisecond {
				st.cursorBlink = !st.cursorBlink
				st.lastBlink = time.Now()
			}
			if st.cursorBlink {
				cursorVisual := textAreaWrapVisualRowForCursor(segments, st.cursorRow, st.cursorCol)
				screenRow := cursorVisual - st.scrollRow
				if screenRow >= 0 && screenRow < visRows && cursorVisual >= 0 && cursorVisual < len(segments) {
					seg := segments[cursorVisual]
					line := st.lines[seg.row]
					cursorCol := st.cursorCol
					if cursorCol < seg.start {
						cursorCol = seg.start
					}
					if cursorCol > seg.end {
						cursorCol = seg.end
					}
					cx := textX + textAreaLineSliceWidth(font, line, seg.start, cursorCol)
					cy := textArea.Min.Y + screenRow*font.Height
					cursorColor := e.AttrColor("cursorColor", gfxtheme.DefaultTheme.Foreground)
					for dy := 0; dy < font.Height; dy++ {
						buf.SetPixel(cx, cy+dy, cursorColor)
					}
				}
			}
		}

		if drawScrollbar {
			trackColor := e.AttrColor("scrollbarTrackColor", core.NewColor(16, 24, 40, 28))
			thumbColor := e.AttrColor("scrollbarThumbColor", core.NewColor(70, 96, 148, 136))
			radius := trackRect.Dx() / 2
			if radius < 1 {
				radius = 1
			}
			if trackColor.A > 0 {
				buf.FillRoundedRect(trackRect, radius, trackColor)
			}
			if thumbColor.A > 0 {
				buf.FillRoundedRect(thumbRect, radius, thumbColor)
			}
		}
		return
	}

	maxScroll := len(st.lines) - visRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if st.scrollRow < 0 {
		st.scrollRow = 0
	}
	if st.scrollRow > maxScroll {
		st.scrollRow = maxScroll
	}

	textW := textArea.Dx()
	drawScrollbar := e.AttrBool("showScrollbar", false) && maxScroll > 0
	var trackRect image.Rectangle
	var thumbRect image.Rectangle
	if drawScrollbar {
		barW := e.AttrInt("scrollbarWidth", 6)
		if barW < 4 {
			barW = 4
		}
		if barW > 14 {
			barW = 14
		}
		margin := e.AttrInt("scrollbarMargin", 2)
		if margin < 0 {
			margin = 0
		}
		trackRect = core.RectXYWH(textArea.Min.X+textArea.Dx()-barW-margin, textArea.Min.Y+margin, barW, textArea.Dy()-margin*2)
		if trackRect.Min.X <= textArea.Min.X || trackRect.Dy() < 8 {
			drawScrollbar = false
		} else {
			textW = trackRect.Min.X - textArea.Min.X - 1
			if textW < 1 {
				drawScrollbar = false
				textW = textArea.Dx()
			}
		}

		if drawScrollbar {
			thumbH := trackRect.Dy() * visRows / len(st.lines)
			minThumb := e.AttrInt("scrollbarThumbMinHeight", 16)
			if minThumb < 8 {
				minThumb = 8
			}
			if thumbH < minThumb {
				thumbH = minThumb
			}
			if thumbH > trackRect.Dy() {
				thumbH = trackRect.Dy()
			}

			travel := trackRect.Dy() - thumbH
			thumbY := trackRect.Min.Y
			if travel > 0 && maxScroll > 0 {
				thumbY += st.scrollRow * travel / maxScroll
			}
			thumbRect = core.RectXYWH(trackRect.Min.X, thumbY, trackRect.Dx(), thumbH)
		}
	}

	if len(st.terminalLines) > 0 {
		drawTerminalTextArea(e, buf, font, st, textArea, textW, visRows)
	} else {
		textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.InputForeground)
		textX := textArea.Min.X

		for i := 0; i < visRows; i++ {
			lineIdx := st.scrollRow + i
			if lineIdx >= len(st.lines) {
				break
			}
			line := st.lines[lineIdx]
			y := textArea.Min.Y + i*font.Height

			startCol := st.scrollCol
			if startCol > len(line) {
				startCol = len(line)
			}
			endCol := textAreaVisibleEndCol(font, line, startCol, textW)
			if endCol < startCol {
				endCol = startCol
			}
			font.DrawText(buf, string(line[startCol:endCol]), textX, y,
				textColor, core.ColorTransparent)
		}

		// Cursor
		if e.focused && !e.AttrBool("readOnly", false) {
			elapsed := time.Since(st.lastBlink)
			if elapsed > 500*time.Millisecond {
				st.cursorBlink = !st.cursorBlink
				st.lastBlink = time.Now()
			}
			if st.cursorBlink {
				screenRow := st.cursorRow - st.scrollRow
				if screenRow >= 0 && screenRow < visRows &&
					st.cursorRow >= 0 && st.cursorRow < len(st.lines) {
					line := st.lines[st.cursorRow]
					startCol := st.scrollCol
					if startCol > len(line) {
						startCol = len(line)
					}
					endCol := textAreaVisibleEndCol(font, line, startCol, textW)
					if st.cursorCol >= startCol && st.cursorCol <= endCol {
						cx := textX + textAreaLineSliceWidth(font, line, startCol, st.cursorCol)
						cy := textArea.Min.Y + screenRow*font.Height
						cursorColor := e.AttrColor("cursorColor", gfxtheme.DefaultTheme.Foreground)
						for dy := 0; dy < font.Height; dy++ {
							buf.SetPixel(cx, cy+dy, cursorColor)
						}
					}
				}
			}
		}
	}

	if drawScrollbar {
		trackColor := e.AttrColor("scrollbarTrackColor", core.NewColor(16, 24, 40, 28))
		thumbColor := e.AttrColor("scrollbarThumbColor", core.NewColor(70, 96, 148, 136))
		radius := trackRect.Dx() / 2
		if radius < 1 {
			radius = 1
		}
		if trackColor.A > 0 {
			buf.FillRoundedRect(trackRect, radius, trackColor)
		}
		if thumbColor.A > 0 {
			buf.FillRoundedRect(thumbRect, radius, thumbColor)
		}
	}
}

func textAreaRuneWidth(font *gfxfont.Font, ch rune) int {
	if font == nil {
		return 8
	}
	w := font.TextWidth(string(ch))
	if w <= 0 {
		w = font.Width
	}
	if w <= 0 {
		w = 8
	}
	return w
}

func drawTerminalTextArea(
	e *Element,
	buf *core.Buffer,
	font *gfxfont.Font,
	st *textAreaState,
	textArea image.Rectangle,
	textW int,
	visRows int,
) {
	textX := textArea.Min.X
	maxX := textX + textW
	defaultTextColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.InputForeground)

	for i := 0; i < visRows; i++ {
		lineIdx := st.scrollRow + i
		if lineIdx >= len(st.terminalLines) {
			break
		}
		line := st.terminalLines[lineIdx]
		y := textArea.Min.Y + i*font.Height

		startCol := st.scrollCol
		if startCol > len(line) {
			startCol = len(line)
		}

		x := textX
		for col := startCol; col < len(line); col++ {
			cell := line[col]
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			w := textAreaRuneWidth(font, ch)
			if x+w > maxX {
				break
			}

			if cell.Bg.A > 0 {
				buf.FillRect(core.RectXYWH(x, y, w, font.Height), cell.Bg)
			}

			fg := cell.Fg
			if fg.A == 0 {
				fg = defaultTextColor
			}
			if ch != ' ' {
				font.DrawText(buf, string(ch), x, y, fg, core.ColorTransparent)
			}
			if cell.Underline {
				uy := y + font.Height - 2
				if uy < y {
					uy = y
				}
				for px := x; px < x+w; px++ {
					buf.SetPixel(px, uy, fg)
				}
			}
			x += w
		}
	}

	if !st.terminalShowCursor {
		return
	}
	if !e.focused && !e.AttrBool("alwaysShowTerminalCursor", false) {
		return
	}

	elapsed := time.Since(st.lastBlink)
	if elapsed > 500*time.Millisecond {
		st.cursorBlink = !st.cursorBlink
		st.lastBlink = time.Now()
	}
	if !st.cursorBlink {
		return
	}

	screenRow := st.terminalCursorRow - st.scrollRow
	if screenRow < 0 || screenRow >= visRows {
		return
	}
	if st.terminalCursorRow < 0 || st.terminalCursorRow >= len(st.terminalLines) {
		return
	}
	line := st.terminalLines[st.terminalCursorRow]
	startCol := st.scrollCol
	if startCol > len(line) {
		startCol = len(line)
	}
	if st.terminalCursorCol < startCol {
		return
	}

	cx := textX
	endCol := st.terminalCursorCol
	if endCol > len(line) {
		endCol = len(line)
	}
	for col := startCol; col < endCol; col++ {
		ch := line[col].Char
		if ch == 0 {
			ch = ' '
		}
		w := textAreaRuneWidth(font, ch)
		if cx+w > maxX {
			return
		}
		cx += w
	}
	if cx < textX || cx >= maxX {
		return
	}

	cy := textArea.Min.Y + screenRow*font.Height
	cursorColor := e.AttrColor("cursorColor", gfxtheme.DefaultTheme.Foreground)
	for dy := 0; dy < font.Height; dy++ {
		buf.SetPixel(cx, cy+dy, cursorColor)
	}
}

func drawTableArea(e *Element, buf *core.Buffer) {
	bounds := e.Bounds()
	bodyFont := elementFont(e, gfxfont.UIFontParagraph)
	headerFont := gfxfont.UIFont(e.Attr("headerTextRole", gfxfont.UIFontSubheading))
	if headerFont == nil {
		headerFont = bodyFont
	}
	st := e.getTextAreaState()
	text := strings.TrimSpace(e.Attr("text", ""))
	if text == "" {
		return
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return
	}

	rows := make([][]string, 0, len(lines))
	maxCols := 0
	for _, line := range lines {
		cols := parseTableColumns(line)
		if len(cols) == 0 {
			continue
		}
		rows = append(rows, cols)
		if len(cols) > maxCols {
			maxCols = len(cols)
		}
	}
	if len(rows) == 0 || maxCols == 0 {
		return
	}

	colWidths := make([]int, maxCols)
	for r, row := range rows {
		for c := 0; c < len(row); c++ {
			measureFont := bodyFont
			if r == 0 {
				measureFont = headerFont
			}
			w := measureFont.TextWidth(row[c]) + 20
			if w > colWidths[c] {
				colWidths[c] = w
			}
		}
	}

	totalW := 0
	for _, w := range colWidths {
		totalW += w
	}
	availW := bounds.Dx() - 2
	if totalW > availW && totalW > 0 {
		for i := range colWidths {
			scaled := colWidths[i] * availW / totalW
			if scaled < 56 {
				scaled = 56
			}
			colWidths[i] = scaled
		}
	}

	rowH := bodyFont.Height + 12
	if headerFont.Height+12 > rowH {
		rowH = headerFont.Height + 12
	}
	headerBg := e.AttrColor("headerBackground", gfxtheme.DefaultTheme.SurfaceRaised)
	divider := e.AttrColor("dividerColor", gfxtheme.DefaultTheme.StrokeDivider)
	textColor := e.AttrColor("textColor", gfxtheme.DefaultTheme.TextSecondary)
	headerTextColor := e.AttrColor("headerTextColor", gfxtheme.DefaultTheme.Foreground)

	headerRect := core.RectXYWH(bounds.Min.X+1, bounds.Min.Y+1, bounds.Dx()-2, rowH)
	buf.FillRoundedRect(headerRect, 8, headerBg)

	x := bounds.Min.X + 1
	for c := 0; c < maxCols; c++ {
		if c < len(rows[0]) {
			ty := bounds.Min.Y + (rowH-headerFont.Height)/2
			headerFont.DrawText(buf, rows[0][c], x+10, ty, headerTextColor, core.ColorTransparent)
		}
		if c > 0 && divider.A > 0 {
			for py := bounds.Min.Y + 2; py < bounds.Min.Y+bounds.Dy()-2; py++ {
				bg := buf.GetPixel(x, py)
				buf.SetPixel(x, py, core.Blend(divider, bg))
			}
		}
		x += colWidths[c]
		if x >= bounds.Min.X+bounds.Dx()-1 {
			break
		}
	}

	visibleDataRows := (bounds.Dy() - 2 - rowH) / rowH
	if visibleDataRows < 1 {
		visibleDataRows = 1
	}
	dataRows := len(rows) - 1
	if dataRows < 0 {
		dataRows = 0
	}
	maxScroll := dataRows - visibleDataRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if st.scrollRow < 0 {
		st.scrollRow = 0
	}
	if st.scrollRow > maxScroll {
		st.scrollRow = maxScroll
	}

	for r := 0; r < visibleDataRows; r++ {
		dataIdx := st.scrollRow + r
		if dataIdx >= dataRows {
			break
		}
		rowVals := rows[dataIdx+1]
		ry := bounds.Min.Y + 1 + (r+1)*rowH
		if ry+rowH > bounds.Min.Y+bounds.Dy()-1 {
			break
		}
		if divider.A > 0 {
			for px := bounds.Min.X + 8; px < bounds.Min.X+bounds.Dx()-8; px++ {
				bg := buf.GetPixel(px, ry)
				buf.SetPixel(px, ry, core.Blend(divider, bg))
			}
		}
		x = bounds.Min.X + 1
		for c := 0; c < len(rowVals) && c < maxCols; c++ {
			ty := ry + (rowH-bodyFont.Height)/2
			cell := rowVals[c]
			cellX := x + 10
			if looksNumericCell(cell) {
				cellW := bodyFont.TextWidth(cell)
				cellX = x + colWidths[c] - cellW - 10
				if cellX < x+10 {
					cellX = x + 10
				}
			}
			bodyFont.DrawText(buf, cell, cellX, ty, textColor, core.ColorTransparent)
			x += colWidths[c]
			if x >= bounds.Min.X+bounds.Dx()-1 {
				break
			}
		}
	}
}

func parseTableColumns(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if strings.Contains(line, "\t") {
		parts := strings.Split(line, "\t")
		cols := make([]string, 0, len(parts))
		for _, part := range parts {
			value := strings.TrimSpace(part)
			if value == "" {
				value = "-"
			}
			cols = append(cols, value)
		}
		return cols
	}

	parts := tableMultiSpaceSplit.Split(line, -1)
	if len(parts) > 1 {
		cols := make([]string, 0, len(parts))
		for _, part := range parts {
			value := strings.TrimSpace(part)
			if value == "" {
				value = "-"
			}
			cols = append(cols, value)
		}
		return cols
	}

	return strings.Fields(line)
}

func looksNumericCell(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" {
		return false
	}

	hasDigit := false
	for _, ch := range s {
		switch {
		case ch >= '0' && ch <= '9':
			hasDigit = true
		case ch == ' ' || ch == '.' || ch == ',' || ch == '+' || ch == '-' || ch == '$' || ch == '%':
		case ch == 'K' || ch == 'M' || ch == 'G' || ch == 'T' || ch == 'B' || ch == 'k' || ch == 'm' || ch == 'g' || ch == 't' || ch == 'b':
		default:
			return false
		}
	}

	return hasDigit
}
