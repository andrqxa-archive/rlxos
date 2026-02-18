package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"avyos.dev/pkg/graphics"
)

// drawElement renders an element to the buffer based on its attributes.
func drawElement(e *Element, buf *graphics.Buffer) {
	bounds := e.bounds
	if bounds.W <= 0 || bounds.H <= 0 {
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
	restoreClip := false
	prevClip := graphics.Rect{}
	prevClipOn := false
	if e.overflowClipped() && len(e.children) > 0 {
		prevClip, prevClipOn = buf.Clip()
		clipRect := e.contentArea()
		if prevClipOn {
			clipRect = clipRect.Intersection(prevClip)
		}
		buf.SetClip(clipRect)
		restoreClip = true
	}

	for _, child := range e.children {
		if child.visible {
			child.Draw(buf)
		}
		child.MarkClean()
	}

	drawOverflowScrollbar(e, buf)

	if restoreClip {
		if prevClipOn {
			buf.SetClip(prevClip)
		} else {
			buf.ClearClip()
		}
	}
}

func drawOverflowScrollbar(e *Element, buf *graphics.Buffer) {
	if !e.overflowScrollable() || e.isRow() {
		return
	}

	content := e.contentArea()
	if content.W < 10 || content.H < 16 {
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

	trackRect := graphics.Rect{
		X: content.X + content.W - barW - margin,
		Y: content.Y + margin,
		W: barW,
		H: content.H - margin*2,
	}
	if trackRect.H < 8 || trackRect.W < 2 {
		return
	}

	contentTotal := content.H + maxScroll
	thumbH := 0
	if contentTotal > 0 {
		thumbH = trackRect.H * content.H / contentTotal
	}
	minThumb := e.AttrInt("scrollbarThumbMinHeight", 22)
	if minThumb < 10 {
		minThumb = 10
	}
	if thumbH < minThumb {
		thumbH = minThumb
	}
	if thumbH > trackRect.H {
		thumbH = trackRect.H
	}

	travel := trackRect.H - thumbH
	thumbY := trackRect.Y
	if travel > 0 {
		thumbY += scrollY * travel / maxScroll
	}
	thumbRect := graphics.Rect{X: trackRect.X, Y: thumbY, W: trackRect.W, H: thumbH}

	trackColor := e.AttrColor("scrollbarTrackColor", graphics.NewColor(16, 24, 40, 28))
	thumbColor := e.AttrColor("scrollbarThumbColor", graphics.NewColor(70, 96, 148, 136))
	radius := barW / 2

	if trackColor.A > 0 {
		buf.FillRoundedRect(trackRect, radius, trackColor)
	}
	if thumbColor.A > 0 {
		buf.FillRoundedRect(thumbRect, radius, thumbColor)
	}
}

func drawElementShadow(e *Element, buf *graphics.Buffer, bounds graphics.Rect, radius int) {
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
		if c := e.AttrColor("gradientTop", graphics.Color{}); c.A > 0 {
			shadowBase = c
		}
		if c := e.AttrColor("gradientBottom", graphics.Color{}); c.A > 0 {
			if shadowBase.A == 0 {
				shadowBase = c
			} else {
				shadowBase = mixColors(shadowBase, c, 0.5)
			}
		}
	}
	if shadowBase.A > 0 {
		shadowBase.A = 255
	}
	c := e.AttrColor("shadowColor", graphics.NewColor(16, 24, 40, 14))
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

	shadowRect := graphics.Rect{X: bounds.X + ox, Y: bounds.Y + oy, W: bounds.W, H: bounds.H}
	startX := shadowRect.X - spread
	if startX < 0 {
		startX = 0
	}
	startY := shadowRect.Y - spread
	if startY < 0 {
		startY = 0
	}
	endX := shadowRect.X + shadowRect.W + spread
	if endX > buf.Width {
		endX = buf.Width
	}
	endY := shadowRect.Y + shadowRect.H + spread
	if endY > buf.Height {
		endY = buf.Height
	}

	spreadF := float64(spread)
	gapF := float64(gap)
	effective := spreadF - gapF
	if effective <= 0 {
		effective = 1
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			d := roundedRectOutsideDistance(float64(x)+0.5, float64(y)+0.5, shadowRect, radius)
			if d > spreadF {
				continue
			}
			if d < gapF {
				continue
			}

			// Soft eased falloff from an edge gap outwards, avoids contour lines.
			dist := d - gapF
			if dist < 0 {
				dist = 0
			}
			t := dist / effective
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			falloff := (1 - t) * (1 - t)
			a := uint8(float64(c.A) * falloff)
			if a == 0 {
				continue
			}

			sc := graphics.NewColor(c.R, c.G, c.B, a)
			dst := buf.GetPixel(x, y)
			// When drawing over transparent pixels (common in layer/popup surfaces),
			// tint shadow using the element surface color but keep soft shadow alpha.
			if shadowBase.A > 0 && dst.A == 0 {
				baseOpaque := graphics.NewColor(shadowBase.R, shadowBase.G, shadowBase.B, 255)
				tinted := sc.Blend(baseOpaque)
				tinted.A = sc.A
				buf.SetPixel(x, y, tinted)
				continue
			}
			buf.SetPixel(x, y, sc.Blend(dst))
		}
	}
}

func drawElementBackground(e *Element, buf *graphics.Buffer, bounds graphics.Rect, radius int) {
	bg := effectiveBackground(e)
	gradTop := e.AttrColor("gradientTop", graphics.Color{})
	gradBottom := e.AttrColor("gradientBottom", graphics.Color{})

	if gradTop.A == 0 && gradBottom.A == 0 {
		if bg.A == 0 {
			return
		}
		if radius > 0 {
			buf.FillRoundedRect(bounds, radius, bg)
		} else {
			buf.FillRect(bounds, bg)
		}
		return
	}

	if gradTop.A == 0 {
		gradTop = bg
	}
	if gradBottom.A == 0 {
		gradBottom = bg
	}
	drawVerticalGradient(buf, bounds, radius, gradTop, gradBottom)
}

func drawVerticalGradient(buf *graphics.Buffer, r graphics.Rect, radius int, top, bottom graphics.Color) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if r.H == 1 {
		fill := top
		if fill.A > 0 {
			if radius > 0 {
				buf.FillRoundedRect(r, radius, fill)
			} else {
				buf.FillRect(r, fill)
			}
		}
		return
	}

	endY := r.Y + r.H
	endX := r.X + r.W
	hm1 := float64(r.H - 1)
	for y := r.Y; y < endY; y++ {
		t := float64(y-r.Y) / hm1
		line := graphics.NewColor(
			uint8(float64(top.R)+(float64(bottom.R)-float64(top.R))*t+0.5),
			uint8(float64(top.G)+(float64(bottom.G)-float64(top.G))*t+0.5),
			uint8(float64(top.B)+(float64(bottom.B)-float64(top.B))*t+0.5),
			uint8(float64(top.A)+(float64(bottom.A)-float64(top.A))*t+0.5),
		)
		if line.A == 0 {
			continue
		}
		for x := r.X; x < endX; x++ {
			if radius > 0 {
				cov := roundedRectCoverageUI(x, y, r, radius)
				if cov <= 0.001 {
					continue
				}
				if cov < 0.999 {
					sc := graphics.NewColor(line.R, line.G, line.B, uint8(float64(line.A)*cov+0.5))
					if sc.A == 0 {
						continue
					}
					dst := buf.GetPixel(x, y)
					buf.SetPixel(x, y, sc.Blend(dst))
					continue
				}
			}
			dst := buf.GetPixel(x, y)
			buf.SetPixel(x, y, line.Blend(dst))
		}
	}
}

func drawElementFocusRing(e *Element, buf *graphics.Buffer, bounds graphics.Rect, radius int) {
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

	ringColor := e.AttrColor("focusRingColor", e.AttrColor("focusedBorderColor", graphics.DefaultTheme.BorderFocused))
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

	for i := 0; i < ringWidth; i++ {
		grow := ringOffset + i
		ringRect := graphics.Rect{
			X: bounds.X - grow,
			Y: bounds.Y - grow,
			W: bounds.W + grow*2,
			H: bounds.H + grow*2,
		}
		ringRadius := radius + grow
		if ringRadius > 0 {
			buf.DrawRoundedRect(ringRect, ringRadius, ringColor)
		} else {
			buf.DrawRect(ringRect, ringColor)
		}
	}
}

func drawText(e *Element, buf *graphics.Buffer, text string) {
	font := elementFont(e, graphics.UIFontParagraph)
	textColor := e.AttrColor("textColor", graphics.DefaultTheme.Foreground)
	align := e.Attr("textAlign", "left")

	content := e.contentArea()
	if e.AttrBool("clipText", false) && content.W > 0 {
		text = trimTextToWidth(font, text, content.W, align)
	}
	if text == "" {
		return
	}
	textW := font.TextWidth(text)
	textH := font.TextHeight(text)

	x := content.X
	switch align {
	case "center":
		x = content.X + (content.W-textW)/2
	case "right":
		x = content.X + content.W - textW
	}
	y := content.Y + (content.H-textH)/2

	font.DrawText(buf, text, x, y, textColor, graphics.ColorTransparent)
}

func trimTextToWidth(font *graphics.Font, text string, maxWidth int, align string) string {
	if text == "" || maxWidth <= 0 {
		return ""
	}
	if font == nil {
		return text
	}
	if font.TextWidth(text) <= maxWidth {
		return text
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}

	runeWidth := func(ch rune) int {
		w := font.TextWidth(string(ch))
		if w <= 0 {
			w = font.Width
		}
		if w <= 0 {
			w = 8
		}
		return w
	}

	dots := "..."
	dotsW := font.TextWidth(dots)
	if dotsW >= maxWidth {
		dots = ""
		dotsW = 0
	}

	if strings.EqualFold(align, "right") {
		budget := maxWidth - dotsW
		if budget < 0 {
			budget = 0
		}
		start := len(runes)
		width := 0
		for start > 0 {
			w := runeWidth(runes[start-1])
			if width+w > budget {
				break
			}
			width += w
			start--
		}
		if start > 0 && dots != "" {
			return dots + string(runes[start:])
		}
		return string(runes[start:])
	}

	budget := maxWidth - dotsW
	if budget < 0 {
		budget = 0
	}
	end := 0
	width := 0
	for end < len(runes) {
		w := runeWidth(runes[end])
		if width+w > budget {
			break
		}
		width += w
		end++
	}
	if end <= 0 {
		if dots != "" && dotsW <= maxWidth {
			return dots
		}
		return ""
	}
	if end < len(runes) && dots != "" {
		return string(runes[:end]) + dots
	}
	return string(runes[:end])
}

// --- Frame title ---

func drawFrameTitle(e *Element, buf *graphics.Buffer, title string, radius int) {
	bounds := e.Bounds()
	font := elementTitleFont(e)
	borderColor := effectiveBorderColor(e)
	titleColor := e.AttrColor("titleColor", graphics.DefaultTheme.Foreground)

	titleH := font.Height / 2
	titleGap := 4

	// Draw border offset down by titleH
	borderRect := graphics.Rect{
		X: bounds.X,
		Y: bounds.Y + titleH,
		W: bounds.W,
		H: bounds.H - titleH,
	}
	if borderColor.A > 0 {
		if radius > 0 {
			buf.DrawRoundedRect(borderRect, radius, borderColor)
		} else {
			buf.DrawRect(borderRect, borderColor)
		}
	}

	// Draw title text on the border
	titleX := bounds.X + 1 + titleGap
	titleY := bounds.Y

	// Clear the border behind the title
	textW := font.TextWidth(title)
	bg := e.AttrColor("background", graphics.Color{})
	clearRect := graphics.Rect{
		X: titleX - 2,
		Y: bounds.Y + titleH,
		W: textW + 4,
		H: 1,
	}
	buf.FillRect(clearRect, bg)

	font.DrawText(buf, title, titleX, titleY, titleColor, graphics.ColorTransparent)
}

// --- Checkbox ---

func drawCheckable(e *Element, buf *graphics.Buffer) {
	bounds := e.Bounds()
	font := elementFont(e, graphics.UIFontParagraph)
	boxSize := font.Height + 4
	if boxSize > bounds.H-2 {
		boxSize = bounds.H - 2
	}
	if boxSize < font.Height {
		boxSize = font.Height
	}
	gap := 8
	isRadio := e.AttrBool("radio", false)

	boxY := bounds.Y + (bounds.H-boxSize)/2
	boxRect := graphics.Rect{X: bounds.X, Y: boxY, W: boxSize, H: boxSize}
	radius := graphics.DefaultTheme.BorderRadius / 2
	if isRadio {
		radius = boxSize / 2
	}

	borderColor := e.AttrColor("boxBorderColor", e.AttrColor("borderColor", graphics.DefaultTheme.Border))
	if e.focused {
		borderColor = e.AttrColor("focusedBoxBorderColor", e.AttrColor("focusedBorderColor", graphics.DefaultTheme.BorderFocused))
	}
	fillColor := e.AttrColor("boxBackground", e.AttrColor("background", graphics.DefaultTheme.InputBackground))
	if fillColor.A > 0 {
		buf.FillRoundedRect(boxRect, radius, fillColor)
	}
	buf.DrawRoundedRect(boxRect, radius, borderColor)

	checkColor := e.AttrColor("checkColor", graphics.DefaultTheme.Primary)

	if e.hovered {
		highlight := graphics.NewColor(checkColor.R, checkColor.G, checkColor.B, 30)
		buf.FillRoundedRect(graphics.Rect{
			X: boxRect.X + 1, Y: boxRect.Y + 1,
			W: boxRect.W - 2, H: boxRect.H - 2,
		}, radius, highlight)
	}

	if e.AttrBool("checked", false) {
		if isRadio {
			dotD := (boxSize * 9) / 20
			dotRect := graphics.Rect{
				X: boxRect.X + (boxSize-dotD)/2,
				Y: boxRect.Y + (boxSize-dotD)/2,
				W: dotD,
				H: dotD,
			}
			buf.FillRoundedRect(dotRect, dotD/2, checkColor)
		} else {
			buf.FillRoundedRect(graphics.Rect{
				X: boxRect.X + 1, Y: boxRect.Y + 1,
				W: boxRect.W - 2, H: boxRect.H - 2,
			}, radius, checkColor)
			bx, by := float64(boxRect.X), float64(boxRect.Y)
			sz := float64(boxSize)
			tickColor := graphics.NewColor(255, 255, 255, 246)
			p1x, p1y := bx+0.24*sz, by+0.54*sz
			p2x, p2y := bx+0.44*sz, by+0.74*sz
			p3x, p3y := bx+0.78*sz, by+0.30*sz
			drawAALine(buf, p1x, p1y, p2x, p2y, 2.2, tickColor)
			drawAALine(buf, p2x, p2y, p3x, p3y, 2.2, tickColor)
		}
	}

	text := e.Attr("text", "")
	if text != "" {
		textColor := e.AttrColor("textColor", graphics.DefaultTheme.Foreground)
		textX := bounds.X + boxSize + gap
		textH := font.TextHeight(text)
		textY := bounds.Y + (bounds.H-textH)/2
		font.DrawText(buf, text, textX, textY, textColor, graphics.ColorTransparent)
	}
}

func drawToggle(e *Element, buf *graphics.Buffer) {
	bounds := e.Bounds()
	font := elementFont(e, graphics.UIFontParagraph)
	text := e.Attr("text", "")

	trackW := 42
	trackH := 24
	r := trackH / 2
	trackX := bounds.X
	trackY := bounds.Y + (bounds.H-trackH)/2
	trackRect := graphics.Rect{X: trackX, Y: trackY, W: trackW, H: trackH}

	offColor := e.AttrColor("offTrackColor", graphics.NewColorHex(0x1B2A4A24))
	onColor := e.AttrColor("onTrackColor", graphics.NewColorHex(0x2F6BBF))
	borderColor := e.AttrColor("trackBorderColor", e.AttrColor("borderColor", graphics.NewColorHex(0x1B2A4A24)))
	if e.focused {
		borderColor = e.AttrColor("focusedTrackBorderColor", e.AttrColor("focusedBorderColor", borderColor))
	}

	position := 0.0
	if e.AttrBool("checked", false) {
		position = 1.0
	}

	trackColor := mixColors(offColor, onColor, position)
	if e.hovered {
		trackColor = mixColors(trackColor, graphics.ColorWhite, 0.06)
	}
	if e.pressed {
		trackColor = mixColors(trackColor, graphics.ColorBlack, 0.08)
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
	thumbRect := graphics.Rect{X: thumbX, Y: thumbY, W: thumbD, H: thumbD}
	drawVerticalGradient(
		buf,
		thumbRect,
		thumbD/2,
		graphics.NewColor(255, 255, 255, 255),
		graphics.NewColor(237, 244, 255, 252),
	)
	outline := graphics.NewColorHex(0x1B2A4A1C)
	if e.AttrBool("checked", false) {
		outline = graphics.NewColorHex(0x1B2A4A26)
	}
	buf.DrawRoundedRect(thumbRect, thumbD/2, outline)

	if text != "" {
		textColor := e.AttrColor("textColor", graphics.DefaultTheme.Foreground)
		tx := trackRect.X + trackRect.W + 8
		ty := bounds.Y + (bounds.H-font.Height)/2
		font.DrawText(buf, text, tx, ty, textColor, graphics.ColorTransparent)
	}
}

func drawAALine(buf *graphics.Buffer, x0, y0, x1, y1, width float64, c graphics.Color) {
	if c.A == 0 || width <= 0 {
		return
	}
	minX := int(math.Floor(math.Min(x0, x1) - width - 1))
	maxX := int(math.Ceil(math.Max(x0, x1) + width + 1))
	minY := int(math.Floor(math.Min(y0, y1) - width - 1))
	maxY := int(math.Ceil(math.Max(y0, y1) + width + 1))
	r := width / 2
	if r < 0.5 {
		r = 0.5
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			d := pointSegmentDistance(px, py, x0, y0, x1, y1)
			cov := r + 0.5 - d
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			a := uint8(float64(c.A)*cov + 0.5)
			if a == 0 {
				continue
			}
			sc := graphics.NewColor(c.R, c.G, c.B, a)
			bg := buf.GetPixel(x, y)
			buf.SetPixel(x, y, sc.Blend(bg))
		}
	}
}

func pointSegmentDistance(px, py, x0, y0, x1, y1 float64) float64 {
	dx := x1 - x0
	dy := y1 - y0
	den := dx*dx + dy*dy
	if den <= 1e-6 {
		return math.Hypot(px-x0, py-y0)
	}
	t := ((px-x0)*dx + (py-y0)*dy) / den
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	cx := x0 + t*dx
	cy := y0 + t*dy
	return math.Hypot(px-cx, py-cy)
}

func drawListView(e *Element, buf *graphics.Buffer) {
	bounds := e.Bounds()
	font := elementFont(e, graphics.UIFontParagraph)
	itemsRaw := e.Attr("items", "")
	if itemsRaw == "" {
		return
	}
	items := strings.Split(itemsRaw, "|")
	rowH := e.AttrInt("rowHeight", 34)
	if rowH < font.Height+8 {
		rowH = font.Height + 8
	}
	visibleRows := bounds.H / rowH
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
	hoverColor := e.AttrColor("hoverBackground", graphics.NewColorHex(0x2F6BFF24))
	selectColor := e.AttrColor("selectedBackground", graphics.NewColorHex(0x2F6BFF24))
	indicatorColor := e.AttrColor("selectedIndicatorColor", graphics.DefaultTheme.Primary)
	indicatorW := e.AttrInt("selectedIndicatorWidth", 3)
	if indicatorW < 0 {
		indicatorW = 0
	}
	textColor := e.AttrColor("textColor", graphics.DefaultTheme.Foreground)
	borderColor := e.AttrColor("borderColor", graphics.NewColorHex(0x1B2A4A24))
	dividerColor := e.AttrColor("dividerColor", graphics.DefaultTheme.StrokeDivider)
	rowRadius := e.AttrInt("rowRadius", 8)

	for i := 0; i < len(items); i++ {
		itemIdx := scrollIndex + i
		if itemIdx >= len(items) {
			break
		}
		y := bounds.Y + i*rowH
		if y+rowH > bounds.Y+bounds.H {
			break
		}
		row := graphics.Rect{X: bounds.X + 1, Y: y, W: bounds.W - 2, H: rowH}
		hovered := itemIdx == e.AttrInt("hoveredIndex", -1)
		if hovered && itemIdx != selected {
			buf.FillRoundedRect(row, rowRadius, hoverColor)
		}
		textInset := 12
		if itemIdx == selected {
			buf.FillRoundedRect(row, rowRadius, selectColor)
			if indicatorW > 0 {
				iw := indicatorW
				if iw > row.W-4 {
					iw = row.W - 4
				}
				if iw > 0 {
					indicatorRect := graphics.Rect{
						X: row.X + 2,
						Y: row.Y + 4,
						W: iw,
						H: row.H - 8,
					}
					if indicatorRect.H < 2 {
						indicatorRect.Y = row.Y + 1
						indicatorRect.H = row.H - 2
					}
					if indicatorRect.H > 0 {
						buf.FillRoundedRect(indicatorRect, iw, indicatorColor)
					}
					textInset += iw + 4
				}
			}
		}
		if i > 0 && dividerColor.A > 0 {
			dy := y
			for x := bounds.X + 8; x < bounds.X+bounds.W-8; x++ {
				bg := buf.GetPixel(x, dy)
				buf.SetPixel(x, dy, dividerColor.Blend(bg))
			}
		}
		tx := row.X + textInset
		ty := row.Y + (row.H-font.Height)/2
		font.DrawText(buf, strings.TrimSpace(items[itemIdx]), tx, ty, textColor, graphics.ColorTransparent)
	}

	if borderColor.A > 0 {
		buf.DrawRoundedRect(bounds, e.AttrInt("borderRadius", 10), borderColor)
	}
}

func mixColors(a, b graphics.Color, t float64) graphics.Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	inv := 1.0 - t
	return graphics.NewColor(
		uint8(float64(a.R)*inv+float64(b.R)*t+0.5),
		uint8(float64(a.G)*inv+float64(b.G)*t+0.5),
		uint8(float64(a.B)*inv+float64(b.B)*t+0.5),
		uint8(float64(a.A)*inv+float64(b.A)*t+0.5),
	)
}

// --- Slider ---

func drawSlidable(e *Element, buf *graphics.Buffer) {
	bounds := e.Bounds()
	st := e.getSliderState()

	orientation := e.Attr("orientation", "horizontal")
	if orientation == "vertical" {
		drawSliderVertical(e, buf, bounds, st)
	} else {
		drawSliderHorizontal(e, buf, bounds, st)
	}
}

func drawSliderHorizontal(e *Element, buf *graphics.Buffer, bounds graphics.Rect, st *sliderState) {
	radius := graphics.DefaultTheme.BorderRadius / 2
	trackColor := e.AttrColor("trackColor", graphics.DefaultTheme.Secondary)
	thumbColor := e.AttrColor("thumbColor", graphics.DefaultTheme.Primary)

	trackY := bounds.Y + (bounds.H-sliderTrackH)/2
	trackRect := graphics.Rect{
		X: bounds.X + sliderThumbW/2, Y: trackY,
		W: bounds.W - sliderThumbW, H: sliderTrackH,
	}
	buf.FillRoundedRect(trackRect, sliderTrackH/2, trackColor)

	thumbX := e.sliderValueToPixelH(bounds)
	filledRect := graphics.Rect{
		X: trackRect.X, Y: trackY,
		W: thumbX - trackRect.X + sliderThumbW/2, H: sliderTrackH,
	}
	if filledRect.W > 0 {
		buf.FillRoundedRect(filledRect, sliderTrackH/2, thumbColor)
	}

	thumbRect := graphics.Rect{
		X: thumbX, Y: bounds.Y + (bounds.H-sliderThumbH)/2,
		W: sliderThumbW, H: sliderThumbH,
	}
	tc := thumbColor
	if st.dragging {
		tc = e.AttrColor("thumbActiveColor", graphics.DefaultTheme.PrimaryActive)
	} else if e.hovered {
		tc = e.AttrColor("thumbHoverColor", graphics.DefaultTheme.PrimaryHover)
	}
	buf.FillRoundedRect(thumbRect, radius, tc)

	borderColor := e.AttrColor("borderColor", graphics.DefaultTheme.Border)
	if e.focused {
		borderColor = e.AttrColor("focusedBorderColor", graphics.DefaultTheme.BorderFocused)
	}
	buf.DrawRoundedRect(thumbRect, radius, borderColor)
}

func drawSliderVertical(e *Element, buf *graphics.Buffer, bounds graphics.Rect, st *sliderState) {
	radius := graphics.DefaultTheme.BorderRadius / 2
	trackColor := e.AttrColor("trackColor", graphics.DefaultTheme.Secondary)
	thumbColor := e.AttrColor("thumbColor", graphics.DefaultTheme.Primary)

	trackX := bounds.X + (bounds.W-sliderTrackH)/2
	trackRect := graphics.Rect{
		X: trackX, Y: bounds.Y + sliderThumbW/2,
		W: sliderTrackH, H: bounds.H - sliderThumbW,
	}
	buf.FillRoundedRect(trackRect, sliderTrackH/2, trackColor)

	thumbY := e.sliderValueToPixelV(bounds)
	filledRect := graphics.Rect{
		X: trackX, Y: thumbY + sliderThumbW/2,
		W: sliderTrackH, H: trackRect.Y + trackRect.H - thumbY - sliderThumbW/2,
	}
	if filledRect.H > 0 {
		buf.FillRoundedRect(filledRect, sliderTrackH/2, thumbColor)
	}

	thumbRect := graphics.Rect{
		X: bounds.X + (bounds.W-sliderThumbH)/2, Y: thumbY,
		W: sliderThumbH, H: sliderThumbW,
	}
	tc := thumbColor
	if st.dragging {
		tc = e.AttrColor("thumbActiveColor", graphics.DefaultTheme.PrimaryActive)
	} else if e.hovered {
		tc = e.AttrColor("thumbHoverColor", graphics.DefaultTheme.PrimaryHover)
	}
	buf.FillRoundedRect(thumbRect, radius, tc)

	borderColor := e.AttrColor("borderColor", graphics.DefaultTheme.Border)
	if e.focused {
		borderColor = e.AttrColor("focusedBorderColor", graphics.DefaultTheme.BorderFocused)
	}
	buf.DrawRoundedRect(thumbRect, radius, borderColor)
}

// --- Progress bar ---

func drawProgress(e *Element, buf *graphics.Buffer) {
	bounds := e.Bounds()
	value := e.AttrFloat("value", 0)
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	fillColor := e.AttrColor("fillColor", graphics.DefaultTheme.Primary)
	fillTop := e.AttrColor("fillGradientTop", graphics.Color{})
	fillBottom := e.AttrColor("fillGradientBottom", graphics.Color{})
	radius := e.AttrInt("borderRadius", graphics.DefaultTheme.BorderRadius)

	innerW := bounds.W - 2
	if innerW > 0 {
		filledW := int(float64(innerW) * value)
		if filledW > 0 {
			fillRect := graphics.Rect{
				X: bounds.X + 1, Y: bounds.Y + 1,
				W: filledW, H: bounds.H - 2,
			}
			if fillTop.A > 0 || fillBottom.A > 0 {
				if fillTop.A == 0 {
					fillTop = fillColor
				}
				if fillBottom.A == 0 {
					fillBottom = fillColor
				}
				drawVerticalGradient(buf, fillRect, radius, fillTop, fillBottom)
			} else {
				buf.FillRoundedRect(fillRect, radius, fillColor)
			}
		}
	}

	if e.AttrBool("showText", true) {
		font := elementFont(e, graphics.UIFontParagraph)
		textColor := e.AttrColor("textColor", graphics.DefaultTheme.Foreground)
		text := fmt.Sprintf("%d%%", int(value*100))
		textW := font.TextWidth(text)
		textH := font.TextHeight(text)
		x := bounds.X + (bounds.W-textW)/2
		y := bounds.Y + (bounds.H-textH)/2
		font.DrawText(buf, text, x, y, textColor, graphics.ColorTransparent)
	}
}

// --- Text input ---

func drawTextInput(e *Element, buf *graphics.Buffer) {
	font := elementFont(e, graphics.UIFontParagraph)
	st := e.getTextInputState()
	textArea := e.contentArea()
	if textArea.W <= 0 || textArea.H <= 0 {
		return
	}

	text := st.text
	isPassword := e.AttrBool("password", false)
	display := textInputDisplayRunes(st, isPassword)

	startCol, endCol := textInputVisibleRange(font, display, st.scrollOffset, textArea.W)
	textY := textArea.Y + (textArea.H-font.Height)/2

	// Placeholder
	if len(text) == 0 && !e.focused {
		placeholder := e.Attr("placeholder", "")
		if placeholder != "" {
			placeholderColor := e.AttrColor("placeholderColor", graphics.ColorGray)
			font.DrawText(buf, placeholder, textArea.X, textY,
				placeholderColor, graphics.ColorTransparent)
		}
		return
	}

	// Display text
	visible := string(display[startCol:endCol])
	textColor := e.AttrColor("textColor", graphics.DefaultTheme.InputForeground)
	font.DrawText(buf, visible, textArea.X, textY,
		textColor, graphics.ColorTransparent)

	// Cursor
	if e.focused && !e.AttrBool("readOnly", false) {
		elapsed := time.Since(st.lastBlink)
		if elapsed > 500*time.Millisecond {
			st.cursorBlink = !st.cursorBlink
			st.lastBlink = time.Now()
		}
		if st.cursorBlink {
			if st.cursorPos >= startCol && st.cursorPos <= endCol {
				cx := textArea.X + textInputSliceWidth(font, display, startCol, st.cursorPos)
				cy := textY
				cursorColor := e.AttrColor("cursorColor", graphics.DefaultTheme.Foreground)
				for dy := 0; dy < font.Height; dy++ {
					buf.SetPixel(cx, cy+dy, cursorColor)
				}
			}
		}
	}
}

// --- Text area ---

func drawTextArea(e *Element, buf *graphics.Buffer) {
	if e.AttrBool("table", false) {
		drawTableArea(e, buf)
		return
	}

	font := elementFont(e, graphics.UIFontParagraph)
	st := e.getTextAreaState()
	textArea := e.contentArea()
	if textArea.W <= 0 || textArea.H <= 0 {
		return
	}
	visRows := textArea.H / font.Height
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
		var trackRect graphics.Rect
		var thumbRect graphics.Rect
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
			trackRect = graphics.Rect{
				X: textArea.X + textArea.W - barW - margin,
				Y: textArea.Y + margin,
				W: barW,
				H: textArea.H - margin*2,
			}
			if trackRect.X <= textArea.X || trackRect.H < 8 {
				drawScrollbar = false
			} else {
				thumbH := trackRect.H * visRows / len(segments)
				minThumb := e.AttrInt("scrollbarThumbMinHeight", 16)
				if minThumb < 8 {
					minThumb = 8
				}
				if thumbH < minThumb {
					thumbH = minThumb
				}
				if thumbH > trackRect.H {
					thumbH = trackRect.H
				}

				travel := trackRect.H - thumbH
				thumbY := trackRect.Y
				if travel > 0 && maxScroll > 0 {
					thumbY += st.scrollRow * travel / maxScroll
				}
				thumbRect = graphics.Rect{
					X: trackRect.X,
					Y: thumbY,
					W: trackRect.W,
					H: thumbH,
				}
			}
		}

		textColor := e.AttrColor("textColor", graphics.DefaultTheme.InputForeground)
		textX := textArea.X
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
			y := textArea.Y + i*font.Height
			font.DrawText(buf, string(line[seg.start:seg.end]), textX, y,
				textColor, graphics.ColorTransparent)
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
					cy := textArea.Y + screenRow*font.Height
					cursorColor := e.AttrColor("cursorColor", graphics.DefaultTheme.Foreground)
					for dy := 0; dy < font.Height; dy++ {
						buf.SetPixel(cx, cy+dy, cursorColor)
					}
				}
			}
		}

		if drawScrollbar {
			trackColor := e.AttrColor("scrollbarTrackColor", graphics.NewColor(16, 24, 40, 28))
			thumbColor := e.AttrColor("scrollbarThumbColor", graphics.NewColor(70, 96, 148, 136))
			radius := trackRect.W / 2
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

	textW := textArea.W
	drawScrollbar := e.AttrBool("showScrollbar", false) && maxScroll > 0
	var trackRect graphics.Rect
	var thumbRect graphics.Rect
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
		trackRect = graphics.Rect{
			X: textArea.X + textArea.W - barW - margin,
			Y: textArea.Y + margin,
			W: barW,
			H: textArea.H - margin*2,
		}
		if trackRect.X <= textArea.X || trackRect.H < 8 {
			drawScrollbar = false
		} else {
			textW = trackRect.X - textArea.X - 1
			if textW < 1 {
				drawScrollbar = false
				textW = textArea.W
			}
		}

		if drawScrollbar {
			thumbH := trackRect.H * visRows / len(st.lines)
			minThumb := e.AttrInt("scrollbarThumbMinHeight", 16)
			if minThumb < 8 {
				minThumb = 8
			}
			if thumbH < minThumb {
				thumbH = minThumb
			}
			if thumbH > trackRect.H {
				thumbH = trackRect.H
			}

			travel := trackRect.H - thumbH
			thumbY := trackRect.Y
			if travel > 0 && maxScroll > 0 {
				thumbY += st.scrollRow * travel / maxScroll
			}
			thumbRect = graphics.Rect{
				X: trackRect.X,
				Y: thumbY,
				W: trackRect.W,
				H: thumbH,
			}
		}
	}

	if len(st.terminalLines) > 0 {
		drawTerminalTextArea(e, buf, font, st, textArea, textW, visRows)
	} else {
		textColor := e.AttrColor("textColor", graphics.DefaultTheme.InputForeground)
		textX := textArea.X

		for i := 0; i < visRows; i++ {
			lineIdx := st.scrollRow + i
			if lineIdx >= len(st.lines) {
				break
			}
			line := st.lines[lineIdx]
			y := textArea.Y + i*font.Height

			startCol := st.scrollCol
			if startCol > len(line) {
				startCol = len(line)
			}
			endCol := textAreaVisibleEndCol(font, line, startCol, textW)
			if endCol < startCol {
				endCol = startCol
			}
			font.DrawText(buf, string(line[startCol:endCol]), textX, y,
				textColor, graphics.ColorTransparent)
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
						cy := textArea.Y + screenRow*font.Height
						cursorColor := e.AttrColor("cursorColor", graphics.DefaultTheme.Foreground)
						for dy := 0; dy < font.Height; dy++ {
							buf.SetPixel(cx, cy+dy, cursorColor)
						}
					}
				}
			}
		}
	}

	if drawScrollbar {
		trackColor := e.AttrColor("scrollbarTrackColor", graphics.NewColor(16, 24, 40, 28))
		thumbColor := e.AttrColor("scrollbarThumbColor", graphics.NewColor(70, 96, 148, 136))
		radius := trackRect.W / 2
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

func textAreaRuneWidth(font *graphics.Font, ch rune) int {
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
	buf *graphics.Buffer,
	font *graphics.Font,
	st *textAreaState,
	textArea graphics.Rect,
	textW int,
	visRows int,
) {
	textX := textArea.X
	maxX := textX + textW
	defaultTextColor := e.AttrColor("textColor", graphics.DefaultTheme.InputForeground)

	for i := 0; i < visRows; i++ {
		lineIdx := st.scrollRow + i
		if lineIdx >= len(st.terminalLines) {
			break
		}
		line := st.terminalLines[lineIdx]
		y := textArea.Y + i*font.Height

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
				buf.FillRect(graphics.Rect{X: x, Y: y, W: w, H: font.Height}, cell.Bg)
			}

			fg := cell.Fg
			if fg.A == 0 {
				fg = defaultTextColor
			}
			if ch != ' ' {
				font.DrawText(buf, string(ch), x, y, fg, graphics.ColorTransparent)
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

	cy := textArea.Y + screenRow*font.Height
	cursorColor := e.AttrColor("cursorColor", graphics.DefaultTheme.Foreground)
	for dy := 0; dy < font.Height; dy++ {
		buf.SetPixel(cx, cy+dy, cursorColor)
	}
}

func drawTableArea(e *Element, buf *graphics.Buffer) {
	bounds := e.Bounds()
	bodyFont := elementFont(e, graphics.UIFontParagraph)
	headerFont := graphics.UIFont(e.Attr("headerTextRole", graphics.UIFontSubheading))
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
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		rows = append(rows, fields)
		if len(fields) > maxCols {
			maxCols = len(fields)
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
	availW := bounds.W - 2
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
	headerBg := e.AttrColor("headerBackground", graphics.DefaultTheme.SurfaceRaised)
	divider := e.AttrColor("dividerColor", graphics.DefaultTheme.StrokeDivider)
	textColor := e.AttrColor("textColor", graphics.DefaultTheme.TextSecondary)
	headerTextColor := e.AttrColor("headerTextColor", graphics.DefaultTheme.Foreground)

	headerRect := graphics.Rect{X: bounds.X + 1, Y: bounds.Y + 1, W: bounds.W - 2, H: rowH}
	buf.FillRoundedRect(headerRect, 8, headerBg)

	x := bounds.X + 1
	for c := 0; c < maxCols; c++ {
		if c < len(rows[0]) {
			ty := bounds.Y + (rowH-headerFont.Height)/2
			headerFont.DrawText(buf, rows[0][c], x+10, ty, headerTextColor, graphics.ColorTransparent)
		}
		if c > 0 && divider.A > 0 {
			for py := bounds.Y + 2; py < bounds.Y+bounds.H-2; py++ {
				bg := buf.GetPixel(x, py)
				buf.SetPixel(x, py, divider.Blend(bg))
			}
		}
		x += colWidths[c]
		if x >= bounds.X+bounds.W-1 {
			break
		}
	}

	visibleDataRows := (bounds.H - 2 - rowH) / rowH
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
		ry := bounds.Y + 1 + (r+1)*rowH
		if ry+rowH > bounds.Y+bounds.H-1 {
			break
		}
		if divider.A > 0 {
			for px := bounds.X + 8; px < bounds.X+bounds.W-8; px++ {
				bg := buf.GetPixel(px, ry)
				buf.SetPixel(px, ry, divider.Blend(bg))
			}
		}
		x = bounds.X + 1
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
			bodyFont.DrawText(buf, cell, cellX, ty, textColor, graphics.ColorTransparent)
			x += colWidths[c]
			if x >= bounds.X+bounds.W-1 {
				break
			}
		}
	}
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
