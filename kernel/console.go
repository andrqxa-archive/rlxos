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

// Framebuffer console — renders text using a built-in 8x16 bitmap font.

const (
	fontWidth  = 8
	fontHeight = 16
)

// Console state
var (
	fbAddr   uintptr
	fbWidth  uint64
	fbHeight uint64
	fbPitch  uint64
	fbBpp    uint16

	conCols   int
	conRows   int
	conCurX   int // current column (character)
	conCurY   int // current row (character)
	conFG     uint32
	conBG     uint32
	conReady  bool
)

// initConsole sets up the framebuffer console from a Limine framebuffer response.
func initConsole(fb *LimineFramebuffer) {
	if fb == nil {
		return
	}
	fbAddr = uintptr(fb.Address)
	fbWidth = fb.Width
	fbHeight = fb.Height
	fbPitch = fb.Pitch
	fbBpp = fb.Bpp

	conCols = int(fbWidth) / fontWidth
	conRows = int(fbHeight) / fontHeight
	conCurX = 0
	conCurY = 0
	conFG = 0xFFCCCCCC // light gray
	conBG = 0xFF1A1A2E // dark blue-black

	// Clear screen
	clearScreen()
	conReady = true
}

// clearScreen fills the framebuffer with the background color.
func clearScreen() {
	if fbAddr == 0 {
		return
	}
	for y := uint64(0); y < fbHeight; y++ {
		row := (*[1 << 20]uint32)(unsafe.Pointer(fbAddr + uintptr(y*fbPitch)))
		for x := uint64(0); x < fbWidth; x++ {
			row[x] = conBG
		}
	}
}

// putPixel writes a single 32-bit pixel to the framebuffer.
//
//go:nosplit
func putPixel(x, y int, color uint32) {
	if x < 0 || y < 0 || uint64(x) >= fbWidth || uint64(y) >= fbHeight {
		return
	}
	offset := uintptr(y)*uintptr(fbPitch) + uintptr(x)*4
	*(*uint32)(unsafe.Pointer(fbAddr + offset)) = color
}

// drawChar renders a single character at character grid position (cx, cy).
func drawChar(cx, cy int, ch byte) {
	if int(ch) >= len(font8x16) {
		ch = '?'
	}
	glyph := font8x16[ch]
	px := cx * fontWidth
	py := cy * fontHeight

	for row := 0; row < fontHeight; row++ {
		bits := glyph[row]
		for col := 0; col < fontWidth; col++ {
			color := conBG
			if bits&(0x80>>uint(col)) != 0 {
				color = conFG
			}
			putPixel(px+col, py+row, color)
		}
	}
}

// scrollUp scrolls the console up by one line.
func scrollUp() {
	// Move pixel rows up by fontHeight
	lineBytes := uintptr(fbPitch)
	src := fbAddr + uintptr(fontHeight)*lineBytes
	dst := fbAddr
	copyLen := uintptr(fbHeight-uint64(fontHeight)) * lineBytes

	// Copy line by line
	srcP := (*[1 << 30]byte)(unsafe.Pointer(src))
	dstP := (*[1 << 30]byte)(unsafe.Pointer(dst))
	for i := uintptr(0); i < copyLen; i++ {
		dstP[i] = srcP[i]
	}

	// Clear the last line
	lastY := int(fbHeight) - fontHeight
	for y := lastY; y < int(fbHeight); y++ {
		row := (*[1 << 20]uint32)(unsafe.Pointer(fbAddr + uintptr(y)*uintptr(fbPitch)))
		for x := uint64(0); x < fbWidth; x++ {
			row[x] = conBG
		}
	}
}

// conPutChar writes a character to the console at the cursor position,
// advancing the cursor and scrolling as needed.
func conPutChar(ch byte) {
	switch ch {
	case '\n':
		conCurX = 0
		conCurY++
	case '\r':
		conCurX = 0
	case '\t':
		conCurX = (conCurX + 4) &^ 3
	default:
		if conCurX >= conCols {
			conCurX = 0
			conCurY++
		}
		drawChar(conCurX, conCurY, ch)
		conCurX++
	}

	// Scroll if past bottom
	for conCurY >= conRows {
		scrollUp()
		conCurY--
	}
}

// conPrint writes a string to the framebuffer console and serial port.
func conPrint(s string) {
	serialPrint(s)
	if !conReady {
		return
	}
	for i := 0; i < len(s); i++ {
		conPutChar(s[i])
	}
}

// conPrintHex writes a hex value to both console and serial.
func conPrintHex(v uint64) {
	serialPrintHex(v)
	if !conReady {
		return
	}
	conPutChar('0')
	conPutChar('x')
	for i := 60; i >= 0; i -= 4 {
		nibble := (v >> uint(i)) & 0xF
		if nibble < 10 {
			conPutChar(byte('0' + nibble))
		} else {
			conPutChar(byte('a' + nibble - 10))
		}
	}
}

// conPrintDec writes a decimal integer to both console and serial.
func conPrintDec(v uint64) {
	serialPrintDec(v)
	if !conReady {
		return
	}
	if v == 0 {
		conPutChar('0')
		return
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	for ; i < len(buf); i++ {
		conPutChar(buf[i])
	}
}
