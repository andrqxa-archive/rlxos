package dl

// ELF64 constants and structures.
// We define our own to avoid importing debug/elf (which is fine) but more
// importantly to have full control over the set of constants and to keep
// the dependency list minimal.  All values come from the ELF specification
// (gABI, x86-64 ABI supplement, and AARCH64 ELF supplement).

// ──────────────────────────────────────────────────────────────────────────────
// File header
// ──────────────────────────────────────────────────────────────────────────────

const (
	elfMagic     = "\x7fELF"
	elfClass64   = 2
	elfData2LSB  = 1 // little-endian
	elfVersionEV = 1
)

// ELF types (e_type).
const (
	etDYN = 3 // shared object / PIE
)

// Machine types (e_machine).
const (
	emX86_64  = 62  // EM_X86_64
	emAARCH64 = 183 // EM_AARCH64
)

// Elf64_Ehdr — 64-byte ELF header.
type elf64Ehdr struct {
	Ident     [16]byte
	Type      uint16
	Machine   uint16
	Version   uint32
	Entry     uint64
	Phoff     uint64
	Shoff     uint64
	Flags     uint32
	Ehsize    uint16
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
	Shstrndx  uint16
}

const elf64EhdrSize = 64

// ──────────────────────────────────────────────────────────────────────────────
// Program header
// ──────────────────────────────────────────────────────────────────────────────

// Segment types (p_type).
const (
	ptNULL    = 0
	ptLOAD    = 1
	ptDYNAMIC = 2
	ptINTERP  = 3
	ptNOTE    = 4
	ptPHDR    = 6
	ptTLS     = 7

	ptGnuEhFrame = 0x6474e550
	ptGnuStack   = 0x6474e551
	ptGnuRelro   = 0x6474e552
	ptGnuProperty = 0x6474e553
)

// Segment flags (p_flags).
const (
	pfX = 0x1
	pfW = 0x2
	pfR = 0x4
)

// Elf64_Phdr — 56-byte program header.
type elf64Phdr struct {
	Type   uint32
	Flags  uint32
	Offset uint64
	Vaddr  uint64
	Paddr  uint64
	Filesz uint64
	Memsz  uint64
	Align  uint64
}

const elf64PhdrSize = 56

// ──────────────────────────────────────────────────────────────────────────────
// Section header (used sparingly — only for debug / fallback)
// ──────────────────────────────────────────────────────────────────────────────

type elf64Shdr struct {
	Name      uint32
	Type      uint32
	Flags     uint64
	Addr      uint64
	Offset    uint64
	Size      uint64
	Link      uint32
	Info      uint32
	Addralign uint64
	Entsize   uint64
}

const elf64ShdrSize = 64

// ──────────────────────────────────────────────────────────────────────────────
// Dynamic entry
// ──────────────────────────────────────────────────────────────────────────────

type elf64Dyn struct {
	Tag int64
	Val uint64
}

const elf64DynSize = 16

// Dynamic tags (d_tag).
const (
	dtNULL         = 0
	dtNEEDED       = 1
	dtPLTRELSZ     = 2
	dtPLTGOT       = 3
	dtHASH         = 4
	dtSTRTAB       = 5
	dtSYMTAB       = 6
	dtRELA         = 7
	dtRELASZ       = 8
	dtRELAENT      = 9
	dtSTRSZ        = 10
	dtSYMENT       = 11
	dtINIT         = 12
	dtFINI         = 13
	dtSONAME       = 14
	dtRPATH        = 15
	dtSYMBOLIC     = 16
	dtREL          = 17
	dtRELSZ        = 18
	dtRELENT       = 19
	dtPLTREL       = 20
	dtDEBUG        = 21
	dtTEXTREL      = 22
	dtJMPREL       = 23
	dtBIND_NOW     = 24
	dtINIT_ARRAY   = 25
	dtFINI_ARRAY   = 26
	dtINIT_ARRAYSZ = 27
	dtFINI_ARRAYSZ = 28
	dtRUNPATH      = 29
	dtFLAGS        = 30
	dtPREINIT_ARRAY   = 32
	dtPREINIT_ARRAYSZ = 33

	dtGnuHash  = 0x6ffffef5 // DT_GNU_HASH
	dtTLSDESC_PLT = 0x6ffffef6
	dtTLSDESC_GOT = 0x6ffffef7
	dtRELACount = 0x6ffffff9
	dtFLAGS_1  = 0x6ffffffb
	dtVERSYM   = 0x6ffffff0
	dtVERDEF   = 0x6ffffffc
	dtVERDEFNUM = 0x6ffffffd
	dtVERNEED  = 0x6ffffffe
	dtVERNEEDNUM = 0x6fffffff
)

// DT_FLAGS bits.
const (
	dfSYMBOLIC  = 0x02
	dfTEXTREL   = 0x04
	dfBIND_NOW  = 0x08
	dfSTATIC_TLS = 0x10
)

// DT_FLAGS_1 bits.
const (
	df1NOW    = 0x00000001
	df1GLOBAL = 0x00000002
	df1NODELETE = 0x00000008
	df1PIE    = 0x08000000
)

// ──────────────────────────────────────────────────────────────────────────────
// Symbol table
// ──────────────────────────────────────────────────────────────────────────────

type elf64Sym struct {
	Name  uint32
	Info  uint8
	Other uint8
	Shndx uint16
	Value uint64
	Size  uint64
}

const elf64SymSize = 24

// Symbol binding (upper 4 bits of st_info).
func stBind(info uint8) uint8 { return info >> 4 }

// Symbol type (lower 4 bits of st_info).
func stType(info uint8) uint8 { return info & 0x0f }

// Symbol visibility (lower 2 bits of st_other).
func stVisibility(other uint8) uint8 { return other & 0x03 }

const (
	stbLOCAL  = 0
	stbGLOBAL = 1
	stbWEAK   = 2

	sttNOTYPE  = 0
	sttOBJECT  = 1
	sttFUNC    = 2
	sttSECTION = 3
	sttFILE    = 4
	sttCOMMON  = 5
	sttTLS     = 6
	sttGNU_IFUNC = 10

	stvDEFAULT   = 0
	stvINTERNAL  = 1
	stvHIDDEN    = 2
	stvPROTECTED = 3

	shnUNDEF = 0
	shnABS   = 0xfff1
	shnCOMMON = 0xfff2
)

// ──────────────────────────────────────────────────────────────────────────────
// Relocations
// ──────────────────────────────────────────────────────────────────────────────

type elf64Rela struct {
	Offset uint64
	Info   uint64
	Addend int64
}

const elf64RelaSize = 24

type elf64Rel struct {
	Offset uint64
	Info   uint64
}

const elf64RelSize = 16

func rSym(info uint64) uint32  { return uint32(info >> 32) }
func rType(info uint64) uint32 { return uint32(info & 0xffffffff) }

// ──────────────────────────────────────────────────────────────────────────────
// Versioning structures
// ──────────────────────────────────────────────────────────────────────────────

// Elf64_Verneed
type elf64Verneed struct {
	Version uint16
	Cnt     uint16
	File    uint32 // offset into strtab
	Aux     uint32 // offset of first Vernaux
	Next    uint32 // offset of next Verneed (0 = last)
}

// Elf64_Vernaux
type elf64Vernaux struct {
	Hash  uint32
	Flags uint16
	Other uint16 // version index
	Name  uint32 // offset into strtab
	Next  uint32 // offset of next Vernaux (0 = last)
}

// Elf64_Verdef
type elf64Verdef struct {
	Version  uint16
	Flags    uint16
	Ndx      uint16
	Cnt      uint16
	Hash     uint32
	Aux      uint32
	Next     uint32
}

// Elf64_Verdaux
type elf64Verdaux struct {
	Name uint32
	Next uint32
}

// Special version indices.
const (
	verFlagBase = 0x1

	verNdxLOCAL  = 0
	verNdxGLOBAL = 1
)

// ──────────────────────────────────────────────────────────────────────────────
// GNU hash table header
// ──────────────────────────────────────────────────────────────────────────────

// gnuHashHeader is the layout at the start of a DT_GNU_HASH section.
// After this header come:
//   bloom[maskwords] uint64
//   buckets[nbuckets] uint32
//   chains[] uint32          (indexed by symbol index - symndx)
type gnuHashHeader struct {
	Nbuckets  uint32
	Symndx    uint32
	Maskwords uint32
	Shift2    uint32
}

// ──────────────────────────────────────────────────────────────────────────────
// SysV hash table header
// ──────────────────────────────────────────────────────────────────────────────

type sysvHashHeader struct {
	Nbucket uint32
	Nchain  uint32
}
