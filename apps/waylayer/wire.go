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

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"syscall"

	"avyos.dev/pkg/fs"
)

// clientConn wraps a Unix domain socket for one connected Wayland client.
type clientConn struct {
	socket  *net.UnixConn
	mu      sync.Mutex
	recvBuf []byte
	recvFDs []int
	closed  bool
}

func newClientConn(uc *net.UnixConn) *clientConn {
	return &clientConn{
		socket:  uc,
		recvBuf: make([]byte, 4096),
	}
}

func (c *clientConn) close() error {
	c.closed = true
	return c.socket.Close()
}

// sendMsg sends a Wayland event message to the client.
func (c *clientConn) sendMsg(objectID uint32, opcode uint16, payload []byte, fds ...int) error {
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

func (c *clientConn) sendWithFDs(data []byte, fds []int) error {
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

// recvMsg reads the next Wayland request message from the client.
func (c *clientConn) recvMsg() (uint32, uint16, []byte, []int, error) {
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

	payloadSize := size - 8
	var payload []byte
	if payloadSize > 0 {
		payload = make([]byte, payloadSize)
		moreFDs, err := c.readFull(payload)
		if err != nil {
			return 0, 0, nil, nil, fmt.Errorf("read payload: %w", err)
		}
		fds = append(fds, moreFDs...)
	}

	return objectID, opcode, payload, fds, nil
}

func (c *clientConn) readFull(buf []byte) ([]int, error) {
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

func (c *clientConn) recvWithFDs(buf []byte) (int, []int, error) {
	rawConn, err := c.socket.SyscallConn()
	if err != nil {
		return 0, nil, err
	}

	oob := make([]byte, syscall.CmsgLen(4*16))
	var n, oobn int
	var recvErr error

	err = rawConn.Read(func(fd uintptr) bool {
		n, oobn, _, _, recvErr = syscall.Recvmsg(int(fd), buf, oob, 0)
		if recvErr == syscall.EAGAIN {
			return false
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

// listen creates a Wayland server socket.
func listen() (*net.UnixListener, string, error) {
	runtimeDir := fs.Resolve("user:")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return nil, "", fmt.Errorf("ensure XDG_RUNTIME_DIR %q: %w", runtimeDir, err)
	}

	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		display = "dev.avyos.waylayer"
	}
	socketPath := runtimeDir + "/" + display

	_ = os.Remove(socketPath)

	addr, err := net.ResolveUnixAddr("unix", socketPath)
	if err != nil {
		return nil, "", err
	}

	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, "", err
	}

	return ln, display, nil
}

// --- Payload encoding helpers ---

func putUint32(buf []byte, offset int, v uint32) {
	binary.LittleEndian.PutUint32(buf[offset:offset+4], v)
}

func putInt32(buf []byte, offset int, v int32) {
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(v))
}

func getUint32(data []byte, offset int) uint32 {
	return binary.LittleEndian.Uint32(data[offset : offset+4])
}

func getInt32(data []byte, offset int) int32 {
	return int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
}

func encodeString(s string) []byte {
	sLen := len(s) + 1
	padded := (sLen + 3) & ^3
	buf := make([]byte, 4+padded)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(sLen))
	copy(buf[4:], s)
	return buf
}

func getString(data []byte, offset int) (string, int) {
	sLen := int(getUint32(data, offset))
	if sLen <= 0 {
		return "", 4
	}
	s := string(data[offset+4 : offset+4+sLen-1])
	padded := (sLen + 3) & ^3
	return s, 4 + padded
}

func intToFixed(v int) int32 {
	return int32(v) << 8
}

func closeFDs(fds []int) {
	for _, fd := range fds {
		syscall.Close(fd)
	}
}
