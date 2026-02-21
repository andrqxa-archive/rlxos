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

import "log"

// xdgSurfaceState represents an xdg_surface.
type xdgSurfaceState struct {
	id         uint32
	surface    *surfaceState
	session    *clientSession
	toplevel   *xdgToplevelState
	popup      *xdgPopupState
	configured bool
}

// xdgToplevelState represents an xdg_toplevel.
type xdgToplevelState struct {
	id      uint32
	xdgSurf *xdgSurfaceState
	title   string
	appID   string
}

// xdgPositionerState tracks xdg_positioner parameters for popup placement.
type xdgPositionerState struct {
	id uint32

	width  int
	height int

	anchorRectX int
	anchorRectY int
	offsetX     int
	offsetY     int
}

// xdgPopupState represents an xdg_popup.
type xdgPopupState struct {
	id       uint32
	xdgSurf  *xdgSurfaceState
	parent   *xdgSurfaceState
	position *xdgPositionerState

	x      int
	y      int
	width  int
	height int
}

// handleXdgWmBaseRequest processes xdg_wm_base requests.
func (s *clientSession) handleXdgWmBaseRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	switch opcode {
	case xdgWmBaseCreatePosOp:
		if len(payload) >= 4 {
			positionerID := getUint32(payload, 0)
			pos := &xdgPositionerState{id: positionerID}
			s.positioners[positionerID] = pos
			s.setHandler(positionerID, s.handleXdgPositionerRequest)
		}

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
		// Client responded to ping

	case xdgWmBaseDestroyOp:
		// Don't remove the handler since other objects may still reference it
	}
}

func (s *clientSession) handleXdgPositionerRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	pos, ok := s.positioners[id]
	if !ok {
		return
	}

	switch opcode {
	case xdgPositionerSetSizeOp:
		if len(payload) >= 8 {
			pos.width = int(getInt32(payload, 0))
			pos.height = int(getInt32(payload, 4))
		}

	case xdgPositionerSetAnchorRectOp:
		if len(payload) >= 16 {
			pos.anchorRectX = int(getInt32(payload, 0))
			pos.anchorRectY = int(getInt32(payload, 4))
		}

	case xdgPositionerSetOffsetOp:
		if len(payload) >= 8 {
			pos.offsetX = int(getInt32(payload, 0))
			pos.offsetY = int(getInt32(payload, 4))
		}

	case xdgPositionerDestroyOp:
		s.removeHandler(id)
		delete(s.positioners, id)
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

			// Create a real display window for this toplevel
			if err := s.server.registerToplevel(s, xdgSurf.surface, toplevel); err != nil {
				log.Printf("failed to create display window: %v", err)
				s.conn.sendMsg(toplevelID, xdgToplevelCloseEvent, nil)
				return
			}

			// Send initial configure with the display window size
			tw := xdgSurf.surface.displayWindow
			if tw != nil && tw.win != nil {
				s.sendToplevelConfigure(toplevel, tw.win.Width, tw.win.Height)
			} else {
				s.sendToplevelConfigure(toplevel, defaultToplevelWidth, defaultToplevelHeight)
			}
		}

	case xdgSurfaceGetPopupOp:
		if len(payload) >= 12 {
			popupID := getUint32(payload, 0)
			parentXDGID := getUint32(payload, 4)
			positionerID := getUint32(payload, 8)

			parent, ok := s.xdgSurfaces[parentXDGID]
			if !ok {
				return
			}

			popup := &xdgPopupState{
				id:      popupID,
				xdgSurf: xdgSurf,
				parent:  parent,
			}
			if pos, ok := s.positioners[positionerID]; ok {
				popup.position = pos
				popup.applyPositioner(pos)
			}
			w, h, clamped := clampSurfaceSize(
				popup.width,
				popup.height,
				defaultPopupWidth,
				defaultPopupHeight,
			)
			if clamped {
				log.Printf("clamped popup size from %dx%d to %dx%d", popup.width, popup.height, w, h)
			}
			popup.width = w
			popup.height = h

			xdgSurf.popup = popup
			s.xdgPopups[popupID] = popup
			s.setHandler(popupID, s.handleXdgPopupRequest)

			if err := s.server.registerPopup(s, xdgSurf.surface, popup); err != nil {
				log.Printf("failed to create popup window: %v", err)
				s.conn.sendMsg(popupID, xdgPopupPopupDoneEvent, nil)
				return
			}

			s.sendPopupConfigure(popup, popup.x, popup.y, popup.width, popup.height)
		}

	case xdgSurfaceAckConfigureOp:
		xdgSurf.configured = true

	case xdgSurfaceSetGeometryOp:
		// Ignore geometry hints

	case xdgSurfaceDestroyOp:
		if xdgSurf.toplevel != nil {
			s.server.removeToplevel(xdgSurf.toplevel)
			s.removeHandler(xdgSurf.toplevel.id)
			delete(s.xdgToplevels, xdgSurf.toplevel.id)
		}
		if xdgSurf.popup != nil {
			s.server.removePopup(xdgSurf.popup)
			s.removeHandler(xdgSurf.popup.id)
			delete(s.xdgPopups, xdgSurf.popup.id)
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
			// Forward title to the display window
			if toplevel.xdgSurf != nil && toplevel.xdgSurf.surface != nil {
				tw := toplevel.xdgSurf.surface.displayWindow
				if tw != nil && tw.win != nil {
					tw.win.SetTitle(title)
				}
			}
		}

	case xdgToplevelSetAppIDOp:
		if len(payload) >= 4 {
			appID, _ := getString(payload, 0)
			toplevel.appID = appID
		}

	case xdgToplevelSetMinSizeOp, xdgToplevelSetMaxSizeOp:
		// Ignore size constraints

	case xdgToplevelDestroyOp:
		s.server.removeToplevel(toplevel)
		if toplevel.xdgSurf != nil {
			toplevel.xdgSurf.toplevel = nil
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

func (s *clientSession) handleXdgPopupRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	popup, ok := s.xdgPopups[id]
	if !ok {
		return
	}

	switch opcode {
	case xdgPopupDestroyOp:
		s.server.removePopup(popup)
		s.removeHandler(id)
		delete(s.xdgPopups, id)
		if popup.xdgSurf != nil {
			popup.xdgSurf.popup = nil
		}

	case xdgPopupGrabOp:
		// The display service controls focus/activation.

	case xdgPopupRepositionOp:
		if len(payload) < 8 {
			return
		}
		positionerID := getUint32(payload, 0)
		token := getUint32(payload, 4)
		if pos, ok := s.positioners[positionerID]; ok {
			popup.position = pos
			popup.applyPositioner(pos)
		}

		w, h, clamped := clampSurfaceSize(
			popup.width,
			popup.height,
			defaultPopupWidth,
			defaultPopupHeight,
		)
		if clamped {
			log.Printf("clamped popup reposition size from %dx%d to %dx%d", popup.width, popup.height, w, h)
		}
		popup.width = w
		popup.height = h

		if popup.xdgSurf != nil && popup.xdgSurf.surface != nil {
			tw := popup.xdgSurf.surface.displayWindow
			if tw != nil && tw.win != nil {
				if popup.width != tw.win.Width || popup.height != tw.win.Height {
					if err := tw.win.Resize(popup.width, popup.height); err != nil {
						log.Printf("popup resize failed: %v", err)
					}
				}
			}
		}

		s.sendPopupConfigure(popup, popup.x, popup.y, popup.width, popup.height)
		if token != 0 {
			p := make([]byte, 4)
			putUint32(p, 0, token)
			s.conn.sendMsg(id, xdgPopupRepositionedEvent, p)
		}
	}
}

func (p *xdgPopupState) applyPositioner(pos *xdgPositionerState) {
	if p == nil || pos == nil {
		return
	}
	p.x = pos.anchorRectX + pos.offsetX
	p.y = pos.anchorRectY + pos.offsetY
	p.width = pos.width
	p.height = pos.height
}

// sendToplevelConfigure sends the xdg_toplevel.configure + xdg_surface.configure sequence.
func (s *clientSession) sendToplevelConfigure(toplevel *xdgToplevelState, width, height int) {
	width, height, _ = clampSurfaceSize(width, height, defaultToplevelWidth, defaultToplevelHeight)

	// xdg_toplevel.configure(width, height, states)
	var statesPayload []byte
	if s.server.isFocusedToplevel(toplevel) {
		statesPayload = make([]byte, 12+4)
		putInt32(statesPayload, 0, int32(width))
		putInt32(statesPayload, 4, int32(height))
		putUint32(statesPayload, 8, 4)
		putUint32(statesPayload, 12, xdgToplevelStateActivated)
	} else {
		statesPayload = make([]byte, 12)
		putInt32(statesPayload, 0, int32(width))
		putInt32(statesPayload, 4, int32(height))
		putUint32(statesPayload, 8, 0)
	}
	s.conn.sendMsg(toplevel.id, xdgToplevelConfigureEvent, statesPayload)

	s.sendSurfaceConfigure(toplevel.xdgSurf)
}

func (s *clientSession) sendPopupConfigure(popup *xdgPopupState, x, y, width, height int) {
	if popup == nil {
		return
	}
	width, height, _ = clampSurfaceSize(width, height, defaultPopupWidth, defaultPopupHeight)
	payload := make([]byte, 16)
	putInt32(payload, 0, int32(x))
	putInt32(payload, 4, int32(y))
	putInt32(payload, 8, int32(width))
	putInt32(payload, 12, int32(height))
	s.conn.sendMsg(popup.id, xdgPopupConfigureEvent, payload)
	s.sendSurfaceConfigure(popup.xdgSurf)
}

func (s *clientSession) sendSurfaceConfigure(xdgSurf *xdgSurfaceState) {
	if xdgSurf == nil {
		return
	}
	serial := s.nextSerial
	s.nextSerial++
	p := make([]byte, 4)
	putUint32(p, 0, serial)
	s.conn.sendMsg(xdgSurf.id, xdgSurfaceConfigureEvent, p)
}
