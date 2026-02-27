//go:build arm64

package dl

// arm64 TLS notes:
//
// AArch64 uses TLS variant I.  The thread pointer (TP) in TPIDR_EL0
// points to the TCB, and the static TLS block follows after the TCB
// (plus a platform-dependent gap of 2*sizeof(void*) = 16 bytes):
//
//   [TCB] [gap=16] [TLS block 1] [TLS block 2] ... <-- TP at start
//
// TPREL = tcb_size + gap + module_tls_offset + sym.Value
//
// For general-dynamic TLS (DTPMOD/DTPREL), dtv[module_id] points to
// the start of the module's block, and dtprel is the offset within.
//
// TLSDESC on AArch64: same two-word descriptor as x86-64.
//   [0] = resolver function pointer (0 = use argument directly)
//   [1] = argument (typically TP-relative offset)
//
// The resolver calling convention on AArch64 passes the descriptor
// address in X0 and expects the result in X0.  Since we resolve
// eagerly, we set resolver=0.
//
// Note: Go manages its own TPIDR_EL0 for goroutine-local storage.
// These relocations are applied for structural correctness.
