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

package sutra

import (
	"encoding/binary"
	"errors"
	"math"
)

// Encoder writes primitive values into a binary buffer.
// All integers are little-endian. Strings are length-prefixed (2-byte LE length).
type Encoder struct {
	buf []byte
}

// NewEncoder creates an encoder with the given initial capacity hint.
func NewEncoder(capacity int) *Encoder {
	return &Encoder{buf: make([]byte, 0, capacity)}
}

func (e *Encoder) PutUint8(v uint8) {
	e.buf = append(e.buf, v)
}

func (e *Encoder) PutUint16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	e.buf = append(e.buf, b[:]...)
}

func (e *Encoder) PutUint32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	e.buf = append(e.buf, b[:]...)
}

func (e *Encoder) PutUint64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	e.buf = append(e.buf, b[:]...)
}

func (e *Encoder) PutInt(v int) {
	e.PutUint64(uint64(int64(v)))
}

func (e *Encoder) PutInt32(v int32) {
	e.PutUint32(uint32(v))
}

func (e *Encoder) PutBool(v bool) {
	if v {
		e.buf = append(e.buf, 1)
	} else {
		e.buf = append(e.buf, 0)
	}
}

func (e *Encoder) PutString(v string) {
	n := len(v)
	if n > math.MaxUint16 {
		n = math.MaxUint16
	}
	e.PutUint16(uint16(n))
	e.buf = append(e.buf, v[:n]...)
}

func (e *Encoder) PutRune(v rune) {
	e.PutInt32(int32(v))
}

func (e *Encoder) PutBytes(v []byte) {
	e.PutUint32(uint32(len(v)))
	e.buf = append(e.buf, v...)
}

// Bytes returns the encoded buffer.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// Decoder reads primitive values from a binary buffer.
// It uses a sticky-error pattern: after the first error, all reads return zero values.
type Decoder struct {
	data []byte
	off  int
	err  error
}

// NewDecoder creates a decoder over the given byte slice.
func NewDecoder(data []byte) *Decoder {
	return &Decoder{data: data}
}

var errShortRead = errors.New("sutra: short binary payload")

func (d *Decoder) need(n int) bool {
	if d.err != nil {
		return false
	}
	if d.off+n > len(d.data) {
		d.err = errShortRead
		return false
	}
	return true
}

func (d *Decoder) Uint8() uint8 {
	if !d.need(1) {
		return 0
	}
	v := d.data[d.off]
	d.off++
	return v
}

func (d *Decoder) Uint16() uint16 {
	if !d.need(2) {
		return 0
	}
	v := binary.LittleEndian.Uint16(d.data[d.off:])
	d.off += 2
	return v
}

func (d *Decoder) Uint32() uint32 {
	if !d.need(4) {
		return 0
	}
	v := binary.LittleEndian.Uint32(d.data[d.off:])
	d.off += 4
	return v
}

func (d *Decoder) Uint64() uint64 {
	if !d.need(8) {
		return 0
	}
	v := binary.LittleEndian.Uint64(d.data[d.off:])
	d.off += 8
	return v
}

func (d *Decoder) Int() int {
	return int(int64(d.Uint64()))
}

func (d *Decoder) Int32() int32 {
	return int32(d.Uint32())
}

func (d *Decoder) Bool() bool {
	return d.Uint8() != 0
}

func (d *Decoder) String() string {
	n := int(d.Uint16())
	if !d.need(n) {
		return ""
	}
	s := string(d.data[d.off : d.off+n])
	d.off += n
	return s
}

func (d *Decoder) Rune() rune {
	return rune(d.Int32())
}

func (d *Decoder) Bytes() []byte {
	n := int(d.Uint32())
	if !d.need(n) {
		return nil
	}
	b := make([]byte, n)
	copy(b, d.data[d.off:d.off+n])
	d.off += n
	return b
}

// Err returns the first error encountered during decoding, or nil.
func (d *Decoder) Err() error {
	return d.err
}
