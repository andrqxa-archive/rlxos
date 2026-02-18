package simd

import "testing"

var simdBenchSink byte

func BenchmarkCopyBGRA(b *testing.B) {
	const pixels = 1920 * 1080
	const bytesPerPixel = 4
	n := pixels * bytesPerPixel

	src := make([]byte, n)
	dst := make([]byte, n)
	fillPattern(src)
	fillPattern(dst)

	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CopyBGRA(dst, src, pixels)
	}
	simdBenchSink = dst[len(dst)-1]
}

func BenchmarkCopyBGRAOpaqueSIMD(b *testing.B) {
	const pixels = 1920 * 1080
	const bytesPerPixel = 4
	n := pixels * bytesPerPixel

	src := make([]byte, n)
	dst := make([]byte, n)
	fillPattern(src)
	fillPattern(dst)

	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CopyBGRAOpaque(dst, src, pixels)
	}
	simdBenchSink = dst[len(dst)-1]
}

func BenchmarkCopyBGRAOpaqueScalar(b *testing.B) {
	const pixels = 1920 * 1080
	const bytesPerPixel = 4
	n := pixels * bytesPerPixel

	src := make([]byte, n)
	dst := make([]byte, n)
	fillPattern(src)
	fillPattern(dst)

	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copyBGRAOpaqueScalar(dst, src, pixels)
	}
	simdBenchSink = dst[len(dst)-1]
}

func BenchmarkBlendOverBGRASIMD(b *testing.B) {
	const pixels = 1920 * 1080
	const bytesPerPixel = 4
	n := pixels * bytesPerPixel

	src := make([]byte, n)
	base := make([]byte, n)
	dst := make([]byte, n)
	fillPattern(src)
	fillPattern(base)

	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(dst, base)
		BlendOverBGRA(dst, src, pixels, 255)
	}
	simdBenchSink = dst[len(dst)-1]
}

func BenchmarkBlendOverBGRAScalar(b *testing.B) {
	const pixels = 1920 * 1080
	const bytesPerPixel = 4
	n := pixels * bytesPerPixel

	src := make([]byte, n)
	base := make([]byte, n)
	dst := make([]byte, n)
	fillPattern(src)
	fillPattern(base)

	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(dst, base)
		blendOverBGRAScalar(dst, src, pixels, 255)
	}
	simdBenchSink = dst[len(dst)-1]
}

func fillPattern(buf []byte) {
	for i := range buf {
		buf[i] = byte((i*17 + 31) & 0xFF)
	}
}

func copyBGRAOpaqueScalar(dst, src []byte, pixels int) {
	n := pixels * 4
	if len(dst) < n {
		n = len(dst) &^ 3
	}
	if len(src) < n {
		n = len(src) &^ 3
	}
	copy(dst[:n], src[:n])
	for i := 3; i < n; i += 4 {
		dst[i] = 0xFF
	}
}

func blendOverBGRAScalar(dst, src []byte, pixels int, opacity uint8) {
	n := pixels * 4
	if len(dst) < n {
		n = len(dst) &^ 3
	}
	if len(src) < n {
		n = len(src) &^ 3
	}

	if opacity == 255 {
		for i := 0; i < n; i += 4 {
			sa := src[i+3]
			if sa == 0 {
				continue
			}
			if sa == 255 {
				dst[i+0] = src[i+0]
				dst[i+1] = src[i+1]
				dst[i+2] = src[i+2]
				dst[i+3] = 255
				continue
			}
			a := uint32(sa)
			inv := uint32(255 - sa)
			db, dg, dr, da := uint32(dst[i+0]), uint32(dst[i+1]), uint32(dst[i+2]), uint32(dst[i+3])
			sb, sg, sr := uint32(src[i+0]), uint32(src[i+1]), uint32(src[i+2])
			dst[i+0] = uint8((sb*a + db*inv + 127) / 255)
			dst[i+1] = uint8((sg*a + dg*inv + 127) / 255)
			dst[i+2] = uint8((sr*a + dr*inv + 127) / 255)
			dst[i+3] = uint8(a + (da*inv+127)/255)
		}
		return
	}

	op := uint32(opacity)
	for i := 0; i < n; i += 4 {
		sa := src[i+3]
		if sa == 0 {
			continue
		}
		a := (uint32(sa) * op) / 255
		if a == 0 {
			continue
		}
		inv := uint32(255) - a
		db, dg, dr, da := uint32(dst[i+0]), uint32(dst[i+1]), uint32(dst[i+2]), uint32(dst[i+3])
		sb, sg, sr := uint32(src[i+0]), uint32(src[i+1]), uint32(src[i+2])
		dst[i+0] = uint8((sb*a + db*inv + 127) / 255)
		dst[i+1] = uint8((sg*a + dg*inv + 127) / 255)
		dst[i+2] = uint8((sr*a + dr*inv + 127) / 255)
		dst[i+3] = uint8(a + (da*inv+127)/255)
	}
}
