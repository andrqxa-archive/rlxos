//go:build amd64

package dl

import "fmt"

// x86-64 relocation types.
const (
	r_X86_64_NONE      = 0
	r_X86_64_64        = 1  // S + A
	r_X86_64_GLOB_DAT  = 6  // S
	r_X86_64_JUMP_SLOT = 7  // S
	r_X86_64_RELATIVE  = 8  // B + A
	r_X86_64_DTPMOD64  = 16 // TLS module ID
	r_X86_64_DTPOFF64  = 17 // TLS offset within module
	r_X86_64_TPOFF64   = 18 // TLS offset from TP (static TLS)
	r_X86_64_TLSDESC   = 36 // TLS descriptor
	r_X86_64_IRELATIVE = 37 // indirect (IFUNC) relative
)

// applyRelocation dispatches a single RELA relocation on amd64.
func (ld *Loader) applyRelocation(obj *Object, rela elf64Rela, localScope []*Object) error {
	typ := rType(rela.Info)
	symIdx := rSym(rela.Info)
	target := obj.Base + uintptr(rela.Offset)

	switch typ {
	case r_X86_64_NONE:
		// No-op.
		return nil

	case r_X86_64_RELATIVE:
		// B + A — no symbol lookup needed.
		writePtr(target, uint64(obj.Base)+uint64(rela.Addend))
		return nil

	case r_X86_64_IRELATIVE:
		// Like RELATIVE but the result is a function pointer to call.
		resolverAddr := uintptr(uint64(obj.Base) + uint64(rela.Addend))
		result := callIFUNCResolver(resolverAddr)
		writePtr(target, uint64(result))
		return nil

	case r_X86_64_64:
		// S + A
		addr, _, err := ld.resolveForReloc(obj, symIdx, localScope)
		if err != nil {
			return ld.relocErr(obj, rela, err)
		}
		writePtr(target, uint64(addr)+uint64(rela.Addend))
		return nil

	case r_X86_64_GLOB_DAT:
		// S (GOT entry)
		addr, _, err := ld.resolveForReloc(obj, symIdx, localScope)
		if err != nil {
			return ld.relocErr(obj, rela, err)
		}
		writePtr(target, uint64(addr))
		return nil

	case r_X86_64_JUMP_SLOT:
		// S (PLT/GOT entry)
		addr, _, err := ld.resolveForReloc(obj, symIdx, localScope)
		if err != nil {
			return ld.relocErr(obj, rela, err)
		}
		writePtr(target, uint64(addr))
		return nil

	case r_X86_64_DTPMOD64:
		// TLS module ID.
		if symIdx != 0 {
			_, sym, err := ld.resolveForReloc(obj, symIdx, localScope)
			if err != nil {
				// For weak, write 0.
				writePtr(target, 0)
				return nil
			}
			_ = sym
			// Find the object that defines this symbol and use its TLS module ID.
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

	case r_X86_64_DTPOFF64:
		// TLS offset within module.
		if symIdx != 0 {
			sym := obj.getSymbol(symIdx)
			writePtr(target, sym.Value+uint64(rela.Addend))
		} else {
			writePtr(target, uint64(rela.Addend))
		}
		return nil

	case r_X86_64_TPOFF64:
		// Static TLS offset from thread pointer.
		// On x86-64, TP points to the end of the static TLS block (variant II).
		// Value = sym.Value + addend - tlsOffset (negative offset from TP).
		if symIdx != 0 {
			sym := obj.getSymbol(symIdx)
			defObj := ld.findDefiningObject(obj, symIdx, localScope)
			if defObj == nil {
				defObj = obj
			}
			val := int64(sym.Value) + rela.Addend - int64(defObj.tlsOffset)
			writePtr(target, uint64(val))
		} else {
			val := rela.Addend - int64(obj.tlsOffset)
			writePtr(target, uint64(val))
		}
		return nil

	case r_X86_64_TLSDESC:
		// TLS descriptor: a pair of function-pointer + argument at target.
		// We fill it with a simple resolver that returns the static TLS offset.
		// For general-dynamic TLS this needs a real resolver; we use the static
		// shortcut when possible.
		if symIdx != 0 {
			sym := obj.getSymbol(symIdx)
			defObj := ld.findDefiningObject(obj, symIdx, localScope)
			if defObj == nil {
				defObj = obj
			}
			val := int64(sym.Value) + rela.Addend - int64(defObj.tlsOffset)
			// Write [resolver_func, argument].  We write a zero resolver
			// (caller must handle) and the offset as argument.
			writePtr(target, 0)          // resolver — 0 means "use argument directly"
			writePtr(target+8, uint64(val))
		} else {
			val := rela.Addend - int64(obj.tlsOffset)
			writePtr(target, 0)
			writePtr(target+8, uint64(val))
		}
		return nil

	default:
		return &RelocError{
			Type:   typ,
			Symbol: obj.symName(obj.getSymbol(symIdx)),
			Object: obj.Path,
			Err:    fmt.Errorf("unsupported x86-64 relocation type %d", typ),
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
