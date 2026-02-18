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

// isrAddrs holds raw code addresses of ISR stubs (vectors 0-47).
// Filled by fillISRTable() in assembly.
var isrAddrs [48]uintptr

// Assembly functions
func fillISRTable()
func cli()
func sti()
func getCR2() uintptr

// installExceptionHandlers populates IDT entries 0-31 with exception stubs.
//
//go:nosplit
func installExceptionHandlers() {
	fillISRTable()
	for i := 0; i < 32; i++ {
		if isrAddrs[i] != 0 {
			setIDTEntry(i, isrAddrs[i], idtInterruptGate)
		}
	}
}

// installIRQHandlers populates IDT entries 32-47 with IRQ stubs.
//
//go:nosplit
func installIRQHandlers() {
	for i := 32; i < 48; i++ {
		if isrAddrs[i] != 0 {
			setIDTEntry(i, isrAddrs[i], idtInterruptGate)
		}
	}
}

// Exception name strings for debug output
var exceptionNames = [32]string{
	"Divide Error", "Debug", "NMI", "Breakpoint",
	"Overflow", "Bound Range", "Invalid Opcode", "Device N/A",
	"Double Fault", "Coproc Seg", "Invalid TSS", "Seg Not Present",
	"Stack Fault", "General Protection", "Page Fault", "Reserved",
	"x87 FPU", "Alignment Check", "Machine Check", "SIMD FP",
	"Virtualization", "Control Prot", "", "", "", "", "", "", "",
	"VMM Comm", "Security", "",
}

// interruptDispatch is called from the assembly common handler.
//
//go:nosplit
//go:noinline
func interruptDispatch(vector, errorCode uint64) {
	if vector < 32 {
		serialPrint("\n!!! EXCEPTION: ")
		if vector < uint64(len(exceptionNames)) {
			serialPrint(exceptionNames[vector])
		}
		serialPrint(" (vector=")
		serialPrintDec(vector)
		serialPrint(", error=")
		serialPrintHex(errorCode)
		serialPrint(")\n")

		if vector == 14 {
			cr2 := getCR2()
			serialPrint("  CR2=")
			serialPrintHex(uint64(cr2))
			serialPrint("\n")
		}

		// Halt on fatal exceptions
		serialPrint("  HALTING.\n")
		halt()
	} else if vector >= 32 && vector < 48 {
		// Hardware IRQ — send EOI to PIC
		if vector >= 40 {
			Outb(0xA0, 0x20)
		}
		Outb(0x20, 0x20)
	}
}

// Ensure the idt variable is accessible from assembly via ·idt symbol.
var _ = unsafe.Sizeof(idt)
