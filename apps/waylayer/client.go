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
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"syscall"
)

// objectHandler is a function that handles requests for a specific object.
type objectHandler func(id uint32, opcode uint16, payload []byte, fds []int)

// clientSession manages one connected Wayland client.
type clientSession struct {
	conn   *clientConn
	server *Server

	// Object dispatch map
	handlers map[uint32]objectHandler
	mu       sync.Mutex

	// Protocol objects
	surfaces     map[uint32]*surfaceState
	shmPools     map[uint32]*shmPoolState
	buffers      map[uint32]*bufferState
	xdgSurfaces  map[uint32]*xdgSurfaceState
	xdgToplevels map[uint32]*xdgToplevelState
	xdgPopups    map[uint32]*xdgPopupState
	positioners  map[uint32]*xdgPositionerState
	subsurfaces  map[uint32]*subsurfaceState
	subMu        sync.RWMutex

	// Seat input objects
	pointerID  uint32
	keyboardID uint32

	// Global IDs for bound globals
	registryID   uint32
	compositorID uint32
	subcompID    uint32
	dataDevMgrID uint32
	outputID     uint32
	outputVer    uint32
	shmID        uint32
	seatID       uint32
	seatVer      uint32
	xdgWmBaseID  uint32

	nextSerial uint32
	closed     bool
}

func newClientSession(uc *net.UnixConn, server *Server) *clientSession {
	s := &clientSession{
		conn:         newClientConn(uc),
		server:       server,
		handlers:     make(map[uint32]objectHandler),
		surfaces:     make(map[uint32]*surfaceState),
		shmPools:     make(map[uint32]*shmPoolState),
		buffers:      make(map[uint32]*bufferState),
		xdgSurfaces:  make(map[uint32]*xdgSurfaceState),
		xdgToplevels: make(map[uint32]*xdgToplevelState),
		xdgPopups:    make(map[uint32]*xdgPopupState),
		positioners:  make(map[uint32]*xdgPositionerState),
		subsurfaces:  make(map[uint32]*subsurfaceState),
		nextSerial:   1,
	}

	// Object 1 = wl_display
	s.setHandler(1, s.handleDisplayRequest)

	return s
}

func (s *clientSession) setHandler(id uint32, handler objectHandler) {
	s.mu.Lock()
	s.handlers[id] = handler
	s.mu.Unlock()
}

func (s *clientSession) removeHandler(id uint32) {
	s.mu.Lock()
	delete(s.handlers, id)
	s.mu.Unlock()
}

func (s *clientSession) getHandler(id uint32) objectHandler {
	s.mu.Lock()
	h := s.handlers[id]
	s.mu.Unlock()
	return h
}

// run reads and dispatches client requests until disconnection.
func (s *clientSession) run() {
	defer s.cleanup()

	for {
		objectID, opcode, payload, fds, err := s.conn.recvMsg()
		if err != nil {
			if !s.closed {
				log.Printf("client disconnect: %v", err)
			}
			return
		}

		handler := s.getHandler(objectID)
		if handler != nil {
			handler(objectID, opcode, payload, fds)
		} else {
			closeFDs(fds)
		}
	}
}

// cleanup removes all objects and surfaces for this client.
func (s *clientSession) cleanup() {
	s.closed = true

	// Remove all toplevels (destroys display windows)
	for _, tl := range s.xdgToplevels {
		s.server.removeToplevel(tl)
	}
	for _, popup := range s.xdgPopups {
		s.server.removePopup(popup)
	}

	s.subMu.Lock()
	for id := range s.subsurfaces {
		delete(s.subsurfaces, id)
	}
	s.subMu.Unlock()

	// Destroy shm pools
	for _, pool := range s.shmPools {
		s.destroyShmPool(pool)
	}

	s.server.removeSession(s)
	s.conn.close()
}

// handleDisplayRequest processes wl_display requests (object 1).
func (s *clientSession) handleDisplayRequest(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	switch opcode {
	case displaySyncOp:
		if len(payload) >= 4 {
			callbackID := getUint32(payload, 0)
			// Send callback.done immediately
			p := make([]byte, 4)
			putUint32(p, 0, 0) // callback_data
			s.conn.sendMsg(callbackID, callbackDoneEvent, p)

			// Send delete_id for the callback
			dp := make([]byte, 4)
			putUint32(dp, 0, callbackID)
			s.conn.sendMsg(1, displayDeleteIDEvent, dp)
		}

	case displayGetRegistryOp:
		if len(payload) >= 4 {
			s.registryID = getUint32(payload, 0)
			s.setHandler(s.registryID, s.handleRegistryRequest)
			s.sendGlobals()
		}
	}
}

// sendGlobals sends wl_registry.global events for all advertised interfaces.
func (s *clientSession) sendGlobals() {
	globals := []struct {
		name    uint32
		iface   string
		version uint32
	}{
		{1, ifaceWlCompositor, versionWlCompositor},
		{2, ifaceWlSubcomp, versionWlSubcomp},
		{3, ifaceWlDataDevMgr, versionWlDataDevMgr},
		{4, ifaceWlOutput, versionWlOutput},
		{5, ifaceWlShm, versionWlShm},
		{6, ifaceXdgWmBase, versionXdgWmBase},
		{7, ifaceWlSeat, versionWlSeat},
	}

	for _, g := range globals {
		ifaceStr := encodeString(g.iface)
		p := make([]byte, 4+len(ifaceStr)+4)
		putUint32(p, 0, g.name)
		copy(p[4:], ifaceStr)
		putUint32(p, 4+len(ifaceStr), g.version)
		s.conn.sendMsg(s.registryID, registryGlobalEvent, p)
	}
}

// handleRegistryRequest processes wl_registry requests.
func (s *clientSession) handleRegistryRequest(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	if opcode != registryBindOp {
		return
	}

	// wl_registry.bind(name, interface, version, new_id)
	if len(payload) < 12 {
		return
	}

	name := getUint32(payload, 0)
	ifaceStr, consumed := getString(payload, 4)
	if 4+consumed+8 > len(payload) {
		return
	}
	reqVersion := getUint32(payload, 4+consumed)
	newID := getUint32(payload, 4+consumed+4)

	_ = ifaceStr

	switch name {
	case 1: // wl_compositor
		s.compositorID = newID
		s.setHandler(newID, s.handleCompositorRequest)

	case 2: // wl_subcompositor
		s.subcompID = newID
		s.setHandler(newID, s.handleSubcompositorRequest)

	case 3: // wl_data_device_manager
		s.dataDevMgrID = newID
		s.setHandler(newID, s.handleDataDeviceManagerRequest)

	case 4: // wl_output
		s.outputID = newID
		s.outputVer = reqVersion
		s.setHandler(newID, s.handleOutputRequest)
		s.sendOutputInfo(newID, reqVersion)
		for _, surf := range s.surfaces {
			if surf != nil && surf.displayWindow != nil {
				s.sendSurfaceEnter(surf.id)
			}
		}

	case 5: // wl_shm
		s.shmID = newID
		s.setHandler(newID, s.handleShmRequest)
		s.sendShmFormats(newID)

	case 6: // xdg_wm_base
		s.xdgWmBaseID = newID
		s.setHandler(newID, s.handleXdgWmBaseRequest)

	case 7: // wl_seat
		if reqVersion > versionWlSeat {
			reqVersion = versionWlSeat
		}
		if reqVersion == 0 {
			reqVersion = 1
		}
		s.seatID = newID
		s.seatVer = reqVersion
		s.setHandler(newID, s.handleSeatRequest)
		s.sendSeatCapabilities(newID)
		if s.seatVer >= 2 {
			s.sendSeatName(newID)
		}

	default:
		log.Printf("client tried to bind unknown global %d (%s)", name, ifaceStr)
	}
}

// sendShmFormats tells the client which pixel formats are supported.
func (s *clientSession) sendShmFormats(shmID uint32) {
	for _, format := range []uint32{shmFormatARGB8888, shmFormatXRGB8888} {
		p := make([]byte, 4)
		putUint32(p, 0, format)
		s.conn.sendMsg(shmID, shmFormatEvent, p)
	}
}

// sendSeatCapabilities tells the client what input devices the seat has.
func (s *clientSession) sendSeatCapabilities(seatID uint32) {
	caps := uint32(seatCapPointer | seatCapKeyboard)
	p := make([]byte, 4)
	putUint32(p, 0, caps)
	s.conn.sendMsg(seatID, seatCapabilitiesEvent, p)
}

// sendSeatName sends the seat name.
func (s *clientSession) sendSeatName(seatID uint32) {
	name := encodeString("seat0")
	s.conn.sendMsg(seatID, seatNameEvent, name)
}

// handleCompositorRequest processes wl_compositor requests.
func (s *clientSession) handleCompositorRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	switch opcode {
	case compositorCreateSurfaceOp:
		if len(payload) >= 4 {
			surfID := getUint32(payload, 0)
			surf := &surfaceState{
				id:      surfID,
				session: s,
			}
			s.surfaces[surfID] = surf
			s.setHandler(surfID, s.handleSurfaceRequest)
		}

	case compositorCreateRegionOp:
		if len(payload) >= 4 {
			regionID := getUint32(payload, 0)
			s.setHandler(regionID, func(_ uint32, _ uint16, _ []byte, fds []int) {
				closeFDs(fds)
			})
		}
	}
}

// handleSeatRequest processes wl_seat requests.
func (s *clientSession) handleSeatRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	switch opcode {
	case seatGetPointerOp:
		if len(payload) >= 4 {
			s.pointerID = getUint32(payload, 0)
			s.setHandler(s.pointerID, s.handlePointerRequest)
		}

	case seatGetKeyboardOp:
		if len(payload) >= 4 {
			s.keyboardID = getUint32(payload, 0)
			s.setHandler(s.keyboardID, s.handleKeyboardRequest)
			s.sendKeymap()
			s.sendKeyboardRepeatInfo()
		}

	case seatReleaseOp:
		if id == s.seatID {
			s.seatID = 0
			s.seatVer = 0
		}
		s.removeHandler(id)
	}
}

func (s *clientSession) handlePointerRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	_ = payload

	switch opcode {
	case pointerSetCursorOp:
		// Cursor images are compositor-driven in display service.
	case pointerReleaseOp:
		if id == s.pointerID {
			s.pointerID = 0
		}
		s.removeHandler(id)
	}
}

func (s *clientSession) handleKeyboardRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	_ = payload

	if opcode == keyboardReleaseOp {
		if id == s.keyboardID {
			s.keyboardID = 0
		}
		s.removeHandler(id)
	}
}

// sendKeymap sends wl_keyboard.keymap to the client.
func (s *clientSession) sendKeymap() {
	fd, size, err := createKeymapFD()
	if err != nil {
		log.Printf("failed to create keymap fd: %v", err)
		p := make([]byte, 8)
		putUint32(p, 0, keyboardKeymapFormatNoKeymap)
		putUint32(p, 4, 0)
		s.conn.sendMsg(s.keyboardID, keyboardKeymapEvent, p)
		return
	}

	p := make([]byte, 8)
	putUint32(p, 0, keyboardKeymapFormatXKBv1)
	putUint32(p, 4, uint32(size))
	if err := s.conn.sendMsg(s.keyboardID, keyboardKeymapEvent, p, fd); err != nil {
		log.Printf("failed to send keymap: %v", err)
	}
	_ = syscall.Close(fd)
}

func (s *clientSession) sendKeyboardRepeatInfo() {
	if s.keyboardID == 0 || s.seatVer < 4 {
		return
	}
	p := make([]byte, 8)
	putInt32(p, 0, 25)  // 25 keys/second
	putInt32(p, 4, 600) // 600 ms delay
	s.conn.sendMsg(s.keyboardID, keyboardRepeatInfoEvent, p)
}

func (s *clientSession) subsurfacesForParent(parent *surfaceState) []*subsurfaceState {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	out := make([]*subsurfaceState, 0, len(s.subsurfaces))
	for _, ss := range s.subsurfaces {
		if ss.parent == parent {
			out = append(out, ss)
		}
	}
	return out
}

func (s *clientSession) removeSubsurfacesForSurface(surface *surfaceState) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	for id, ss := range s.subsurfaces {
		if ss.surface == surface || ss.parent == surface {
			delete(s.subsurfaces, id)
			s.removeHandler(id)
		}
	}
}

func (s *clientSession) handleDataDeviceManagerRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	switch opcode {
	case dataDeviceMgrCreateDataSourceOp:
		if len(payload) < 4 {
			return
		}
		sourceID := getUint32(payload, 0)
		s.setHandler(sourceID, func(_ uint32, _ uint16, _ []byte, fds []int) {
			closeFDs(fds)
		})
	case dataDeviceMgrGetDataDeviceOp:
		if len(payload) < 8 {
			return
		}
		dataDeviceID := getUint32(payload, 0)
		s.setHandler(dataDeviceID, s.handleDataDeviceRequest)
	}
}

func (s *clientSession) handleDataDeviceRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	switch opcode {
	case dataDeviceStartDragOp, dataDeviceSetSelectionOp:
	case dataDeviceReleaseOp:
		s.removeHandler(id)
	}
	_ = payload
}

func (s *clientSession) handleOutputRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	if opcode == outputReleaseOp {
		s.removeHandler(id)
	}
	_ = payload
}

func (s *clientSession) sendOutputInfo(outputID uint32, boundVersion uint32) {
	if boundVersion > versionWlOutput {
		boundVersion = versionWlOutput
	}
	if boundVersion == 0 {
		boundVersion = 1
	}

	// Use reasonable defaults; allow runtime override for integration testing.
	width := envInt("WAYLAYER_OUTPUT_WIDTH", 1920)
	height := envInt("WAYLAYER_OUTPUT_HEIGHT", 1080)
	if w, h, clamped := clampSurfaceSize(width, height, 1920, 1080); clamped {
		log.Printf("clamped output mode from %dx%d to %dx%d", width, height, w, h)
		width, height = w, h
	}

	makeStr := encodeString("AvyOS")
	modelStr := encodeString("Virtual-1")

	// geometry(x, y, phys_w, phys_h, subpixel, make, model, transform)
	geom := make([]byte, 24+len(makeStr)+len(modelStr))
	putInt32(geom, 0, 0)
	putInt32(geom, 4, 0)
	putInt32(geom, 8, 0)
	putInt32(geom, 12, 0)
	putUint32(geom, 16, 0)
	copy(geom[20:], makeStr)
	offset := 20 + len(makeStr)
	copy(geom[offset:], modelStr)
	offset += len(modelStr)
	putInt32(geom, offset, 0)
	s.conn.sendMsg(outputID, outputGeometryEvent, geom)

	// mode(flags, width, height, refresh_mHz)
	mode := make([]byte, 16)
	putUint32(mode, 0, outputModeCurrent|outputModePreferred)
	putInt32(mode, 4, int32(width))
	putInt32(mode, 8, int32(height))
	putInt32(mode, 12, 60000)
	s.conn.sendMsg(outputID, outputModeEvent, mode)

	if boundVersion >= 2 {
		scale := make([]byte, 4)
		putInt32(scale, 0, 1)
		s.conn.sendMsg(outputID, outputScaleEvent, scale)
	}

	if boundVersion >= 4 {
		name := encodeString("Virtual-1")
		desc := encodeString("Waylayer display translator")
		s.conn.sendMsg(outputID, outputNameEvent, name)
		s.conn.sendMsg(outputID, outputDescEvent, desc)
	}
	if boundVersion >= 2 {
		s.conn.sendMsg(outputID, outputDoneEvent, nil)
	}
}

func (s *clientSession) sendSurfaceEnter(surfaceID uint32) {
	if surfaceID == 0 || s.outputID == 0 {
		return
	}
	p := make([]byte, 4)
	putUint32(p, 0, s.outputID)
	s.conn.sendMsg(surfaceID, surfaceEnterEvent, p)
}

func (s *clientSession) sendSurfaceLeave(surfaceID uint32) {
	if surfaceID == 0 || s.outputID == 0 {
		return
	}
	p := make([]byte, 4)
	putUint32(p, 0, s.outputID)
	s.conn.sendMsg(surfaceID, surfaceLeaveEvent, p)
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// serial returns the next event serial and increments.
func (s *clientSession) serial() uint32 {
	s.nextSerial++
	return s.nextSerial - 1
}
