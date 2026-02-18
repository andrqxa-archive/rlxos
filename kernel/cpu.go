/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import "unsafe"

// MSR addresses
const (
	msrEFER      = 0xC0000080 // Extended Feature Enable Register
	msrSTAR      = 0xC0000081 // SYSCALL target address
	msrLSTAR     = 0xC0000082 // Long mode SYSCALL target
	msrCSTAR     = 0xC0000083 // Compatibility mode SYSCALL target
	msrSFMASK    = 0xC0000084 // SYSCALL flag mask
	msrFSBASE    = 0xC0000100 // FS base address
	msrGSBASE    = 0xC0000101 // GS base address
	msrKGSBASE   = 0xC0000102 // Kernel GS base (for SWAPGS)

	eferSCE = 1 << 0 // SYSCALL/SYSRET enable bit
)

// GDT segment selectors
const (
	gdtNull       = 0x00
	gdtKernelCode = 0x08 // Ring 0 64-bit code
	gdtKernelData = 0x10 // Ring 0 data
	gdtUserData   = 0x18 // Ring 3 data (needed for SYSRET ordering)
	gdtUserCode   = 0x20 // Ring 3 64-bit code
	gdtTSS        = 0x28 // TSS descriptor (16 bytes for 64-bit TSS)
	gdtEntries    = 7    // null + kcode + kdata + udata + ucode + tss(lo) + tss(hi)
)

// GDT descriptor entry (8 bytes)
type gdtEntry uint64

// GDTR/IDTR pointer structure (10 bytes: 2-byte limit + 8-byte base)
type descriptorPointer struct {
	Limit uint16
	Base  uint64
}

// IDT gate entry (16 bytes for 64-bit mode)
type idtEntry struct {
	OffsetLow  uint16
	Selector   uint16
	IST        uint8 // IST index (bits 0-2), rest reserved
	TypeAttr   uint8 // gate type + DPL + present
	OffsetMid  uint16
	OffsetHigh uint32
	Reserved   uint32
}

const (
	idtEntryCount = 256

	// Gate types
	idtInterruptGate = 0x8E // Present, DPL=0, 64-bit interrupt gate
	idtTrapGate      = 0x8F // Present, DPL=0, 64-bit trap gate
	idtUserIntGate   = 0xEE // Present, DPL=3, 64-bit interrupt gate
)

// Global tables (static allocation, no heap needed at init time)
var (
	gdt     [gdtEntries]gdtEntry
	gdtPtr  descriptorPointer
	idt     [idtEntryCount]idtEntry
	idtPtr  descriptorPointer
)

// makeGDTEntry constructs a GDT segment descriptor.
//
//go:nosplit
func makeGDTEntry(base uint32, limit uint32, access uint8, flags uint8) gdtEntry {
	var e uint64
	e |= uint64(limit & 0xFFFF)
	e |= uint64(base&0xFFFF) << 16
	e |= uint64((base>>16)&0xFF) << 32
	e |= uint64(access) << 40
	e |= uint64((limit>>16)&0x0F) << 48
	e |= uint64(flags&0x0F) << 52
	e |= uint64((base>>24)&0xFF) << 56
	return gdtEntry(e)
}

// initGDT sets up the Global Descriptor Table for 64-bit mode.
//
//go:nosplit
func initGDT() {
	gdt[0] = 0                                                     // Null descriptor
	gdt[1] = makeGDTEntry(0, 0xFFFFF, 0x9A, 0x0A)                // Kernel code: 64-bit, execute/read
	gdt[2] = makeGDTEntry(0, 0xFFFFF, 0x92, 0x0C)                // Kernel data: read/write
	gdt[3] = makeGDTEntry(0, 0xFFFFF, 0xF2, 0x0C)                // User data: read/write, DPL=3
	gdt[4] = makeGDTEntry(0, 0xFFFFF, 0xFA, 0x0A)                // User code: 64-bit, execute/read, DPL=3
	// TSS entries (5,6) will be set up later when needed

	gdtPtr.Limit = uint16(unsafe.Sizeof(gdt) - 1)
	gdtPtr.Base = uint64(uintptr(unsafe.Pointer(&gdt[0])))
}

// setIDTEntry configures an IDT entry for a given interrupt vector.
//
//go:nosplit
func setIDTEntry(vector int, handler uintptr, typeAttr uint8) {
	idt[vector].OffsetLow = uint16(handler)
	idt[vector].Selector = gdtKernelCode
	idt[vector].IST = 0
	idt[vector].TypeAttr = typeAttr
	idt[vector].OffsetMid = uint16(handler >> 16)
	idt[vector].OffsetHigh = uint32(handler >> 32)
	idt[vector].Reserved = 0
}

// initIDT sets up the Interrupt Descriptor Table.
//
//go:nosplit
func initIDT() {
	// Install exception handlers (0-31)
	installExceptionHandlers()

	// Install IRQ handlers (32-47) for future PIC/APIC use
	installIRQHandlers()

	idtPtr.Limit = uint16(unsafe.Sizeof(idt) - 1)
	idtPtr.Base = uint64(uintptr(unsafe.Pointer(&idt[0])))
}

// enableSyscallInstruction enables the SYSCALL/SYSRET mechanism via MSRs.
// This allows the Go runtime's syscall instruction to be intercepted.
//
//go:nosplit
func enableSyscallInstruction() {
	// Enable SCE (SYSCALL Enable) bit in EFER MSR
	efer := rdmsr(msrEFER)
	wrmsr(msrEFER, efer|eferSCE)

	// STAR MSR layout:
	//   bits 47:32 = SYSCALL CS/SS base (kernel): CS=base, SS=base+8
	//   bits 63:48 = SYSRET CS/SS base (return): SS=base+8, CS64=base+16
	// SYSCALL entry: CS=0x08 (kcode), SS=0x10 (kdata)
	// We don't use SYSRET (we use IRETQ to return), but set it for completeness.
	star := uint64(gdtKernelCode)<<32 | uint64(0x10)<<48
	wrmsr(msrSTAR, star)

	// LSTAR: SYSCALL entry point for 64-bit mode
	wrmsr(msrLSTAR, uint64(syscallEntrypoint()))

	// SFMASK: Clear IF (interrupt flag) on SYSCALL entry
	wrmsr(msrSFMASK, 0x200) // Bit 9 = IF
}

// Assembly-implemented functions
func loadGDTasm(ptr *descriptorPointer)
func loadGDTdone()
func loadIDTasm(ptr *descriptorPointer)
func halt()
func rdmsr(msr uint32) uint64
func wrmsr(msr uint32, val uint64)
func syscallEntrypoint() uintptr

// loadGDT initializes and loads the GDT.
// Called from assembly during early boot.
//
//go:nosplit
func loadGDT() {
	initGDT()
	loadGDTasm(&gdtPtr)
}

// loadIDT initializes and loads the IDT.
// Called from assembly during early boot.
//
//go:nosplit
func loadIDT() {
	initIDT()
	loadIDTasm(&idtPtr)
}

// enableSyscall sets up the SYSCALL/SYSRET MSRs.
// Called from assembly during early boot.
//
//go:nosplit
func enableSyscall() {
	enableSyscallInstruction()
}
