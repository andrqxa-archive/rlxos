package core

import "math"

// FillRoundedRect fills a rectangle with rounded corners.
func (b *Buffer) FillRoundedRect(r Rect, radius int, c Color) {
	if radius <= 0 {
		b.FillRect(r, c)
		return
	}
	radius = clampRoundedRadius(r, radius)
	x0, y0, x1, y1 := clampRectToBuffer(r, b.Width, b.Height)
	if b.clipOn {
		if x0 < b.clip.X {
			x0 = b.clip.X
		}
		if y0 < b.clip.Y {
			y0 = b.clip.Y
		}
		clipX1 := b.clip.X + b.clip.W
		clipY1 := b.clip.Y + b.clip.H
		if x1 > clipX1 {
			x1 = clipX1
		}
		if y1 > clipY1 {
			y1 = clipY1
		}
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cov := roundedRectCoverage(x, y, r, radius)
			if cov >= 0.999 {
				if c.A == 255 {
					b.SetPixel(x, y, c)
				} else {
					drawCoveragePixel(b, x, y, c, 1)
				}
				continue
			}
			if cov <= 0.001 {
				continue
			}
			cov = math.Sqrt(cov)
			drawCoveragePixel(b, x, y, c, cov)
		}
	}
}

// DrawRoundedRect draws a rounded rectangle outline.
func (b *Buffer) DrawRoundedRect(r Rect, radius int, c Color) {
	if radius <= 0 {
		b.DrawRect(r, c)
		return
	}
	radius = clampRoundedRadius(r, radius)
	x0, y0, x1, y1 := clampRectToBuffer(r, b.Width, b.Height)
	if b.clipOn {
		if x0 < b.clip.X {
			x0 = b.clip.X
		}
		if y0 < b.clip.Y {
			y0 = b.clip.Y
		}
		clipX1 := b.clip.X + b.clip.W
		clipY1 := b.clip.Y + b.clip.H
		if x1 > clipX1 {
			x1 = clipX1
		}
		if y1 > clipY1 {
			y1 = clipY1
		}
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}

	inner := Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}
	innerRadius := radius - 1
	if innerRadius < 0 {
		innerRadius = 0
	}

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			outer := roundedRectCoverageExact(x, y, r, radius)
			if outer <= 0.001 {
				continue
			}
			cov := outer
			if inner.W > 0 && inner.H > 0 {
				innerCov := roundedRectCoverageExact(x, y, inner, innerRadius)
				cov -= innerCov
				if cov < 0 {
					cov = 0
				}
			}
			if cov >= 0.999 {
				drawCoveragePixel(b, x, y, c, 1)
				continue
			}
			if cov <= 0.001 {
				continue
			}
			drawCoveragePixel(b, x, y, c, cov)
		}
	}
}

func clampRectToBuffer(r Rect, bw, bh int) (x0, y0, x1, y1 int) {
	x0, y0 = r.X, r.Y
	x1, y1 = r.X+r.W, r.Y+r.H
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

func clampRoundedRadius(r Rect, radius int) int {
	if radius < 0 {
		return 0
	}
	if radius > r.W/2 {
		radius = r.W / 2
	}
	if radius > r.H/2 {
		radius = r.H / 2
	}
	return radius
}

func pointInRoundedRectSample(x, y int, r Rect, radius int) bool {
	return pointInRoundedRectAt(float64(x)+0.5, float64(y)+0.5, r, radius)
}

func pointInRoundedRectAt(px, py float64, r Rect, radius int) bool {
	if radius <= 0 {
		if px < float64(r.X) || px >= float64(r.X+r.W) || py < float64(r.Y) || py >= float64(r.Y+r.H) {
			return false
		}
		return true
	}
	return pointInRoundedRectAtClamped(px, py, r, clampRoundedRadius(r, radius))
}

func pointInRoundedRectAtClamped(px, py float64, r Rect, radius int) bool {
	if px < float64(r.X) || px >= float64(r.X+r.W) || py < float64(r.Y) || py >= float64(r.Y+r.H) {
		return false
	}
	if radius <= 0 {
		return true
	}

	halfW := float64(r.W) / 2.0
	halfH := float64(r.H) / 2.0
	rr := float64(radius)

	cx := float64(r.X) + halfW
	cy := float64(r.Y) + halfH
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

func roundedRectCoverage(x, y int, r Rect, radius int) float64 {
	if radius <= 0 {
		if r.ContainsXY(x, y) {
			return 1
		}
		return 0
	}
	radius = clampRoundedRadius(r, radius)
	if radius <= 0 {
		if r.ContainsXY(x, y) {
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

func roundedRectCoverageExact(x, y int, r Rect, radius int) float64 {
	if radius <= 0 {
		if r.ContainsXY(x, y) {
			return 1
		}
		return 0
	}
	radius = clampRoundedRadius(r, radius)
	if radius <= 0 {
		if r.ContainsXY(x, y) {
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

func roundedRectSignedDistance(px, py float64, r Rect, radius int) float64 {
	if r.W <= 0 || r.H <= 0 {
		return 1
	}
	if radius <= 0 {
		dx := math.Max(math.Max(float64(r.X)-px, 0), px-float64(r.X+r.W))
		dy := math.Max(math.Max(float64(r.Y)-py, 0), py-float64(r.Y+r.H))
		if dx > 0 || dy > 0 {
			return math.Hypot(dx, dy)
		}
		inside := math.Min(px-float64(r.X), float64(r.X+r.W)-px)
		insideY := math.Min(py-float64(r.Y), float64(r.Y+r.H)-py)
		if insideY < inside {
			inside = insideY
		}
		return -inside
	}

	halfW := float64(r.W) / 2.0
	halfH := float64(r.H) / 2.0
	rr := float64(radius)
	if rr > halfW {
		rr = halfW
	}
	if rr > halfH {
		rr = halfH
	}

	cx := float64(r.X) + halfW
	cy := float64(r.Y) + halfH
	qx := absFloat(px-cx) - (halfW - rr)
	qy := absFloat(py-cy) - (halfH - rr)
	ox := math.Max(qx, 0)
	oy := math.Max(qy, 0)
	outside := math.Hypot(ox, oy)
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - rr
}

func drawCoveragePixel(b *Buffer, x, y int, c Color, coverage float64) {
	if coverage <= 0 {
		return
	}
	a := uint8(float64(c.A)*coverage + 0.5)
	if a == 0 {
		return
	}
	sc := Color{R: c.R, G: c.G, B: c.B, A: a}
	if a == 255 {
		b.SetPixel(x, y, sc)
		return
	}
	bg := b.GetPixel(x, y)
	b.SetPixel(x, y, sc.Blend(bg))
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
