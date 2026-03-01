package dl

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

// dynamicInfo holds parsed values extracted from the PT_DYNAMIC segment.
// All pointer fields are relative offsets (vaddrs) until rebase() is called
// to convert them to absolute addresses using the load bias.
type dynamicInfo struct {
	// These are the raw vaddr values from DT entries; rebased to absolute
	// addresses after mapping.
	strtabVaddr  uint64
	symtabVaddr  uint64
	hashVaddr    uint64
	gnuHashVaddr uint64

	relaVaddr uint64
	relaSz    uint64
	relaEnt   uint64

	relVaddr uint64
	relSz    uint64
	relEnt   uint64

	pltRel      uint64 // 7 (RELA) or 17 (REL)
	jmprelVaddr uint64
	jmprelSz    uint64
	pltgotVaddr uint64

	initVaddr         uint64
	finiVaddr         uint64
	initArrayVaddr    uint64
	initArraySz       uint64
	finiArrayVaddr    uint64
	finiArraySz       uint64
	preinitArrayVaddr uint64
	preinitArraySz    uint64

	strsz  uint64
	syment uint64

	versymVaddr  uint64
	verdefVaddr  uint64
	verdefNum    uint64
	verneedVaddr uint64
	verneedNum   uint64

	soname  uint32 // strtab offset
	rpath   uint32 // strtab offset
	runpath uint32 // strtab offset

	flags  uint64
	flags1 uint64

	bindNow   bool
	symbolic  bool
	textrel   bool
	staticTLS bool

	pltrelSz uint64 // DT_PLTRELSZ

	relaCount uint64 // DT_RELACOUNT (hint for RELATIVE relocs)

	tlsdescPLT uint64
	tlsdescGOT uint64

	needed []uint32 // strtab offsets for DT_NEEDED entries
}

// parseDynamic reads the dynamic section from a PT_DYNAMIC segment.  The
// segment contents are read from the mapped memory at base + phdr.Vaddr.
func parseDynamic(base uintptr, phdr elf64Phdr) ([]elf64Dyn, *dynamicInfo, error) {
	if phdr.Type != ptDYNAMIC {
		return nil, nil, fmt.Errorf("not a PT_DYNAMIC segment")
	}

	addr := base + uintptr(phdr.Vaddr)
	n := int(phdr.Memsz / uint64(elf64DynSize))
	if n == 0 {
		return nil, nil, fmt.Errorf("empty dynamic segment")
	}

	entries := make([]elf64Dyn, 0, n)
	info := &dynamicInfo{
		syment: elf64SymSize, // default
	}

	for i := 0; i < n; i++ {
		off := uintptr(i) * elf64DynSize
		p := unsafe.Pointer(addr + off)
		b := unsafe.Slice((*byte)(p), elf64DynSize)
		d := elf64Dyn{
			Tag: int64(binary.LittleEndian.Uint64(b[0:])),
			Val: binary.LittleEndian.Uint64(b[8:]),
		}
		if d.Tag == dtNULL {
			break
		}
		entries = append(entries, d)

		switch d.Tag {
		case dtNEEDED:
			info.needed = append(info.needed, uint32(d.Val))
		case dtSTRTAB:
			info.strtabVaddr = d.Val
		case dtSYMTAB:
			info.symtabVaddr = d.Val
		case dtSTRSZ:
			info.strsz = d.Val
		case dtSYMENT:
			info.syment = d.Val
		case dtHASH:
			info.hashVaddr = d.Val
		case dtGnuHash:
			info.gnuHashVaddr = d.Val
		case dtRELA:
			info.relaVaddr = d.Val
		case dtRELASZ:
			info.relaSz = d.Val
		case dtRELAENT:
			info.relaEnt = d.Val
		case dtREL:
			info.relVaddr = d.Val
		case dtRELSZ:
			info.relSz = d.Val
		case dtRELENT:
			info.relEnt = d.Val
		case dtPLTREL:
			info.pltRel = d.Val
		case dtJMPREL:
			info.jmprelVaddr = d.Val
		case dtPLTRELSZ:
			info.pltrelSz = d.Val
		case dtPLTGOT:
			info.pltgotVaddr = d.Val
		case dtINIT:
			info.initVaddr = d.Val
		case dtFINI:
			info.finiVaddr = d.Val
		case dtINIT_ARRAY:
			info.initArrayVaddr = d.Val
		case dtINIT_ARRAYSZ:
			info.initArraySz = d.Val
		case dtFINI_ARRAY:
			info.finiArrayVaddr = d.Val
		case dtFINI_ARRAYSZ:
			info.finiArraySz = d.Val
		case dtPREINIT_ARRAY:
			info.preinitArrayVaddr = d.Val
		case dtPREINIT_ARRAYSZ:
			info.preinitArraySz = d.Val
		case dtSONAME:
			info.soname = uint32(d.Val)
		case dtRPATH:
			info.rpath = uint32(d.Val)
		case dtRUNPATH:
			info.runpath = uint32(d.Val)
		case dtFLAGS:
			info.flags = d.Val
			if d.Val&dfBIND_NOW != 0 {
				info.bindNow = true
			}
			if d.Val&dfSYMBOLIC != 0 {
				info.symbolic = true
			}
			if d.Val&dfTEXTREL != 0 {
				info.textrel = true
			}
			if d.Val&dfSTATIC_TLS != 0 {
				info.staticTLS = true
			}
		case dtFLAGS_1:
			info.flags1 = d.Val
			if d.Val&df1NOW != 0 {
				info.bindNow = true
			}
		case dtBIND_NOW:
			info.bindNow = true
		case dtSYMBOLIC:
			info.symbolic = true
		case dtTEXTREL:
			info.textrel = true
		case dtVERSYM:
			info.versymVaddr = d.Val
		case dtVERDEF:
			info.verdefVaddr = d.Val
		case dtVERDEFNUM:
			info.verdefNum = d.Val
		case dtVERNEED:
			info.verneedVaddr = d.Val
		case dtVERNEEDNUM:
			info.verneedNum = d.Val
		case dtRELACount:
			info.relaCount = d.Val
		case dtTLSDESC_PLT:
			info.tlsdescPLT = d.Val
		case dtTLSDESC_GOT:
			info.tlsdescGOT = d.Val
		}
	}

	return entries, info, nil
}

// rebaseDynamic converts vaddr fields in dynamicInfo to absolute addresses
// using the given load bias, and populates the Object's fields.
func (obj *Object) rebaseDynamic() {
	d := &obj.dyn
	bias := uint64(obj.Base)

	// String and symbol tables.
	if d.strtabVaddr != 0 {
		obj.strtab = uintptr(d.strtabVaddr + bias)
	}
	obj.strsz = d.strsz
	if d.symtabVaddr != 0 {
		obj.symtab = uintptr(d.symtabVaddr + bias)
	}
	obj.syment = d.syment
	if obj.syment == 0 {
		obj.syment = elf64SymSize
	}

	// Hash tables.
	if d.gnuHashVaddr != 0 {
		obj.gnuHash = parseGnuHash(uintptr(d.gnuHashVaddr + bias))
	}
	if d.hashVaddr != 0 {
		obj.sysvHash = parseSysvHash(uintptr(d.hashVaddr + bias))
	}

	// RELA relocations.
	if d.relaVaddr != 0 {
		obj.relaAddr = uintptr(d.relaVaddr + bias)
		obj.relaSz = d.relaSz
		obj.relaEnt = d.relaEnt
		if obj.relaEnt == 0 {
			obj.relaEnt = elf64RelaSize
		}
	}

	// REL relocations (uncommon on 64-bit but handle gracefully).
	if d.relVaddr != 0 {
		obj.relAddr = uintptr(d.relVaddr + bias)
		obj.relSz = d.relSz
		obj.relEnt = d.relEnt
		if obj.relEnt == 0 {
			obj.relEnt = elf64RelSize
		}
	}

	// PLT relocations.
	obj.pltRel = d.pltRel
	if d.jmprelVaddr != 0 {
		obj.jmprelAddr = uintptr(d.jmprelVaddr + bias)
		obj.jmprelSz = d.pltrelSz
	}
	if d.pltgotVaddr != 0 {
		obj.pltgot = uintptr(d.pltgotVaddr + bias)
	}

	// Init / fini.
	if d.initVaddr != 0 {
		obj.initFunc = uintptr(d.initVaddr + bias)
	}
	if d.finiVaddr != 0 {
		obj.finiFunc = uintptr(d.finiVaddr + bias)
	}
	if d.initArrayVaddr != 0 {
		obj.initArray = uintptr(d.initArrayVaddr + bias)
		obj.initArraySz = d.initArraySz
	}
	if d.finiArrayVaddr != 0 {
		obj.finiArray = uintptr(d.finiArrayVaddr + bias)
		obj.finiArraySz = d.finiArraySz
	}
	if d.preinitArrayVaddr != 0 {
		obj.preinitArray = uintptr(d.preinitArrayVaddr + bias)
		obj.preinitArraySz = d.preinitArraySz
	}

	// Versioning.
	if d.versymVaddr != 0 {
		obj.versym = uintptr(d.versymVaddr + bias)
	}
	if d.verdefVaddr != 0 {
		obj.verdef = uintptr(d.verdefVaddr + bias)
		obj.verdefNum = d.verdefNum
	}
	if d.verneedVaddr != 0 {
		obj.verneed = uintptr(d.verneedVaddr + bias)
		obj.verneedNum = d.verneedNum
	}

	// Flags.
	obj.bindNow = d.bindNow
	obj.symbolic = d.symbolic
	obj.textrel = d.textrel
	obj.staticTLS = d.staticTLS
	if d.flags1&df1NODELETE != 0 {
		obj.nodelete = true
	}

	// Resolve string-table references.
	if d.soname != 0 {
		obj.Soname = obj.getString(d.soname)
	}
	if d.rpath != 0 {
		obj.Rpath = obj.getString(d.rpath)
	}
	if d.runpath != 0 {
		obj.Runpath = obj.getString(d.runpath)
	}
	for _, off := range d.needed {
		obj.Needed = append(obj.Needed, obj.getString(off))
	}
}
