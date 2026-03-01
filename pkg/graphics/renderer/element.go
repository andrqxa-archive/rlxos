package renderer

import (
	"image"
	"image/color"
	"math"
	"strings"

	gfxfont "avyos.dev/pkg/graphics/fonts"
	core "avyos.dev/pkg/graphics/pixmap"
)

// MixColors linearly blends color a->b by t in [0..1].
func MixColors(a, b color.NRGBA, t float64) color.NRGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	inv := 1.0 - t
	return core.NewColor(
		uint8(float64(a.R)*inv+float64(b.R)*t+0.5),
		uint8(float64(a.G)*inv+float64(b.G)*t+0.5),
		uint8(float64(a.B)*inv+float64(b.B)*t+0.5),
		uint8(float64(a.A)*inv+float64(b.A)*t+0.5),
	)
}

// DrawVerticalGradient draws a vertical gradient in r, clipping to rounded corners when radius>0.
func DrawVerticalGradient(buf *core.Buffer, r image.Rectangle, radius int, top, bottom color.NRGBA) {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return
	}
	if r.Dy() == 1 {
		fill := top
		if fill.A == 0 {
			return
		}
		endX := r.Min.X + r.Dx()
		for x := r.Min.X; x < endX; x++ {
			if radius > 0 {
				cov := RoundedRectCoverage(x, r.Min.Y, r, radius)
				if cov <= 0.001 {
					continue
				}
				alpha := uint8(float64(fill.A)*cov + 0.5)
				if alpha == 0 {
					continue
				}
				sc := core.NewColor(fill.R, fill.G, fill.B, alpha)
				dst := buf.GetPixel(x, r.Min.Y)
				buf.SetPixel(x, r.Min.Y, core.Blend(sc, dst))
				continue
			}
			dst := buf.GetPixel(x, r.Min.Y)
			buf.SetPixel(x, r.Min.Y, core.Blend(fill, dst))
		}
		return
	}

	endY := r.Min.Y + r.Dy()
	endX := r.Min.X + r.Dx()
	hm1 := float64(r.Dy() - 1)
	for y := r.Min.Y; y < endY; y++ {
		t := float64(y-r.Min.Y) / hm1
		line := core.NewColor(
			uint8(float64(top.R)+(float64(bottom.R)-float64(top.R))*t+0.5),
			uint8(float64(top.G)+(float64(bottom.G)-float64(top.G))*t+0.5),
			uint8(float64(top.B)+(float64(bottom.B)-float64(top.B))*t+0.5),
			uint8(float64(top.A)+(float64(bottom.A)-float64(top.A))*t+0.5),
		)
		if line.A == 0 {
			continue
		}
		for x := r.Min.X; x < endX; x++ {
			if radius > 0 {
				cov := RoundedRectCoverage(x, y, r, radius)
				if cov <= 0.001 {
					continue
				}
				if cov < 0.999 {
					sc := core.NewColor(line.R, line.G, line.B, uint8(float64(line.A)*cov+0.5))
					if sc.A == 0 {
						continue
					}
					dst := buf.GetPixel(x, y)
					buf.SetPixel(x, y, core.Blend(sc, dst))
					continue
				}
			}
			dst := buf.GetPixel(x, y)
			buf.SetPixel(x, y, core.Blend(line, dst))
		}
	}
}

// DrawRoundedShadow draws a soft shadow outside a rounded rect.
// shadowRect is the target rounded rectangle already offset into its final position.
func DrawRoundedShadow(
	buf *core.Buffer,
	shadowRect image.Rectangle,
	radius int,
	shadowColor color.NRGBA,
	shadowBase color.NRGBA,
	spread int,
	gap int,
) {
	if shadowColor.A == 0 || spread < 1 {
		return
	}
	if gap < 0 {
		gap = 0
	}
	if gap >= spread {
		gap = spread - 1
	}

	startX := shadowRect.Min.X - spread
	if startX < 0 {
		startX = 0
	}
	startY := shadowRect.Min.Y - spread
	if startY < 0 {
		startY = 0
	}
	endX := shadowRect.Min.X + shadowRect.Dx() + spread
	if endX > buf.Width {
		endX = buf.Width
	}
	endY := shadowRect.Min.Y + shadowRect.Dy() + spread
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
			d := RoundedRectOutsideDistance(float64(x)+0.5, float64(y)+0.5, shadowRect, radius)
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
			a := uint8(float64(shadowColor.A) * falloff)
			if a == 0 {
				continue
			}

			sc := core.NewColor(shadowColor.R, shadowColor.G, shadowColor.B, a)
			dst := buf.GetPixel(x, y)
			// When drawing over transparent pixels (common in layer/popup surfaces),
			// tint shadow using the element surface color but keep soft shadow alpha.
			if shadowBase.A > 0 && dst.A == 0 {
				baseOpaque := core.NewColor(shadowBase.R, shadowBase.G, shadowBase.B, 255)
				tinted := core.Blend(sc, baseOpaque)
				tinted.A = sc.A
				buf.SetPixel(x, y, tinted)
				continue
			}
			buf.SetPixel(x, y, core.Blend(sc, dst))
		}
	}
}

// DrawFocusRing draws an outward focus ring around bounds.
func DrawFocusRing(buf *core.Buffer, bounds image.Rectangle, radius int, ringColor color.NRGBA, ringWidth, ringOffset int) {
	if ringColor.A == 0 || ringWidth < 1 {
		return
	}
	if ringOffset < 0 {
		ringOffset = 0
	}
	for i := 0; i < ringWidth; i++ {
		grow := ringOffset + i
		ringRect := core.RectXYWH(bounds.Min.X-grow, bounds.Min.Y-grow, bounds.Dx()+grow*2, bounds.Dy()+grow*2)
		ringRadius := radius + grow
		if ringRadius > 0 {
			buf.DrawRoundedRect(ringRect, ringRadius, ringColor)
		} else {
			buf.DrawRect(ringRect, ringColor)
		}
	}
}

// TrimTextToWidth shortens text to fit maxWidth, keeping right-alignment behavior.
func TrimTextToWidth(font *gfxfont.Font, text string, maxWidth int, align string) string {
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

// DrawAALine draws an antialiased line with width and alpha blending.
func DrawAALine(buf *core.Buffer, x0, y0, x1, y1, width float64, c color.NRGBA) {
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
			sc := core.NewColor(c.R, c.G, c.B, a)
			bg := buf.GetPixel(x, y)
			buf.SetPixel(x, y, core.Blend(sc, bg))
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

// DrawListRowFill draws either a solid or gradient list row fill.
func DrawListRowFill(buf *core.Buffer, row image.Rectangle, radius int, solid, top, bottom color.NRGBA) {
	if row.Dx() <= 0 || row.Dy() <= 0 {
		return
	}
	if top.A == 0 && bottom.A == 0 {
		if solid.A == 0 {
			return
		}
		if radius > 0 {
			buf.FillRoundedRect(row, radius, solid)
		} else if solid.A == 255 {
			buf.FillRect(row, solid)
		} else {
			// Keep translucent row fills as overlays instead of replacing alpha.
			DrawVerticalGradient(buf, row, 0, solid, solid)
		}
		return
	}
	if top.A == 0 {
		top = solid
	}
	if bottom.A == 0 {
		bottom = solid
	}
	DrawVerticalGradient(buf, row, radius, top, bottom)
}
