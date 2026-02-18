//go:build !amd64

package simd

func copyBGRAOpaqueFast(dst, src []byte, n int) bool {
	return false
}
