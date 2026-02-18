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

// Package sutra provides an IPC framework for inter-process communication.
package sutra

import (
	"encoding/binary"
	"errors"
	"io"
)

// Well-known endpoint IDs.
const (
	IDBus       uint32 = 0          // Deprecated bus ID (kept for compatibility)
	IDService   uint32 = 0x00000001 // Service endpoint ID
	IDClient    uint32 = 0x80000000 // Client ID range start
	IDBroadcast uint32 = 0xFFFFFFFF // Broadcast to all
)

// Event types. Bus events are deprecated and kept for compatibility.
const (
	EventRegister    uint16 = 0x0001 // Deprecated
	EventUnregister  uint16 = 0x0002 // Deprecated
	EventConnect     uint16 = 0x0003 // Client connected
	EventDisconnect  uint16 = 0x0004 // Client disconnected
	EventPing        uint16 = 0x0005 // Deprecated
	EventPong        uint16 = 0x0006 // Deprecated
	EventError       uint16 = 0x0007 // Error response
	EventLookup      uint16 = 0x0008 // Deprecated
	EventLookupReply uint16 = 0x0009 // Deprecated
	EventUserBase    uint16 = 0x0100 // Start of user-defined events

	// Service control events (init system)
	EventInitServiceStart    uint16 = 0x0110 // Start a service
	EventInitServiceStop     uint16 = 0x0111 // Stop a service
	EventInitServiceRestart  uint16 = 0x0112 // Restart a service
	EventInitServiceStatus   uint16 = 0x0113 // Get service status
	EventInitServiceList     uint16 = 0x0114 // List all services
	EventInitServicePoweroff uint16 = 0x115  // Power off system
	EventInitServiceReboot   uint16 = 0x116  // Reboot system
)

// HeaderSize is the size of the transaction header in bytes
const HeaderSize = 12 // 4 + 4 + 2 + 2

// MaxPayloadSize is the maximum payload size
const MaxPayloadSize = 65535

// Transaction represents a message in the Sutra IPC system.
// Header format: [Sender:4][Destination:4][Event:2][Size:2][Payload:Size]
type Transaction struct {
	Sender      uint32 // Sender ID
	Destination uint32 // Destination ID (or broadcast)
	Event       uint16 // Event type
	Payload     []byte // Variable-length payload
}

// Size returns the payload size
func (t *Transaction) Size() uint16 {
	return uint16(len(t.Payload))
}

// TotalSize returns the total transaction size including header
func (t *Transaction) TotalSize() int {
	return HeaderSize + len(t.Payload)
}

// Encode writes the transaction to a byte slice
func (t *Transaction) Encode() []byte {
	size := t.Size()
	buf := make([]byte, HeaderSize+int(size))

	binary.LittleEndian.PutUint32(buf[0:4], t.Sender)
	binary.LittleEndian.PutUint32(buf[4:8], t.Destination)
	binary.LittleEndian.PutUint16(buf[8:10], t.Event)
	binary.LittleEndian.PutUint16(buf[10:12], size)

	if size > 0 {
		copy(buf[12:], t.Payload)
	}

	return buf
}

// WriteTo writes the transaction to an io.Writer
func (t *Transaction) WriteTo(w io.Writer) (int64, error) {
	data := t.Encode()
	total := 0
	for total < len(data) {
		n, err := w.Write(data[total:])
		total += n
		if err != nil {
			return int64(total), err
		}
		if n == 0 {
			return int64(total), io.ErrUnexpectedEOF
		}
	}
	return int64(total), nil
}

// DecodeTransaction decodes a transaction from a byte slice
func DecodeTransaction(data []byte) (*Transaction, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("data too short for transaction header")
	}

	size := binary.LittleEndian.Uint16(data[10:12])
	if len(data) < HeaderSize+int(size) {
		return nil, errors.New("data too short for payload")
	}

	t := &Transaction{
		Sender:      binary.LittleEndian.Uint32(data[0:4]),
		Destination: binary.LittleEndian.Uint32(data[4:8]),
		Event:       binary.LittleEndian.Uint16(data[8:10]),
	}

	if size > 0 {
		t.Payload = make([]byte, size)
		copy(t.Payload, data[12:12+size])
	}

	return t, nil
}

// ReadTransaction reads a transaction from an io.Reader
func ReadTransaction(r io.Reader) (*Transaction, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	size := binary.LittleEndian.Uint16(header[10:12])

	t := &Transaction{
		Sender:      binary.LittleEndian.Uint32(header[0:4]),
		Destination: binary.LittleEndian.Uint32(header[4:8]),
		Event:       binary.LittleEndian.Uint16(header[8:10]),
	}

	if size > 0 {
		t.Payload = make([]byte, size)
		if _, err := io.ReadFull(r, t.Payload); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// NewTransaction creates a new transaction
func NewTransaction(sender, dest uint32, event uint16, payload []byte) *Transaction {
	return &Transaction{
		Sender:      sender,
		Destination: dest,
		Event:       event,
		Payload:     payload,
	}
}

// Reply creates a reply transaction to this one
func (t *Transaction) Reply(event uint16, payload []byte) *Transaction {
	return &Transaction{
		Sender:      t.Destination,
		Destination: t.Sender,
		Event:       event,
		Payload:     payload,
	}
}

// Error creates an error reply
func (t *Transaction) Error(message string) *Transaction {
	return t.Reply(EventError, []byte(message))
}

// PayloadString returns payload as string
func (t *Transaction) PayloadString() string {
	return string(t.Payload)
}

// PayloadUint32 returns first 4 bytes of payload as uint32
func (t *Transaction) PayloadUint32() uint32 {
	if len(t.Payload) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(t.Payload)
}

// EncodeUint32 encodes a uint32 as payload bytes
func EncodeUint32(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

// EncodeString encodes a string with length prefix
func EncodeString(s string) []byte {
	buf := make([]byte, 2+len(s))
	binary.LittleEndian.PutUint16(buf[0:2], uint16(len(s)))
	copy(buf[2:], s)
	return buf
}

// DecodeString decodes a length-prefixed string
func DecodeString(data []byte) (string, int) {
	if len(data) < 2 {
		return "", 0
	}
	length := int(binary.LittleEndian.Uint16(data[0:2]))
	if len(data) < 2+length {
		return "", 0
	}
	return string(data[2 : 2+length]), 2 + length
}
