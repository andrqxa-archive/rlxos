package renderer

import (
	"image"
	"math"

	core "avyos.dev/pkg/graphics/pixmap"
)

func RoundedRectOutsideDistance(px, py float64, r image.Rectangle, radius int) float64 {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return 0
	}
	rr := float64(radius)
	halfW := float64(r.Dx()) / 2.0
	halfH := float64(r.Dy()) / 2.0
	if rr < 0 {
		rr = 0
	}
	if rr > halfW {
		rr = halfW
	}
	if rr > halfH {
		rr = halfH
	}

	cx := float64(r.Min.X) + halfW
	cy := float64(r.Min.Y) + halfH
	x := math.Abs(px-cx) - (halfW - rr)
	y := math.Abs(py-cy) - (halfH - rr)

	qx := math.Max(x, 0)
	qy := math.Max(y, 0)
	outside := math.Hypot(qx, qy)
	inside := math.Min(math.Max(x, y), 0)
	sdf := outside + inside - rr
	if sdf < 0 {
		return 0
	}
	return sdf
}

func PointInRoundedRect(x, y int, r image.Rectangle, radius int) bool {
	return PointInRoundedRectAt(float64(x)+0.5, float64(y)+0.5, r, radius)
}

func PointInRoundedRectAt(px, py float64, r image.Rectangle, radius int) bool {
	if radius <= 0 {
		if px < float64(r.Min.X) || px >= float64(r.Min.X+r.Dx()) || py < float64(r.Min.Y) || py >= float64(r.Min.Y+r.Dy()) {
			return false
		}
		return true
	}
	return PointInRoundedRectAtClamped(px, py, r, ClampRoundedRadius(r, radius))
}

func PointInRoundedRectAtClamped(px, py float64, r image.Rectangle, radius int) bool {
	if px < float64(r.Min.X) || px >= float64(r.Min.X+r.Dx()) || py < float64(r.Min.Y) || py >= float64(r.Min.Y+r.Dy()) {
		return false
	}
	if radius <= 0 {
		return true
	}

	rr := float64(radius)
	halfW := float64(r.Dx()) / 2.0
	halfH := float64(r.Dy()) / 2.0
	if rr > halfW {
		rr = halfW
	}
	if rr > halfH {
		rr = halfH
	}

	cx := float64(r.Min.X) + halfW
	cy := float64(r.Min.Y) + halfH
	qx := math.Abs(px-cx) - (halfW - rr)
	qy := math.Abs(py-cy) - (halfH - rr)
	if qx < 0 {
		qx = 0
	}
	if qy < 0 {
		qy = 0
	}
	return (qx*qx + qy*qy) <= (rr * rr)
}

func RoundedRectCoverage(x, y int, r image.Rectangle, radius int) float64 {
	if radius <= 0 {
		if core.RectContainsXY(r, x, y) {
			return 1
		}
		return 0
	}
	radius = ClampRoundedRadius(r, radius)
	if radius <= 0 {
		if core.RectContainsXY(r, x, y) {
			return 1
		}
		return 0
	}

	px := float64(x) + 0.5
	py := float64(y) + 0.5
	sd := RoundedRectSignedDistance(px, py, r, radius)
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
			if PointInRoundedRectAtClamped(px, py, r, radius) {
				inside++
			}
		}
	}
	return float64(inside) / float64(samples*samples)
}

func ClampRoundedRadius(r image.Rectangle, radius int) int {
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

func RoundedRectSignedDistance(px, py float64, r image.Rectangle, radius int) float64 {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return 1
	}
	if radius <= 0 {
		dx := math.Max(math.Max(float64(r.Min.X)-px, 0), px-float64(r.Min.X+r.Dx()))
		dy := math.Max(math.Max(float64(r.Min.Y)-py, 0), py-float64(r.Min.Y+r.Dy()))
		if dx > 0 || dy > 0 {
			return math.Hypot(dx, dy)
		}
		inside := math.Min(px-float64(r.Min.X), float64(r.Min.X+r.Dx())-px)
		insideY := math.Min(py-float64(r.Min.Y), float64(r.Min.Y+r.Dy())-py)
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
	qx := math.Abs(px-cx) - (halfW - rr)
	qy := math.Abs(py-cy) - (halfH - rr)
	ox := math.Max(qx, 0)
	oy := math.Max(qy, 0)
	outside := math.Hypot(ox, oy)
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - rr
}
