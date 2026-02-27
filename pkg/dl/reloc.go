package dl

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

// relocateObject applies all relocations for an object.  It handles RELA,
// REL (if present), and PLT/JMPREL tables.  The object must already be
// mapped and writable (call makeWritable first).
func (ld *Loader) relocateObject(obj *Object, localScope []*Object, eager bool) error {
	if obj.relocated {
		return nil
	}

	ld.logf("relocating %s (base=0x%x)", obj.Path, obj.Base)

	// Make segments writable for relocation patching.
	if err := obj.makeWritable(); err != nil {
		return &LoadError{Path: obj.Path, Err: fmt.Errorf("make writable: %w", err)}
	}

	// Process RELA table.
	if obj.relaAddr != 0 && obj.relaSz > 0 {
		n := obj.relaSz / obj.relaEnt
		for i := uint64(0); i < n; i++ {
			rela := readRela(obj.relaAddr + uintptr(i*obj.relaEnt))
			if err := ld.applyRelocation(obj, rela, localScope); err != nil {
				return err
			}
			obj.relocCount++
		}
	}

	// Process REL table (uncommon on 64-bit but handle it).
	if obj.relAddr != 0 && obj.relSz > 0 {
		n := obj.relSz / obj.relEnt
		for i := uint64(0); i < n; i++ {
			rel := readRel(obj.relAddr + uintptr(i*obj.relEnt))
			// Convert REL to RELA with addend=0.
			rela := elf64Rela{
				Offset: rel.Offset,
				Info:   rel.Info,
				Addend: 0,
			}
			if err := ld.applyRelocation(obj, rela, localScope); err != nil {
				return err
			}
			obj.relocCount++
		}
	}

	// Process PLT/JMPREL table.
	if obj.jmprelAddr != 0 && obj.jmprelSz > 0 {
		bindNow := eager || obj.bindNow
		if obj.pltRel == dtRELA || obj.pltRel == 0 {
			// RELA format PLT relocations.
			n := obj.jmprelSz / elf64RelaSize
			for i := uint64(0); i < n; i++ {
				rela := readRela(obj.jmprelAddr + uintptr(i*elf64RelaSize))
				if bindNow {
					if err := ld.applyRelocation(obj, rela, localScope); err != nil {
						return err
					}
				} else {
					// Lazy: write the PLT stub address (base + addend) so the
					// PLT entry jumps back to the resolver stub.  Since we
					// don't have a real PLT resolver trampoline in Go, we
					// resolve eagerly anyway for correctness.
					if err := ld.applyRelocation(obj, rela, localScope); err != nil {
						return err
					}
				}
				obj.relocCount++
			}
		} else if obj.pltRel == dtREL {
			// REL format PLT relocations.
			n := obj.jmprelSz / elf64RelSize
			for i := uint64(0); i < n; i++ {
				rel := readRel(obj.jmprelAddr + uintptr(i*elf64RelSize))
				rela := elf64Rela{
					Offset: rel.Offset,
					Info:   rel.Info,
					Addend: 0,
				}
				if err := ld.applyRelocation(obj, rela, localScope); err != nil {
					return err
				}
				obj.relocCount++
			}
		}
	}

	// Restore original protections.
	if err := obj.restoreProtections(); err != nil {
		return &LoadError{Path: obj.Path, Err: fmt.Errorf("restore protections: %w", err)}
	}

	// Apply RELRO (make GOT etc. read-only).
	if err := obj.applyRELRO(); err != nil {
		return &LoadError{Path: obj.Path, Err: fmt.Errorf("apply RELRO: %w", err)}
	}

	obj.relocated = true
	ld.stats.TotalRelocs += obj.relocCount

	ld.logf("relocated %s: %d relocations applied", obj.Path, obj.relocCount)
	return nil
}

// readRela reads an Elf64_Rela from mapped memory.
func readRela(addr uintptr) elf64Rela {
	p := unsafe.Pointer(addr)
	b := unsafe.Slice((*byte)(p), elf64RelaSize)
	return elf64Rela{
		Offset: binary.LittleEndian.Uint64(b[0:]),
		Info:   binary.LittleEndian.Uint64(b[8:]),
		Addend: int64(binary.LittleEndian.Uint64(b[16:])),
	}
}

// readRel reads an Elf64_Rel from mapped memory.
func readRel(addr uintptr) elf64Rel {
	p := unsafe.Pointer(addr)
	b := unsafe.Slice((*byte)(p), elf64RelSize)
	return elf64Rel{
		Offset: binary.LittleEndian.Uint64(b[0:]),
		Info:   binary.LittleEndian.Uint64(b[8:]),
	}
}

// writePtr writes a 64-bit value to the given absolute address.
func writePtr(addr uintptr, val uint64) {
	p := (*uint64)(unsafe.Pointer(addr))
	*p = val
}

// readPtr reads a 64-bit value from the given absolute address.
func readPtr(addr uintptr) uint64 {
	p := (*uint64)(unsafe.Pointer(addr))
	return *p
}
