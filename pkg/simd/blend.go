package simd

// BlendOverBGRA alpha-blends src over dst for pixels BGRA pixels.
// opacity scales source alpha (0..255).
func BlendOverBGRA(dst, src []byte, pixels int, opacity uint8) {
	if pixels <= 0 {
		return
	}
	n := pixels * 4
	if n <= 0 {
		return
	}
	if len(dst) < n {
		n = len(dst) &^ 3
	}
	if len(src) < n {
		n = len(src) &^ 3
	}
	if n <= 0 {
		return
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
