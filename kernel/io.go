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

// Port I/O functions - implemented in io_amd64.s

func Outb(port uint16, val uint8)
func Inb(port uint16) uint8
func Outw(port uint16, val uint16)
func Inw(port uint16) uint16
func Outl(port uint16, val uint32)
func Inl(port uint16) uint32
func IoWait()
