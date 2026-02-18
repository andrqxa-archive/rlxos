#include "textflag.h"

// _entry is the kernel entry point. Limine loads our ELF and jumps here.
// We set up hardware before the Go runtime initializes so that the
// runtime's SYSCALL instructions are intercepted by our handler.
//
// Build with: go build -ldflags="-E main._entry" -o kernel.elf ./kernel/
//
// Boot sequence:
//   1. Limine → _entry (this function)
//   2. Set up GDT, IDT, SYSCALL MSRs, serial, memory
//   3. Fake argc/argv on stack
//   4. Jump to _rt0_amd64 → runtime.rt0_go → runtime init → main.main

TEXT ·_entry(SB),NOSPLIT|NOFRAME,$0
	// Limine provides us a valid stack and long mode (64-bit).
	// Interrupts are disabled.

	// CRITICAL: Do not call Go functions before runtime init.
	// Their ABI prologue expects TLS/G state to exist and can fault.
	// Only perform minimal assembly-only setup here.
	CALL ·initSerialEarly(SB)
	CALL ·enableSSEEarly(SB)
	CALL ·enableSyscallEarly(SB)

	// Bare-metal path: do not enter the Linux Go runtime.
	LEAQ ·bootMsg(SB), SI
	CALL ·serialWriteStringEarly(SB)

entry_halt:
	CLI
	HLT
	JMP entry_halt

// serialWriteStringEarly writes a NUL-terminated ASCII string at SI.
TEXT ·serialWriteStringEarly(SB),NOSPLIT|NOFRAME,$0
serial_next:
	MOVB (SI), AL
	CMPB AL, $0
	JE   serial_done

	// Wait for THR empty (LSR bit 5 set).
	MOVW $0x03FD, DX
serial_wait:
	INB
	TESTB $0x20, AL
	JE   serial_wait

	// Write byte.
	MOVW $0x03F8, DX
	MOVB (SI), AL
	OUTB

	INCQ SI
	JMP  serial_next

serial_done:
	RET

DATA ·bootMsg+0(SB)/41, $"[boot] AvyOS bare entry (no Go runtime)\r\n"
DATA ·bootMsg+41(SB)/1, $0
GLOBL ·bootMsg(SB),RODATA,$42
