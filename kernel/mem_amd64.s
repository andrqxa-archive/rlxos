#include "textflag.h"

// func memzeroAsm(addr, size uintptr)
TEXT ·memzeroAsm(SB),NOSPLIT,$0-16
	MOVQ addr+0(FP), DI
	MOVQ size+8(FP), CX
	XORQ AX, AX

	// Clear 8 bytes at a time first.
	MOVQ CX, DX
	SHRQ $3, CX
	REP; STOSQ

	// Clear trailing bytes.
	MOVQ DX, CX
	ANDQ $7, CX
	REP; STOSB
	RET
