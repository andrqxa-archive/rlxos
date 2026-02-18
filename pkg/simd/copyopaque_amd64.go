//go:build amd64

package simd

//go:noescape
func copyBGRAOpaqueAsm(dst, src *byte, n uintptr)

func copyBGRAOpaqueFast(dst, src []byte, n int) bool {
	// For very small ranges, Go scalar path is faster than asm call overhead.
	if n < 64 {
		return false
	}
	copyBGRAOpaqueAsm(&dst[0], &src[0], uintptr(n))
	return true
}
