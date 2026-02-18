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

import "sync"

type subsurfaceState struct {
	id      uint32
	surface *surfaceState
	parent  *surfaceState
	session *clientSession

	mu     sync.RWMutex
	x, y   int
	desync bool
}

func (ss *subsurfaceState) position() (int, int) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.x, ss.y
}

func (s *clientSession) handleSubcompositorRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	switch opcode {
	case subcompositorGetSubsurfaceOp:
		// get_subsurface(id, surface, parent)
		if len(payload) < 12 {
			return
		}
		subID := getUint32(payload, 0)
		surfID := getUint32(payload, 4)
		parentID := getUint32(payload, 8)

		surf, ok := s.surfaces[surfID]
		if !ok {
			return
		}
		parent, ok := s.surfaces[parentID]
		if !ok {
			return
		}

		ss := &subsurfaceState{
			id:      subID,
			surface: surf,
			parent:  parent,
			session: s,
		}
		s.subMu.Lock()
		s.subsurfaces[subID] = ss
		s.subMu.Unlock()
		s.setHandler(subID, s.handleSubsurfaceRequest)

	case subcompositorDestroyOp:
		// wl_subcompositor is a global; ignore destroy.
	}
}

func (s *clientSession) handleSubsurfaceRequest(id uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)

	s.subMu.RLock()
	ss := s.subsurfaces[id]
	s.subMu.RUnlock()
	if ss == nil {
		return
	}

	switch opcode {
	case subsurfaceDestroyOp:
		s.removeHandler(id)
		s.subMu.Lock()
		delete(s.subsurfaces, id)
		s.subMu.Unlock()
		// Subsurface destroyed; next parent commit will skip it

	case subsurfaceSetPositionOp:
		if len(payload) < 8 {
			return
		}
		x := int(getInt32(payload, 0))
		y := int(getInt32(payload, 4))
		ss.mu.Lock()
		ss.x = x
		ss.y = y
		ss.mu.Unlock()

	case subsurfacePlaceAboveOp, subsurfacePlaceBelowOp:
		// Keep insertion order for now.

	case subsurfaceSetSyncOp:
		ss.mu.Lock()
		ss.desync = false
		ss.mu.Unlock()

	case subsurfaceSetDesyncOp:
		ss.mu.Lock()
		ss.desync = true
		ss.mu.Unlock()
	}
}
