package dl

// assignTLS allocates a TLS module ID and static TLS offset for an object
// that has a PT_TLS segment.  Must be called under ld.mu.
func (ld *Loader) assignTLS(obj *Object) {
	if obj.tlsPhdr == nil {
		return
	}

	ld.nextTLSModuleID++
	obj.tlsModuleID = ld.nextTLSModuleID

	// Align the current offset.
	align := obj.tlsAlign
	if align == 0 {
		align = 1
	}
	ld.tlsOffset = (ld.tlsOffset + align - 1) &^ (align - 1)

	// Assign offset.  Architecture-specific layout (variant I vs II) is
	// handled by the relocation code; here we just allocate monotonically.
	obj.tlsOffset = ld.tlsOffset + obj.tlsMemSz
	ld.tlsOffset = obj.tlsOffset

	ld.logf("TLS: %s module=%d offset=%d memsz=%d align=%d",
		obj.Path, obj.tlsModuleID, obj.tlsOffset, obj.tlsMemSz, obj.tlsAlign)
}

// findDefiningObject finds the object that defines the symbol referenced
// at symIdx in obj.  Used for TLS relocations that need the defining
// object's TLS module info.
func (ld *Loader) findDefiningObject(obj *Object, symIdx uint32, localScope []*Object) *Object {
	sym := obj.getSymbol(symIdx)
	name := obj.symName(sym)

	// If defined in this object, return self.
	if sym.Shndx != shnUNDEF {
		return obj
	}

	// Search local scope.
	for _, candidate := range localScope {
		_, csym, ok := candidate.findSymbol(name)
		if ok && csym.Shndx != shnUNDEF {
			return candidate
		}
	}

	// Search global scope.
	for _, candidate := range ld.global {
		_, csym, ok := candidate.findSymbol(name)
		if ok && csym.Shndx != shnUNDEF {
			return candidate
		}
	}

	return nil
}
