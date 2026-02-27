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

package compositor

import (
	"fmt"
	"sync"
	"syscall"

	graphics "avyos.dev/pkg/graphics/input"
)

// surfaceState represents a wl_surface created by a client.
type surfaceState struct {
	id      uint32
	session *clientSession

	// Double-buffered state
	pending surfacePending
	current surfacePending

	// Rendered content (pixel data copied from client's shm buffer)
	// Protected by bufMu — commitSurface writes, compositor reads.
	bufMu  sync.Mutex
	buffer *graphics.Buffer

	// Frame callbacks to fire on next commit
	frameCallbacks []uint32
}

// getBuffer returns the current rendered buffer (thread-safe).
func (s *surfaceState) getBuffer() *graphics.Buffer {
	s.bufMu.Lock()
	b := s.buffer
	s.bufMu.Unlock()
	return b
}

// surfacePending holds the pending or committed surface state.
type surfacePending struct {
	bufferID  uint32
	bufferX   int32
	bufferY   int32
	damage    []graphics.Rect
	hasBuffer bool
}

// shmPoolState represents a wl_shm_pool created by a client.
type shmPoolState struct {
	id   uint32
	fd   int
	data []byte // mmap'd region
	size int
}

// bufferState represents a wl_buffer created from a shm pool.
type bufferState struct {
	id     uint32
	pool   *shmPoolState
	offset int
	width  int
	height int
	stride int
	format uint32
}

// handleSurfaceRequest processes wl_surface requests.
func (s *clientSession) handleSurfaceRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	surf, ok := s.surfaces[id]
	if !ok {
		return
	}

	switch opcode {
	case surfaceAttachOp:
		if len(payload) >= 12 {
			bufID := getUint32(payload, 0)
			dx := getInt32(payload, 4)
			dy := getInt32(payload, 8)
			surf.pending.bufferID = bufID
			surf.pending.bufferX = dx
			surf.pending.bufferY = dy
			surf.pending.hasBuffer = true
		}

	case surfaceDamageOp, surfaceDamageBufferOp:
		if len(payload) >= 16 {
			x := int(getInt32(payload, 0))
			y := int(getInt32(payload, 4))
			w := int(getInt32(payload, 8))
			h := int(getInt32(payload, 12))
			surf.pending.damage = append(surf.pending.damage, graphics.Rect{X: x, Y: y, W: w, H: h})
		}

	case surfaceFrameOp:
		if len(payload) >= 4 {
			cbID := getUint32(payload, 0)
			surf.frameCallbacks = append(surf.frameCallbacks, cbID)
			s.setHandler(cbID, nil) // client-side object, no requests expected
		}

	case surfaceCommitOp:
		s.commitSurface(surf)

	case surfaceDestroyOp:
		s.removeHandler(id)
		delete(s.surfaces, id)
		s.compositor.removeSurface(surf)

	case surfaceSetOpaqueRegionOp, surfaceSetInputRegionOp:
		// Ignore region hints
	}
}

// commitSurface applies pending state and updates the rendered buffer.
func (s *clientSession) commitSurface(surf *surfaceState) {
	surf.current = surf.pending
	surf.pending = surfacePending{}

	if surf.current.hasBuffer {
		buf, ok := s.buffers[surf.current.bufferID]
		if ok {
			// Copy pixel data from shm buffer into a NEW graphics.Buffer.
			// Always create a new buffer to avoid data races with the
			// compositor goroutine reading the old buffer during compositing.
			pool := buf.pool
			if pool.data != nil && buf.offset+buf.stride*buf.height <= len(pool.data) {
				newBuf := graphics.NewBuffer(buf.width, buf.height)
				src := pool.data[buf.offset:]
				for y := 0; y < buf.height; y++ {
					srcRow := src[y*buf.stride : y*buf.stride+buf.width*4]
					dstOff := y * newBuf.Stride
					copy(newBuf.Data[dstOff:dstOff+buf.width*4], srcRow)
				}
				// Swap in the fully-populated buffer under lock
				surf.bufMu.Lock()
				surf.buffer = newBuf
				surf.bufMu.Unlock()
			}

			// Update window dimensions
			s.compositor.updateWindowSize(surf, buf.width, buf.height)

			// Release the buffer back to the client
			s.conn.sendMsg(surf.current.bufferID, bufferReleaseEvent, nil)
		}
	}

	// Fire frame callbacks
	if len(surf.frameCallbacks) > 0 {
		ms := s.compositor.timestamp()
		for _, cbID := range surf.frameCallbacks {
			p := make([]byte, 4)
			putUint32(p, 0, uint32(ms))
			s.conn.sendMsg(cbID, callbackDoneEvent, p)

			// Send delete_id for the callback
			dp := make([]byte, 4)
			putUint32(dp, 0, cbID)
			s.conn.sendMsg(1, displayDeleteIDEvent, dp)
		}
		surf.frameCallbacks = nil
	}

	// Notify compositor to redraw
	s.compositor.requestRedraw()
}

// handleShmRequest processes wl_shm requests.
func (s *clientSession) handleShmRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	switch opcode {
	case shmCreatePoolOp:
		if len(payload) >= 8 && len(fds) >= 1 {
			poolID := getUint32(payload, 0)
			size := int(getInt32(payload, 4))
			fd := fds[0]
			// Close remaining fds
			closeFDs(fds[1:])

			pool := &shmPoolState{
				id:   poolID,
				fd:   fd,
				size: size,
			}

			// mmap the shared memory
			data, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
			if err == nil {
				pool.data = data
			}

			s.shmPools[poolID] = pool
			s.setHandler(poolID, s.handleShmPoolRequest)
		} else {
			closeFDs(fds)
		}

	default:
		closeFDs(fds)
	}
}

// handleShmPoolRequest processes wl_shm_pool requests.
func (s *clientSession) handleShmPoolRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	pool, ok := s.shmPools[id]
	if !ok {
		return
	}

	switch opcode {
	case shmPoolCreateBufferOp:
		if len(payload) >= 24 {
			bufID := getUint32(payload, 0)
			offset := int(getInt32(payload, 4))
			width := int(getInt32(payload, 8))
			height := int(getInt32(payload, 12))
			stride := int(getInt32(payload, 16))
			format := getUint32(payload, 20)

			buf := &bufferState{
				id:     bufID,
				pool:   pool,
				offset: offset,
				width:  width,
				height: height,
				stride: stride,
				format: format,
			}
			s.buffers[bufID] = buf
			s.setHandler(bufID, s.handleBufferRequest)
		}

	case shmPoolResizeOp:
		if len(payload) >= 4 {
			newSize := int(getInt32(payload, 0))
			// Remap
			if pool.data != nil {
				syscall.Munmap(pool.data)
			}
			data, err := syscall.Mmap(pool.fd, 0, newSize, syscall.PROT_READ, syscall.MAP_SHARED)
			if err == nil {
				pool.data = data
				pool.size = newSize
			}
		}

	case shmPoolDestroyOp:
		s.destroyShmPool(pool)
		s.removeHandler(id)
		delete(s.shmPools, id)
	}
}

// handleBufferRequest processes wl_buffer requests.
func (s *clientSession) handleBufferRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	if opcode == bufferDestroyOp {
		s.removeHandler(id)
		delete(s.buffers, id)
	}
}

// destroyShmPool cleans up a shm pool.
func (s *clientSession) destroyShmPool(pool *shmPoolState) {
	if pool.data != nil {
		syscall.Munmap(pool.data)
		pool.data = nil
	}
	if pool.fd >= 0 {
		syscall.Close(pool.fd)
		pool.fd = -1
	}
}

// destroySurface is used when cleaning up.
func (s *clientSession) destroySurface(id uint32) error {
	surf, ok := s.surfaces[id]
	if !ok {
		return fmt.Errorf("surface %d not found", id)
	}
	s.compositor.removeSurface(surf)
	delete(s.surfaces, id)
	return nil
}
