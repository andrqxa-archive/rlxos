package drm

import (
	"testing"

	graphics "avyos.dev/pkg/graphics/input"
)

var drmBenchSink byte

func BenchmarkCopyRectsToScanoutFullHD(b *testing.B) {
	const w, h = 1920, 1080
	be := &Backend{
		width:      w,
		height:     h,
		backBuffer: graphics.NewBuffer(w, h),
	}
	fill := be.backBuffer.Data
	for i := range fill {
		fill[i] = byte((i*29 + 7) & 0xFF)
	}

	dst := make([]byte, w*h*4)
	rects := []graphics.Rect{{X: 0, Y: 0, W: w, H: h}}

	b.SetBytes(int64(w * h * 4))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = be.copyRectsToScanout(dst, w*4, be.backBuffer.Stride, rects)
	}
	drmBenchSink = dst[len(dst)-1]
}

func BenchmarkCopyRectsToScanoutDamage24(b *testing.B) {
	const w, h = 1920, 1080
	be := &Backend{
		width:      w,
		height:     h,
		backBuffer: graphics.NewBuffer(w, h),
	}
	fill := be.backBuffer.Data
	for i := range fill {
		fill[i] = byte((i*13 + 11) & 0xFF)
	}

	dst := make([]byte, w*h*4)
	rects := makeDamageRects24(w, h)

	totalBytes := int64(0)
	for _, r := range rects {
		totalBytes += int64(r.W * r.H * 4)
	}
	b.SetBytes(totalBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = be.copyRectsToScanout(dst, w*4, be.backBuffer.Stride, rects)
	}
	drmBenchSink = dst[len(dst)-1]
}

func makeDamageRects24(w, h int) []graphics.Rect {
	out := make([]graphics.Rect, 0, 24)
	cellW := w / 8
	cellH := h / 3
	for row := 0; row < 3; row++ {
		for col := 0; col < 8; col++ {
			x := col * cellW
			y := row * cellH
			rw := cellW
			rh := cellH
			if col == 7 {
				rw = w - x
			}
			if row == 2 {
				rh = h - y
			}
			out = append(out, graphics.Rect{X: x, Y: y, W: rw, H: rh})
		}
	}
	return out
}
