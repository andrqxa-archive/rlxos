//go:build !amd64

package simd

func setAlphaOpaqueBGRA(dst []byte) {
	n := len(dst) &^ 3
	for i := 3; i < n; i += 4 {
		dst[i] = 0xFF
	}
}
