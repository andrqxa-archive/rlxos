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
	"log"
	"net"
	"sync"
	"time"

	displayapi "avyos.dev/api/display"
	"avyos.dev/pkg/graphics"
)

// toplevelWindow maps a Wayland xdg_toplevel to an AvyOS display window.
type toplevelWindow struct {
	win      *displayapi.ClientWindow
	surface  *surfaceState
	toplevel *xdgToplevelState
	session  *clientSession

	// Per-window pointer state
	pointerEntered bool
	// Per-window keyboard state
	keyboardEntered bool

	// Modifier tracking
	modifiers graphics.Modifiers
	capsLock  bool

	// Track pressed keys for keyboard enter
	pressedKeys []uint32
}

// Server is the Wayland-to-AvyOS display bridge.
// It listens for Wayland clients and creates native AvyOS windows for each toplevel.
type Server struct {
	display  *displayapi.DisplayClient
	listener *net.UnixListener
	name     string

	sessions []*clientSession
	windows  map[uint32]*toplevelWindow // display windowID → toplevel

	mu      sync.Mutex
	quit    chan struct{}
	startMs int64
}

// NewServer creates a new server.
func NewServer() *Server {
	return &Server{
		windows: make(map[uint32]*toplevelWindow),
		quit:    make(chan struct{}),
		startMs: time.Now().UnixMilli(),
	}
}

// Run starts the server. Blocks until quit.
func (s *Server) Run() error {
	// Connect to display server
	cl, err := displayapi.Dial()
	if err != nil {
		return fmt.Errorf("connect to display server: %w", err)
	}
	s.display = cl
	defer cl.Close()

	// Create Wayland server socket
	ln, name, err := listen()
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln
	s.name = name
	defer ln.Close()

	log.Printf("waylayer listening on %s", name)

	// Accept loop for Wayland clients
	go s.acceptLoop()

	// Event loop for display server events
	for {
		select {
		case <-s.quit:
			return nil
		default:
		}

		ev := s.display.Poll()
		if ev == nil {
			// Idle — wait briefly
			select {
			case <-s.quit:
				return nil
			case <-time.After(2 * time.Millisecond):
			}
			continue
		}

		s.handleDisplayEvent(ev)
	}
}

// Quit stops the server.
func (s *Server) Quit() {
	select {
	case <-s.quit:
	default:
		close(s.quit)
	}
}

// acceptLoop accepts new Wayland client connections.
func (s *Server) acceptLoop() {
	for {
		uc, err := s.listener.AcceptUnix()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}

		log.Printf("new Wayland client connected")

		session := newClientSession(uc, s)
		s.mu.Lock()
		s.sessions = append(s.sessions, session)
		s.mu.Unlock()

		go session.run()
	}
}

// handleDisplayEvent routes a display server event to the appropriate Wayland client.
func (s *Server) handleDisplayEvent(ev *displayapi.Event) {
	s.mu.Lock()
	tw := s.windows[ev.WindowID]
	s.mu.Unlock()

	if tw == nil {
		return
	}

	switch ev.Type {
	case displayapi.ClientEventPointerEnter:
		s.handlePointerEnter(tw, ev.X, ev.Y)

	case displayapi.ClientEventPointerLeave:
		s.handlePointerLeave(tw)

	case displayapi.ClientEventPointerMotion:
		s.handlePointerMotion(tw, ev.X, ev.Y)

	case displayapi.ClientEventPointerButton:
		s.handlePointerButton(tw, ev.Button, ev.Pressed)

	case displayapi.ClientEventKey:
		s.handleKey(tw, ev.Key, ev.Char, ev.Pressed)

	case displayapi.ClientEventFocus:
		s.handleFocus(tw, ev.Focused)

	case displayapi.ClientEventConfigure:
		s.handleConfigure(tw, ev.Width, ev.Height)

	case displayapi.ClientEventClose:
		s.handleClose(tw)
	}
}

// handlePointerEnter sends wl_pointer.enter to the client.
func (s *Server) handlePointerEnter(tw *toplevelWindow, x, y int) {
	sess := tw.session
	if sess.pointerID == 0 {
		return
	}
	tw.pointerEntered = true
	serial := sess.serial()
	p := make([]byte, 16)
	putUint32(p, 0, serial)
	putUint32(p, 4, tw.surface.id)
	putInt32(p, 8, intToFixed(x))
	putInt32(p, 12, intToFixed(y))
	sess.conn.sendMsg(sess.pointerID, pointerEnterEvent, p)
	sendPointerFrame(sess)
}

// handlePointerLeave sends wl_pointer.leave to the client.
func (s *Server) handlePointerLeave(tw *toplevelWindow) {
	sess := tw.session
	if sess.pointerID == 0 || !tw.pointerEntered {
		return
	}
	tw.pointerEntered = false
	serial := sess.serial()
	p := make([]byte, 8)
	putUint32(p, 0, serial)
	putUint32(p, 4, tw.surface.id)
	sess.conn.sendMsg(sess.pointerID, pointerLeaveEvent, p)
	sendPointerFrame(sess)
}

// handlePointerMotion sends wl_pointer.motion to the client.
func (s *Server) handlePointerMotion(tw *toplevelWindow, x, y int) {
	sess := tw.session
	if sess.pointerID == 0 {
		return
	}
	if !tw.pointerEntered {
		s.handlePointerEnter(tw, x, y)
		return
	}
	ms := uint32(time.Now().UnixMilli())
	p := make([]byte, 12)
	putUint32(p, 0, ms)
	putInt32(p, 4, intToFixed(x))
	putInt32(p, 8, intToFixed(y))
	sess.conn.sendMsg(sess.pointerID, pointerMotionEvent, p)
	sendPointerFrame(sess)
}

// handlePointerButton sends wl_pointer.button to the client.
func (s *Server) handlePointerButton(tw *toplevelWindow, button int, pressed bool) {
	sess := tw.session
	if sess.pointerID == 0 {
		return
	}
	serial := sess.serial()
	ms := uint32(time.Now().UnixMilli())
	var state uint32
	if pressed {
		state = pointerButtonPressed
	}
	p := make([]byte, 16)
	putUint32(p, 0, serial)
	putUint32(p, 4, ms)
	putUint32(p, 8, uint32(button))
	putUint32(p, 12, state)
	sess.conn.sendMsg(sess.pointerID, pointerButtonEvent, p)
	sendPointerFrame(sess)
}

// handleKey sends wl_keyboard.key to the client.
func (s *Server) handleKey(tw *toplevelWindow, key graphics.Key, char rune, pressed bool) {
	sess := tw.session
	if sess.keyboardID == 0 {
		return
	}

	keycode := keyToEvdev(key)
	if keycode == 0 {
		return
	}

	// Track pressed keys
	if pressed {
		tw.pressedKeys = append(tw.pressedKeys, uint32(keycode))
	} else {
		for i, k := range tw.pressedKeys {
			if k == uint32(keycode) {
				tw.pressedKeys = append(tw.pressedKeys[:i], tw.pressedKeys[i+1:]...)
				break
			}
		}
	}

	serial := sess.serial()
	ms := uint32(time.Now().UnixMilli())
	var state uint32
	if pressed {
		state = keyboardKeyPressed
	}

	p := make([]byte, 16)
	putUint32(p, 0, serial)
	putUint32(p, 4, ms)
	putUint32(p, 8, uint32(keycode))
	putUint32(p, 12, state)
	sess.conn.sendMsg(sess.keyboardID, keyboardKeyEvent, p)

	// Update and send modifiers
	s.updateModifiers(tw, key, pressed)
	s.sendModifiers(tw)
}

// handleFocus sends keyboard enter/leave and configure events.
func (s *Server) handleFocus(tw *toplevelWindow, focused bool) {
	sess := tw.session
	if focused {
		if sess.keyboardID != 0 && !tw.keyboardEntered {
			tw.keyboardEntered = true
			serial := sess.serial()
			keysArraySize := len(tw.pressedKeys) * 4
			p := make([]byte, 8+4+keysArraySize)
			putUint32(p, 0, serial)
			putUint32(p, 4, tw.surface.id)
			putUint32(p, 8, uint32(keysArraySize))
			for i, k := range tw.pressedKeys {
				putUint32(p, 12+i*4, k)
			}
			sess.conn.sendMsg(sess.keyboardID, keyboardEnterEvent, p)
		}
		// Send configure with activated state
		if tw.toplevel != nil && tw.win != nil {
			sess.sendToplevelConfigure(tw.toplevel, tw.win.Width, tw.win.Height)
		}
	} else {
		if sess.keyboardID != 0 && tw.keyboardEntered {
			tw.keyboardEntered = false
			serial := sess.serial()
			p := make([]byte, 8)
			putUint32(p, 0, serial)
			putUint32(p, 4, tw.surface.id)
			sess.conn.sendMsg(sess.keyboardID, keyboardLeaveEvent, p)
		}
		// Send configure without activated state
		if tw.toplevel != nil && tw.win != nil {
			sess.sendToplevelConfigure(tw.toplevel, tw.win.Width, tw.win.Height)
		}
	}
}

// handleConfigure forwards display configure to xdg_toplevel.configure.
func (s *Server) handleConfigure(tw *toplevelWindow, width, height int) {
	if tw.toplevel == nil {
		return
	}
	if tw.win != nil {
		tw.win.Width = width
		tw.win.Height = height
	}
	tw.session.sendToplevelConfigure(tw.toplevel, width, height)
}

// handleClose sends xdg_toplevel.close to the client.
func (s *Server) handleClose(tw *toplevelWindow) {
	if tw.toplevel != nil {
		tw.session.conn.sendMsg(tw.toplevel.id, xdgToplevelCloseEvent, nil)
	}
}

// registerToplevel creates an AvyOS display window for a new xdg_toplevel.
func (s *Server) registerToplevel(session *clientSession, surface *surfaceState, toplevel *xdgToplevelState) error {
	win, err := s.display.CreateWindow(800, 600)
	if err != nil {
		return fmt.Errorf("create display window: %w", err)
	}

	tw := &toplevelWindow{
		win:      win,
		surface:  surface,
		toplevel: toplevel,
		session:  session,
	}

	s.mu.Lock()
	s.windows[win.ID] = tw
	s.mu.Unlock()

	// Store reverse mapping on the surface
	surface.displayWindow = tw

	log.Printf("created display window %d for toplevel %q", win.ID, toplevel.title)

	return nil
}

// removeToplevel destroys the display window for a toplevel.
func (s *Server) removeToplevel(toplevel *xdgToplevelState) {
	s.mu.Lock()
	var winID uint32
	for id, tw := range s.windows {
		if tw.toplevel == toplevel {
			winID = id
			if tw.surface != nil {
				tw.surface.displayWindow = nil
			}
			delete(s.windows, id)
			break
		}
	}
	s.mu.Unlock()

	if winID != 0 {
		// Find the toplevelWindow to destroy
		s.mu.Lock()
		// Already deleted above, but we need the win reference
		s.mu.Unlock()
	}
}

// removeSurface removes any window associated with a surface.
func (s *Server) removeSurface(surf *surfaceState) {
	s.mu.Lock()
	for id, tw := range s.windows {
		if tw.surface == surf {
			delete(s.windows, id)
			tw.win.Destroy()
			break
		}
	}
	s.mu.Unlock()
}

// removeSession removes all windows for a disconnected client.
func (s *Server) removeSession(session *clientSession) {
	s.mu.Lock()
	// Destroy display windows
	for id, tw := range s.windows {
		if tw.session == session {
			tw.win.Destroy()
			delete(s.windows, id)
		}
	}

	// Remove session
	for i, sess := range s.sessions {
		if sess == session {
			s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
}

// isFocusedToplevel checks if a toplevel's display window has focus.
// In the new design, this is tracked via the focus events from the display server.
func (s *Server) isFocusedToplevel(toplevel *xdgToplevelState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tw := range s.windows {
		if tw.toplevel == toplevel {
			return tw.keyboardEntered
		}
	}
	return false
}

// windowForSurface finds the toplevelWindow for a given surface.
func (s *Server) windowForSurface(surf *surfaceState) *toplevelWindow {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tw := range s.windows {
		if tw.surface == surf {
			return tw
		}
	}
	return nil
}

// timestamp returns milliseconds since server start.
func (s *Server) timestamp() uint32 {
	return uint32(time.Now().UnixMilli() - s.startMs)
}

// updateModifiers tracks modifier key state.
func (s *Server) updateModifiers(tw *toplevelWindow, key graphics.Key, pressed bool) {
	switch key {
	case graphics.KeyLeftShift, graphics.KeyRightShift:
		if pressed {
			tw.modifiers |= graphics.ModShift
		} else {
			tw.modifiers &^= graphics.ModShift
		}
	case graphics.KeyLeftCtrl, graphics.KeyRightCtrl:
		if pressed {
			tw.modifiers |= graphics.ModCtrl
		} else {
			tw.modifiers &^= graphics.ModCtrl
		}
	case graphics.KeyLeftAlt, graphics.KeyRightAlt:
		if pressed {
			tw.modifiers |= graphics.ModAlt
		} else {
			tw.modifiers &^= graphics.ModAlt
		}
	case graphics.KeyCapsLock:
		if pressed {
			tw.capsLock = !tw.capsLock
			if tw.capsLock {
				tw.modifiers |= graphics.ModCapsLock
			} else {
				tw.modifiers &^= graphics.ModCapsLock
			}
		}
	}
}

// sendModifiers sends wl_keyboard.modifiers to the client.
func (s *Server) sendModifiers(tw *toplevelWindow) {
	sess := tw.session
	if sess.keyboardID == 0 {
		return
	}

	var depressed uint32
	if tw.modifiers&graphics.ModShift != 0 {
		depressed |= 1 // Shift
	}
	if tw.modifiers&graphics.ModCtrl != 0 {
		depressed |= 4 // Control
	}
	if tw.modifiers&graphics.ModAlt != 0 {
		depressed |= 8 // Mod1
	}

	var locked uint32
	if tw.modifiers&graphics.ModCapsLock != 0 {
		locked |= 2 // Lock
	}

	serial := sess.serial()
	p := make([]byte, 20)
	putUint32(p, 0, serial)
	putUint32(p, 4, depressed)
	putUint32(p, 8, 0) // latched
	putUint32(p, 12, locked)
	putUint32(p, 16, 0) // group
	sess.conn.sendMsg(sess.keyboardID, keyboardModifiersEvent, p)
}

// sendPointerFrame sends wl_pointer.frame event.
func sendPointerFrame(session *clientSession) {
	if session.pointerID != 0 {
		session.conn.sendMsg(session.pointerID, pointerFrameEvent, nil)
	}
}
