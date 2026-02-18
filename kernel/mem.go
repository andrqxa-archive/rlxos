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

const pageSize = 4096

func memzeroAsm(addr, size uintptr)

// Bump allocator state.
// Initialized before Go runtime starts (in initMem, called from assembly).
// All addresses are virtual (HHDM-mapped) so the Go runtime can use them directly.
var (
	heapStart   uintptr
	heapCurrent uintptr
	heapEnd     uintptr
	brkCurrent  uintptr
	hhdmOffset  uintptr // Limine HHDM offset: virtual = hhdmOffset + physical

	// Fallback heap used before Limine memmap init runs in main().
	// Must be large enough for early Go runtime reservations.
	earlyHeap [256 << 20]byte
)

const (
	// Provisional early heap window if Limine memmap is not yet available.
	// This assumes a typical QEMU memory size (>= 2 GiB) and avoids low-memory
	// regions used by firmware/bootloader/kernel during runtime bootstrap.
	earlyPhysHeapStart = uintptr(1 << 30)   // 1 GiB
	earlyPhysHeapSize  = uintptr(768 << 20) // 768 MiB
)

// ptrOf converts a uintptr to unsafe.Pointer.
//
//go:nosplit
func ptrOf(addr uintptr) unsafe.Pointer {
	return unsafe.Pointer(addr)
}

// initMem parses the Limine memory map and sets up the bump allocator.
// Called from assembly before Go runtime init. Must be //go:nosplit, no alloc.
//
//go:nosplit
func initMem() {
	// Get HHDM offset first — needed to convert physical → virtual addresses.
	// Limine maps all physical memory at hhdmOffset + physAddr.
	hhdmResp := getHHDMResponse()
	if hhdmResp != nil {
		hhdmOffset = uintptr(hhdmResp.Offset)
	}

	serialPrint("mem: HHDM offset ")
	serialPrintHex(uint64(hhdmOffset))
	serialPrint("\n")

	resp := getMemmapResponse()
	if resp == nil {
		serialPrint("mem: no memory map!\n")
		return
	}

	// Find the largest USABLE region for our heap.
	// Iterate via raw pointer arithmetic (no allocations).
	var bestBase, bestLen uint64
	bestBase, bestLen = findLargestUsableMemmapRange(resp)

	if bestLen == 0 {
		serialPrint("mem: no usable memory!\n")
		return
	}

	// Align base up to page boundary, then convert to virtual via HHDM.
	physStart := uintptr((bestBase + pageSize - 1) &^ (pageSize - 1))
	physEnd := uintptr(bestBase + bestLen)

	heapStart = hhdmOffset + physStart
	heapCurrent = heapStart
	heapEnd = hhdmOffset + physEnd
	brkCurrent = heapStart

	serialPrint("mem: heap phys ")
	serialPrintHex(uint64(physStart))
	serialPrint(" - ")
	serialPrintHex(uint64(physEnd))
	serialPrint("\n")
	serialPrint("mem: heap virt ")
	serialPrintHex(uint64(heapStart))
	serialPrint(" - ")
	serialPrintHex(uint64(heapEnd))
	serialPrint(" (")
	serialPrintDec((uint64(heapEnd) - uint64(heapStart)) / (1024 * 1024))
	serialPrint(" MB)\n")
}

// findLargestUsableMemmapRange scans Limine memmap and returns the largest
// usable physical range.
//
//go:nosplit
func findLargestUsableMemmapRange(resp *LimineMemmapResponse) (uint64, uint64) {
	var bestBase, bestLen uint64
	ptrs := (*[256]*LimineMemmapEntry)(unsafe.Pointer(resp.Entries))
	for i := uint64(0); i < resp.EntryCount && i < 256; i++ {
		entry := ptrs[i]
		if entry.Type == LimineMemmapUsable && entry.Length > bestLen {
			bestBase = entry.Base
			bestLen = entry.Length
		}
	}
	return bestBase, bestLen
}

// ensureHeapInitialized provisions a temporary heap for very early runtime
// syscalls before initMem() has parsed the Limine memmap.
//
//go:nosplit
func ensureHeapInitialized() {
	if heapEnd > heapStart {
		return
	}

	// Prefer a real Limine-provided heap even before main() runs.
	if hhdmResp := getHHDMResponse(); hhdmResp != nil {
		hhdmOffset = uintptr(hhdmResp.Offset)
		if resp := getMemmapResponse(); resp != nil {
			if bestBase, bestLen := findLargestUsableMemmapRange(resp); bestLen != 0 {
				physStart := uintptr((bestBase + pageSize - 1) &^ (pageSize - 1))
				physEnd := uintptr(bestBase + bestLen)
				heapStart = hhdmOffset + physStart
				heapCurrent = heapStart
				heapEnd = hhdmOffset + physEnd
				brkCurrent = heapStart
				return
			}
		}

		// If memmap is not available yet, use a provisional high-memory window.
		heapStart = hhdmOffset + earlyPhysHeapStart
		heapCurrent = heapStart
		heapEnd = heapStart + earlyPhysHeapSize
		brkCurrent = heapStart
		return
	}

	// Fallback to static emergency heap if Limine data is unavailable.
	base := uintptr(unsafe.Pointer(&earlyHeap[0]))
	base = (base + pageSize - 1) &^ (pageSize - 1)
	heapStart = base
	heapCurrent = base
	heapEnd = uintptr(unsafe.Pointer(&earlyHeap[len(earlyHeap)-1])) + 1
	heapEnd = heapEnd &^ (pageSize - 1)
	brkCurrent = heapStart
}

// doMmap implements SYS_mmap for the Go runtime.
// Simple bump allocator: allocates page-aligned memory, never frees.
//
//go:nosplit
func doMmap(addr, length, prot, flags, fd, offset uintptr) uintptr {
	if length == 0 {
		return negErrno(eINVAL)
	}
	ensureHeapInitialized()

	// Page-align the length
	length = (length + pageSize - 1) &^ (pageSize - 1)

	// MAP_FIXED mappings must use the requested address exactly.
	if flags&mapFixed != 0 {
		if addr == 0 || (addr&(pageSize-1)) != 0 {
			return negErrno(eINVAL)
		}
		return addr
	}

	// Check if we have enough memory
	if heapCurrent+length > heapEnd {
		serialPrint("mem: OOM! requested=")
		serialPrintDec(uint64(length))
		serialPrint("\n")
		return negErrno(eNOMEM)
	}

	// Bump allocate
	result := heapCurrent
	heapCurrent += length

	// Zero the memory
	memzero(result, length)

	return result
}

// doMunmap implements SYS_munmap.
// We support LIFO reclaim for bump allocations so runtime reserve/retry loops
// don't permanently leak large regions during bootstrap.
//
//go:nosplit
func doMunmap(addr, length uintptr) uintptr {
	if length == 0 {
		return 0
	}
	ensureHeapInitialized()
	length = (length + pageSize - 1) &^ (pageSize - 1)

	if addr >= heapStart && addr+length == heapCurrent {
		heapCurrent = addr
		if brkCurrent > heapCurrent {
			brkCurrent = heapCurrent
		}
	}
	return 0
}

// doBrk implements SYS_brk for the Go runtime.
//
//go:nosplit
func doBrk(addr uintptr) uintptr {
	ensureHeapInitialized()
	if addr == 0 {
		return brkCurrent
	}
	if addr > heapEnd {
		return brkCurrent // refuse
	}
	if addr > brkCurrent {
		memzero(brkCurrent, addr-brkCurrent)
	}
	brkCurrent = addr
	return brkCurrent
}

// doFutex implements SYS_futex for the Go runtime.
// MVP: single-core, so FUTEX_WAIT just spins and FUTEX_WAKE is a no-op.
//
//go:nosplit
func doFutex(uaddr, futexOp, val, timeout, uaddr2, val3 uintptr) uintptr {
	op := futexOp & 0x7F // mask out FUTEX_PRIVATE_FLAG

	switch op {
	case 0: // FUTEX_WAIT
		// Check if *uaddr == val; if not, return EAGAIN.
		cur := *(*uint32)(ptrOf(uaddr))
		if uint32(val) != cur {
			return negErrno(eAGAIN)
		}
		// Single-core bootstrap shim: force a wake transition.
		*(*uint32)(ptrOf(uaddr)) = uint32(val) + 1
		return 0
	case 1: // FUTEX_WAKE
		// Report one waiter woken.
		return 1
	default:
		return negErrno(eNOSYS)
	}
}

// memzero zeroes a memory region.
//
//go:nosplit
func memzero(addr, size uintptr) {
	memzeroAsm(addr, size)
}
