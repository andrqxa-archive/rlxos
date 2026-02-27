package dl

import "unsafe"

// runInitFunctions calls the constructors for an object:
//  1. DT_PREINIT_ARRAY (only for the main executable, but we call it if present)
//  2. DT_INIT
//  3. DT_INIT_ARRAY entries in order
//
// Dependencies must be initialized before their dependents (caller ensures
// this by calling in topological order).
func (obj *Object) runInitFunctions() {
	if obj.initialized {
		return
	}
	obj.initialized = true

	// Pre-init array (rare; mainly for the main executable).
	if obj.preinitArray != 0 && obj.preinitArraySz > 0 {
		callFuncArray(obj.preinitArray, obj.preinitArraySz)
	}

	// DT_INIT (single function).
	if obj.initFunc != 0 {
		callFunc(obj.initFunc)
	}

	// DT_INIT_ARRAY (array of function pointers).
	if obj.initArray != 0 && obj.initArraySz > 0 {
		callFuncArray(obj.initArray, obj.initArraySz)
	}
}

// runFiniFunctions calls the destructors for an object:
//  1. DT_FINI_ARRAY entries in reverse order
//  2. DT_FINI
//
// Dependents must be finalized before their dependencies (caller ensures
// reverse dependency order).
func (obj *Object) runFiniFunctions() {
	if !obj.initialized {
		return // never initialized → don't finalize
	}

	// DT_FINI_ARRAY (in reverse order).
	if obj.finiArray != 0 && obj.finiArraySz > 0 {
		callFuncArrayReverse(obj.finiArray, obj.finiArraySz)
	}

	// DT_FINI.
	if obj.finiFunc != 0 {
		callFunc(obj.finiFunc)
	}
}

// callFunc calls a void(*)(void) at the given address.
func callFunc(addr uintptr) {
	// We use the same mechanism as Call0 — syscall.Syscall with the function
	// pointer.  This is a void(void) function.
	Call0(addr)
}

// callFuncArray calls each function pointer in an array sequentially.
// arrayAddr points to the start of the array; arraySz is the total byte
// size (not element count).
func callFuncArray(arrayAddr uintptr, arraySz uint64) {
	n := arraySz / 8 // each entry is a 64-bit pointer
	for i := uint64(0); i < n; i++ {
		fnPtr := *(*uintptr)(unsafe.Pointer(arrayAddr + uintptr(i*8)))
		if fnPtr == 0 || fnPtr == ^uintptr(0) {
			continue // skip null / sentinel entries
		}
		Call0(fnPtr)
	}
}

// callFuncArrayReverse calls each function pointer in an array in reverse order.
func callFuncArrayReverse(arrayAddr uintptr, arraySz uint64) {
	n := arraySz / 8
	for i := int64(n) - 1; i >= 0; i-- {
		fnPtr := *(*uintptr)(unsafe.Pointer(arrayAddr + uintptr(i*8)))
		if fnPtr == 0 || fnPtr == ^uintptr(0) {
			continue
		}
		Call0(fnPtr)
	}
}
