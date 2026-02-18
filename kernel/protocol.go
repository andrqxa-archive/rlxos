/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import "unsafe"

// Limine protocol magic constants
const (
	limineCommonMagic0 = 0xc7b1dd30df4c8b88
	limineCommonMagic1 = 0x0a82e883a194f07b
)

// Memory map entry types
const (
	LimineMemmapUsable              = 0
	LimineMemmapReserved            = 1
	LimineMemmapACPIReclaimable     = 2
	LimineMemmapACPINVS             = 3
	LimineMemmapBadMemory           = 4
	LimineMemmapBootloaderReclaim   = 5
	LimineMemmapExecutableAndModule = 6
	LimineMemmapFramebuffer         = 7
	LimineMemmapACPITables          = 8
)

// Framebuffer memory model
const (
	LimineFramebufferRGB = 1
)

// LimineUUID matches struct limine_uuid
type LimineUUID struct {
	A uint32
	B uint16
	C uint16
	D [8]byte
}

// LimineFile matches struct limine_file
type LimineFile struct {
	Revision      uint64
	Address       unsafe.Pointer
	Size          uint64
	Path          *byte
	String        *byte
	MediaType     uint32
	Unused        uint32
	TFTPIp        uint32
	TFTPPort      uint32
	PartitionIdx  uint32
	MBRDiskID     uint32
	GPTDiskUUID   LimineUUID
	GPTPartUUID   LimineUUID
	PartUUID      LimineUUID
}

// LimineVideoMode matches struct limine_video_mode
type LimineVideoMode struct {
	Pitch         uint64
	Width         uint64
	Height        uint64
	Bpp           uint16
	MemoryModel   uint8
	RedMaskSize   uint8
	RedMaskShift  uint8
	GreenMaskSize uint8
	GreenMaskShift uint8
	BlueMaskSize  uint8
	BlueMaskShift uint8
}

// LimineFramebuffer matches struct limine_framebuffer
type LimineFramebuffer struct {
	Address        unsafe.Pointer
	Width          uint64
	Height         uint64
	Pitch          uint64
	Bpp            uint16
	MemoryModel    uint8
	RedMaskSize    uint8
	RedMaskShift   uint8
	GreenMaskSize  uint8
	GreenMaskShift uint8
	BlueMaskSize   uint8
	BlueMaskShift  uint8
	Unused         [7]uint8
	EDIDSize       uint64
	EDID           unsafe.Pointer
	ModeCount      uint64
	Modes          **LimineVideoMode
}

// LimineFramebufferResponse matches struct limine_framebuffer_response
type LimineFramebufferResponse struct {
	Revision         uint64
	FramebufferCount uint64
	Framebuffers     **LimineFramebuffer
}

// LimineMemmapEntry matches struct limine_memmap_entry
type LimineMemmapEntry struct {
	Base   uint64
	Length uint64
	Type   uint64
}

// LimineMemmapResponse matches struct limine_memmap_response
type LimineMemmapResponse struct {
	Revision   uint64
	EntryCount uint64
	Entries    **LimineMemmapEntry
}

// LimineHHDMResponse matches struct limine_hhdm_response
type LimineHHDMResponse struct {
	Revision uint64
	Offset   uint64
}

// LimineBootloaderInfoResponse matches struct limine_bootloader_info_response
type LimineBootloaderInfoResponse struct {
	Revision uint64
	Name     *byte
	Version  *byte
}

// LimineExecutableAddressResponse matches struct limine_executable_address_response
type LimineExecutableAddressResponse struct {
	Revision     uint64
	PhysicalBase uint64
	VirtualBase  uint64
}

// LimineEntryPointResponse matches struct limine_entry_point_response
type LimineEntryPointResponse struct {
	Revision uint64
}

// LimineDateAtBootResponse matches struct limine_date_at_boot_response
type LimineDateAtBootResponse struct {
	Revision  uint64
	Timestamp int64
}

// LimineRequest is the common request header (id[4] + revision + response pointer)
type LimineRequest struct {
	ID       [4]uint64
	Revision uint64
	Response unsafe.Pointer
}

// LimineStackSizeRequest has an extra stack_size field
type LimineStackSizeRequest struct {
	ID        [4]uint64
	Revision  uint64
	Response  unsafe.Pointer
	StackSize uint64
}

// Helper to read a null-terminated C string from a pointer
//
//go:nosplit
func cstring(p *byte) string {
	if p == nil {
		return ""
	}
	var n int
	for ptr := p; *ptr != 0; ptr = (*byte)(unsafe.Add(unsafe.Pointer(ptr), 1)) {
		n++
	}
	return unsafe.String(p, n)
}

