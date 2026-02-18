package simd

import "testing"

func TestCopyBGRAOpaque(t *testing.T) {
	src := []byte{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
	}
	dst := make([]byte, len(src))

	CopyBGRAOpaque(dst, src, 3)

	for i := 0; i < 3; i++ {
		base := i * 4
		if dst[base+0] != src[base+0] || dst[base+1] != src[base+1] || dst[base+2] != src[base+2] {
			t.Fatalf("pixel %d RGB mismatch: got %v want %v", i, dst[base:base+3], src[base:base+3])
		}
		if dst[base+3] != 0xFF {
			t.Fatalf("pixel %d alpha mismatch: got %d want 255", i, dst[base+3])
		}
	}
}

func TestCopyBGRAClampsToWholePixels(t *testing.T) {
	src := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	dst := make([]byte, 6) // only one full pixel fits

	CopyBGRA(dst, src, 2)

	if dst[0] != 1 || dst[1] != 2 || dst[2] != 3 || dst[3] != 4 {
		t.Fatalf("first pixel mismatch: got %v", dst[:4])
	}
	if dst[4] != 0 || dst[5] != 0 {
		t.Fatalf("partial pixel should stay untouched: got %v", dst[4:])
	}
}

func TestBlendOverBGRA(t *testing.T) {
	dst := []byte{
		10, 20, 30, 255,
		100, 110, 120, 255,
	}
	src := []byte{
		200, 180, 160, 0, // transparent -> unchanged
		50, 60, 70, 255, // opaque -> replace
	}

	BlendOverBGRA(dst, src, 2, 255)

	if dst[0] != 10 || dst[1] != 20 || dst[2] != 30 || dst[3] != 255 {
		t.Fatalf("transparent source should not change first pixel: %v", dst[:4])
	}
	if dst[4] != 50 || dst[5] != 60 || dst[6] != 70 || dst[7] != 255 {
		t.Fatalf("opaque source should replace second pixel: %v", dst[4:8])
	}
}

func TestBlendOverBGRAReference(t *testing.T) {
	const pixels = 257
	const n = pixels * 4

	src := make([]byte, n)
	dstA := make([]byte, n)
	dstB := make([]byte, n)
	for i := 0; i < n; i++ {
		src[i] = byte((i*37 + 11) & 0xFF)
		dstA[i] = byte((i*53 + 23) & 0xFF)
		dstB[i] = dstA[i]
	}

	refBlendOverBGRA(dstA, src, pixels, 255)
	BlendOverBGRA(dstB, src, pixels, 255)
	for i := 0; i < n; i++ {
		if dstA[i] != dstB[i] {
			t.Fatalf("opacity 255 mismatch at byte %d: got %d want %d", i, dstB[i], dstA[i])
		}
	}

	for i := 0; i < n; i++ {
		dstA[i] = byte((i*13 + 7) & 0xFF)
		dstB[i] = dstA[i]
	}
	refBlendOverBGRA(dstA, src, pixels, 180)
	BlendOverBGRA(dstB, src, pixels, 180)
	for i := 0; i < n; i++ {
		if dstA[i] != dstB[i] {
			t.Fatalf("opacity 180 mismatch at byte %d: got %d want %d", i, dstB[i], dstA[i])
		}
	}
}

func refBlendOverBGRA(dst, src []byte, pixels int, opacity uint8) {
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
