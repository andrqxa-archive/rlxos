#include "textflag.h"

DATA ·alphaMask16+0(SB)/8, $0xFF000000FF000000
DATA ·alphaMask16+8(SB)/8, $0xFF000000FF000000
GLOBL ·alphaMask16(SB), RODATA, $16

// func setAlphaOpaqueBGRAAsm(dst *byte, n uintptr)
TEXT ·setAlphaOpaqueBGRAAsm(SB), NOSPLIT, $0-16
	MOVQ dst+0(FP), DI
	MOVQ n+8(FP), CX

	CMPQ CX, $16
	JL scalar

	MOVOU ·alphaMask16(SB), X0

simd_loop:
	CMPQ CX, $16
	JL scalar
	MOVOU (DI), X1
	POR X0, X1
	MOVOU X1, (DI)
	ADDQ $16, DI
	SUBQ $16, CX
	JMP simd_loop

scalar:
	CMPQ CX, $4
	JL done
scalar_loop:
	MOVB $0xFF, 3(DI)
	ADDQ $4, DI
	SUBQ $4, CX
	CMPQ CX, $4
	JGE scalar_loop

done:
	RET

// func copyBGRAOpaqueAsm(dst, src *byte, n uintptr)
TEXT ·copyBGRAOpaqueAsm(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX

	CMPQ CX, $16
	JL copy_scalar

	MOVOU ·alphaMask16(SB), X0

copy_simd_loop:
	CMPQ CX, $16
	JL copy_scalar
	MOVOU (SI), X1
	POR X0, X1
	MOVOU X1, (DI)
	ADDQ $16, SI
	ADDQ $16, DI
	SUBQ $16, CX
	JMP copy_simd_loop

copy_scalar:
	CMPQ CX, $4
	JL copy_done
copy_scalar_loop:
	MOVL (SI), AX
	ORL $0xFF000000, AX
	MOVL AX, (DI)
	ADDQ $4, SI
	ADDQ $4, DI
	SUBQ $4, CX
	CMPQ CX, $4
	JGE copy_scalar_loop

copy_done:
	RET
