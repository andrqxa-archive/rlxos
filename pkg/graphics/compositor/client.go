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
	"log"
	"net"
	"sync"
)

// objectHandler is a function that handles requests for a specific object.
type objectHandler func(id uint32, opcode uint16, payload []byte, fds []int)

// clientSession manages one connected Wayland client.
type clientSession struct {
	conn       *clientConn
	compositor *Compositor

	// Object dispatch map
	handlers map[uint32]objectHandler
	mu       sync.Mutex

	// Protocol objects
	surfaces        map[uint32]*surfaceState
	shmPools        map[uint32]*shmPoolState
	buffers         map[uint32]*bufferState
	xdgSurfaces     map[uint32]*xdgSurfaceState
	xdgToplevels    map[uint32]*xdgToplevelState
	layerSurfaces   map[uint32]*layerSurfaceState
	foreignManagers map[uint32]*foreignManager

	// Seat input objects
	pointerID  uint32
	keyboardID uint32

	// Global IDs for bound globals
	registryID       uint32
	compositorID     uint32
	shmID            uint32
	seatID           uint32
	xdgWmBaseID      uint32
	layerShellID     uint32
	foreignManagerID uint32

	nextSerial uint32
	closed     bool
}

func newClientSession(uc *net.UnixConn, comp *Compositor) *clientSession {
	s := &clientSession{
		conn:            newClientConn(uc),
		compositor:      comp,
		handlers:        make(map[uint32]objectHandler),
		surfaces:        make(map[uint32]*surfaceState),
		shmPools:        make(map[uint32]*shmPoolState),
		buffers:         make(map[uint32]*bufferState),
		xdgSurfaces:     make(map[uint32]*xdgSurfaceState),
		xdgToplevels:    make(map[uint32]*xdgToplevelState),
		layerSurfaces:   make(map[uint32]*layerSurfaceState),
		foreignManagers: make(map[uint32]*foreignManager),
		nextSerial:      1,
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

	// Remove all windows
	for _, tl := range s.xdgToplevels {
		s.compositor.removeWindow(tl)
	}

	// Remove layer surfaces
	for _, ls := range s.layerSurfaces {
		s.compositor.removeLayerSurface(ls)
	}

	// Remove foreign managers
	s.compositor.foreign.removeSession(s)

	// Destroy shm pools
	for _, pool := range s.shmPools {
		s.destroyShmPool(pool)
	}

	s.compositor.removeSession(s)
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
		{2, ifaceWlShm, versionWlShm},
		{3, ifaceXdgWmBase, versionXdgWmBase},
		{4, ifaceWlSeat, versionWlSeat},
		{5, ifaceLayerShell, versionLayerShell},
		{6, ifaceForeignManager, versionForeignManager},
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
	// Payload: name(4) + interface_string + version(4) + new_id(4)
	if len(payload) < 12 {
		return
	}

	name := getUint32(payload, 0)
	ifaceStr, consumed := getString(payload, 4)
	if 4+consumed+8 > len(payload) {
		return
	}
	// version at offset 4+consumed
	newID := getUint32(payload, 4+consumed+4)

	_ = ifaceStr // We dispatch by name (global ID)

	switch name {
	case 1: // wl_compositor
		s.compositorID = newID
		s.setHandler(newID, s.handleCompositorRequest)

	case 2: // wl_shm
		s.shmID = newID
		s.setHandler(newID, s.handleShmRequest)
		// Advertise supported formats
		s.sendShmFormats(newID)

	case 3: // xdg_wm_base
		s.xdgWmBaseID = newID
		s.setHandler(newID, s.handleXdgWmBaseRequest)

	case 4: // wl_seat
		s.seatID = newID
		s.setHandler(newID, s.handleSeatRequest)
		// Send capabilities
		s.sendSeatCapabilities(newID)
		s.sendSeatName(newID)

	case 5: // zwlr_layer_shell_v1
		s.layerShellID = newID
		s.setHandler(newID, s.handleLayerShellRequest)

	case 6: // zwlr_foreign_toplevel_manager_v1
		s.foreignManagerID = newID
		fm := &foreignManager{
			id:      newID,
			session: s,
			handles: make(map[*xdgToplevelState]uint32),
			nextID:  0,
		}
		s.foreignManagers[newID] = fm
		s.compositor.foreign.addManager(fm)
		s.setHandler(newID, s.handleForeignManagerRequest)

		// Send existing toplevels
		s.compositor.sendExistingToplevels(fm)

	default:
		// Unknown global
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
		// We don't implement regions, but register a no-op handler
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
			s.setHandler(s.pointerID, func(_ uint32, _ uint16, _ []byte, fds []int) {
				closeFDs(fds)
			})
		}

	case seatGetKeyboardOp:
		if len(payload) >= 4 {
			s.keyboardID = getUint32(payload, 0)
			s.setHandler(s.keyboardID, func(_ uint32, _ uint16, _ []byte, fds []int) {
				closeFDs(fds)
			})
			// Send keymap
			s.sendKeymap()
		}
	}
}

// sendKeymap sends wl_keyboard.keymap to the client.
func (s *clientSession) sendKeymap() {
	fd, size, err := createKeymapFD()
	if err != nil {
		log.Printf("failed to create keymap fd: %v", err)
		// Send no-keymap as fallback
		p := make([]byte, 8)
		putUint32(p, 0, keyboardKeymapFormatNoKeymap)
		putUint32(p, 4, 0)
		s.conn.sendMsg(s.keyboardID, keyboardKeymapEvent, p)
		return
	}

	// wl_keyboard.keymap: format(uint32), fd(passed via SCM_RIGHTS), size(uint32)
	p := make([]byte, 8)
	putUint32(p, 0, keyboardKeymapFormatXKBv1)
	putUint32(p, 4, uint32(size))
	s.conn.sendMsg(s.keyboardID, keyboardKeymapEvent, p, fd)

}

// serial returns the next event serial and increments.
func (s *clientSession) serial() uint32 {
	s.nextSerial++
	return s.nextSerial - 1
}
