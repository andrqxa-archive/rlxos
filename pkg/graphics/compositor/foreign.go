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

import "sync"

// foreignManager tracks one client's zwlr_foreign_toplevel_manager_v1 binding.
type foreignManager struct {
	id      uint32
	session *clientSession
	stopped bool
	// Map from xdgToplevelState pointer to the handle object ID we sent to this manager
	handles map[*xdgToplevelState]uint32
	nextID  uint32
}

// foreignManagerList is the compositor-level list of all foreign manager subscriptions.
type foreignManagerList struct {
	mu       sync.Mutex
	managers []*foreignManager
}

func newForeignManagerList() *foreignManagerList {
	return &foreignManagerList{}
}

// addManager registers a new foreign toplevel manager client.
func (fl *foreignManagerList) addManager(fm *foreignManager) {
	fl.mu.Lock()
	fl.managers = append(fl.managers, fm)
	fl.mu.Unlock()
}

// removeManager unregisters a foreign manager.
func (fl *foreignManagerList) removeManager(fm *foreignManager) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	for i, m := range fl.managers {
		if m == fm {
			fl.managers = append(fl.managers[:i], fl.managers[i+1:]...)
			return
		}
	}
}

// removeSession removes all managers belonging to a session.
func (fl *foreignManagerList) removeSession(session *clientSession) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	for i := 0; i < len(fl.managers); i++ {
		if fl.managers[i].session == session {
			fl.managers = append(fl.managers[:i], fl.managers[i+1:]...)
			i--
		}
	}
}

// notifyNewToplevel tells all managers about a new toplevel.
func (fl *foreignManagerList) notifyNewToplevel(toplevel *xdgToplevelState, ownerSession *clientSession) {
	fl.mu.Lock()
	managers := make([]*foreignManager, len(fl.managers))
	copy(managers, fl.managers)
	fl.mu.Unlock()

	for _, fm := range managers {
		if fm.stopped {
			continue
		}
		fm.sendToplevel(toplevel)
	}
}

// notifyToplevelTitle tells all managers the title changed.
func (fl *foreignManagerList) notifyToplevelTitle(toplevel *xdgToplevelState) {
	fl.mu.Lock()
	managers := make([]*foreignManager, len(fl.managers))
	copy(managers, fl.managers)
	fl.mu.Unlock()

	for _, fm := range managers {
		if fm.stopped {
			continue
		}
		handleID, ok := fm.handles[toplevel]
		if !ok {
			continue
		}
		fm.session.conn.sendMsg(handleID, foreignHandleTitleEvent, encodeString(toplevel.title))
		fm.session.conn.sendMsg(handleID, foreignHandleDoneEvent, nil)
	}
}

// notifyToplevelAppID tells all managers the app_id changed.
func (fl *foreignManagerList) notifyToplevelAppID(toplevel *xdgToplevelState) {
	fl.mu.Lock()
	managers := make([]*foreignManager, len(fl.managers))
	copy(managers, fl.managers)
	fl.mu.Unlock()

	for _, fm := range managers {
		if fm.stopped {
			continue
		}
		handleID, ok := fm.handles[toplevel]
		if !ok {
			continue
		}
		fm.session.conn.sendMsg(handleID, foreignHandleAppIDEvent, encodeString(toplevel.appID))
		fm.session.conn.sendMsg(handleID, foreignHandleDoneEvent, nil)
	}
}

// notifyToplevelState tells all managers the state changed (activated, etc.).
func (fl *foreignManagerList) notifyToplevelState(toplevel *xdgToplevelState, activated bool) {
	fl.mu.Lock()
	managers := make([]*foreignManager, len(fl.managers))
	copy(managers, fl.managers)
	fl.mu.Unlock()

	for _, fm := range managers {
		if fm.stopped {
			continue
		}
		handleID, ok := fm.handles[toplevel]
		if !ok {
			continue
		}
		fm.sendState(handleID, activated)
	}
}

// notifyToplevelClosed tells all managers a toplevel was closed.
func (fl *foreignManagerList) notifyToplevelClosed(toplevel *xdgToplevelState) {
	fl.mu.Lock()
	managers := make([]*foreignManager, len(fl.managers))
	copy(managers, fl.managers)
	fl.mu.Unlock()

	for _, fm := range managers {
		if fm.stopped {
			continue
		}
		handleID, ok := fm.handles[toplevel]
		if !ok {
			continue
		}
		fm.session.conn.sendMsg(handleID, foreignHandleClosedEvent, nil)
		delete(fm.handles, toplevel)
	}
}

// sendToplevel sends a new toplevel event + initial state to a manager.
func (fm *foreignManager) sendToplevel(toplevel *xdgToplevelState) {
	// Allocate a new handle ID in the client's object space
	// We use the manager's nextID counter to avoid collisions
	fm.nextID++
	handleID := fm.nextID + 0xFF000000 // Use high range to avoid collision with client IDs

	fm.handles[toplevel] = handleID

	// Send toplevel event: new_id
	p := make([]byte, 4)
	putUint32(p, 0, handleID)
	fm.session.conn.sendMsg(fm.id, foreignManagerToplevelEvent, p)

	// Register handler for handle requests
	fm.session.setHandler(handleID, func(id uint32, opcode uint16, payload []byte, fds []int) {
		closeFDs(fds)
		fm.handleHandleRequest(toplevel, id, opcode, payload)
	})

	// Send initial properties
	fm.session.conn.sendMsg(handleID, foreignHandleTitleEvent, encodeString(toplevel.title))
	if toplevel.appID != "" {
		fm.session.conn.sendMsg(handleID, foreignHandleAppIDEvent, encodeString(toplevel.appID))
	}
	// Send state
	fm.sendState(handleID, fm.session.compositor.isFocusedToplevel(toplevel))
}

// sendState sends the current state array for a handle.
func (fm *foreignManager) sendState(handleID uint32, activated bool) {
	var states []uint32
	if activated {
		states = append(states, foreignStateActivated)
	}

	// wl_array: 4 bytes size + data
	arraySize := len(states) * 4
	p := make([]byte, 4+arraySize)
	putUint32(p, 0, uint32(arraySize))
	for i, s := range states {
		putUint32(p, 4+i*4, s)
	}
	fm.session.conn.sendMsg(handleID, foreignHandleStateEvent, p)
	fm.session.conn.sendMsg(handleID, foreignHandleDoneEvent, nil)
}

// handleHandleRequest processes requests on a foreign toplevel handle.
func (fm *foreignManager) handleHandleRequest(toplevel *xdgToplevelState, id uint32, opcode uint16, payload []byte) {
	switch opcode {
	case foreignHandleActivateOp:
		// Activate: raise and focus the window
		fm.session.compositor.activateToplevel(toplevel)

	case foreignHandleCloseOp:
		// Send close to the owning client
		fm.session.compositor.closeToplevel(toplevel)

	case foreignHandleDestroyOp:
		fm.session.removeHandler(id)
		delete(fm.handles, toplevel)

	case foreignHandleSetMaximizedOp, foreignHandleUnsetMaximizedOp,
		foreignHandleSetMinimizedOp, foreignHandleUnsetMinimizedOp,
		foreignHandleSetFullscreenOp, foreignHandleUnsetFullscreenOp:
		// Not implemented yet

	case foreignHandleSetRectangleOp:
		// Ignore rectangle hints
	}
}

// handleForeignManagerRequest processes zwlr_foreign_toplevel_manager_v1 requests.
func (s *clientSession) handleForeignManagerRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	fm := s.foreignManagers[id]
	if fm == nil {
		return
	}

	switch opcode {
	case foreignManagerStopOp:
		fm.stopped = true
		// Send finished
		s.conn.sendMsg(id, foreignManagerFinishedEvent, nil)
		s.compositor.foreign.removeManager(fm)
		s.removeHandler(id)
		delete(s.foreignManagers, id)
	}
}
