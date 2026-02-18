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

// COM1 serial port driver for early debug output.

const (
	com1Port = 0x3F8

	// Register offsets
	serialData = 0 // Data register (read/write)
	serialIER  = 1 // Interrupt Enable Register
	serialFIFO = 2 // FIFO Control Register
	serialLCR  = 3 // Line Control Register
	serialMCR  = 4 // Modem Control Register
	serialLSR  = 5 // Line Status Register

	// Line Status Register bits
	serialLSRTHRE = 0x20 // Transmitter Holding Register Empty
)

// initSerial configures COM1 at 115200 baud, 8N1.
// Called from assembly during early boot, before Go runtime init.
//
//go:nosplit
func initSerial() {
	port := uint16(com1Port)

	Outb(port+serialIER, 0x00)  // Disable all interrupts
	Outb(port+serialLCR, 0x80)  // Enable DLAB (set baud rate divisor)
	Outb(port+serialData, 0x01) // Divisor low byte: 115200 baud
	Outb(port+serialIER, 0x00)  // Divisor high byte
	Outb(port+serialLCR, 0x03)  // 8 bits, no parity, one stop bit
	Outb(port+serialFIFO, 0xC7) // Enable FIFO, clear, 14-byte threshold
	Outb(port+serialMCR, 0x0B)  // IRQs enabled, RTS/DSR set
}

// serialTransmitReady returns true when the transmit buffer is empty.
//
//go:nosplit
func serialTransmitReady() bool {
	return Inb(com1Port+serialLSR)&serialLSRTHRE != 0
}

// serialWriteByte sends a single byte over COM1.
//
//go:nosplit
func serialWriteByte(b byte) {
	// Avoid hard deadlock if UART is unavailable/misconfigured.
	for i := 0; i < 1000000; i++ {
		if serialTransmitReady() {
			Outb(com1Port+serialData, b)
			return
		}
	}
}

// serialPrint writes a string to COM1.
//
//go:nosplit
func serialPrint(s string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			serialWriteByte('\r')
		}
		serialWriteByte(s[i])
	}
}

// serialPrintHex writes a 64-bit hex value to COM1.
//
//go:nosplit
func serialPrintHex(v uint64) {
	serialPrint("0x")
	for i := 60; i >= 0; i -= 4 {
		nibble := (v >> uint(i)) & 0xF
		if nibble < 10 {
			serialWriteByte(byte('0' + nibble))
		} else {
			serialWriteByte(byte('a' + nibble - 10))
		}
	}
}

// serialPrintDec writes a decimal integer to COM1.
//
//go:nosplit
func serialPrintDec(v uint64) {
	if v == 0 {
		serialWriteByte('0')
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
		serialWriteByte(buf[i])
	}
}
