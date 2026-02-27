package dl

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"
)

// Object represents a single loaded ELF shared object.  It contains the parsed
// headers, memory mappings, dynamic tables, symbol/string tables, relocation
// info, TLS metadata, and dependency list.
type Object struct {
	// Identity
	Path   string // resolved filesystem path
	Soname string // DT_SONAME if present

	// Memory layout
	Base     uintptr // load bias (difference between mapped and vaddr)
	MinVaddr uint64  // lowest vaddr of PT_LOAD segments
	MaxVaddr uint64  // highest vaddr+memsz (page-aligned)
	MapSize  uintptr // total mapped region size

	// Mapped region base (the actual mmap return value)
	mapBase uintptr

	// Program headers (in-memory copy)
	Phdrs []elf64Phdr

	// Dynamic section entries (raw list)
	dynEntries []elf64Dyn

	// Parsed dynamic info
	dyn dynamicInfo

	// Symbol and string tables (pointers into mapped memory)
	symtab  uintptr // DT_SYMTAB absolute address
	strtab  uintptr // DT_STRTAB absolute address
	strsz   uint64  // DT_STRSZ
	syment  uint64  // DT_SYMENT (should be 24)

	// Hash tables
	gnuHash *gnuHashTable
	sysvHash *sysvHashTable

	// Versioning tables (absolute addresses in mapped memory)
	versym   uintptr // DT_VERSYM — array of uint16, one per symbol
	verdef   uintptr // DT_VERDEF
	verdefNum uint64
	verneed  uintptr // DT_VERNEED
	verneedNum uint64

	// Relocation tables (absolute addresses + sizes)
	relaAddr uintptr
	relaSz   uint64
	relaEnt  uint64
	relAddr  uintptr
	relSz    uint64
	relEnt   uint64
	pltRel   uint64 // DT_PLTREL: 7=RELA, 17=REL
	jmprelAddr uintptr
	jmprelSz   uint64
	pltgot   uintptr // DT_PLTGOT

	// RELRO region
	relroAddr uintptr
	relroSize uintptr

	// TLS
	tlsPhdr    *elf64Phdr // PT_TLS program header, if present
	tlsModuleID uint64    // assigned module ID for TLS
	tlsOffset   uint64    // offset into static TLS block
	tlsImage   uintptr   // absolute address of TLS init image
	tlsImageSz uint64    // size of TLS init image (filesz)
	tlsMemSz   uint64    // total TLS allocation (memsz)
	tlsAlign   uint64    // TLS alignment

	// Init / fini
	initFunc      uintptr   // DT_INIT
	finiFunc      uintptr   // DT_FINI
	initArray     uintptr   // DT_INIT_ARRAY
	initArraySz   uint64    // DT_INIT_ARRAYSZ
	finiArray     uintptr   // DT_FINI_ARRAY
	finiArraySz   uint64    // DT_FINI_ARRAYSZ
	preinitArray  uintptr
	preinitArraySz uint64

	// Dependencies (sonames from DT_NEEDED)
	Needed []string

	// RPATH / RUNPATH for dependency search
	Rpath   string
	Runpath string

	// Flags
	bindNow    bool // DT_BIND_NOW or DF_BIND_NOW or DF_1_NOW
	symbolic   bool // DT_SYMBOLIC or DF_SYMBOLIC
	staticTLS  bool // DF_STATIC_TLS
	textrel    bool // DT_TEXTREL or DF_TEXTREL
	nodelete   bool // DF_1_NODELETE
	deepbind   bool // opened with RTLD_DEEPBIND

	// State
	refcount    int
	relocated   bool
	initialized bool
	global      bool // in global scope

	// Back-pointer to owning loader for symbol resolution during relocation.
	loader *Loader

	// Statistics
	relocCount uint64
}

// readELFHeader reads and validates the ELF64 header from the given file.
func readELFHeader(f *os.File, path string) (*elf64Ehdr, error) {
	var buf [elf64EhdrSize]byte
	if _, err := f.ReadAt(buf[:], 0); err != nil {
		return nil, &LoadError{Path: path, Err: fmt.Errorf("read header: %w", err)}
	}

	// Validate magic.
	if string(buf[0:4]) != elfMagic {
		return nil, &LoadError{Path: path, Err: ErrNotELF}
	}
	// Must be 64-bit, little-endian.
	if buf[4] != elfClass64 {
		return nil, &FormatError{Path: path, Detail: "not ELF64 (ELFCLASS64)"}
	}
	if buf[5] != elfData2LSB {
		return nil, &FormatError{Path: path, Detail: "not little-endian (ELFDATA2LSB)"}
	}
	if buf[6] != elfVersionEV {
		return nil, &FormatError{Path: path, Detail: "unsupported ELF version"}
	}

	hdr := &elf64Ehdr{}
	hdr.Ident = [16]byte(buf[0:16])
	hdr.Type = binary.LittleEndian.Uint16(buf[16:])
	hdr.Machine = binary.LittleEndian.Uint16(buf[18:])
	hdr.Version = binary.LittleEndian.Uint32(buf[20:])
	hdr.Entry = binary.LittleEndian.Uint64(buf[24:])
	hdr.Phoff = binary.LittleEndian.Uint64(buf[32:])
	hdr.Shoff = binary.LittleEndian.Uint64(buf[40:])
	hdr.Flags = binary.LittleEndian.Uint32(buf[48:])
	hdr.Ehsize = binary.LittleEndian.Uint16(buf[52:])
	hdr.Phentsize = binary.LittleEndian.Uint16(buf[54:])
	hdr.Phnum = binary.LittleEndian.Uint16(buf[56:])
	hdr.Shentsize = binary.LittleEndian.Uint16(buf[58:])
	hdr.Shnum = binary.LittleEndian.Uint16(buf[60:])
	hdr.Shstrndx = binary.LittleEndian.Uint16(buf[62:])

	// Must be ET_DYN (shared object or PIE).
	if hdr.Type != etDYN {
		return nil, &LoadError{Path: path, Err: ErrNotShared}
	}

	// Validate machine type for current arch.
	if err := validateMachine(hdr.Machine); err != nil {
		return nil, &LoadError{Path: path, Err: err}
	}

	return hdr, nil
}

// readPhdrs reads all program headers from the file.
func readPhdrs(f *os.File, hdr *elf64Ehdr, path string) ([]elf64Phdr, error) {
	if hdr.Phentsize < elf64PhdrSize {
		return nil, &FormatError{Path: path, Detail: fmt.Sprintf("phentsize %d < %d", hdr.Phentsize, elf64PhdrSize)}
	}
	n := int(hdr.Phnum)
	phdrs := make([]elf64Phdr, n)
	buf := make([]byte, int(hdr.Phentsize)*n)
	if _, err := f.ReadAt(buf, int64(hdr.Phoff)); err != nil {
		return nil, &LoadError{Path: path, Err: fmt.Errorf("read phdrs: %w", err)}
	}
	for i := 0; i < n; i++ {
		off := i * int(hdr.Phentsize)
		b := buf[off:]
		phdrs[i] = elf64Phdr{
			Type:   binary.LittleEndian.Uint32(b[0:]),
			Flags:  binary.LittleEndian.Uint32(b[4:]),
			Offset: binary.LittleEndian.Uint64(b[8:]),
			Vaddr:  binary.LittleEndian.Uint64(b[16:]),
			Paddr:  binary.LittleEndian.Uint64(b[24:]),
			Filesz: binary.LittleEndian.Uint64(b[32:]),
			Memsz:  binary.LittleEndian.Uint64(b[40:]),
			Align:  binary.LittleEndian.Uint64(b[48:]),
		}
	}
	return phdrs, nil
}

// getString reads a NUL-terminated string from the mapped strtab at the given
// offset.  It is safe only while the object is mapped.
func (obj *Object) getString(offset uint32) string {
	if obj.strtab == 0 {
		return ""
	}
	base := obj.strtab + uintptr(offset)
	// Walk until NUL or safety limit.
	max := int(obj.strsz) - int(offset)
	if max <= 0 || max > 65536 {
		max = 65536
	}
	p := unsafe.Pointer(base)
	s := unsafe.Slice((*byte)(p), max)
	for i, b := range s {
		if b == 0 {
			return string(s[:i])
		}
	}
	return string(s)
}

// getSymbol reads the i-th Elf64_Sym from the mapped symtab.
func (obj *Object) getSymbol(idx uint32) elf64Sym {
	addr := obj.symtab + uintptr(idx)*uintptr(obj.syment)
	p := unsafe.Pointer(addr)
	b := unsafe.Slice((*byte)(p), elf64SymSize)
	return elf64Sym{
		Name:  binary.LittleEndian.Uint32(b[0:]),
		Info:  b[4],
		Other: b[5],
		Shndx: binary.LittleEndian.Uint16(b[6:]),
		Value: binary.LittleEndian.Uint64(b[8:]),
		Size:  binary.LittleEndian.Uint64(b[16:]),
	}
}

// symbolAddress returns the absolute address of a defined symbol.
func (obj *Object) symbolAddress(sym elf64Sym) uintptr {
	if sym.Shndx == shnABS {
		return uintptr(sym.Value)
	}
	return obj.Base + uintptr(sym.Value)
}

// symName returns the name of a symbol.
func (obj *Object) symName(sym elf64Sym) string {
	return obj.getString(sym.Name)
}
