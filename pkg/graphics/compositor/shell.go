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

// xdgSurfaceState represents an xdg_surface.
type xdgSurfaceState struct {
	id         uint32
	surface    *surfaceState
	session    *clientSession
	toplevel   *xdgToplevelState
	configured bool
}

// xdgToplevelState represents an xdg_toplevel.
type xdgToplevelState struct {
	id      uint32
	xdgSurf *xdgSurfaceState
	title   string
	appID   string
}

// handleXdgWmBaseRequest processes xdg_wm_base requests.
func (s *clientSession) handleXdgWmBaseRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	switch opcode {
	case xdgWmBaseGetXdgSurfaceOp:
		if len(payload) >= 8 {
			xdgSurfID := getUint32(payload, 0)
			surfID := getUint32(payload, 4)

			surf, ok := s.surfaces[surfID]
			if !ok {
				return
			}

			xdgSurf := &xdgSurfaceState{
				id:      xdgSurfID,
				surface: surf,
				session: s,
			}
			s.xdgSurfaces[xdgSurfID] = xdgSurf
			s.setHandler(xdgSurfID, s.handleXdgSurfaceRequest)
		}

	case xdgWmBasePongOp:
		// Client responded to ping, good

	case xdgWmBaseDestroyOp:
		// Don't remove the handler since other objects may still reference it
	}
}

// handleXdgSurfaceRequest processes xdg_surface requests.
func (s *clientSession) handleXdgSurfaceRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	xdgSurf, ok := s.xdgSurfaces[id]
	if !ok {
		return
	}

	switch opcode {
	case xdgSurfaceGetToplevelOp:
		if len(payload) >= 4 {
			toplevelID := getUint32(payload, 0)

			toplevel := &xdgToplevelState{
				id:      toplevelID,
				xdgSurf: xdgSurf,
				title:   "Untitled",
			}
			xdgSurf.toplevel = toplevel
			s.xdgToplevels[toplevelID] = toplevel
			s.setHandler(toplevelID, s.handleXdgToplevelRequest)

			// Register window with compositor
			s.compositor.registerToplevel(s, xdgSurf.surface, toplevel)

			// Send initial configure: width=0, height=0 means client chooses
			s.sendToplevelConfigure(toplevel, 0, 0)
		}

	case xdgSurfaceAckConfigureOp:
		xdgSurf.configured = true

	case xdgSurfaceSetGeometryOp:
		// Ignore geometry hints for now

	case xdgSurfaceDestroyOp:
		if xdgSurf.toplevel != nil {
			s.removeHandler(xdgSurf.toplevel.id)
			delete(s.xdgToplevels, xdgSurf.toplevel.id)
		}
		s.removeHandler(id)
		delete(s.xdgSurfaces, id)
	}
}

// handleXdgToplevelRequest processes xdg_toplevel requests.
func (s *clientSession) handleXdgToplevelRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	toplevel, ok := s.xdgToplevels[id]
	if !ok {
		return
	}

	switch opcode {
	case xdgToplevelSetTitleOp:
		if len(payload) >= 4 {
			title, _ := getString(payload, 0)
			toplevel.title = title
			s.compositor.requestRedraw()
			s.compositor.foreign.notifyToplevelTitle(toplevel)
		}

	case xdgToplevelSetAppIDOp:
		if len(payload) >= 4 {
			appID, _ := getString(payload, 0)
			toplevel.appID = appID
			s.compositor.foreign.notifyToplevelAppID(toplevel)
		}

	case xdgToplevelSetMinSizeOp, xdgToplevelSetMaxSizeOp:
		// Ignore size constraints for now

	case xdgToplevelDestroyOp:
		if toplevel.xdgSurf != nil {
			s.compositor.removeWindow(toplevel)
		}
		s.removeHandler(id)
		delete(s.xdgToplevels, id)

	case xdgToplevelSetMaximizedOp, xdgToplevelUnsetMaximizedOp,
		xdgToplevelSetFullscreenOp, xdgToplevelUnsetFullscreenOp,
		xdgToplevelSetMinimizedOp:
		// Ignore for now

	case xdgToplevelMoveOp, xdgToplevelResizeOp, xdgToplevelShowWindowMenuOp,
		xdgToplevelSetParentOp:
		// Ignore interactive move/resize requests
	}
}

// sendToplevelConfigure sends the xdg_toplevel.configure + xdg_surface.configure sequence.
func (s *clientSession) sendToplevelConfigure(toplevel *xdgToplevelState, width, height int) {
	serial := s.nextSerial
	s.nextSerial++

	// xdg_toplevel.configure(width, height, states)
	// states is an array: 4 bytes length prefix + 4 bytes per state
	var statesPayload []byte
	if s.compositor.isFocusedToplevel(toplevel) {
		// Send activated state
		statesPayload = make([]byte, 12+4) // width + height + array(1 element)
		putInt32(statesPayload, 0, int32(width))
		putInt32(statesPayload, 4, int32(height))
		// wl_array: size=4, then the data
		putUint32(statesPayload, 8, 4)
		putUint32(statesPayload, 12, xdgToplevelStateActivated)
	} else {
		statesPayload = make([]byte, 12) // width + height + empty array
		putInt32(statesPayload, 0, int32(width))
		putInt32(statesPayload, 4, int32(height))
		putUint32(statesPayload, 8, 0) // empty array
	}
	s.conn.sendMsg(toplevel.id, xdgToplevelConfigureEvent, statesPayload)

	// xdg_surface.configure(serial)
	p := make([]byte, 4)
	putUint32(p, 0, serial)
	s.conn.sendMsg(toplevel.xdgSurf.id, xdgSurfaceConfigureEvent, p)
}
