#include "textflag.h"

// func loadGDTasm(ptr *descriptorPointer)
TEXT ·loadGDTasm(SB),NOSPLIT,$0-8
	MOVQ ptr+0(FP), AX
	LGDT (AX)
	// Reload data segment registers
	MOVW $0x10, AX   // Kernel data segment
	MOVW AX, DS
	MOVW AX, ES
	MOVW AX, SS
	MOVW $0x00, AX   // Clear FS and GS
	MOVW AX, FS
	MOVW AX, GS
	// Reload CS via far return: push CS selector, push return address, RETFQ
	PUSHQ $0x08
	LEAQ  ·loadGDTdone(SB), AX
	PUSHQ AX
	BYTE $0x48; BYTE $0xCB  // REX.W RETF

TEXT ·loadGDTdone(SB),NOSPLIT,$0-0
	RET

// func loadIDTasm(ptr *descriptorPointer)
TEXT ·loadIDTasm(SB),NOSPLIT,$0-8
	MOVQ ptr+0(FP), AX
	LIDT (AX)
	RET

// func halt()
TEXT ·halt(SB),NOSPLIT,$0-0
halt_loop:
	CLI
	HLT
	JMP halt_loop

// func rdmsr(msr uint32) uint64
TEXT ·rdmsr(SB),NOSPLIT,$0-16
	MOVL msr+0(FP), CX
	RDMSR                    // Result in EDX:EAX
	SHLQ $32, DX
	ORQ  DX, AX
	MOVQ AX, ret+8(FP)
	RET

// func wrmsr(msr uint32, val uint64)
TEXT ·wrmsr(SB),NOSPLIT,$0-16
	MOVL msr+0(FP), CX
	MOVQ val+8(FP), AX
	MOVQ AX, DX
	SHRQ $32, DX             // EDX = high 32 bits
	// EAX already has low 32 bits
	WRMSR
	RET

// func cli()
TEXT ·cli(SB),NOSPLIT,$0-0
	CLI
	RET

// func sti()
TEXT ·sti(SB),NOSPLIT,$0-0
	STI
	RET

// func getCR2() uintptr
TEXT ·getCR2(SB),NOSPLIT,$0-8
	MOVQ CR2, AX
	MOVQ AX, ret+0(FP)
	RET

// func enableSyscallEarly()
// Minimal SYSCALL setup used before Go runtime initialization.
// Uses firmware's current GDT selectors (CS=0x28, SS=0x30).
TEXT ·enableSyscallEarly(SB),NOSPLIT,$0-0
	// EFER.SCE = 1
	MOVL $0xC0000080, CX
	RDMSR
	ORL  $0x1, AX
	WRMSR

	// STAR: kernel CS base in bits 47:32, SYSRET base in bits 63:48.
	// Value: (0x28 << 32) | (0x10 << 48)
	MOVL $0xC0000081, CX
	MOVL $0x00000000, AX
	MOVL $0x00100028, DX
	WRMSR

	// LSTAR: syscall entry point
	MOVL $0xC0000082, CX
	LEAQ syscallEntry(SB), AX
	MOVQ AX, DX
	SHRQ $32, DX
	WRMSR

	// SFMASK: clear IF on syscall entry
	MOVL $0xC0000084, CX
	MOVL $0x00000200, AX
	XORL DX, DX
	WRMSR
	RET

// func enableSSEEarly()
// Enable SSE state so regabi prologues using XMM instructions do not fault.
TEXT ·enableSSEEarly(SB),NOSPLIT,$0-0
	// CR0: clear EM (bit 2), set MP (bit 1)
	MOVQ CR0, AX
	ANDQ $~(1<<2), AX
	ORQ  $(1<<1), AX
	MOVQ AX, CR0

	// Clear TS bit so FPU/SSE instructions are allowed.
	CLTS

	// CR4: OSFXSR (bit 9) + OSXMMEXCPT (bit 10)
	MOVQ CR4, AX
	ORQ  $(1<<9)|(1<<10), AX
	MOVQ AX, CR4
	RET

// func initSerialEarly()
// Configure COM1 for early runtime debug output before Go runtime is alive.
TEXT ·initSerialEarly(SB),NOSPLIT,$0-0
	// Disable interrupts.
	MOVW $0x03F9, DX
	MOVB $0x00, AL
	OUTB

	// Enable DLAB.
	MOVW $0x03FB, DX
	MOVB $0x80, AL
	OUTB

	// Divisor low/high for 115200 baud (1).
	MOVW $0x03F8, DX
	MOVB $0x01, AL
	OUTB
	MOVW $0x03F9, DX
	MOVB $0x00, AL
	OUTB

	// 8N1.
	MOVW $0x03FB, DX
	MOVB $0x03, AL
	OUTB

	// FIFO enable/clear, 14-byte threshold.
	MOVW $0x03FA, DX
	MOVB $0xC7, AL
	OUTB

	// IRQs enabled, RTS/DSR set.
	MOVW $0x03FC, DX
	MOVB $0x0B, AL
	OUTB
	RET
