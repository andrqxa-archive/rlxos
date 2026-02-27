package dl

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// pageSize is the system page size, assumed 4096 on both amd64 and arm64.
const pageSize = 4096

// pageAlign rounds v up to the next page boundary.
func pageAlign(v uint64) uint64 {
	return (v + pageSize - 1) &^ (pageSize - 1)
}

// pageAlignDown rounds v down to the previous page boundary.
func pageAlignDown(v uint64) uint64 {
	return v &^ (pageSize - 1)
}

// elfProtToMmap converts ELF p_flags to mmap PROT_* flags.
func elfProtToMmap(pflags uint32) int {
	prot := 0
	if pflags&pfR != 0 {
		prot |= syscall.PROT_READ
	}
	if pflags&pfW != 0 {
		prot |= syscall.PROT_WRITE
	}
	if pflags&pfX != 0 {
		prot |= syscall.PROT_EXEC
	}
	return prot
}

// mapObject maps all PT_LOAD segments of an ELF shared object into memory.
// It returns the Object with Base, mapBase, MapSize, and phdr-derived fields
// populated.  RELRO is recorded but not yet applied (call applyRELRO after
// relocations).
//
// Strategy:
//  1. Compute the total virtual address range (min vaddr to max vaddr+memsz).
//  2. Reserve a contiguous region with a single anonymous MAP_PRIVATE mmap.
//  3. For each PT_LOAD segment, mmap the file content at the correct offset
//     within the reserved region.
//  4. Zero-fill any BSS portion (memsz > filesz).
func mapObject(f *os.File, phdrs []elf64Phdr, path string) (*Object, error) {
	// Step 1: compute address range.
	var minVaddr, maxVaddr uint64
	first := true
	for i := range phdrs {
		if phdrs[i].Type != ptLOAD {
			continue
		}
		lo := pageAlignDown(phdrs[i].Vaddr)
		hi := pageAlign(phdrs[i].Vaddr + phdrs[i].Memsz)
		if first || lo < minVaddr {
			minVaddr = lo
		}
		if first || hi > maxVaddr {
			maxVaddr = hi
		}
		first = false
	}
	if first {
		return nil, &LoadError{Path: path, Err: fmt.Errorf("no PT_LOAD segments")}
	}
	totalSize := maxVaddr - minVaddr
	if totalSize == 0 {
		return nil, &LoadError{Path: path, Err: fmt.Errorf("zero-size mapping")}
	}

	// Step 2: reserve the entire region.  We use PROT_NONE + MAP_ANONYMOUS
	// so that unmapped gaps cause SIGSEGV rather than leaking data.
	base, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP,
		0,
		uintptr(totalSize),
		syscall.PROT_NONE,
		syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS,
		^uintptr(0), // fd = -1
		0,
	)
	if errno != 0 {
		return nil, &LoadError{Path: path, Err: fmt.Errorf("mmap reserve: %w", errno)}
	}

	obj := &Object{
		Path:     path,
		Base:     base - uintptr(minVaddr), // load bias
		MinVaddr: minVaddr,
		MaxVaddr: maxVaddr,
		MapSize:  uintptr(totalSize),
		mapBase:  base,
		Phdrs:    phdrs,
	}

	fd := f.Fd()

	// Step 3: map each PT_LOAD segment.
	for i := range phdrs {
		ph := &phdrs[i]
		if ph.Type != ptLOAD {
			continue
		}

		segStart := pageAlignDown(ph.Vaddr)
		segEnd := pageAlign(ph.Vaddr + ph.Memsz)
		segFileEnd := pageAlign(ph.Vaddr + ph.Filesz)
		mapAddr := base + uintptr(segStart-minVaddr)
		mapLen := segEnd - segStart
		fileOff := pageAlignDown(ph.Offset)

		prot := elfProtToMmap(ph.Flags)

		if ph.Filesz > 0 {
			// Map the file-backed portion.
			fileMappedLen := segFileEnd - segStart
			if fileMappedLen > mapLen {
				fileMappedLen = mapLen
			}
			_, _, errno := syscall.Syscall6(
				syscall.SYS_MMAP,
				mapAddr,
				uintptr(fileMappedLen),
				uintptr(prot),
				syscall.MAP_PRIVATE|syscall.MAP_FIXED,
				fd,
				uintptr(fileOff),
			)
			if errno != 0 {
				// Clean up the reservation on failure.
				syscall.Syscall(syscall.SYS_MUNMAP, base, uintptr(totalSize), 0)
				return nil, &LoadError{Path: path, Err: fmt.Errorf("mmap segment %d: %w", i, errno)}
			}
		}

		// Step 4: handle BSS (memsz > filesz).
		if ph.Memsz > ph.Filesz {
			// Zero the partial page after file content.
			bssStart := obj.Base + uintptr(ph.Vaddr+ph.Filesz)
			pageEnd := uintptr(pageAlign(uint64(bssStart)))
			if pageEnd > bssStart {
				// We need write permission to zero-fill.
				if prot&syscall.PROT_WRITE == 0 {
					syscall.Syscall(syscall.SYS_MPROTECT, uintptr(pageAlignDown(uint64(bssStart))), pageSize, uintptr(prot|syscall.PROT_WRITE))
				}
				zeroLen := pageEnd - bssStart
				zeroSlice := unsafe.Slice((*byte)(unsafe.Pointer(bssStart)), zeroLen)
				for j := range zeroSlice {
					zeroSlice[j] = 0
				}
				if prot&syscall.PROT_WRITE == 0 {
					syscall.Syscall(syscall.SYS_MPROTECT, uintptr(pageAlignDown(uint64(bssStart))), pageSize, uintptr(prot))
				}
			}

			// Map anonymous pages for the remaining BSS beyond file-backed pages.
			if segEnd > segFileEnd {
				anonAddr := base + uintptr(segFileEnd-minVaddr)
				anonLen := segEnd - segFileEnd
				_, _, errno := syscall.Syscall6(
					syscall.SYS_MMAP,
					anonAddr,
					uintptr(anonLen),
					uintptr(prot),
					syscall.MAP_PRIVATE|syscall.MAP_FIXED|syscall.MAP_ANONYMOUS,
					^uintptr(0),
					0,
				)
				if errno != 0 {
					syscall.Syscall(syscall.SYS_MUNMAP, base, uintptr(totalSize), 0)
					return nil, &LoadError{Path: path, Err: fmt.Errorf("mmap bss segment %d: %w", i, errno)}
				}
			}
		}
	}

	// Record RELRO and TLS program headers.
	for i := range phdrs {
		switch phdrs[i].Type {
		case ptGnuRelro:
			obj.relroAddr = obj.Base + uintptr(phdrs[i].Vaddr)
			obj.relroSize = uintptr(phdrs[i].Memsz)
		case ptTLS:
			obj.tlsPhdr = &phdrs[i]
			obj.tlsImage = obj.Base + uintptr(phdrs[i].Vaddr)
			obj.tlsImageSz = phdrs[i].Filesz
			obj.tlsMemSz = phdrs[i].Memsz
			obj.tlsAlign = phdrs[i].Align
		}
	}

	return obj, nil
}

// applyRELRO marks the GNU_RELRO region as read-only.  Must be called after
// all relocations have been applied.
func (obj *Object) applyRELRO() error {
	if obj.relroSize == 0 {
		return nil
	}
	addr := uintptr(pageAlignDown(uint64(obj.relroAddr)))
	end := uintptr(pageAlign(uint64(obj.relroAddr + obj.relroSize)))
	size := end - addr
	_, _, errno := syscall.Syscall(syscall.SYS_MPROTECT, addr, size, syscall.PROT_READ)
	if errno != 0 {
		return fmt.Errorf("mprotect RELRO: %w", errno)
	}
	return nil
}

// makeWritable temporarily makes the full mapped region writable for
// relocations.  Call restoreProtections afterwards.
func (obj *Object) makeWritable() error {
	for i := range obj.Phdrs {
		ph := &obj.Phdrs[i]
		if ph.Type != ptLOAD {
			continue
		}
		segStart := uintptr(pageAlignDown(ph.Vaddr))
		segEnd := uintptr(pageAlign(ph.Vaddr + ph.Memsz))
		addr := obj.Base + segStart
		size := segEnd - segStart
		prot := elfProtToMmap(ph.Flags) | syscall.PROT_WRITE
		_, _, errno := syscall.Syscall(syscall.SYS_MPROTECT, addr, size, uintptr(prot))
		if errno != 0 {
			return fmt.Errorf("mprotect writable: %w", errno)
		}
	}
	return nil
}

// restoreProtections restores the original page protections for all PT_LOAD
// segments.
func (obj *Object) restoreProtections() error {
	for i := range obj.Phdrs {
		ph := &obj.Phdrs[i]
		if ph.Type != ptLOAD {
			continue
		}
		segStart := uintptr(pageAlignDown(ph.Vaddr))
		segEnd := uintptr(pageAlign(ph.Vaddr + ph.Memsz))
		addr := obj.Base + segStart
		size := segEnd - segStart
		prot := elfProtToMmap(ph.Flags)
		_, _, errno := syscall.Syscall(syscall.SYS_MPROTECT, addr, size, uintptr(prot))
		if errno != 0 {
			return fmt.Errorf("mprotect restore: %w", errno)
		}
	}
	return nil
}

// unmapObject releases all memory mappings for the object.
func (obj *Object) unmapObject() {
	if obj.mapBase != 0 && obj.MapSize != 0 {
		syscall.Syscall(syscall.SYS_MUNMAP, obj.mapBase, obj.MapSize, 0)
		obj.mapBase = 0
		obj.MapSize = 0
	}
}
