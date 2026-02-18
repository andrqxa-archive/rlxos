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

// limineRequestBlock is a packed struct containing all Limine markers and
// requests. Using a single struct guarantees the Go linker keeps them in
// order: start_marker < requests < end_marker, as Limine requires.
// All values are statically initialized so they're present in the ELF binary
// on disk (Limine scans the binary before executing any code).
type limineRequestBlock struct {
	StartMarker [4]uint64
	Framebuffer LimineRequest
	Memmap      LimineRequest
	HHDM        LimineRequest
	BootInfo    LimineRequest
	ExecAddr    LimineRequest
	StackSize   LimineStackSizeRequest
	EndMarker   [2]uint64
}

var limineRequests = limineRequestBlock{
	StartMarker: [4]uint64{
		0xf6b8f4b39de7d1ae, 0xfab91a6940fcb9cf,
		0x785c6ed015d3e316, 0x181e920a7852b9d9,
	},
	Framebuffer: LimineRequest{
		ID: [4]uint64{limineCommonMagic0, limineCommonMagic1, 0x9d5827dcd881dd75, 0xa3148604f6fab11b},
	},
	Memmap: LimineRequest{
		ID: [4]uint64{limineCommonMagic0, limineCommonMagic1, 0x67cf3d9d378a806f, 0xe304acdfc50c3c62},
	},
	HHDM: LimineRequest{
		ID: [4]uint64{limineCommonMagic0, limineCommonMagic1, 0x48dcf1cb8ad2b852, 0x63984e959a98244b},
	},
	BootInfo: LimineRequest{
		ID: [4]uint64{limineCommonMagic0, limineCommonMagic1, 0xf55038d8e2a1202f, 0x279426fcf5f59740},
	},
	ExecAddr: LimineRequest{
		ID: [4]uint64{limineCommonMagic0, limineCommonMagic1, 0x71ba76863cc55f63, 0xb2644a48c516a487},
	},
	StackSize: LimineStackSizeRequest{
		ID:        [4]uint64{limineCommonMagic0, limineCommonMagic1, 0x224ef0460a8e8926, 0xe1cb0fc25f46ea3d},
		StackSize: 256 * 1024,
	},
	EndMarker: [2]uint64{0xadc0e0531bb10d03, 0x9572709f31764c62},
}

// Limine base revision — scanned independently by the bootloader.
// [2] is set to 0 by the bootloader if it supports our requested revision.
var baseRevision = [3]uint64{0xf9562b2d5c95a6c8, 0x6a7b384944536bdc, 2}

// Accessors

func getFramebufferResponse() *LimineFramebufferResponse {
	if limineRequests.Framebuffer.Response == nil {
		return nil
	}
	return (*LimineFramebufferResponse)(limineRequests.Framebuffer.Response)
}

func getMemmapResponse() *LimineMemmapResponse {
	if limineRequests.Memmap.Response == nil {
		return nil
	}
	return (*LimineMemmapResponse)(limineRequests.Memmap.Response)
}

func getHHDMResponse() *LimineHHDMResponse {
	if limineRequests.HHDM.Response == nil {
		return nil
	}
	return (*LimineHHDMResponse)(limineRequests.HHDM.Response)
}

func getBootloaderInfoResponse() *LimineBootloaderInfoResponse {
	if limineRequests.BootInfo.Response == nil {
		return nil
	}
	return (*LimineBootloaderInfoResponse)(limineRequests.BootInfo.Response)
}

func getExecutableAddressResponse() *LimineExecutableAddressResponse {
	if limineRequests.ExecAddr.Response == nil {
		return nil
	}
	return (*LimineExecutableAddressResponse)(limineRequests.ExecAddr.Response)
}

// keepRequests prevents dead-code elimination of our request struct.
//
//go:nosplit
//go:noinline
func keepRequests() {
	keepAlive(unsafe.Pointer(&limineRequests))
	keepAlive(unsafe.Pointer(&baseRevision))
}

//go:nosplit
//go:noinline
func keepAlive(p unsafe.Pointer) {}
