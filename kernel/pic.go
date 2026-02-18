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

// 8259 PIC remapping.
// By default, PIC IRQs 0-7 map to vectors 8-15, which overlap with CPU exceptions.
// We remap them to vectors 32-47.

const (
	pic1Cmd  = 0x20
	pic1Data = 0x21
	pic2Cmd  = 0xA0
	pic2Data = 0xA1
)

// remapPIC remaps the 8259 PIC to use vectors 32-47.
// Called from assembly during early boot.
//
//go:nosplit
func remapPIC() {
	// Save masks
	mask1 := Inb(pic1Data)
	mask2 := Inb(pic2Data)

	// ICW1: begin initialization sequence
	Outb(pic1Cmd, 0x11)
	IoWait()
	Outb(pic2Cmd, 0x11)
	IoWait()

	// ICW2: vector offsets
	Outb(pic1Data, 0x20) // Master PIC: vectors 32-39
	IoWait()
	Outb(pic2Data, 0x28) // Slave PIC: vectors 40-47
	IoWait()

	// ICW3: cascade wiring
	Outb(pic1Data, 0x04) // Slave on IRQ2
	IoWait()
	Outb(pic2Data, 0x02) // Slave cascade identity
	IoWait()

	// ICW4: 8086 mode
	Outb(pic1Data, 0x01)
	IoWait()
	Outb(pic2Data, 0x01)
	IoWait()

	// Restore saved masks (mask all for now — we don't use hardware IRQs yet)
	_ = mask1
	_ = mask2
	Outb(pic1Data, 0xFF)
	Outb(pic2Data, 0xFF)
}
