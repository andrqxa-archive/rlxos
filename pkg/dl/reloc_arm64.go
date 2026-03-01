//go:build arm64

package dl

import "fmt"

// AArch64 relocation types.
const (
	r_AARCH64_NONE         = 0
	r_AARCH64_ABS64        = 257  // S + A
	r_AARCH64_GLOB_DAT     = 1025 // S + A
	r_AARCH64_JUMP_SLOT    = 1026 // S
	r_AARCH64_RELATIVE     = 1027 // Delta(S) + A  (= B + A)
	r_AARCH64_TLS_DTPMOD64 = 1028 // TLS module ID
	r_AARCH64_TLS_DTPREL64 = 1029 // TLS offset in module
	r_AARCH64_TLS_TPREL64  = 1030 // TP-relative offset
	r_AARCH64_TLSDESC      = 1031 // TLS descriptor
	r_AARCH64_IRELATIVE    = 1032 // indirect (IFUNC) relative
)

// applyRelocation dispatches a single RELA relocation on arm64.
func (ld *Loader) applyRelocation(obj *Object, rela elf64Rela, localScope []*Object) error {
	typ := rType(rela.Info)
	symIdx := rSym(rela.Info)
	target := obj.Base + uintptr(rela.Offset)

	switch typ {
	case r_AARCH64_NONE:
		return nil

	case r_AARCH64_RELATIVE:
		// B + A
		writePtr(target, uint64(obj.Base)+uint64(rela.Addend))
		return nil

	case r_AARCH64_IRELATIVE:
		resolverAddr := uintptr(uint64(obj.Base) + uint64(rela.Addend))
		result := callIFUNCResolver(resolverAddr)
		writePtr(target, uint64(result))
		return nil

	case r_AARCH64_ABS64:
		// S + A
		addr, _, err := ld.resolveForReloc(obj, symIdx, localScope)
		if err != nil {
			return ld.relocErr(obj, rela, err)
		}
		writePtr(target, uint64(addr)+uint64(rela.Addend))
		return nil

	case r_AARCH64_GLOB_DAT:
		// S + A
		addr, _, err := ld.resolveForReloc(obj, symIdx, localScope)
		if err != nil {
			return ld.relocErr(obj, rela, err)
		}
		writePtr(target, uint64(addr)+uint64(rela.Addend))
		return nil

	case r_AARCH64_JUMP_SLOT:
		// S
		addr, _, err := ld.resolveForReloc(obj, symIdx, localScope)
		if err != nil {
			return ld.relocErr(obj, rela, err)
		}
		writePtr(target, uint64(addr))
		return nil

	case r_AARCH64_TLS_DTPMOD64:
		if symIdx != 0 {
			defObj := ld.findDefiningObject(obj, symIdx, localScope)
			if defObj != nil {
				writePtr(target, defObj.tlsModuleID)
			} else {
				writePtr(target, obj.tlsModuleID)
			}
		} else {
			writePtr(target, obj.tlsModuleID)
		}
		return nil

	case r_AARCH64_TLS_DTPREL64:
		if symIdx != 0 {
			sym := obj.getSymbol(symIdx)
			writePtr(target, sym.Value+uint64(rela.Addend))
		} else {
			writePtr(target, uint64(rela.Addend))
		}
		return nil

	case r_AARCH64_TLS_TPREL64:
		// On AArch64, variant I TLS: TP points to the start of the TCB,
		// followed by the TLS blocks.  Offset = tlsOffset + sym.Value + A.
		if symIdx != 0 {
			sym := obj.getSymbol(symIdx)
			defObj := ld.findDefiningObject(obj, symIdx, localScope)
			if defObj == nil {
				defObj = obj
			}
			val := int64(defObj.tlsOffset) + int64(sym.Value) + rela.Addend
			writePtr(target, uint64(val))
		} else {
			val := int64(obj.tlsOffset) + rela.Addend
			writePtr(target, uint64(val))
		}
		return nil

	case r_AARCH64_TLSDESC:
		// TLS descriptor pair [resolver, argument].
		if symIdx != 0 {
			sym := obj.getSymbol(symIdx)
			defObj := ld.findDefiningObject(obj, symIdx, localScope)
			if defObj == nil {
				defObj = obj
			}
			val := int64(defObj.tlsOffset) + int64(sym.Value) + rela.Addend
			writePtr(target, 0)
			writePtr(target+8, uint64(val))
		} else {
			val := int64(obj.tlsOffset) + rela.Addend
			writePtr(target, 0)
			writePtr(target+8, uint64(val))
		}
		return nil

	default:
		return &RelocError{
			Type:   typ,
			Symbol: obj.symName(obj.getSymbol(symIdx)),
			Object: obj.Path,
			Err:    fmt.Errorf("unsupported AArch64 relocation type %d", typ),
		}
	}
}

// relocErr wraps a relocation resolution error with context.
func (ld *Loader) relocErr(obj *Object, rela elf64Rela, err error) error {
	symIdx := rSym(rela.Info)
	name := ""
	if symIdx != 0 {
		name = obj.symName(obj.getSymbol(symIdx))
	}
	return &RelocError{
		Type:   rType(rela.Info),
		Symbol: name,
		Object: obj.Path,
		Err:    err,
	}
}
