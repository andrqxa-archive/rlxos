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
	core "avyos.dev/pkg/graphics/pixmap"
)

// layerSurfaceState represents a zwlr_layer_surface_v1.
type layerSurfaceState struct {
	id      uint32
	surface *surfaceState
	session *clientSession

	layer            uint32
	anchor           uint32
	exclusiveZone    int32
	marginTop        int32
	marginRight      int32
	marginBottom     int32
	marginLeft       int32
	desiredWidth     int32
	desiredHeight    int32
	keyboardInteract uint32
	configured       bool
	configureSerial  uint32
}

// handleLayerShellRequest processes zwlr_layer_shell_v1 requests.
func (s *clientSession) handleLayerShellRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	switch opcode {
	case layerShellGetLayerSurfaceOp:
		// get_layer_surface(id, surface, output, layer, namespace)
		if len(payload) < 16 {
			return
		}
		lsID := getUint32(payload, 0)
		surfID := getUint32(payload, 4)
		// output at offset 8 — we ignore (single output)
		layer := getUint32(payload, 12)
		// namespace is a string after offset 16, we skip it

		surf, ok := s.surfaces[surfID]
		if !ok {
			return
		}

		ls := &layerSurfaceState{
			id:      lsID,
			surface: surf,
			session: s,
			layer:   layer,
		}
		s.layerSurfaces[lsID] = ls
		s.setHandler(lsID, s.handleLayerSurfaceRequest)

		// Register with compositor
		s.compositor.addLayerSurface(ls)

	case layerShellDestroyOp:
		// No-op; the global persists
	}
}

// handleLayerSurfaceRequest processes zwlr_layer_surface_v1 requests.
func (s *clientSession) handleLayerSurfaceRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	ls, ok := s.layerSurfaces[id]
	if !ok {
		return
	}

	switch opcode {
	case layerSurfaceSetSizeOp:
		if len(payload) >= 8 {
			ls.desiredWidth = getInt32(payload, 0)
			ls.desiredHeight = getInt32(payload, 4)
		}

	case layerSurfaceSetAnchorOp:
		if len(payload) >= 4 {
			ls.anchor = getUint32(payload, 0)
		}

	case layerSurfaceSetExclusiveZoneOp:
		if len(payload) >= 4 {
			ls.exclusiveZone = getInt32(payload, 0)
		}

	case layerSurfaceSetMarginOp:
		if len(payload) >= 16 {
			ls.marginTop = getInt32(payload, 0)
			ls.marginRight = getInt32(payload, 4)
			ls.marginBottom = getInt32(payload, 8)
			ls.marginLeft = getInt32(payload, 12)
		}

	case layerSurfaceSetKeyboardInteractOp:
		if len(payload) >= 4 {
			ls.keyboardInteract = getUint32(payload, 0)
		}

	case layerSurfaceAckConfigureOp:
		ls.configured = true

	case layerSurfaceSetLayerOp:
		if len(payload) >= 4 {
			ls.layer = getUint32(payload, 0)
		}

	case layerSurfaceDestroyOp:
		s.compositor.removeLayerSurface(ls)
		s.removeHandler(id)
		delete(s.layerSurfaces, id)

	case layerSurfaceGetPopupOp:
		// Ignore popup parenting
	}
}

// sendLayerSurfaceConfigure sends a configure event to a layer surface.
func (s *clientSession) sendLayerSurfaceConfigure(ls *layerSurfaceState, width, height int) {
	serial := s.serial()
	ls.configureSerial = serial

	p := make([]byte, 12)
	putUint32(p, 0, serial)
	putUint32(p, 4, uint32(width))
	putUint32(p, 8, uint32(height))
	s.conn.sendMsg(ls.id, layerSurfaceConfigureEvent, p)
}

// LayerSurface represents a layer surface in the compositor's render list.
type LayerSurface struct {
	state   *layerSurfaceState
	session *clientSession
	x, y    int
	width   int
	height  int
}

// computeLayerGeometry calculates position and size for a layer surface.
func (ls *LayerSurface) computeGeometry(screenW, screenH int, exclusives [4]int32) {
	anchor := ls.state.anchor
	w := int(ls.state.desiredWidth)
	h := int(ls.state.desiredHeight)

	// If anchored to both horizontal edges, stretch width
	if anchor&(layerAnchorLeft|layerAnchorRight) == (layerAnchorLeft | layerAnchorRight) {
		w = screenW - int(ls.state.marginLeft) - int(ls.state.marginRight)
	}
	// If anchored to both vertical edges, stretch height
	if anchor&(layerAnchorTop|layerAnchorBottom) == (layerAnchorTop | layerAnchorBottom) {
		h = screenH - int(ls.state.marginTop) - int(ls.state.marginBottom)
	}

	if w <= 0 {
		w = screenW
	}
	if h <= 0 {
		h = screenH
	}

	ls.width = w
	ls.height = h

	// Compute position based on anchor
	switch {
	case anchor&layerAnchorLeft != 0 && anchor&layerAnchorRight != 0:
		ls.x = int(ls.state.marginLeft)
	case anchor&layerAnchorRight != 0:
		ls.x = screenW - w - int(ls.state.marginRight)
	case anchor&layerAnchorLeft != 0:
		ls.x = int(ls.state.marginLeft)
	default:
		ls.x = (screenW - w) / 2
	}

	switch {
	case anchor&layerAnchorTop != 0 && anchor&layerAnchorBottom != 0:
		ls.y = int(ls.state.marginTop)
	case anchor&layerAnchorBottom != 0:
		ls.y = screenH - h - int(ls.state.marginBottom)
	case anchor&layerAnchorTop != 0:
		ls.y = int(ls.state.marginTop)
	default:
		ls.y = (screenH - h) / 2
	}
}

// drawLayerSurface renders a layer surface onto the framebuffer.
func drawLayerSurface(buf *core.Buffer, ls *LayerSurface) {
	if ls.state.surface == nil {
		return
	}
	clientBuf := ls.state.surface.getBuffer()
	if clientBuf == nil {
		return
	}
	buf.BlitOpaque(clientBuf, ls.x, ls.y)
}
