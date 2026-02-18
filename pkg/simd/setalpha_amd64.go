//go:build amd64

package simd

//go:noescape
func setAlphaOpaqueBGRAAsm(dst *byte, n uintptr)

func setAlphaOpaqueBGRA(dst []byte) {
	n := len(dst) &^ 3
	if n <= 0 {
		return
	}
	// For tiny spans, scalar is cheaper than an asm call.
	if n < 32 {
		for i := 3; i < n; i += 4 {
			dst[i] = 0xFF
		}
		return
	}
	setAlphaOpaqueBGRAAsm(&dst[0], uintptr(n))
}
