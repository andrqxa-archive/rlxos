package dl

import (
	"encoding/binary"
	"unsafe"
)

// ──────────────────────────────────────────────────────────────────────────────
// GNU hash table
// ──────────────────────────────────────────────────────────────────────────────

// gnuHashTable is the parsed in-memory representation of a DT_GNU_HASH
// section.  All pointers refer directly into the mapped ELF image.
type gnuHashTable struct {
	nbuckets  uint32
	symndx    uint32 // first symbol index covered by the hash
	maskwords uint32
	shift2    uint32
	bloom     uintptr // -> [maskwords]uint64
	buckets   uintptr // -> [nbuckets]uint32
	chains    uintptr // -> []uint32 (indexed by sym_index - symndx)
}

// parseGnuHash builds a gnuHashTable from the mapped address.
func parseGnuHash(addr uintptr) *gnuHashTable {
	p := unsafe.Pointer(addr)
	hdr := unsafe.Slice((*byte)(p), 16)
	t := &gnuHashTable{
		nbuckets:  binary.LittleEndian.Uint32(hdr[0:]),
		symndx:    binary.LittleEndian.Uint32(hdr[4:]),
		maskwords: binary.LittleEndian.Uint32(hdr[8:]),
		shift2:    binary.LittleEndian.Uint32(hdr[12:]),
	}
	t.bloom = addr + 16
	t.buckets = t.bloom + uintptr(t.maskwords)*8
	t.chains = t.buckets + uintptr(t.nbuckets)*4
	return t
}

// gnuHashStr computes the GNU hash of a symbol name (djb2-style).
func gnuHashStr(name string) uint32 {
	var h uint32 = 5381
	for i := 0; i < len(name); i++ {
		h = h*33 + uint32(name[i])
	}
	return h
}

// gnuLookup searches for a symbol by name in the GNU hash table.
// Returns the symbol index (into .dynsym) or 0 if not found.
func (t *gnuHashTable) gnuLookup(name string, obj *Object) (uint32, bool) {
	if t.nbuckets == 0 {
		return 0, false
	}

	h := gnuHashStr(name)

	// Bloom filter check.
	wordIdx := (h / 64) % t.maskwords
	bloomWord := *(*uint64)(unsafe.Pointer(t.bloom + uintptr(wordIdx)*8))
	mask := uint64(1)<<(h%64) | uint64(1)<<((h>>t.shift2)%64)
	if bloomWord&mask != mask {
		return 0, false
	}

	// Bucket lookup.
	bucket := h % t.nbuckets
	symIdx := *(*uint32)(unsafe.Pointer(t.buckets + uintptr(bucket)*4))
	if symIdx == 0 {
		return 0, false
	}

	// Walk the chain.
	for {
		chainVal := *(*uint32)(unsafe.Pointer(t.chains + uintptr(symIdx-t.symndx)*4))
		// Compare hash (lower 31 bits; bit 0 of chain entry is the stop bit).
		if (chainVal | 1) == (h | 1) {
			sym := obj.getSymbol(symIdx)
			if obj.symName(sym) == name {
				return symIdx, true
			}
		}
		// Stop bit: if bit 0 is set, this is the last entry in the chain.
		if chainVal&1 != 0 {
			break
		}
		symIdx++
	}
	return 0, false
}

// ──────────────────────────────────────────────────────────────────────────────
// SysV hash table
// ──────────────────────────────────────────────────────────────────────────────

type sysvHashTable struct {
	nbucket uint32
	nchain  uint32
	buckets uintptr // -> [nbucket]uint32
	chains  uintptr // -> [nchain]uint32
}

func parseSysvHash(addr uintptr) *sysvHashTable {
	p := unsafe.Pointer(addr)
	hdr := unsafe.Slice((*byte)(p), 8)
	t := &sysvHashTable{
		nbucket: binary.LittleEndian.Uint32(hdr[0:]),
		nchain:  binary.LittleEndian.Uint32(hdr[4:]),
	}
	t.buckets = addr + 8
	t.chains = t.buckets + uintptr(t.nbucket)*4
	return t
}

// sysvHashStr computes the ELF SysV hash.
func sysvHashStr(name string) uint32 {
	var h uint32
	for i := 0; i < len(name); i++ {
		h = (h << 4) + uint32(name[i])
		g := h & 0xf0000000
		if g != 0 {
			h ^= g >> 24
		}
		h &= ^g
	}
	return h
}

// sysvLookup searches for a symbol by name in the SysV hash table.
func (t *sysvHashTable) sysvLookup(name string, obj *Object) (uint32, bool) {
	if t.nbucket == 0 {
		return 0, false
	}
	h := sysvHashStr(name)
	bucket := h % t.nbucket
	symIdx := *(*uint32)(unsafe.Pointer(t.buckets + uintptr(bucket)*4))
	for symIdx != 0 {
		if symIdx >= t.nchain {
			break
		}
		sym := obj.getSymbol(symIdx)
		if obj.symName(sym) == name {
			return symIdx, true
		}
		symIdx = *(*uint32)(unsafe.Pointer(t.chains + uintptr(symIdx)*4))
	}
	return 0, false
}

// ──────────────────────────────────────────────────────────────────────────────
// Symbol lookup (unified)
// ──────────────────────────────────────────────────────────────────────────────

// findSymbol searches for a symbol by name in a single object.  It prefers
// GNU hash when available, falling back to SysV hash.  Returns the symbol
// index and the Elf64_Sym, or ok=false.
func (obj *Object) findSymbol(name string) (uint32, elf64Sym, bool) {
	var idx uint32
	var found bool

	if obj.gnuHash != nil {
		idx, found = obj.gnuHash.gnuLookup(name, obj)
	} else if obj.sysvHash != nil {
		idx, found = obj.sysvHash.sysvLookup(name, obj)
	}
	if !found {
		return 0, elf64Sym{}, false
	}

	sym := obj.getSymbol(idx)

	// Skip undefined symbols (unless IFUNC resolver that is defined).
	if sym.Shndx == shnUNDEF {
		return 0, elf64Sym{}, false
	}
	bind := stBind(sym.Info)
	if bind == stbLOCAL {
		return 0, elf64Sym{}, false
	}

	return idx, sym, true
}

// ──────────────────────────────────────────────────────────────────────────────
// Symbol versioning
// ──────────────────────────────────────────────────────────────────────────────

// symbolVersion returns the version index for symbol at idx, or 0 if no
// versioning is present.
func (obj *Object) symbolVersion(idx uint32) uint16 {
	if obj.versym == 0 {
		return 0
	}
	addr := obj.versym + uintptr(idx)*2
	return *(*uint16)(unsafe.Pointer(addr))
}

// versionName resolves a version index to its name string by searching
// verdef and verneed tables.
func (obj *Object) versionName(verIdx uint16) string {
	// Mask off the hidden bit.
	verIdx &= 0x7fff

	if verIdx <= verNdxGLOBAL {
		return "" // local or global — no specific version
	}

	// Search VERDEF entries (versions this object defines).
	if obj.verdef != 0 {
		addr := obj.verdef
		for i := uint64(0); i < obj.verdefNum; i++ {
			p := unsafe.Pointer(addr)
			b := unsafe.Slice((*byte)(p), 20)
			vd := elf64Verdef{
				Version: binary.LittleEndian.Uint16(b[0:]),
				Flags:   binary.LittleEndian.Uint16(b[2:]),
				Ndx:     binary.LittleEndian.Uint16(b[4:]),
				Cnt:     binary.LittleEndian.Uint16(b[6:]),
				Hash:    binary.LittleEndian.Uint32(b[8:]),
				Aux:     binary.LittleEndian.Uint32(b[12:]),
				Next:    binary.LittleEndian.Uint32(b[16:]),
			}
			if vd.Ndx == verIdx {
				// Read first Verdaux to get the name.
				auxAddr := addr + uintptr(vd.Aux)
				ap := unsafe.Pointer(auxAddr)
				ab := unsafe.Slice((*byte)(ap), 8)
				nameOff := binary.LittleEndian.Uint32(ab[0:])
				return obj.getString(nameOff)
			}
			if vd.Next == 0 {
				break
			}
			addr += uintptr(vd.Next)
		}
	}

	// Search VERNEED entries (versions this object requires).
	if obj.verneed != 0 {
		addr := obj.verneed
		for i := uint64(0); i < obj.verneedNum; i++ {
			p := unsafe.Pointer(addr)
			b := unsafe.Slice((*byte)(p), 16)
			vn := elf64Verneed{
				Version: binary.LittleEndian.Uint16(b[0:]),
				Cnt:     binary.LittleEndian.Uint16(b[2:]),
				File:    binary.LittleEndian.Uint32(b[4:]),
				Aux:     binary.LittleEndian.Uint32(b[8:]),
				Next:    binary.LittleEndian.Uint32(b[12:]),
			}
			auxAddr := addr + uintptr(vn.Aux)
			for j := uint16(0); j < vn.Cnt; j++ {
				ap := unsafe.Pointer(auxAddr)
				ab := unsafe.Slice((*byte)(ap), 16)
				vna := elf64Vernaux{
					Hash:  binary.LittleEndian.Uint32(ab[0:]),
					Flags: binary.LittleEndian.Uint16(ab[4:]),
					Other: binary.LittleEndian.Uint16(ab[6:]),
					Name:  binary.LittleEndian.Uint32(ab[8:]),
					Next:  binary.LittleEndian.Uint32(ab[12:]),
				}
				if vna.Other == verIdx {
					return obj.getString(vna.Name)
				}
				if vna.Next == 0 {
					break
				}
				auxAddr += uintptr(vna.Next)
			}
			if vn.Next == 0 {
				break
			}
			addr += uintptr(vn.Next)
		}
	}

	return ""
}

// matchVersion checks whether a candidate symbol's version matches the
// requested version string.  If requestedVer is empty, any version matches.
// When doing an unversioned lookup, we accept both default (hidden bit clear)
// and hidden (hidden bit set) versions — the hash table already selects the
// unique symbol entry, so there's no ambiguity.
func (obj *Object) matchVersion(symIdx uint32, requestedVer string) bool {
	if requestedVer == "" {
		// No specific version requested — accept any.
		return true
	}
	verIdx := obj.symbolVersion(symIdx)
	if verIdx == 0 || verIdx == verNdxGLOBAL {
		return false // unversioned can't match a specific version request
	}
	name := obj.versionName(verIdx)
	return name == requestedVer
}

// ──────────────────────────────────────────────────────────────────────────────
// Scoped symbol resolution
// ──────────────────────────────────────────────────────────────────────────────

// lookupSymbol resolves a symbol across scopes.  Search order:
//  1. If DEEPBIND, search the requesting object first.
//  2. Local scope (handle's objects).
//  3. Global scope (all RTLD_GLOBAL objects in load order).
//
// Returns the absolute address of the symbol.
func (ld *Loader) lookupSymbol(name, version string, self *Object, localScope []*Object) (uintptr, error) {
	ld.stats.SymbolLookups++

	var weakResult uintptr
	var weakFound bool

	search := func(obj *Object) (uintptr, bool) {
		idx, sym, ok := obj.findSymbol(name)
		if !ok {
			return 0, false
		}
		if !obj.matchVersion(idx, version) {
			return 0, false
		}
		addr := resolveSymAddr(obj, sym)
		if stBind(sym.Info) == stbWEAK {
			if !weakFound {
				weakResult = addr
				weakFound = true
			}
			return 0, false // keep looking for strong
		}
		return addr, true
	}

	// DEEPBIND: search self first.
	if self != nil && self.deepbind {
		if addr, ok := search(self); ok {
			return addr, nil
		}
	}

	// Local scope.
	for _, obj := range localScope {
		if addr, ok := search(obj); ok {
			return addr, nil
		}
	}

	// Global scope.
	for _, obj := range ld.global {
		if addr, ok := search(obj); ok {
			return addr, nil
		}
	}

	if weakFound {
		return weakResult, nil
	}

	objName := "<unknown>"
	if self != nil {
		objName = self.Path
	}
	return 0, &SymbolError{
		Name:    name,
		Version: version,
		Object:  objName,
		Err:     ErrSymbolNotFound,
	}
}

// resolveSymAddr computes the final address for a symbol, handling IFUNC.
func resolveSymAddr(obj *Object, sym elf64Sym) uintptr {
	addr := obj.symbolAddress(sym)
	if stType(sym.Info) == sttGNU_IFUNC {
		addr = callIFUNCResolver(addr)
	}
	return addr
}

// resolveForReloc resolves a symbol referenced from a relocation entry.
// Returns the symbol's absolute address and the resolved Elf64_Sym.
// Weak undefined symbols resolve to 0.
func (ld *Loader) resolveForReloc(obj *Object, symIdx uint32, localScope []*Object) (uintptr, elf64Sym, error) {
	sym := obj.getSymbol(symIdx)
	name := obj.symName(sym)

	if sym.Shndx != shnUNDEF {
		// Defined in this object.
		return resolveSymAddr(obj, sym), sym, nil
	}

	// Undefined: look it up across scopes.
	var version string
	verIdx := obj.symbolVersion(symIdx)
	if verIdx > verNdxGLOBAL {
		version = obj.versionName(verIdx)
	}

	addr, err := ld.lookupSymbol(name, version, obj, localScope)
	if err != nil {
		if stBind(sym.Info) == stbWEAK {
			return 0, sym, nil
		}
		return 0, sym, err
	}
	return addr, sym, nil
}
