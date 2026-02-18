#include "textflag.h"

// Exception/interrupt handler stubs for x86-64.
// Each stub pushes the vector number and jumps to a common handler.
// For exceptions that don't push an error code, we push a dummy 0.

// Common interrupt frame after stub pushes:
//   [SS, RSP, RFLAGS, CS, RIP]  <- pushed by CPU
//   [error_code]                 <- pushed by CPU or stub (dummy 0)
//   [vector_number]              <- pushed by stub

// Macro-like approach: define stubs manually for each vector.
// Exceptions 0-31, vectors 8, 10-14, 17, 21, 29, 30 push error codes.

// --- Exception stubs WITHOUT error code (push dummy 0) ---

TEXT ·isr0(SB),NOSPLIT,$0   // Divide Error
	PUSHQ $0; PUSHQ $0; JMP isrCommon(SB)
TEXT ·isr1(SB),NOSPLIT,$0   // Debug
	PUSHQ $0; PUSHQ $1; JMP isrCommon(SB)
TEXT ·isr2(SB),NOSPLIT,$0   // NMI
	PUSHQ $0; PUSHQ $2; JMP isrCommon(SB)
TEXT ·isr3(SB),NOSPLIT,$0   // Breakpoint
	PUSHQ $0; PUSHQ $3; JMP isrCommon(SB)
TEXT ·isr4(SB),NOSPLIT,$0   // Overflow
	PUSHQ $0; PUSHQ $4; JMP isrCommon(SB)
TEXT ·isr5(SB),NOSPLIT,$0   // Bound Range Exceeded
	PUSHQ $0; PUSHQ $5; JMP isrCommon(SB)
TEXT ·isr6(SB),NOSPLIT,$0   // Invalid Opcode
	PUSHQ $0; PUSHQ $6; JMP isrCommon(SB)
TEXT ·isr7(SB),NOSPLIT,$0   // Device Not Available
	PUSHQ $0; PUSHQ $7; JMP isrCommon(SB)

// --- Exception stubs WITH error code (CPU pushes it) ---

TEXT ·isr8(SB),NOSPLIT,$0   // Double Fault
	PUSHQ $8; JMP isrCommon(SB)

TEXT ·isr9(SB),NOSPLIT,$0   // Coprocessor Segment Overrun (legacy)
	PUSHQ $0; PUSHQ $9; JMP isrCommon(SB)

TEXT ·isr10(SB),NOSPLIT,$0  // Invalid TSS
	PUSHQ $10; JMP isrCommon(SB)
TEXT ·isr11(SB),NOSPLIT,$0  // Segment Not Present
	PUSHQ $11; JMP isrCommon(SB)
TEXT ·isr12(SB),NOSPLIT,$0  // Stack-Segment Fault
	PUSHQ $12; JMP isrCommon(SB)
TEXT ·isr13(SB),NOSPLIT,$0  // General Protection Fault
	PUSHQ $13; JMP isrCommon(SB)
TEXT ·isr14(SB),NOSPLIT,$0  // Page Fault
	PUSHQ $14; JMP isrCommon(SB)

TEXT ·isr15(SB),NOSPLIT,$0  // Reserved
	PUSHQ $0; PUSHQ $15; JMP isrCommon(SB)
TEXT ·isr16(SB),NOSPLIT,$0  // x87 FPU Error
	PUSHQ $0; PUSHQ $16; JMP isrCommon(SB)

TEXT ·isr17(SB),NOSPLIT,$0  // Alignment Check
	PUSHQ $17; JMP isrCommon(SB)

TEXT ·isr18(SB),NOSPLIT,$0  // Machine Check
	PUSHQ $0; PUSHQ $18; JMP isrCommon(SB)
TEXT ·isr19(SB),NOSPLIT,$0  // SIMD FP Exception
	PUSHQ $0; PUSHQ $19; JMP isrCommon(SB)
TEXT ·isr20(SB),NOSPLIT,$0  // Virtualization Exception
	PUSHQ $0; PUSHQ $20; JMP isrCommon(SB)

TEXT ·isr21(SB),NOSPLIT,$0  // Control Protection Exception
	PUSHQ $21; JMP isrCommon(SB)

// Vectors 22-28: reserved, no error code
TEXT ·isr22(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $22; JMP isrCommon(SB)
TEXT ·isr23(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $23; JMP isrCommon(SB)
TEXT ·isr24(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $24; JMP isrCommon(SB)
TEXT ·isr25(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $25; JMP isrCommon(SB)
TEXT ·isr26(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $26; JMP isrCommon(SB)
TEXT ·isr27(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $27; JMP isrCommon(SB)
TEXT ·isr28(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $28; JMP isrCommon(SB)

TEXT ·isr29(SB),NOSPLIT,$0  // VMM Communication Exception
	PUSHQ $29; JMP isrCommon(SB)
TEXT ·isr30(SB),NOSPLIT,$0  // Security Exception
	PUSHQ $30; JMP isrCommon(SB)

TEXT ·isr31(SB),NOSPLIT,$0  // Reserved
	PUSHQ $0; PUSHQ $31; JMP isrCommon(SB)

// --- IRQ stubs (vectors 32-47) ---

TEXT ·irq0(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $32; JMP isrCommon(SB)
TEXT ·irq1(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $33; JMP isrCommon(SB)
TEXT ·irq2(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $34; JMP isrCommon(SB)
TEXT ·irq3(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $35; JMP isrCommon(SB)
TEXT ·irq4(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $36; JMP isrCommon(SB)
TEXT ·irq5(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $37; JMP isrCommon(SB)
TEXT ·irq6(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $38; JMP isrCommon(SB)
TEXT ·irq7(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $39; JMP isrCommon(SB)
TEXT ·irq8(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $40; JMP isrCommon(SB)
TEXT ·irq9(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $41; JMP isrCommon(SB)
TEXT ·irq10(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $42; JMP isrCommon(SB)
TEXT ·irq11(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $43; JMP isrCommon(SB)
TEXT ·irq12(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $44; JMP isrCommon(SB)
TEXT ·irq13(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $45; JMP isrCommon(SB)
TEXT ·irq14(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $46; JMP isrCommon(SB)
TEXT ·irq15(SB),NOSPLIT,$0
	PUSHQ $0; PUSHQ $47; JMP isrCommon(SB)

// --- fillISRTable: writes ISR stub addresses into ·isrAddrs Go variable ---

TEXT ·fillISRTable(SB),NOSPLIT,$0-0
	LEAQ ·isrAddrs(SB), DI
	LEAQ ·isr0(SB), AX;  MOVQ AX, 0*8(DI)
	LEAQ ·isr1(SB), AX;  MOVQ AX, 1*8(DI)
	LEAQ ·isr2(SB), AX;  MOVQ AX, 2*8(DI)
	LEAQ ·isr3(SB), AX;  MOVQ AX, 3*8(DI)
	LEAQ ·isr4(SB), AX;  MOVQ AX, 4*8(DI)
	LEAQ ·isr5(SB), AX;  MOVQ AX, 5*8(DI)
	LEAQ ·isr6(SB), AX;  MOVQ AX, 6*8(DI)
	LEAQ ·isr7(SB), AX;  MOVQ AX, 7*8(DI)
	LEAQ ·isr8(SB), AX;  MOVQ AX, 8*8(DI)
	LEAQ ·isr9(SB), AX;  MOVQ AX, 9*8(DI)
	LEAQ ·isr10(SB), AX; MOVQ AX, 10*8(DI)
	LEAQ ·isr11(SB), AX; MOVQ AX, 11*8(DI)
	LEAQ ·isr12(SB), AX; MOVQ AX, 12*8(DI)
	LEAQ ·isr13(SB), AX; MOVQ AX, 13*8(DI)
	LEAQ ·isr14(SB), AX; MOVQ AX, 14*8(DI)
	LEAQ ·isr15(SB), AX; MOVQ AX, 15*8(DI)
	LEAQ ·isr16(SB), AX; MOVQ AX, 16*8(DI)
	LEAQ ·isr17(SB), AX; MOVQ AX, 17*8(DI)
	LEAQ ·isr18(SB), AX; MOVQ AX, 18*8(DI)
	LEAQ ·isr19(SB), AX; MOVQ AX, 19*8(DI)
	LEAQ ·isr20(SB), AX; MOVQ AX, 20*8(DI)
	LEAQ ·isr21(SB), AX; MOVQ AX, 21*8(DI)
	LEAQ ·isr22(SB), AX; MOVQ AX, 22*8(DI)
	LEAQ ·isr23(SB), AX; MOVQ AX, 23*8(DI)
	LEAQ ·isr24(SB), AX; MOVQ AX, 24*8(DI)
	LEAQ ·isr25(SB), AX; MOVQ AX, 25*8(DI)
	LEAQ ·isr26(SB), AX; MOVQ AX, 26*8(DI)
	LEAQ ·isr27(SB), AX; MOVQ AX, 27*8(DI)
	LEAQ ·isr28(SB), AX; MOVQ AX, 28*8(DI)
	LEAQ ·isr29(SB), AX; MOVQ AX, 29*8(DI)
	LEAQ ·isr30(SB), AX; MOVQ AX, 30*8(DI)
	LEAQ ·isr31(SB), AX; MOVQ AX, 31*8(DI)
	LEAQ ·irq0(SB), AX;  MOVQ AX, 32*8(DI)
	LEAQ ·irq1(SB), AX;  MOVQ AX, 33*8(DI)
	LEAQ ·irq2(SB), AX;  MOVQ AX, 34*8(DI)
	LEAQ ·irq3(SB), AX;  MOVQ AX, 35*8(DI)
	LEAQ ·irq4(SB), AX;  MOVQ AX, 36*8(DI)
	LEAQ ·irq5(SB), AX;  MOVQ AX, 37*8(DI)
	LEAQ ·irq6(SB), AX;  MOVQ AX, 38*8(DI)
	LEAQ ·irq7(SB), AX;  MOVQ AX, 39*8(DI)
	LEAQ ·irq8(SB), AX;  MOVQ AX, 40*8(DI)
	LEAQ ·irq9(SB), AX;  MOVQ AX, 41*8(DI)
	LEAQ ·irq10(SB), AX; MOVQ AX, 42*8(DI)
	LEAQ ·irq11(SB), AX; MOVQ AX, 43*8(DI)
	LEAQ ·irq12(SB), AX; MOVQ AX, 44*8(DI)
	LEAQ ·irq13(SB), AX; MOVQ AX, 45*8(DI)
	LEAQ ·irq14(SB), AX; MOVQ AX, 46*8(DI)
	LEAQ ·irq15(SB), AX; MOVQ AX, 47*8(DI)
	RET

// --- Common handler: save regs, call Go dispatcher, restore, IRETQ ---

TEXT isrCommon(SB),NOSPLIT,$0
	PUSHQ AX
	PUSHQ BX
	PUSHQ CX
	PUSHQ DX
	PUSHQ SI
	PUSHQ DI
	PUSHQ BP
	PUSHQ R8
	PUSHQ R9
	PUSHQ R10
	PUSHQ R11
	PUSHQ R12
	PUSHQ R13
	PUSHQ R14
	PUSHQ R15

	// Prepare ABI0 stack args for: interruptDispatch(vector, errorCode uint64)
	// After 15 pushes (120 bytes): vector at 120(SP), error_code at 128(SP)
	MOVQ 120(SP), AX   // vector
	MOVQ 128(SP), BX   // error_code
	SUBQ $16, SP
	MOVQ AX, 0(SP)     // arg0: vector
	MOVQ BX, 8(SP)     // arg1: errorCode
	CALL ·interruptDispatch(SB)
	ADDQ $16, SP

	POPQ R15
	POPQ R14
	POPQ R13
	POPQ R12
	POPQ R11
	POPQ R10
	POPQ R9
	POPQ R8
	POPQ BP
	POPQ DI
	POPQ SI
	POPQ DX
	POPQ CX
	POPQ BX
	POPQ AX

	ADDQ $16, SP
	IRETQ
