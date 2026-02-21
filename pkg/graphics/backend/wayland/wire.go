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

package wayland

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"
)

// conn wraps a Unix domain socket for the Wayland wire protocol.
// Messages are: object_id (uint32), size_and_opcode (uint32), payload...
// Size includes the 8-byte header. Opcode is in the lower 16 bits of size_and_opcode.
type conn struct {
	socket  *net.UnixConn
	mu      sync.Mutex
	recvBuf []byte
	recvFDs []int
}

// dial connects to the Wayland compositor socket.
func dial() (*conn, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return nil, fmt.Errorf("XDG_RUNTIME_DIR not set")
	}

	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		display = "waylayer"
	}

	sockPath := runtimeDir + "/" + display

	addr, err := net.ResolveUnixAddr("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", sockPath, err)
	}

	uc, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", sockPath, err)
	}

	return &conn{
		socket:  uc,
		recvBuf: make([]byte, 4096),
	}, nil
}

// close closes the connection.
func (c *conn) close() error {
	return c.socket.Close()
}

// sendMsg sends a Wayland message with optional file descriptors.
func (c *conn) sendMsg(objectID uint32, opcode uint16, payload []byte, fds ...int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	totalSize := 8 + len(payload)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], objectID)
	binary.LittleEndian.PutUint32(header[4:8], uint32(totalSize)<<16|uint32(opcode))

	msg := make([]byte, 0, totalSize)
	msg = append(msg, header...)
	msg = append(msg, payload...)

	if len(fds) > 0 {
		return c.sendWithFDs(msg, fds)
	}

	_, err := c.socket.Write(msg)
	return err
}

// sendWithFDs sends data with file descriptors using sendmsg.
func (c *conn) sendWithFDs(data []byte, fds []int) error {
	rawConn, err := c.socket.SyscallConn()
	if err != nil {
		return err
	}

	rights := syscall.UnixRights(fds...)

	var sendErr error
	err = rawConn.Control(func(fd uintptr) {
		sendErr = syscall.Sendmsg(int(fd), data, rights, nil, 0)
	})
	if err != nil {
		return err
	}
	return sendErr
}

// recvMsg reads the next Wayland message from the socket.
// Returns object ID, opcode, payload, and any received file descriptors.
func (c *conn) recvMsg() (uint32, uint16, []byte, []int, error) {
	// Read header (8 bytes)
	header := make([]byte, 8)
	fds, err := c.readFull(header)
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("read header: %w", err)
	}

	objectID := binary.LittleEndian.Uint32(header[0:4])
	sizeOpcode := binary.LittleEndian.Uint32(header[4:8])
	size := int(sizeOpcode >> 16)
	opcode := uint16(sizeOpcode & 0xFFFF)

	if size < 8 {
		return 0, 0, nil, nil, fmt.Errorf("invalid message size %d", size)
	}

	// Read payload
	payloadSize := size - 8
	var payload []byte
	var moreFDs []int
	if payloadSize > 0 {
		payload = make([]byte, payloadSize)
		moreFDs, err = c.readFull(payload)
		if err != nil {
			return 0, 0, nil, nil, fmt.Errorf("read payload: %w", err)
		}
		fds = append(fds, moreFDs...)
	}

	return objectID, opcode, payload, fds, nil
}

// readFull reads exactly len(buf) bytes, collecting any passed file descriptors.
func (c *conn) readFull(buf []byte) ([]int, error) {
	// If we have leftover FDs from a previous read, return them
	if len(c.recvFDs) > 0 && len(buf) == 0 {
		fds := c.recvFDs
		c.recvFDs = nil
		return fds, nil
	}

	var allFDs []int
	offset := 0

	for offset < len(buf) {
		n, fds, err := c.recvWithFDs(buf[offset:])
		if err != nil {
			return allFDs, err
		}
		if n == 0 {
			return allFDs, fmt.Errorf("connection closed")
		}
		offset += n
		allFDs = append(allFDs, fds...)
	}

	return allFDs, nil
}

// recvWithFDs reads data and file descriptors using recvmsg.
func (c *conn) recvWithFDs(buf []byte) (int, []int, error) {
	rawConn, err := c.socket.SyscallConn()
	if err != nil {
		return 0, nil, err
	}

	oob := make([]byte, syscall.CmsgLen(4*16)) // room for up to 16 fds
	var n, oobn int
	var recvErr error

	err = rawConn.Read(func(fd uintptr) bool {
		n, oobn, _, _, recvErr = syscall.Recvmsg(int(fd), buf, oob, 0)
		if recvErr == syscall.EAGAIN {
			return false // tell rawConn.Read to retry
		}
		return true
	})
	if err != nil {
		return 0, nil, err
	}
	if recvErr != nil {
		return 0, nil, recvErr
	}

	var fds []int
	if oobn > 0 {
		msgs, err := syscall.ParseSocketControlMessage(oob[:oobn])
		if err == nil {
			for _, msg := range msgs {
				parsedFDs, err := syscall.ParseUnixRights(&msg)
				if err == nil {
					fds = append(fds, parsedFDs...)
				}
			}
		}
	}

	return n, fds, nil
}

// --- Payload encoding helpers ---

// putUint32 appends a uint32 to a byte slice.
func putUint32(buf []byte, offset int, v uint32) {
	binary.LittleEndian.PutUint32(buf[offset:offset+4], v)
}

// putInt32 appends an int32 to a byte slice.
func putInt32(buf []byte, offset int, v int32) {
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(v))
}

// encodeString encodes a Wayland string: uint32 length (including NUL), then
// the string bytes, a NUL terminator, and padding to 4-byte alignment.
func encodeString(s string) []byte {
	sLen := len(s) + 1 // include NUL
	padded := (sLen + 3) & ^3
	buf := make([]byte, 4+padded)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(sLen))
	copy(buf[4:], s)
	// NUL and padding are already zero
	return buf
}

// getUint32 reads a uint32 from a byte slice at the given offset.
func getUint32(data []byte, offset int) uint32 {
	return binary.LittleEndian.Uint32(data[offset : offset+4])
}

// getInt32 reads an int32 from a byte slice at the given offset.
func getInt32(data []byte, offset int) int32 {
	return int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
}

// getString reads a Wayland string from a byte slice at the given offset.
// Returns the string and the number of bytes consumed (including length prefix and padding).
func getString(data []byte, offset int) (string, int) {
	sLen := int(getUint32(data, offset))
	if sLen <= 0 {
		return "", 4
	}
	s := string(data[offset+4 : offset+4+sLen-1]) // exclude NUL
	padded := (sLen + 3) & ^3
	return s, 4 + padded
}

// fixedToFloat converts a wl_fixed_t (24.8 fixed-point) to float64.
func fixedToFloat(v int32) float64 {
	return float64(v) / 256.0
}

// fixedToInt converts a wl_fixed_t to int by rounding.
func fixedToInt(v int32) int {
	return int(v) >> 8
}

// syscallClose closes a file descriptor (best-effort).
func syscallClose(fd int) {
	syscall.Close(fd)
}
