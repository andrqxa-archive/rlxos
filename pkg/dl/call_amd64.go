//go:build amd64

package dl

import "syscall"

// Note on calling convention: syscall.RawSyscall(trap, a1, a2, a3) on Linux
// amd64 places trap→RAX, a1→RDI, a2→RSI, a3→RDX and executes SYSCALL.
// When 'trap' is a function pointer rather than a syscall number, this
// effectively performs an indirect call with up to 3 integer arguments — but
// via the SYSCALL instruction, NOT a CALL instruction.
//
// WARNING: This is a crude mechanism.  It works for simple C functions that
// only use integer/pointer arguments (puts, strlen, etc.) but is NOT safe for
// general use.  Floating-point arguments are not supported.

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

// callIFUNCResolver calls an STT_GNU_IFUNC resolver function and returns
// the resolved address.  On x86-64 the resolver takes no arguments and
// returns a function pointer in rax.
func callIFUNCResolver(addr uintptr) uintptr {
	r1, _, _ := syscall.RawSyscall(addr, 0, 0, 0)
	return r1
}
