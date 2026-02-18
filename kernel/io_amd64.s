#include "textflag.h"

// func Outb(port uint16, val uint8)
TEXT ·Outb(SB),NOSPLIT,$0-3
	MOVW port+0(FP), DX
	MOVB val+2(FP), AL
	OUTB
	RET

// func Inb(port uint16) uint8
TEXT ·Inb(SB),NOSPLIT,$0-9
	MOVW port+0(FP), DX
	INB
	MOVB AL, ret+8(FP)
	RET

// func Outw(port uint16, val uint16)
TEXT ·Outw(SB),NOSPLIT,$0-4
	MOVW port+0(FP), DX
	MOVW val+2(FP), AX
	OUTW
	RET

// func Inw(port uint16) uint16
TEXT ·Inw(SB),NOSPLIT,$0-10
	MOVW port+0(FP), DX
	INW
	MOVW AX, ret+8(FP)
	RET

// func Outl(port uint16, val uint32)
TEXT ·Outl(SB),NOSPLIT,$0-8
	MOVW port+0(FP), DX
	MOVL val+4(FP), AX
	OUTL
	RET

// func Inl(port uint16) uint32
TEXT ·Inl(SB),NOSPLIT,$0-12
	MOVW port+0(FP), DX
	INL
	MOVL AX, ret+8(FP)
	RET

// func IoWait()
// Short I/O delay by writing to unused port 0x80
TEXT ·IoWait(SB),NOSPLIT,$0-0
	MOVB $0, AL
	MOVW $0x80, DX
	OUTB
	RET
