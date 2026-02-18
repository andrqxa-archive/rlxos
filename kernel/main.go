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

// _entry is the ELF entry point, defined in rt0_amd64.s.
// It executes a bare-metal assembly path and does not enter runtime.rt0_go.
// main() is currently unused in this mode.
func _entry()

func main() {
	// Now that runtime is initialized, do platform bring-up in Go.
	initSerial()
	serialPrint("[boot] serial ready\n")
	loadGDT()
	serialPrint("[boot] gdt ready\n")
	loadIDT()
	serialPrint("[boot] idt ready\n")
	enableSyscall()
	serialPrint("[boot] syscall ready\n")
	initMem()
	serialPrint("[boot] memory ready\n")
	remapPIC()
	serialPrint("[boot] pic ready\n")

	// Ensure Limine request variables survive dead-code elimination
	keepRequests()

	// Initialize font data
	initFont()

	// Check base revision
	if baseRevision[2] != 0 {
		serialPrint("FATAL: Limine base revision not supported\n")
		halt()
	}

	serialPrint("\n=== AvyOS Kernel ===\n\n")

	// Print bootloader info
	if info := getBootloaderInfoResponse(); info != nil {
		serialPrint("Bootloader: ")
		serialPrint(cstring(info.Name))
		serialPrint(" ")
		serialPrint(cstring(info.Version))
		serialPrint("\n")
	}

	// Initialize framebuffer console
	if fbResp := getFramebufferResponse(); fbResp != nil && fbResp.FramebufferCount > 0 {
		fb := *(*LimineFramebuffer)(unsafe.Pointer(*fbResp.Framebuffers))
		_ = fb
		fbs := unsafe.Slice(fbResp.Framebuffers, fbResp.FramebufferCount)
		initConsole(fbs[0])

		conPrint("AvyOS Kernel\n")
		conPrint("============\n\n")

		conPrint("Framebuffer: ")
		conPrintDec(fbs[0].Width)
		conPrint("x")
		conPrintDec(fbs[0].Height)
		conPrint(" ")
		conPrintDec(uint64(fbs[0].Bpp))
		conPrint("bpp\n")
	} else {
		serialPrint("WARNING: No framebuffer available\n")
	}

	// Print HHDM info
	if hhdm := getHHDMResponse(); hhdm != nil {
		conPrint("HHDM offset: ")
		conPrintHex(hhdm.Offset)
		conPrint("\n")
	}

	// Print executable address
	if addr := getExecutableAddressResponse(); addr != nil {
		conPrint("Kernel phys: ")
		conPrintHex(addr.PhysicalBase)
		conPrint("\n")
		conPrint("Kernel virt: ")
		conPrintHex(addr.VirtualBase)
		conPrint("\n")
	}

	// Print memory map
	if mmResp := getMemmapResponse(); mmResp != nil {
		conPrint("\nMemory Map:\n")
		ptrs := unsafe.Slice(mmResp.Entries, mmResp.EntryCount)
		for i := uint64(0); i < mmResp.EntryCount; i++ {
			entry := ptrs[i]
			conPrint("  ")
			conPrintHex(entry.Base)
			conPrint(" - ")
			conPrintHex(entry.Base + entry.Length)
			conPrint(" ")
			conPrint(memmapTypeName(entry.Type))
			conPrint("\n")
		}
	}

	// Print heap status
	conPrint("\nHeap: ")
	conPrintHex(uint64(heapStart))
	conPrint(" - ")
	conPrintHex(uint64(heapEnd))
	conPrint(" (used: ")
	conPrintDec(uint64(heapCurrent-heapStart) / 1024)
	conPrint(" KB)\n")

	conPrint("\nKernel initialized. Halting.\n")

	// Halt the CPU
	for {
		halt()
	}
}

func memmapTypeName(t uint64) string {
	switch t {
	case LimineMemmapUsable:
		return "Usable"
	case LimineMemmapReserved:
		return "Reserved"
	case LimineMemmapACPIReclaimable:
		return "ACPI Reclaimable"
	case LimineMemmapACPINVS:
		return "ACPI NVS"
	case LimineMemmapBadMemory:
		return "Bad Memory"
	case LimineMemmapBootloaderReclaim:
		return "Bootloader Reclaimable"
	case LimineMemmapExecutableAndModule:
		return "Kernel/Modules"
	case LimineMemmapFramebuffer:
		return "Framebuffer"
	case LimineMemmapACPITables:
		return "ACPI Tables"
	default:
		return "Unknown"
	}
}
