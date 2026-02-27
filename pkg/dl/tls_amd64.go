//go:build amd64

package dl

// amd64 TLS notes:
//
// x86-64 uses TLS variant II (Drepper).  The thread pointer (TP) in FS base
// points to the TCB, and the static TLS block grows downward from TP:
//
//   [TLS block N] ... [TLS block 1] [TCB] <-- TP (FS base)
//
// TPOFF = -(total_static_tls_size - module_tls_offset + sym.Value)
//
// For general-dynamic TLS (DTPMOD/DTPOFF), the dtv[module_id] points to the
// module's TLS block, and dtpoff is the offset within that block.
//
// TLSDESC on x86-64: the descriptor is two 64-bit words:
//   [0] = resolver function pointer (or 0 for static TLS shortcut)
//   [1] = argument (typically the TP-relative offset)
//
// Since we don't inject a trampoline into the loaded code, we resolve
// TLSDESC eagerly at relocation time and set resolver=0 (meaning the
// caller should read the argument directly as the TP offset).
//
// Note: actually calling TLS-using code from Go is not straightforward
// because Go manages its own FS base.  These relocations are applied for
// structural correctness of the loaded image.
