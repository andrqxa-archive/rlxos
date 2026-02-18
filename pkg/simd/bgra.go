package simd

// CopyBGRA copies exactly pixels BGRA pixels (4 bytes each) from src to dst.
// If either slice is too small, it copies the maximum whole-pixel prefix.
func CopyBGRA(dst, src []byte, pixels int) {
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
	copy(dst[:n], src[:n])
}

// CopyBGRAOpaque copies exactly pixels BGRA pixels from src to dst
// and forces alpha to 0xFF for each copied pixel.
func CopyBGRAOpaque(dst, src []byte, pixels int) {
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
	if copyBGRAOpaqueFast(dst[:n], src[:n], n) {
		return
	}
	copy(dst[:n], src[:n])
	setAlphaOpaqueBGRA(dst[:n])
}
