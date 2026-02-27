package dl

import "unsafe"

// ──────────────────────────────────────────────────────────────────────────────
// Calling native functions from Go
// ──────────────────────────────────────────────────────────────────────────────
//
// These helpers call void/integer/pointer-returning C functions using
// syscall.Syscall / Syscall6.  This works because syscall.Syscall on Linux
// is essentially a wrapper that sets up a C-ABI-compatible call through a
// function pointer (the first argument).
//
// IMPORTANT LIMITATIONS:
//   - Only integer/pointer arguments and return values are supported.
//   - Floating-point arguments (passed in XMM/NEON registers) are NOT
//     supported — the syscall wrappers do not set FP registers.
//   - Keep examples limited to integer/pointer signatures (puts, strlen, etc).
//   - The callee must follow the platform ABI (SysV AMD64 or AAPCS64).
//
// Memory pinning: Go's garbage collector may move objects.  When passing
// Go-allocated memory (e.g. from CString) to native code, ensure the pointer
// is not the only reference — keep a local variable alive.  In practice,
// CString returns a byte slice whose backing array won't be collected while
// the slice is reachable.

// CString allocates a NUL-terminated byte slice suitable for passing to C
// functions expecting a const char*.  The returned uintptr points to the
// first byte.  The caller must keep the returned []byte alive for the
// duration of the call (to prevent GC from collecting it).
//
// Example:
//
//	s, p := dl.CString("hello")
//	_ = s // keep alive
//	dl.Call1(putsAddr, p)
func CString(s string) ([]byte, uintptr) {
	b := make([]byte, len(s)+1)
	copy(b, s)
	b[len(s)] = 0
	return b, uintptr(unsafe.Pointer(&b[0]))
}

// CStringPtr is a convenience that returns just the pointer.  The caller
// must ensure the string stays referenced until after the native call returns.
// In practice, the Go compiler will keep the string argument alive.
func CStringPtr(s string) uintptr {
	b := append([]byte(s), 0)
	return uintptr(unsafe.Pointer(&b[0]))
}
