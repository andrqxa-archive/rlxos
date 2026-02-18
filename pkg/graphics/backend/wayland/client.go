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
	"sync"
)

// Wayland protocol object IDs
const (
	objDisplay = 1 // wl_display is always object 1
)

// wl_display opcodes (client → server)
const (
	displaySyncOp        = 0
	displayGetRegistryOp = 1
)

// wl_display events (server → client)
const (
	displayErrorEvent    = 0
	displayDeleteIDEvent = 1
)

// wl_registry opcodes (client → server)
const (
	registryBindOp = 0
)

// wl_registry events (server → client)
const (
	registryGlobalEvent       = 0
	registryGlobalRemoveEvent = 1
)

// wl_compositor opcodes
const (
	compositorCreateSurfaceOp = 0
)

// wl_surface opcodes
const (
	surfaceDestroyOp         = 0
	surfaceAttachOp          = 1
	surfaceDamageOp          = 2
	surfaceFrameOp           = 3
	surfaceSetOpaqueRegionOp = 4
	surfaceSetInputRegionOp  = 5
	surfaceCommitOp          = 6
	surfaceDamageBufferOp    = 9
)

// wl_callback events
const (
	callbackDoneEvent = 0
)

// messageHandler handles a received Wayland event.
type messageHandler func(objectID uint32, opcode uint16, payload []byte, fds []int)

// client manages the Wayland protocol connection.
type client struct {
	conn       *conn
	nextID     uint32
	mu         sync.Mutex
	handlers   map[uint32]messageHandler
	registry   uint32
	compositor uint32
	shm        uint32
	shmFormats []uint32
	xdgWmBase  uint32
	seat       uint32
	surface    uint32

	// Sync support
	syncDone chan struct{}
	syncID   uint32
}

// newClient creates a new Wayland client and connects to the compositor.
func newClient() (*client, error) {
	c, err := dial()
	if err != nil {
		return nil, err
	}

	cl := &client{
		conn:     c,
		nextID:   2, // 1 is wl_display
		handlers: make(map[uint32]messageHandler),
	}

	// Register handler for wl_display
	cl.handlers[objDisplay] = cl.handleDisplay

	return cl, nil
}

// close closes the client connection.
func (cl *client) close() error {
	return cl.conn.close()
}

// allocID allocates a new object ID.
func (cl *client) allocID() uint32 {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	id := cl.nextID
	cl.nextID++
	return id
}

// setHandler registers a handler for an object ID.
func (cl *client) setHandler(id uint32, handler messageHandler) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.handlers[id] = handler
}

// removeHandler removes the handler for an object ID.
func (cl *client) removeHandler(id uint32) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	delete(cl.handlers, id)
}

// getHandler returns the handler for an object ID.
func (cl *client) getHandler(id uint32) messageHandler {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.handlers[id]
}

// dispatch reads and dispatches one message from the compositor.
func (cl *client) dispatch() error {
	objectID, opcode, payload, fds, err := cl.conn.recvMsg()
	if err != nil {
		return err
	}

	handler := cl.getHandler(objectID)
	if handler != nil {
		handler(objectID, opcode, payload, fds)
	} else {
		// Close any received fds we can't handle
		for _, fd := range fds {
			syscallClose(fd)
		}
	}

	return nil
}

// roundtrip sends a sync request and blocks until the callback fires.
func (cl *client) roundtrip() error {
	callbackID := cl.allocID()
	cl.syncDone = make(chan struct{})
	cl.syncID = callbackID

	cl.setHandler(callbackID, cl.handleCallback)

	// wl_display.sync(callback)
	payload := make([]byte, 4)
	putUint32(payload, 0, callbackID)
	if err := cl.conn.sendMsg(objDisplay, displaySyncOp, payload); err != nil {
		return err
	}

	// Dispatch until sync callback fires
	for {
		select {
		case <-cl.syncDone:
			cl.removeHandler(callbackID)
			return nil
		default:
			if err := cl.dispatch(); err != nil {
				return err
			}
		}
	}
}

// getRegistry requests the global registry.
func (cl *client) getRegistry() error {
	cl.registry = cl.allocID()
	cl.setHandler(cl.registry, cl.handleRegistry)

	payload := make([]byte, 4)
	putUint32(payload, 0, cl.registry)
	return cl.conn.sendMsg(objDisplay, displayGetRegistryOp, payload)
}

// bindGlobals performs the initial handshake: get registry, roundtrip to discover globals.
func (cl *client) bindGlobals() error {
	if err := cl.getRegistry(); err != nil {
		return fmt.Errorf("get registry: %w", err)
	}

	if err := cl.roundtrip(); err != nil {
		return fmt.Errorf("roundtrip: %w", err)
	}

	if cl.compositor == 0 {
		return fmt.Errorf("wl_compositor not found")
	}
	if cl.shm == 0 {
		return fmt.Errorf("wl_shm not found")
	}
	if cl.xdgWmBase == 0 {
		return fmt.Errorf("xdg_wm_base not found")
	}

	// Do another roundtrip to get shm formats
	if err := cl.roundtrip(); err != nil {
		return fmt.Errorf("roundtrip for shm formats: %w", err)
	}

	return nil
}

// createSurface creates a wl_surface.
func (cl *client) createSurface() error {
	cl.surface = cl.allocID()

	payload := make([]byte, 4)
	putUint32(payload, 0, cl.surface)
	return cl.conn.sendMsg(cl.compositor, compositorCreateSurfaceOp, payload)
}

// surfaceAttach attaches a buffer to the surface.
func (cl *client) surfaceAttach(bufferID uint32, x, y int32) error {
	payload := make([]byte, 12)
	putUint32(payload, 0, bufferID)
	putInt32(payload, 4, x)
	putInt32(payload, 8, y)
	return cl.conn.sendMsg(cl.surface, surfaceAttachOp, payload)
}

// surfaceDamage marks a region as needing redraw.
func (cl *client) surfaceDamage(x, y, w, h int32) error {
	payload := make([]byte, 16)
	putInt32(payload, 0, x)
	putInt32(payload, 4, y)
	putInt32(payload, 8, w)
	putInt32(payload, 12, h)
	return cl.conn.sendMsg(cl.surface, surfaceDamageOp, payload)
}

// surfaceCommit commits pending changes.
func (cl *client) surfaceCommit() error {
	return cl.conn.sendMsg(cl.surface, surfaceCommitOp, nil)
}

// --- Event handlers ---

func (cl *client) handleDisplay(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	switch opcode {
	case displayErrorEvent:
		if len(payload) >= 12 {
			objID := getUint32(payload, 0)
			code := getUint32(payload, 4)
			msg, _ := getString(payload, 8)
			fmt.Printf("wayland: display error: object %d, code %d: %s\n", objID, code, msg)
		}
	case displayDeleteIDEvent:
		// Object ID can be reused, but we don't bother
	}
}

func (cl *client) handleCallback(_ uint32, opcode uint16, _ []byte, fds []int) {
	closeFDs(fds)
	if opcode == callbackDoneEvent {
		if cl.syncDone != nil {
			close(cl.syncDone)
		}
	}
}

func (cl *client) handleRegistry(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	switch opcode {
	case registryGlobalEvent:
		if len(payload) < 8 {
			return
		}
		name := getUint32(payload, 0)
		iface, consumed := getString(payload, 4)
		version := getUint32(payload, 4+consumed)
		cl.handleGlobal(name, iface, version)
	case registryGlobalRemoveEvent:
		// Global removed, ignore for now
	}
}

func (cl *client) handleGlobal(name uint32, iface string, version uint32) {
	switch iface {
	case "wl_compositor":
		cl.compositor = cl.allocID()
		cl.registryBind(name, iface, min32(version, 4), cl.compositor)

	case "wl_shm":
		cl.shm = cl.allocID()
		cl.setHandler(cl.shm, cl.handleShm)
		cl.registryBind(name, iface, min32(version, 1), cl.shm)

	case "xdg_wm_base":
		cl.xdgWmBase = cl.allocID()
		cl.setHandler(cl.xdgWmBase, cl.handleXdgWmBase)
		cl.registryBind(name, iface, min32(version, 2), cl.xdgWmBase)

	case "wl_seat":
		if cl.seat == 0 { // only bind first seat
			cl.seat = cl.allocID()
			cl.setHandler(cl.seat, cl.handleSeat)
			cl.registryBind(name, iface, min32(version, 5), cl.seat)
		}
	}
}

// registryBind sends wl_registry.bind.
func (cl *client) registryBind(name uint32, iface string, version, newID uint32) {
	ifaceEnc := encodeString(iface)
	payload := make([]byte, 4+len(ifaceEnc)+4+4)
	putUint32(payload, 0, name)
	copy(payload[4:], ifaceEnc)
	off := 4 + len(ifaceEnc)
	putUint32(payload, off, version)
	putUint32(payload, off+4, newID)
	cl.conn.sendMsg(cl.registry, registryBindOp, payload)
}

// handleShm handles wl_shm events (format advertisements).
func (cl *client) handleShm(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	if opcode == 0 && len(payload) >= 4 { // wl_shm.format
		format := getUint32(payload, 0)
		cl.shmFormats = append(cl.shmFormats, format)
	}
}

// handleXdgWmBase and handleSeat are placeholders, set up by xdg.go and seat.go
func (cl *client) handleXdgWmBase(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	// xdg_wm_base.ping
	if opcode == 0 && len(payload) >= 4 {
		serial := getUint32(payload, 0)
		// pong
		resp := make([]byte, 4)
		putUint32(resp, 0, serial)
		cl.conn.sendMsg(cl.xdgWmBase, 3, resp) // opcode 3 = pong
	}
}

func (cl *client) handleSeat(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	// Default no-op; overridden by Backend
}

func closeFDs(fds []int) {
	for _, fd := range fds {
		syscallClose(fd)
	}
}

func min32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
