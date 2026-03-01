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
	"fmt"
	"image"
	"log"
	"syscall"

	core "avyos.dev/pkg/graphics/pixmap"
)

// surfaceState represents a wl_surface created by a client.
type surfaceState struct {
	id      uint32
	session *clientSession

	// Double-buffered state
	pending surfacePending
	current surfacePending

	// Frame callbacks to fire on next commit
	frameCallbacks []uint32

	// Link to the display window (set when xdg_toplevel is created)
	displayWindow *toplevelWindow
}

// surfacePending holds the pending or committed surface state.
type surfacePending struct {
	bufferID  uint32
	bufferX   int32
	bufferY   int32
	damage    []image.Rectangle
	hasBuffer bool
	attached  bool
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
			surf.pending.attached = true
			surf.pending.hasBuffer = bufID != 0
		}

	case surfaceDamageOp, surfaceDamageBufferOp:
		if len(payload) >= 16 {
			x := int(getInt32(payload, 0))
			y := int(getInt32(payload, 4))
			w := int(getInt32(payload, 8))
			h := int(getInt32(payload, 12))
			surf.pending.damage = append(surf.pending.damage, core.RectXYWH(x, y, w, h))
		}

	case surfaceFrameOp:
		if len(payload) >= 4 {
			cbID := getUint32(payload, 0)
			surf.frameCallbacks = append(surf.frameCallbacks, cbID)
			s.setHandler(cbID, nil)
		}

	case surfaceCommitOp:
		s.commitSurface(surf)

	case surfaceDestroyOp:
		s.removeSubsurfacesForSurface(surf)
		s.removeHandler(id)
		delete(s.surfaces, id)
		s.server.removeSurface(surf)

	case surfaceSetOpaqueRegionOp, surfaceSetInputRegionOp:
		// Ignore region hints
	}
}

// commitSurface applies pending state and copies pixels to the display window.
func (s *clientSession) commitSurface(surf *surfaceState) {
	pending := surf.pending
	surf.pending = surfacePending{}
	releaseBufferID := uint32(0)

	if pending.attached {
		surf.current.bufferID = pending.bufferID
		surf.current.bufferX = pending.bufferX
		surf.current.bufferY = pending.bufferY
		surf.current.hasBuffer = pending.hasBuffer
		if pending.hasBuffer {
			releaseBufferID = pending.bufferID
		}
	}
	surf.current.damage = pending.damage

	tw := surf.displayWindow
	if !surf.current.hasBuffer {
		if tw != nil && tw.win != nil {
			dstBuf := tw.win.Buffer()
			if dstBuf != nil {
				dstBuf.Clear(core.ColorTransparent)
				tw.win.DamageAll()
			}
		}
	} else {
		buf, ok := s.buffers[surf.current.bufferID]
		if ok {
			pool := buf.pool
			if pool.data != nil && validBufferBounds(buf.offset, buf.width, buf.height, buf.stride, len(pool.data)) {
				if tw != nil && tw.win != nil {
					// Resize the display window if the buffer size changed
					targetW, targetH, clamped := clampSurfaceSize(buf.width, buf.height, tw.win.Width, tw.win.Height)
					if clamped {
						log.Printf("clamped wl_buffer size from %dx%d to %dx%d", buf.width, buf.height, targetW, targetH)
					}
					if targetW != tw.win.Width || targetH != tw.win.Height {
						if err := tw.win.Resize(targetW, targetH); err != nil {
							log.Printf("display resize failed: %v", err)
						}
					}

					// Copy pixel data directly to the display window's shared buffer
					dstBuf := tw.win.Buffer()
					if dstBuf != nil {
						src := pool.data[buf.offset : buf.offset+buf.stride*buf.height]
						copyHeight := buf.height
						if copyHeight > dstBuf.Height {
							copyHeight = dstBuf.Height
						}

						copyWidthPx := buf.width
						if copyWidthPx > dstBuf.Width {
							copyWidthPx = dstBuf.Width
						}
						if copyWidthPx > 0 && copyHeight > 0 {
							rowBytes := copyWidthPx * 4
							if rowBytes > buf.stride {
								rowBytes = buf.stride
							}
							if rowBytes > dstBuf.Stride {
								rowBytes = dstBuf.Stride
							}

							if rowBytes > 0 {
								for y := 0; y < copyHeight; y++ {
									srcOff := y * buf.stride
									dstOff := y * dstBuf.Stride
									if srcOff+rowBytes > len(src) || dstOff+rowBytes > len(dstBuf.Data) {
										continue
									}

									srcRow := src[srcOff : srcOff+rowBytes]
									dstRow := dstBuf.Data[dstOff : dstOff+rowBytes]
									copy(dstRow, srcRow)
									if promoteOpaqueAlpha(buf.format, dstRow) {
										// Some clients submit XRGB or ARGB with undefined alpha for opaque content.
										// Promote alpha to keep content visible in display compositing.
										forceOpaqueAlpha(dstRow)
									}
								}
							}
						}

						// Composite subsurfaces onto the same buffer
						for _, ss := range s.subsurfacesForParent(surf) {
							if ss == nil || ss.surface == nil {
								continue
							}
							s.compositeSubsurface(dstBuf, ss)
						}

						tw.win.DamageAll()
					}
				}
			} else if pool.data != nil {
				log.Printf("ignoring invalid wl_buffer bounds (offset=%d width=%d height=%d stride=%d pool=%d)",
					buf.offset, buf.width, buf.height, buf.stride, len(pool.data))
			}
		}
	}
	if releaseBufferID != 0 {
		// Release exactly once per attach; commits without attach keep current buffer.
		s.conn.sendMsg(releaseBufferID, bufferReleaseEvent, nil)
	}

	// Fire frame callbacks
	if len(surf.frameCallbacks) > 0 {
		ms := s.server.timestamp()
		for _, cbID := range surf.frameCallbacks {
			p := make([]byte, 4)
			putUint32(p, 0, uint32(ms))
			s.conn.sendMsg(cbID, callbackDoneEvent, p)

			dp := make([]byte, 4)
			putUint32(dp, 0, cbID)
			s.conn.sendMsg(1, displayDeleteIDEvent, dp)
		}
		surf.frameCallbacks = nil
	}
}

// compositeSubsurface draws a subsurface onto the parent's display buffer.
func (s *clientSession) compositeSubsurface(dstBuf *core.Buffer, ss *subsurfaceState) {
	subSurf := ss.surface
	if subSurf == nil || !subSurf.current.hasBuffer {
		return
	}
	subBuf, ok := s.buffers[subSurf.current.bufferID]
	if !ok {
		return
	}
	pool := subBuf.pool
	if pool.data == nil || !validBufferBounds(subBuf.offset, subBuf.width, subBuf.height, subBuf.stride, len(pool.data)) {
		return
	}

	sx, sy := ss.position()
	src := pool.data[subBuf.offset:]
	for y := 0; y < subBuf.height; y++ {
		dy := sy + y
		if dy < 0 || dy >= dstBuf.Height {
			continue
		}
		srcRow := src[y*subBuf.stride : y*subBuf.stride+subBuf.width*4]
		for x := 0; x < subBuf.width; x++ {
			dx := sx + x
			if dx < 0 || dx >= dstBuf.Width {
				continue
			}
			srcOff := x * 4
			dstOff := dy*dstBuf.Stride + dx*4
			copy(dstBuf.Data[dstOff:dstOff+4], srcRow[srcOff:srcOff+4])
			if subBuf.format == shmFormatXRGB8888 {
				dstBuf.Data[dstOff+3] = 0xFF
			} else if subBuf.format == shmFormatARGB8888 {
				r := dstBuf.Data[dstOff+2]
				g := dstBuf.Data[dstOff+1]
				b := dstBuf.Data[dstOff+0]
				a := dstBuf.Data[dstOff+3]
				if a == 0 && (r != 0 || g != 0 || b != 0) {
					dstBuf.Data[dstOff+3] = 0xFF
				}
			}
		}
	}
}

func forceOpaqueAlpha(row []byte) {
	for i := 3; i < len(row); i += 4 {
		row[i] = 0xFF
	}
}

func promoteOpaqueAlpha(format uint32, row []byte) bool {
	if format == shmFormatXRGB8888 {
		return true
	}
	if format != shmFormatARGB8888 {
		return false
	}

	// Promote only when ARGB row has non-zero color channels but zero alpha throughout.
	alphaZero := true
	hasColor := false
	for i := 0; i+3 < len(row); i += 4 {
		if row[i+3] != 0 {
			alphaZero = false
			break
		}
		if row[i] != 0 || row[i+1] != 0 || row[i+2] != 0 {
			hasColor = true
		}
	}
	return alphaZero && hasColor
}

// handleShmRequest processes wl_shm requests.
func (s *clientSession) handleShmRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	switch opcode {
	case shmCreatePoolOp:
		if len(payload) >= 8 && len(fds) >= 1 {
			poolID := getUint32(payload, 0)
			size := int(getInt32(payload, 4))
			fd := fds[0]
			closeFDs(fds[1:])
			if size <= 0 {
				log.Printf("ignoring wl_shm_pool with invalid size %d", size)
				syscall.Close(fd)
				return
			}

			pool := &shmPoolState{
				id:   poolID,
				fd:   fd,
				size: size,
			}

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
			if width <= 0 || height <= 0 || stride <= 0 || offset < 0 {
				log.Printf("ignoring wl_buffer %d with invalid geometry offset=%d size=%dx%d stride=%d",
					bufID, offset, width, height, stride)
				return
			}
			w, h, clamped := clampSurfaceSize(width, height, width, height)
			if clamped {
				log.Printf("clamped wl_buffer %d size from %dx%d to %dx%d", bufID, width, height, w, h)
			}
			width = w
			height = h
			if !validBufferBounds(offset, width, height, stride, pool.size) {
				log.Printf("ignoring wl_buffer %d with out-of-bounds payload (offset=%d size=%dx%d stride=%d pool=%d)",
					bufID, offset, width, height, stride, pool.size)
				return
			}

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
			if newSize <= 0 {
				log.Printf("ignoring wl_shm_pool resize with invalid size %d", newSize)
				return
			}
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
	s.server.removeSurface(surf)
	delete(s.surfaces, id)
	return nil
}
