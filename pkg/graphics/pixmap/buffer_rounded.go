package pixmap

import (
	"image"
	"image/color"
	"math"
)

// FillRoundedRect fills r with rounded corners.
func (b *Buffer) FillRoundedRect(r image.Rectangle, radius int, c color.Color) {
	if b == nil {
		return
	}
	if radius <= 0 {
		b.FillRect(r, c)
		return
	}
	radius = clampRoundedRadius(r, radius)
	x0, y0, x1, y1 := clampRectToBuffer(r, b.Width, b.Height)
	if b.clipOn {
		if x0 < b.clip.Min.X {
			x0 = b.clip.Min.X
		}
		if y0 < b.clip.Min.Y {
			y0 = b.clip.Min.Y
		}
		if x1 > b.clip.Max.X {
			x1 = b.clip.Max.X
		}
		if y1 > b.clip.Max.Y {
			y1 = b.clip.Max.Y
		}
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	n := ToNRGBA(c)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cov := roundedRectCoverage(x, y, r, radius)
			if cov >= 0.999 {
				if n.A == 255 {
					b.SetPixel(x, y, n)
				} else {
					drawCoveragePixel(b, x, y, n, 1)
				}
				continue
			}
			if cov <= 0.001 {
				continue
			}
			cov = math.Sqrt(cov)
			drawCoveragePixel(b, x, y, n, cov)
		}
	}
}

// DrawRoundedRect draws rounded rectangle outline.
func (b *Buffer) DrawRoundedRect(r image.Rectangle, radius int, c color.Color) {
	if b == nil {
		return
	}
	if radius <= 0 {
		b.DrawRect(r, c)
		return
	}
	radius = clampRoundedRadius(r, radius)
	x0, y0, x1, y1 := clampRectToBuffer(r, b.Width, b.Height)
	if b.clipOn {
		if x0 < b.clip.Min.X {
			x0 = b.clip.Min.X
		}
		if y0 < b.clip.Min.Y {
			y0 = b.clip.Min.Y
		}
		if x1 > b.clip.Max.X {
			x1 = b.clip.Max.X
		}
		if y1 > b.clip.Max.Y {
			y1 = b.clip.Max.Y
		}
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}

	inner := image.Rect(r.Min.X+1, r.Min.Y+1, r.Max.X-1, r.Max.Y-1)
	innerRadius := radius - 1
	if innerRadius < 0 {
		innerRadius = 0
	}
	n := ToNRGBA(c)

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			outer := roundedRectCoverageExact(x, y, r, radius)
			if outer <= 0.001 {
				continue
			}
			cov := outer
			if !inner.Empty() {
				innerCov := roundedRectCoverageExact(x, y, inner, innerRadius)
				cov -= innerCov
				if cov < 0 {
					cov = 0
				}
			}
			if cov >= 0.999 {
				drawCoveragePixel(b, x, y, n, 1)
				continue
			}
			if cov <= 0.001 {
				continue
			}
			drawCoveragePixel(b, x, y, n, cov)
		}
	}
}

func clampRectToBuffer(r image.Rectangle, bw, bh int) (x0, y0, x1, y1 int) {
	x0, y0 = r.Min.X, r.Min.Y
	x1, y1 = r.Max.X, r.Max.Y
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > bw {
		x1 = bw
	}
	if y1 > bh {
		y1 = bh
	}
	return x0, y0, x1, y1
}

func clampRoundedRadius(r image.Rectangle, radius int) int {
	if radius < 0 {
		return 0
	}
	if radius > r.Dx()/2 {
		radius = r.Dx() / 2
	}
	if radius > r.Dy()/2 {
		radius = r.Dy() / 2
	}
	return radius
}

func pointInRoundedRectAt(px, py float64, r image.Rectangle, radius int) bool {
	if radius <= 0 {
		return px >= float64(r.Min.X) && px < float64(r.Max.X) && py >= float64(r.Min.Y) && py < float64(r.Max.Y)
	}
	return pointInRoundedRectAtClamped(px, py, r, clampRoundedRadius(r, radius))
}

func pointInRoundedRectAtClamped(px, py float64, r image.Rectangle, radius int) bool {
	if px < float64(r.Min.X) || px >= float64(r.Max.X) || py < float64(r.Min.Y) || py >= float64(r.Max.Y) {
		return false
	}
	if radius <= 0 {
		return true
	}

	halfW := float64(r.Dx()) / 2.0
	halfH := float64(r.Dy()) / 2.0
	rr := float64(radius)
	cx := float64(r.Min.X) + halfW
	cy := float64(r.Min.Y) + halfH
	qx := absFloat(px-cx) - (halfW - rr)
	qy := absFloat(py-cy) - (halfH - rr)
	if qx < 0 {
		qx = 0
	}
	if qy < 0 {
		qy = 0
	}
	return (qx*qx + qy*qy) <= (rr * rr)
}

func roundedRectCoverage(x, y int, r image.Rectangle, radius int) float64 {
	if radius <= 0 {
		if RectContainsXY(r, x, y) {
			return 1
		}
		return 0
	}
	radius = clampRoundedRadius(r, radius)
	if radius <= 0 {
		if RectContainsXY(r, x, y) {
			return 1
		}
		return 0
	}

	px := float64(x) + 0.5
	py := float64(y) + 0.5
	sd := roundedRectSignedDistance(px, py, r, radius)
	const halfPixelDiag = 0.7071067811865476
	if sd <= -halfPixelDiag {
		return 1
	}
	if sd >= halfPixelDiag {
		return 0
	}

	const samples = 4
	step := 1.0 / float64(samples)
	offset := step / 2.0
	inside := 0
	for sy := 0; sy < samples; sy++ {
		py := float64(y) + offset + float64(sy)*step
		for sx := 0; sx < samples; sx++ {
			px := float64(x) + offset + float64(sx)*step
			if pointInRoundedRectAtClamped(px, py, r, radius) {
				inside++
			}
		}
	}
	return float64(inside) / float64(samples*samples)
}

func roundedRectCoverageExact(x, y int, r image.Rectangle, radius int) float64 {
	if radius <= 0 {
		if RectContainsXY(r, x, y) {
			return 1
		}
		return 0
	}
	radius = clampRoundedRadius(r, radius)
	if radius <= 0 {
		if RectContainsXY(r, x, y) {
			return 1
		}
		return 0
	}
	const samples = 4
	step := 1.0 / float64(samples)
	offset := step / 2.0
	inside := 0
	for sy := 0; sy < samples; sy++ {
		py := float64(y) + offset + float64(sy)*step
		for sx := 0; sx < samples; sx++ {
			px := float64(x) + offset + float64(sx)*step
			if pointInRoundedRectAtClamped(px, py, r, radius) {
				inside++
			}
		}
	}
	return float64(inside) / float64(samples*samples)
}

func roundedRectSignedDistance(px, py float64, r image.Rectangle, radius int) float64 {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return 1
	}
	if radius <= 0 {
		dx := math.Max(math.Max(float64(r.Min.X)-px, 0), px-float64(r.Max.X))
		dy := math.Max(math.Max(float64(r.Min.Y)-py, 0), py-float64(r.Max.Y))
		if dx > 0 || dy > 0 {
			return math.Hypot(dx, dy)
		}
		inside := math.Min(px-float64(r.Min.X), float64(r.Max.X)-px)
		insideY := math.Min(py-float64(r.Min.Y), float64(r.Max.Y)-py)
		if insideY < inside {
			inside = insideY
		}
		return -inside
	}

	halfW := float64(r.Dx()) / 2.0
	halfH := float64(r.Dy()) / 2.0
	rr := float64(radius)
	if rr > halfW {
		rr = halfW
	}
	if rr > halfH {
		rr = halfH
	}

	cx := float64(r.Min.X) + halfW
	cy := float64(r.Min.Y) + halfH
	qx := absFloat(px-cx) - (halfW - rr)
	qy := absFloat(py-cy) - (halfH - rr)
	ox := math.Max(qx, 0)
	oy := math.Max(qy, 0)
	outside := math.Hypot(ox, oy)
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - rr
}

func drawCoveragePixel(b *Buffer, x, y int, c color.NRGBA, coverage float64) {
	if coverage <= 0 {
		return
	}
	a := uint8(float64(c.A)*coverage + 0.5)
	if a == 0 {
		return
	}
	sc := color.NRGBA{R: c.R, G: c.G, B: c.B, A: a}
	if a == 255 {
		b.SetPixel(x, y, sc)
		return
	}
	bg := b.GetPixel(x, y)
	b.SetPixel(x, y, Blend(sc, bg))
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
