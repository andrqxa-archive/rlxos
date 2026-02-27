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
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	graphics "avyos.dev/pkg/graphics/input"
)

// WL_SHM_FORMAT_ARGB8888 = 0 (BGRA in memory on little-endian)
const shmFormatARGB8888 = 0

// wl_shm opcodes
const (
	shmCreatePoolOp = 0
)

// wl_shm_pool opcodes
const (
	shmPoolCreateBufferOp = 0
	shmPoolDestroyOp      = 1
)

// wl_buffer events
const (
	bufferReleaseEvent = 0
)

// shmPool manages a shared memory pool for pixel buffers.
type shmPool struct {
	cl      *client
	poolID  uint32
	fd      int
	data    []byte
	size    int
	buffers [2]*shmBuffer
	current int // index of the back buffer
}

// shmBuffer represents one wl_buffer backed by shared memory.
type shmBuffer struct {
	id       uint32
	offset   int
	width    int
	height   int
	stride   int
	released bool
	buf      *graphics.Buffer
}

// memfdCreate creates an anonymous file via memfd_create syscall.
func memfdCreate(name string) (int, error) {
	nameBytes := append([]byte(name), 0)

	var sysNum uintptr
	switch runtime.GOARCH {
	case "amd64":
		sysNum = 319
	case "arm64":
		sysNum = 279
	case "386":
		sysNum = 356
	default:
		sysNum = 319 // best guess
	}

	fd, _, errno := syscall.Syscall(
		sysNum,
		uintptr(unsafe.Pointer(&nameBytes[0])),
		1, // MFD_CLOEXEC
		0,
	)
	if errno != 0 {
		return -1, fmt.Errorf("memfd_create: %w", errno)
	}
	return int(fd), nil
}

// newShmPool creates a shared memory pool for double-buffered rendering.
func newShmPool(cl *client, width, height int) (*shmPool, error) {
	stride := width * 4
	bufSize := stride * height
	totalSize := bufSize * 2 // double buffer

	fd, err := memfdCreate("graphics-wl-shm")
	if err != nil {
		return nil, err
	}

	if err := syscall.Ftruncate(fd, int64(totalSize)); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("ftruncate: %w", err)
	}

	data, err := syscall.Mmap(fd, 0, totalSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("mmap: %w", err)
	}

	pool := &shmPool{
		cl:   cl,
		fd:   fd,
		data: data,
		size: totalSize,
	}

	// Create wl_shm_pool
	pool.poolID = cl.allocID()
	payload := make([]byte, 8)
	putUint32(payload, 0, pool.poolID)
	putInt32(payload, 4, int32(totalSize))
	if err := cl.conn.sendMsg(cl.shm, shmCreatePoolOp, payload, fd); err != nil {
		pool.destroy()
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Create two buffers from the pool
	for i := 0; i < 2; i++ {
		offset := i * bufSize
		buf := &shmBuffer{
			id:       cl.allocID(),
			offset:   offset,
			width:    width,
			height:   height,
			stride:   stride,
			released: true,
		}
		buf.buf = &graphics.Buffer{
			Width:  width,
			Height: height,
			Stride: stride,
			Format: graphics.PixelFormatBGRA,
			Data:   data[offset : offset+bufSize],
		}

		// wl_shm_pool.create_buffer(id, offset, width, height, stride, format)
		p := make([]byte, 24)
		putUint32(p, 0, buf.id)
		putInt32(p, 4, int32(offset))
		putInt32(p, 8, int32(width))
		putInt32(p, 12, int32(height))
		putInt32(p, 16, int32(stride))
		putUint32(p, 20, shmFormatARGB8888)
		if err := cl.conn.sendMsg(pool.poolID, shmPoolCreateBufferOp, p); err != nil {
			pool.destroy()
			return nil, fmt.Errorf("create buffer %d: %w", i, err)
		}

		cl.setHandler(buf.id, func(_ uint32, opcode uint16, _ []byte, fds []int) {
			closeFDs(fds)
			if opcode == bufferReleaseEvent {
				buf.released = true
			}
		})

		pool.buffers[i] = buf
	}

	return pool, nil
}

// backBuffer returns the current back buffer for drawing.
func (p *shmPool) backBuffer() *shmBuffer {
	return p.buffers[p.current]
}

// swap swaps the front and back buffers.
func (p *shmPool) swap() {
	p.current = 1 - p.current
}

// resize recreates the pool and buffers for a new size.
func (p *shmPool) resize(width, height int) error {
	if width == p.buffers[0].width && height == p.buffers[0].height {
		return nil
	}

	// Remove old buffer handlers
	for _, buf := range p.buffers {
		if buf != nil {
			p.cl.removeHandler(buf.id)
		}
	}

	// Unmap old memory
	if p.data != nil {
		syscall.Munmap(p.data)
	}

	stride := width * 4
	bufSize := stride * height
	totalSize := bufSize * 2

	if err := syscall.Ftruncate(p.fd, int64(totalSize)); err != nil {
		return fmt.Errorf("ftruncate: %w", err)
	}

	data, err := syscall.Mmap(p.fd, 0, totalSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("mmap: %w", err)
	}

	p.data = data
	p.size = totalSize

	// Resize the pool on the server
	// wl_shm_pool.resize is not always needed since we recreate buffers
	// But we need to destroy old buffers and create new ones

	for i := 0; i < 2; i++ {
		offset := i * bufSize
		buf := &shmBuffer{
			id:       p.cl.allocID(),
			offset:   offset,
			width:    width,
			height:   height,
			stride:   stride,
			released: true,
		}
		buf.buf = &graphics.Buffer{
			Width:  width,
			Height: height,
			Stride: stride,
			Format: graphics.PixelFormatBGRA,
			Data:   data[offset : offset+bufSize],
		}

		pl := make([]byte, 24)
		putUint32(pl, 0, buf.id)
		putInt32(pl, 4, int32(offset))
		putInt32(pl, 8, int32(width))
		putInt32(pl, 12, int32(height))
		putInt32(pl, 16, int32(stride))
		putUint32(pl, 20, shmFormatARGB8888)
		if err := p.cl.conn.sendMsg(p.poolID, shmPoolCreateBufferOp, pl); err != nil {
			return fmt.Errorf("create buffer %d: %w", i, err)
		}

		p.cl.setHandler(buf.id, func(_ uint32, opcode uint16, _ []byte, fds []int) {
			closeFDs(fds)
			if opcode == bufferReleaseEvent {
				buf.released = true
			}
		})

		p.buffers[i] = buf
	}

	p.current = 0
	return nil
}

// destroy cleans up the pool resources.
func (p *shmPool) destroy() {
	for _, buf := range p.buffers {
		if buf != nil {
			p.cl.removeHandler(buf.id)
		}
	}
	if p.data != nil {
		syscall.Munmap(p.data)
		p.data = nil
	}
	if p.fd >= 0 {
		syscall.Close(p.fd)
		p.fd = -1
	}
}
