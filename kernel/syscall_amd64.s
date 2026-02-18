#include "textflag.h"

// SYSCALL handler for x86-64.
//
// The Go runtime uses SYSCALL for all Linux system calls.
// On SYSCALL entry (done by CPU):
//   RCX = return RIP, R11 = saved RFLAGS
//   RSP unchanged (no ring transition)
//   IF cleared (by SFMASK)
//
// Linux syscall ABI:
//   RAX = number, RDI/RSI/RDX/R10/R8/R9 = args
//   Returns in RAX (-errno on error)
//
// Go runtime knows SYSCALL clobbers RCX and R11, so we
// only need to preserve the other registers and put the
// return value in RAX. We return via STI + JMP RCX.

// func syscallEntrypoint() uintptr
TEXT ·syscallEntrypoint(SB),NOSPLIT,$0-8
	LEAQ syscallEntry(SB), AX
	MOVQ AX, ret+0(FP)
	RET

TEXT syscallEntry(SB),NOSPLIT|NOFRAME,$0
	// Save return address (RCX is clobbered by our work)
	MOVQ CX, ·sysRetAddr(SB)

	// Early bootstrap fast-path: runtime may issue arch_prctl(ARCH_SET_FS)
	// before Go TLS state is fully usable. Handle it without calling Go.
	CMPQ AX, $158          // arch_prctl
	JNE  syscall_slow
	CMPQ DI, $0x1002       // ARCH_SET_FS
	JE   syscall_setfs
	CMPQ DI, $0x1003       // ARCH_GET_FS
	JE   syscall_getfs

syscall_slow:
	// Save caller-saved registers we'll clobber
	PUSHQ BX
	PUSHQ BP
	PUSHQ R12
	PUSHQ R13
	PUSHQ R14
	PUSHQ R15

	// Store syscall number and args into globals
	MOVQ AX, ·sysNum(SB)
	MOVQ DI, ·sysArgs+0(SB)
	MOVQ SI, ·sysArgs+8(SB)
	MOVQ DX, ·sysArgs+16(SB)
	MOVQ R10, ·sysArgs+24(SB)
	MOVQ R8, ·sysArgs+32(SB)
	MOVQ R9, ·sysArgs+40(SB)

	// Call Go dispatcher (reads/writes globals)
	CALL ·handleSyscall(SB)

	// Return value
	MOVQ ·sysRet(SB), AX

	// Restore callee-saved registers
	POPQ R15
	POPQ R14
	POPQ R13
	POPQ R12
	POPQ BP
	POPQ BX

	// Restore return address
	MOVQ ·sysRetAddr(SB), CX

	// Re-enable interrupts (SFMASK cleared IF on entry)
	STI

	// Return to caller
	JMP CX

syscall_setfs:
	MOVL $0xC0000100, CX   // IA32_FS_BASE
	MOVQ SI, AX
	MOVQ AX, DX
	SHRQ $32, DX
	WRMSR
	XORQ AX, AX
	MOVQ ·sysRetAddr(SB), CX
	STI
	JMP CX

syscall_getfs:
	MOVL $0xC0000100, CX   // IA32_FS_BASE
	RDMSR
	SHLQ $32, DX
	ORQ  DX, AX
	MOVQ AX, 0(SI)
	XORQ AX, AX
	MOVQ ·sysRetAddr(SB), CX
	STI
	JMP CX
