//go:build arm64

package dl

import "syscall"

// Note on calling convention: syscall.RawSyscall(trap, a1, a2, a3) on Linux
// arm64 places trap→X8, a1→X0, a2→X1, a3→X2 and executes SVC #0.
// WARNING: Same caveats as amd64 — this is crude and only works for simple
// integer/pointer-argument functions.

// Call0 calls a native void(*)(void) function at addr.
func Call0(addr uintptr) uintptr {
	r1, _, _ := syscall.RawSyscall(addr, 0, 0, 0)
	return r1
}

// Call1 calls a native function with 1 integer/pointer argument.
func Call1(addr uintptr, a1 uintptr) uintptr {
	r1, _, _ := syscall.RawSyscall(addr, a1, 0, 0)
	return r1
}

// Call2 calls a native function with 2 integer/pointer arguments.
func Call2(addr uintptr, a1, a2 uintptr) uintptr {
	r1, _, _ := syscall.RawSyscall(addr, a1, a2, 0)
	return r1
}

// Call3 calls a native function with 3 integer/pointer arguments.
func Call3(addr uintptr, a1, a2, a3 uintptr) uintptr {
	r1, _, _ := syscall.RawSyscall(addr, a1, a2, a3)
	return r1
}

// Call6 calls a native function with up to 6 integer/pointer arguments.
func Call6(addr uintptr, a1, a2, a3, a4, a5, a6 uintptr) uintptr {
	r1, _, _ := syscall.RawSyscall6(addr, a1, a2, a3, a4, a5, a6)
	return r1
}

// callIFUNCResolver calls an STT_GNU_IFUNC resolver.  On AArch64, the
// resolver may take (hwcap, hwcap2) and returns a function pointer.
func callIFUNCResolver(addr uintptr) uintptr {
	r1, _, _ := syscall.RawSyscall(addr, 0, 0, 0)
	return r1
}
