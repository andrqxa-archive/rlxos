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
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	displayapi "avyos.dev/api/display"
	"avyos.dev/pkg/graphics"
	"avyos.dev/pkg/identity"
	"avyos.dev/pkg/simd"
	"avyos.dev/pkg/sutra"
)

const (
	targetFPS           = 60
	windowOpacity       = uint8(180)
	decorHeight         = 44
	windowCornerRadius  = 12
	windowShadowSpread  = 9
	windowShadowOffsetX = 0
	windowShadowOffsetY = 0
	windowShadowTopClip = 2
	layerShadowSpread   = 8
	layerShadowOffsetX  = 0
	layerShadowOffsetY  = 0
	layerShadowTopClip  = 2
	resizeGripSize      = 14
	minWindowWidth      = 220
	minWindowHeight     = 140
	titleTextInset      = 14
	titleButtonSize     = 18
	titleButtonGap      = 8
	titleButtonRightPad = 10
	procClockTicks      = 100 // Linux USER_HZ fallback for /proc/[pid]/stat.
)

const (
	WindowNormal uint32 = 0
	WindowLayer  uint32 = 1
	WindowPopup  uint32 = 2
)

const (
	LayerBackground = displayapi.LayerBackground
	LayerBottom     = displayapi.LayerBottom
	LayerTop        = displayapi.LayerTop
	LayerOverlay    = displayapi.LayerOverlay
)

const (
	AnchorTop              = displayapi.AnchorTop
	AnchorBottom           = displayapi.AnchorBottom
	AnchorLeft             = displayapi.AnchorLeft
	AnchorRight            = displayapi.AnchorRight
	AnchorHorizontalCenter = displayapi.AnchorHorizontalCenter
	AnchorVerticalCenter   = displayapi.AnchorVerticalCenter
)

const (
	WindowFlagFocused   = displayapi.WindowFlagFocused
	WindowFlagMinimized = displayapi.WindowFlagMinimized
	WindowFlagMaximized = displayapi.WindowFlagMaximized
	WindowFlagVisible   = displayapi.WindowFlagVisible
)

const (
	WindowActionToggleMinimize = displayapi.WindowActionToggleMinimize
	WindowActionMinimize       = displayapi.WindowActionMinimize
	WindowActionRestore        = displayapi.WindowActionRestore
	WindowActionToggleMaximize = displayapi.WindowActionToggleMaximize
	WindowActionFocus          = displayapi.WindowActionFocus
	WindowActionClose          = displayapi.WindowActionClose
)

// window is a client window managed by the server.
type window struct {
	id        uint32
	x, y      int
	width     int
	height    int
	stride    int
	title     string
	focused   bool
	minimized bool
	maximized bool
	restoreX  int
	restoreY  int
	restoreW  int
	restoreH  int

	// Window type and properties.
	winType    uint32  // WindowNormal, WindowLayer, WindowPopup
	layer      uint32  // LayerBackground..LayerOverlay (for layers)
	anchor     uint32  // Anchor bitmask (for layers)
	exclusive  int     // Exclusive zone pixels (for layers)
	parent     *window // Parent window (for popups)
	popupX     int     // Offset relative to parent (for popups)
	popupY     int
	opaqueHint bool // Hint for fully opaque layer surfaces

	// Shm mapping from the client.
	shmFD     int
	shmData   []byte
	shmKey    string
	shmStride int
	shmWidth  int
	shmHeight int

	// Compositor-owned copy of the client pixels.
	bufMu  sync.RWMutex
	buffer *graphics.Buffer

	sess *session
}

func (w *window) getBuffer() *graphics.Buffer {
	w.bufMu.RLock()
	b := w.buffer
	w.bufMu.RUnlock()
	return b
}

// screenPos returns the actual screen position for this window.
// For popups, this is computed from the parent's position.
func (w *window) screenPos() (int, int) {
	if w.winType == WindowPopup && w.parent != nil {
		px, py := w.parent.screenPos()
		return px + w.popupX, py + w.popupY
	}
	return w.x, w.y
}

// session tracks one connected client (Sutra connection).
type session struct {
	id      uint32
	windows map[uint32]*window
}

// userSession tracks a user's login session across multiple client connections.
type userSession struct {
	id      uint32          // Session ID (= UID)
	uid     uint32          // User UID
	active  bool            // Is this the visible session
	clients map[uint32]bool // Sutra client IDs belonging to this session
	focused *window         // Saved focused window for this session
}

type shortcutBinding struct {
	ShortcutID uint32
	WindowID   uint32
	Scope      uint32
	Key        graphics.Key
	Modifiers  graphics.Modifiers
}

// Server is the display compositor.
type Server struct {
	fb      graphics.Backend
	input   graphics.InputHandler
	service *sutra.Service

	sessions map[uint32]*session
	windows  []*window    // normal windows, z-order: back → front
	layers   [4][]*window // [Background][Bottom][Top][Overlay]
	popups   []*window    // popup windows
	nextWID  uint32

	// Usable area (screen minus exclusive zones from layers).
	usableArea graphics.Rect

	// Input state
	mouseX, mouseY int
	focused        *window
	hovered        *window

	// Drag
	dragging                   bool
	dragWin                    *window
	dragStartX, dragStartY     int
	dragWinX, dragWinY         int
	resizing                   bool
	resizeWin                  *window
	resizeStartX, resizeStartY int
	resizeStartW, resizeStartH int

	mu   sync.Mutex
	quit chan struct{}

	// Dirty tracking for partial flush.
	dirtyMu        sync.Mutex
	dirty          bool
	dirtyRects     []graphics.Rect
	prevCursorRect graphics.Rect

	// FPS
	frameCount int
	fps        int
	lastFPSMs  int64

	// Diagnostics overlay (top-right)
	diagRect   graphics.Rect
	diagLines  []string
	cpuPct     float64
	rssBytes   uint64
	heapBytes  uint64
	lastCPUSec float64
	lastCPUAt  time.Time

	drawDebug    bool
	lastDamagePx uint64
	damagePerSec uint64
	damageAccum  uint64
	damageTick   uint64

	redrawPerSec uint64
	redrawAccum  uint64
	redrawTick   uint64

	// Multi-session support
	userSessions    map[uint32]*userSession // session_id -> userSession
	activeSessionID uint32                  // Currently visible session (0 = system)
	clientToSession map[uint32]uint32       // sutra_client_id -> session_id
	shortcuts       map[uint32]map[uint32]shortcutBinding
}

type rectBatchFlusher interface {
	FlushRects([]graphics.Rect) error
}

// NewServer creates a display server.
func NewServer(fb graphics.Backend, input graphics.InputHandler) *Server {
	return &Server{
		fb:              fb,
		input:           input,
		nextWID:         1,
		quit:            make(chan struct{}),
		sessions:        make(map[uint32]*session),
		userSessions:    make(map[uint32]*userSession),
		clientToSession: make(map[uint32]uint32),
		shortcuts:       make(map[uint32]map[uint32]shortcutBinding),
	}
}

// markDirty adds a rectangle to the dirty region.
func (s *Server) markDirty(r graphics.Rect) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	s.dirtyMu.Lock()
	s.dirty = true
	s.dirtyRects = append(s.dirtyRects, r)
	// Bound memory/work under pathological damage storms.
	if len(s.dirtyRects) > 128 {
		merged := s.dirtyRects[0]
		for _, d := range s.dirtyRects[1:] {
			merged = merged.Union(d)
		}
		s.dirtyRects = s.dirtyRects[:1]
		s.dirtyRects[0] = merged
	}
	s.dirtyMu.Unlock()
}

// markAllDirty marks the entire screen dirty.
func (s *Server) markAllDirty() {
	w, h := s.fb.Size()
	s.dirtyMu.Lock()
	s.dirty = true
	s.dirtyRects = s.dirtyRects[:0]
	s.dirtyRects = append(s.dirtyRects, graphics.Rect{X: 0, Y: 0, W: w, H: h})
	s.dirtyMu.Unlock()
}

func (s *Server) hasDirty() bool {
	s.dirtyMu.Lock()
	dirty := s.dirty
	s.dirtyMu.Unlock()
	return dirty
}

func (s *Server) consumeDirtyRects(bounds graphics.Rect, maxRects int) []graphics.Rect {
	s.dirtyMu.Lock()
	if !s.dirty {
		s.dirtyMu.Unlock()
		return nil
	}
	rects := make([]graphics.Rect, len(s.dirtyRects))
	copy(rects, s.dirtyRects)
	s.dirty = false
	s.dirtyRects = s.dirtyRects[:0]
	s.dirtyMu.Unlock()
	return normalizeDamageRectsDisplay(rects, bounds, maxRects)
}

// windowScreenRect returns the screen-space bounding rect of a window (including decorations).
func windowScreenRect(win *window) graphics.Rect {
	switch win.winType {
	case WindowNormal:
		frame := graphics.Rect{X: win.x - 1, Y: win.y - 1, W: win.width + 2, H: win.height + decorHeight + 2}
		shadow := shadowBounds(frame, windowShadowSpread, windowShadowOffsetX, windowShadowOffsetY, windowShadowTopClip)
		if shadow.W > 0 && shadow.H > 0 {
			return frame.Union(shadow)
		}
		return frame
	case WindowPopup:
		sx, sy := win.screenPos()
		return graphics.Rect{X: sx, Y: sy, W: win.width, H: win.height}
	default: // layers
		frame := graphics.Rect{X: win.x, Y: win.y, W: win.width, H: win.height}
		if win.layer == LayerBackground {
			return frame
		}
		shadow := shadowBounds(frame, layerShadowSpread, layerShadowOffsetX, layerShadowOffsetY, layerShadowTopClip)
		if shadow.W > 0 && shadow.H > 0 {
			return frame.Union(shadow)
		}
		return frame
	}
}

func shadowBounds(r graphics.Rect, spread, offsetX, offsetY, topClip int) graphics.Rect {
	shadow := graphics.Rect{
		X: r.X - spread + offsetX,
		Y: r.Y - spread + offsetY,
		W: r.W + 2*spread,
		H: r.H + 2*spread,
	}
	minY := r.Y + topClip
	if shadow.Y < minY {
		d := minY - shadow.Y
		shadow.Y = minY
		shadow.H -= d
	}
	if shadow.H < 0 {
		shadow.H = 0
	}
	return shadow
}

// Run starts the server. Blocks until Quit.
func (s *Server) Run() error {
	if err := s.fb.Open(); err != nil {
		return fmt.Errorf("open framebuffer: %w", err)
	}
	defer s.fb.Close()
	log.Printf("graphics backend running: %s", s.fb.Info())

	if err := s.input.Open(); err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	w, h := s.fb.Size()
	s.input.SetScreenSize(w, h)
	s.input.Start()
	defer s.input.Close()

	// Initialize usable area to full screen
	s.usableArea = graphics.Rect{X: 0, Y: 0, W: w, H: h}

	svc, err := sutra.NewService(displayapi.ServiceName, "")
	if err != nil {
		return fmt.Errorf("start display sutra service: %w", err)
	}
	s.service = svc
	defer svc.Close()
	RegisterHandlers(svc, s)

	// Clean up sessions when clients disconnect.
	svc.Handle(sutra.EventDisconnect, func(t *sutra.Transaction) ([]byte, error) {
		clientID := t.PayloadUint32()
		s.mu.Lock()
		sess, ok := s.sessions[clientID]
		// Clean up client-to-session mapping
		if uid, mapped := s.clientToSession[clientID]; mapped {
			delete(s.clientToSession, clientID)
			if us, exists := s.userSessions[uid]; exists {
				delete(us.clients, clientID)
			}
		}
		delete(s.shortcuts, clientID)
		s.mu.Unlock()
		if ok {
			s.removeSession(sess)
			log.Printf("Cleaned up session for disconnected client %d", clientID)
		}
		return nil, nil
	})

	log.Printf("Display server running (%dx%d)", w, h)

	// Draw first frame.
	s.markAllDirty()
	s.refreshDiagnostics()

	// Main loop: adaptive cadence.
	// - While dirty, composite at target FPS.
	// - While idle, exponentially back off polling to reduce CPU.
	// - Force a lightweight composite at 1Hz so diagnostics stay fresh.
	frameDur := time.Second / targetFPS
	const (
		idleSleepMin = 2 * time.Millisecond
		idleSleepMax = 32 * time.Millisecond
	)
	idleSleep := idleSleepMin
	nextFrame := time.Now()
	lastComposite := time.Now()

	for {
		select {
		case <-s.quit:
			return nil
		default:
		}

		s.processInput()

		now := time.Now()
		if s.hasDirty() {
			if now.Before(nextFrame) {
				remaining := nextFrame.Sub(now)
				if remaining > 0 {
					time.Sleep(remaining)
				}
				continue
			}
			framesBehind := now.Sub(nextFrame) / frameDur
			nextFrame = nextFrame.Add((framesBehind + 1) * frameDur)
			s.composite()
			lastComposite = time.Now()
			idleSleep = idleSleepMin
			continue
		}

		// Keep diagnostics counters live even during long idle periods.
		if now.Sub(lastComposite) >= time.Second {
			s.composite()
			lastComposite = time.Now()
			idleSleep = idleSleepMin
			nextFrame = lastComposite
			continue
		}

		time.Sleep(idleSleep)
		if idleSleep < idleSleepMax {
			idleSleep *= 2
			if idleSleep > idleSleepMax {
				idleSleep = idleSleepMax
			}
		}
		nextFrame = time.Now()
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

// --- Accept & client loop ---

func (s *Server) ensureSession(sender uint32) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sender]
	if ok {
		return sess
	}
	sess = &session{id: sender, windows: make(map[uint32]*window)}
	s.sessions[sender] = sess

	// Map client to user session via peer credentials
	if _, mapped := s.clientToSession[sender]; !mapped {
		if uid, ok := s.service.GetClientUID(sender); ok {
			s.clientToSession[sender] = uid
			if us, exists := s.userSessions[uid]; exists {
				us.clients[sender] = true
			}
		}
	}
	return sess
}

// senderUID resolves the Unix UID of the requesting client.
func (s *Server) senderUID(sender uint32) (uint32, error) {
	if s.service == nil {
		return 0, fmt.Errorf("service unavailable")
	}
	uid, ok := s.service.GetClientUID(sender)
	if !ok {
		return 0, fmt.Errorf("unable to resolve caller identity")
	}
	return uid, nil
}

// requireSessionAdmin ensures only privileged callers can mutate global
// session state (register/switch sessions).
func (s *Server) requireSessionAdmin(sender uint32) error {
	uid, err := s.senderUID(sender)
	if err != nil {
		return err
	}
	if uid != 0 {
		return fmt.Errorf("permission denied")
	}
	return nil
}

func (s *Server) CreateWindow(sender uint32, req displayapi.CreateWindowRequest) (displayapi.WindowCreatedEvent, error) {
	sess := s.ensureSession(sender)
	return s.handleCreateWindow(sess, req.Width, req.Height, req.Stride, req.ShmKey)
}

func (s *Server) DestroyWindow(sender uint32, req displayapi.DestroyWindowRequest) (displayapi.Empty, error) {
	sess := s.ensureSession(sender)
	s.handleDestroyWindow(sess, req.WindowID)
	return displayapi.Empty{}, nil
}

func (s *Server) SetTitle(sender uint32, req displayapi.SetTitleRequest) (displayapi.Empty, error) {
	sess := s.ensureSession(sender)
	s.mu.Lock()
	if win, ok := sess.windows[req.WindowID]; ok {
		win.title = req.Title
	}
	s.mu.Unlock()
	return displayapi.Empty{}, nil
}

func (s *Server) Damage(sender uint32, req displayapi.DamageRequest) (displayapi.Empty, error) {
	sess := s.ensureSession(sender)
	s.handleDamage(sess, req.WindowID, req.X, req.Y, req.Width, req.Height)
	return displayapi.Empty{}, nil
}

func (s *Server) Resize(sender uint32, req displayapi.ResizeRequest) (displayapi.Empty, error) {
	sess := s.ensureSession(sender)
	if err := s.handleResize(sess, req.WindowID, req.Width, req.Height, req.Stride, req.ShmKey); err != nil {
		return displayapi.Empty{}, err
	}
	return displayapi.Empty{}, nil
}

func (s *Server) Move(sender uint32, req displayapi.MoveRequest) (displayapi.Empty, error) {
	sess := s.ensureSession(sender)
	s.mu.Lock()
	if win, ok := sess.windows[req.WindowID]; ok {
		win.x = req.X
		win.y = req.Y
	}
	s.mu.Unlock()
	return displayapi.Empty{}, nil
}

func (s *Server) CreateLayer(sender uint32, req displayapi.CreateLayerRequest) (displayapi.WindowCreatedEvent, error) {
	sess := s.ensureSession(sender)
	return s.handleCreateLayer(sess, req.Layer, req.Anchor, req.Exclusive, req.OpaqueHint, req.Width, req.Height, req.Stride, req.ShmKey)
}

func (s *Server) CreatePopup(sender uint32, req displayapi.CreatePopupRequest) (displayapi.WindowCreatedEvent, error) {
	sess := s.ensureSession(sender)
	return s.handleCreatePopup(sess, req.ParentID, req.X, req.Y, req.Width, req.Height, req.Stride, req.ShmKey)
}

func (s *Server) ListWindows(sender uint32, req displayapi.Empty) (displayapi.WindowListEvent, error) {
	_ = sender
	_ = req
	return s.windowList(), nil
}

func (s *Server) SetWindowState(sender uint32, req displayapi.SetWindowStateRequest) (displayapi.Empty, error) {
	_ = sender
	s.handleSetWindowState(req.WindowID, req.Action)
	return displayapi.Empty{}, nil
}

func (s *Server) RegisterSession(sender uint32, req displayapi.RegisterSessionRequest) (displayapi.Empty, error) {
	if err := s.requireSessionAdmin(sender); err != nil {
		return displayapi.Empty{}, err
	}
	if req.SessionID != req.UID {
		return displayapi.Empty{}, fmt.Errorf("invalid session registration: session id %d must match uid %d", req.SessionID, req.UID)
	}
	if _, err := identity.LookupByID(uint(req.UID)); err != nil {
		return displayapi.Empty{}, fmt.Errorf("unknown identity for uid %d", req.UID)
	}

	s.mu.Lock()
	if existing, exists := s.userSessions[req.SessionID]; exists {
		if existing.uid != req.UID {
			s.mu.Unlock()
			return displayapi.Empty{}, fmt.Errorf("session %d already registered for uid %d", req.SessionID, existing.uid)
		}
		s.mu.Unlock()
		return displayapi.Empty{}, nil
	}

	clients := make(map[uint32]bool)
	for cid, uid := range s.clientToSession {
		if uid == req.UID {
			clients[cid] = true
		}
	}
	s.userSessions[req.SessionID] = &userSession{
		id:      req.SessionID,
		uid:     req.UID,
		clients: clients,
	}
	s.mu.Unlock()
	log.Printf("Registered user session %d (uid=%d)", req.SessionID, req.UID)
	return displayapi.Empty{}, nil
}

func (s *Server) SetActiveSession(sender uint32, req displayapi.SetActiveSessionRequest) (displayapi.Empty, error) {
	if err := s.requireSessionAdmin(sender); err != nil {
		return displayapi.Empty{}, err
	}

	s.mu.Lock()
	oldID := s.activeSessionID
	newID := req.SessionID
	if newID != 0 {
		if _, exists := s.userSessions[newID]; !exists {
			s.mu.Unlock()
			return displayapi.Empty{}, fmt.Errorf("unknown session %d", newID)
		}
	}

	if oldID == newID {
		s.mu.Unlock()
		return displayapi.Empty{}, nil
	}

	// Save focused window for old session
	if oldSess, ok := s.userSessions[oldID]; ok {
		oldSess.focused = s.focused
		oldSess.active = false
	}

	s.activeSessionID = newID

	// Restore focused window for new session
	if newSess, ok := s.userSessions[newID]; ok {
		newSess.active = true
		s.focused = newSess.focused
	} else {
		s.focused = nil
	}

	// Collect client IDs for suspended/resumed notifications
	var suspendClients, resumeClients []uint32
	if oldSess, ok := s.userSessions[oldID]; ok {
		for cid := range oldSess.clients {
			suspendClients = append(suspendClients, cid)
		}
	}
	if newSess, ok := s.userSessions[newID]; ok {
		for cid := range newSess.clients {
			resumeClients = append(resumeClients, cid)
		}
	}
	s.mu.Unlock()

	// Send suspend/resume events outside lock
	ev := displayapi.ActiveSessionEvent{SessionID: newID}
	payload := ev.MarshalBinary()
	for _, cid := range suspendClients {
		_ = s.service.Send(cid, displayapi.EventSessionSuspended, payload)
	}
	for _, cid := range resumeClients {
		_ = s.service.Send(cid, displayapi.EventSessionResumed, payload)
	}
	_ = s.service.Broadcast(displayapi.EventSessionActivated, payload)

	s.markAllDirty()
	log.Printf("Switched active session %d -> %d", oldID, newID)
	return displayapi.Empty{}, nil
}

func (s *Server) GetActiveSession(sender uint32, req displayapi.Empty) (displayapi.ActiveSessionEvent, error) {
	_ = sender
	_ = req
	s.mu.Lock()
	id := s.activeSessionID
	s.mu.Unlock()
	return displayapi.ActiveSessionEvent{SessionID: id}, nil
}

func normalizeShortcutModifiers(mod graphics.Modifiers) graphics.Modifiers {
	return mod & (graphics.ModShift | graphics.ModCtrl | graphics.ModAlt)
}

func (s *Server) RegisterShortcut(sender uint32, req displayapi.RegisterShortcutRequest) (displayapi.Empty, error) {
	sess := s.ensureSession(sender)
	if req.ShortcutID == 0 {
		return displayapi.Empty{}, fmt.Errorf("invalid shortcut id")
	}
	if req.Key == graphics.KeyNone {
		return displayapi.Empty{}, fmt.Errorf("invalid shortcut key")
	}
	if req.Scope != displayapi.ShortcutScopeGlobal && req.Scope != displayapi.ShortcutScopeClient {
		return displayapi.Empty{}, fmt.Errorf("invalid shortcut scope %d", req.Scope)
	}
	if req.Scope == displayapi.ShortcutScopeGlobal {
		if err := s.requireSessionAdmin(sender); err != nil {
			return displayapi.Empty{}, err
		}
	}

	if req.Scope == displayapi.ShortcutScopeClient && req.WindowID != 0 {
		s.mu.Lock()
		_, exists := sess.windows[req.WindowID]
		s.mu.Unlock()
		if !exists {
			return displayapi.Empty{}, fmt.Errorf("window %d not found in client session", req.WindowID)
		}
	}

	mods := normalizeShortcutModifiers(graphics.Modifiers(req.Modifiers))

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.shortcuts[sender]; !ok {
		s.shortcuts[sender] = make(map[uint32]shortcutBinding)
	}

	for owner, bindings := range s.shortcuts {
		for sid, existing := range bindings {
			if owner == sender && sid == req.ShortcutID {
				continue
			}
			if existing.Scope != req.Scope || existing.Key != req.Key || normalizeShortcutModifiers(existing.Modifiers) != mods {
				continue
			}
			if req.Scope == displayapi.ShortcutScopeGlobal {
				return displayapi.Empty{}, fmt.Errorf("global shortcut already registered")
			}
			if owner == sender && existing.WindowID == req.WindowID {
				return displayapi.Empty{}, fmt.Errorf("client shortcut already registered")
			}
		}
	}

	s.shortcuts[sender][req.ShortcutID] = shortcutBinding{
		ShortcutID: req.ShortcutID,
		WindowID:   req.WindowID,
		Scope:      req.Scope,
		Key:        req.Key,
		Modifiers:  mods,
	}
	return displayapi.Empty{}, nil
}

func (s *Server) UnregisterShortcut(sender uint32, req displayapi.UnregisterShortcutRequest) (displayapi.Empty, error) {
	s.mu.Lock()
	if bindings, ok := s.shortcuts[sender]; ok {
		delete(bindings, req.ShortcutID)
		if len(bindings) == 0 {
			delete(s.shortcuts, sender)
		}
	}
	s.mu.Unlock()
	return displayapi.Empty{}, nil
}

func (s *Server) resolveKeyTargetLocked() *window {
	for i := len(s.layers[LayerOverlay]) - 1; i >= 0; i-- {
		target := s.layers[LayerOverlay][i]
		if s.isWindowVisible(target) {
			return target
		}
	}
	if s.focused == nil {
		return nil
	}
	if !s.isWindowVisible(s.focused) || s.focused.minimized {
		return nil
	}
	return s.focused
}

func (s *Server) matchShortcutLocked(target *window, ev *graphics.Event) (uint32, displayapi.ShortcutEvent, bool) {
	mods := normalizeShortcutModifiers(ev.Modifiers)

	// Client-scoped shortcuts for the key target have priority.
	if target != nil && target.sess != nil {
		owner := target.sess.id
		if bindings, ok := s.shortcuts[owner]; ok {
			var anyWindow shortcutBinding
			hasAnyWindow := false
			for _, binding := range bindings {
				if binding.Scope != displayapi.ShortcutScopeClient {
					continue
				}
				if binding.Key != ev.Key || normalizeShortcutModifiers(binding.Modifiers) != mods {
					continue
				}
				if binding.WindowID == target.id {
					return owner, displayapi.ShortcutEvent{
						ShortcutID: binding.ShortcutID,
						WindowID:   target.id,
						Scope:      binding.Scope,
						Key:        ev.Key,
						Modifiers:  uint8(mods),
					}, true
				}
				if binding.WindowID == 0 && !hasAnyWindow {
					anyWindow = binding
					hasAnyWindow = true
				}
			}
			if hasAnyWindow {
				return owner, displayapi.ShortcutEvent{
					ShortcutID: anyWindow.ShortcutID,
					WindowID:   target.id,
					Scope:      anyWindow.Scope,
					Key:        ev.Key,
					Modifiers:  uint8(mods),
				}, true
			}
		}
	}

	// Fallback to global shortcuts.
	windowID := uint32(0)
	if target != nil {
		windowID = target.id
	}
	for owner, bindings := range s.shortcuts {
		for _, binding := range bindings {
			if binding.Scope != displayapi.ShortcutScopeGlobal {
				continue
			}
			if binding.Key != ev.Key || normalizeShortcutModifiers(binding.Modifiers) != mods {
				continue
			}
			return owner, displayapi.ShortcutEvent{
				ShortcutID: binding.ShortcutID,
				WindowID:   windowID,
				Scope:      binding.Scope,
				Key:        ev.Key,
				Modifiers:  uint8(mods),
			}, true
		}
	}

	return 0, displayapi.ShortcutEvent{}, false
}

// isWindowVisible returns true if the window belongs to the active session or system session (UID 0).
// Must be called with s.mu held.
func (s *Server) isWindowVisible(win *window) bool {
	if win.sess == nil {
		return true
	}
	uid, ok := s.clientToSession[win.sess.id]
	if !ok {
		return true // unmapped client, show by default
	}
	return uid == 0 || uid == s.activeSessionID
}

// filterVisible returns a snapshot of windows that belong to the active session or system session.
// Must be called with s.mu held.
func (s *Server) filterVisible(src []*window) []*window {
	out := make([]*window, 0, len(src))
	for _, w := range src {
		if s.isWindowVisible(w) {
			out = append(out, w)
		}
	}
	return out
}

// --- Window operations ---

func openShmRO(path string, size int) (int, []byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
	if err != nil {
		return -1, nil, err
	}
	data, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		_ = syscall.Close(fd)
		return -1, nil, err
	}
	return fd, data, nil
}

func (s *Server) mmapAndCopy(shmKey string, w, h, stride int) (int, []byte, *graphics.Buffer, error) {
	size := stride * h
	fd, data, err := openShmRO(shmKey, size)
	if err != nil {
		return -1, nil, nil, err
	}
	buf := graphics.NewBuffer(w, h)
	for y := 0; y < h; y++ {
		srcOff := y * stride
		dstOff := y * buf.Stride
		copy(buf.Data[dstOff:dstOff+w*4], data[srcOff:srcOff+w*4])
	}
	return fd, data, buf, nil
}

func (s *Server) handleCreateWindow(sess *session, w, h, stride int, shmKey string) (displayapi.WindowCreatedEvent, error) {
	fd, data, buf, err := s.mmapAndCopy(shmKey, w, h, stride)
	if err != nil {
		return displayapi.WindowCreatedEvent{}, fmt.Errorf("mmap: %w", err)
	}

	s.mu.Lock()
	wid := s.nextWID
	s.nextWID++

	win := &window{
		id:        wid,
		width:     w,
		height:    h,
		stride:    stride,
		shmFD:     fd,
		shmData:   data,
		shmKey:    shmKey,
		shmStride: stride,
		shmWidth:  w,
		shmHeight: h,
		buffer:    buf,
		winType:   WindowNormal,
		sess:      sess,
	}

	// Position with cascade offset within usable area
	offset := len(s.windows) * 30
	win.x = s.usableArea.X + 50 + offset
	win.y = s.usableArea.Y + 50 + offset
	if win.x+w > s.usableArea.X+s.usableArea.W {
		win.x = s.usableArea.X + 50
	}
	if win.y+h+decorHeight > s.usableArea.Y+s.usableArea.H {
		win.y = s.usableArea.Y + 50
	}

	sess.windows[wid] = win
	s.windows = append(s.windows, win)

	// Focus new window
	if s.focused != nil {
		s.focused.focused = false
		s.sendFocus(s.focused, false)
	}
	s.focused = win
	win.focused = true
	s.mu.Unlock()

	s.markDirty(windowScreenRect(win))

	out := displayapi.WindowCreatedEvent{WindowID: wid, X: win.x, Y: win.y}
	s.sendFocus(win, true)
	return out, nil
}

func windowListFlags(win *window) uint32 {
	var flags uint32
	if win.focused {
		flags |= WindowFlagFocused
	}
	if win.minimized {
		flags |= WindowFlagMinimized
	}
	if win.maximized {
		flags |= WindowFlagMaximized
	}
	if !win.minimized {
		flags |= WindowFlagVisible
	}
	return flags
}

func (s *Server) windowList() displayapi.WindowListEvent {
	type snap struct {
		id    uint32
		flags uint32
		title string
	}
	s.mu.Lock()
	snaps := make([]snap, 0, len(s.windows))
	for _, win := range s.windows {
		if !s.isWindowVisible(win) {
			continue
		}
		title := win.title
		if title == "" {
			title = "Untitled"
		}
		snaps = append(snaps, snap{
			id:    win.id,
			flags: windowListFlags(win),
			title: title,
		})
	}
	s.mu.Unlock()

	out := make([]displayapi.WindowListItem, 0, len(snaps))
	for _, sn := range snaps {
		out = append(out, displayapi.WindowListItem{
			ID:    sn.id,
			Flags: sn.flags,
			Title: sn.title,
		})
	}
	return displayapi.WindowListEvent{Windows: out}
}

func (s *Server) findNormalWindowLocked(wid uint32) *window {
	for _, win := range s.windows {
		if win.id == wid {
			return win
		}
	}
	return nil
}

func (s *Server) sendConfigure(win *window) {
	_ = EmitConfigure(s.service, win.sess.id, displayapi.ConfigureEvent{
		WindowID: win.id,
		Width:    win.width,
		Height:   win.height,
	})
}

func (s *Server) restoreWindowLocked(win *window) {
	oldRect := windowScreenRect(win)
	if win.minimized {
		win.minimized = false
	}
	if win.maximized {
		win.maximized = false
		if win.restoreW > 0 && win.restoreH > 0 {
			win.x = win.restoreX
			win.y = win.restoreY
			win.width = win.restoreW
			win.height = win.restoreH
			s.sendMoved(win)
			s.sendConfigure(win)
		}
	}
	s.markDirty(oldRect)
	s.markDirty(windowScreenRect(win))
}

func (s *Server) maximizeWindowLocked(win *window) {
	if win.maximized {
		s.restoreWindowLocked(win)
		return
	}

	oldRect := windowScreenRect(win)
	win.restoreX = win.x
	win.restoreY = win.y
	win.restoreW = win.width
	win.restoreH = win.height

	win.minimized = false
	win.maximized = true
	win.x = s.usableArea.X
	win.y = s.usableArea.Y
	win.width = s.usableArea.W
	win.height = s.usableArea.H - decorHeight
	if win.height < minWindowHeight {
		win.height = minWindowHeight
	}

	s.sendMoved(win)
	s.sendConfigure(win)
	s.markDirty(oldRect)
	s.markDirty(windowScreenRect(win))
}

func (s *Server) minimizeWindowLocked(win *window) bool {
	if win.minimized {
		return false
	}
	oldRect := windowScreenRect(win)
	win.minimized = true
	lostFocus := false
	if s.focused == win {
		s.focused = nil
		lostFocus = true
	}
	win.focused = false
	if s.hovered == win {
		s.hovered = nil
	}
	s.markDirty(oldRect)
	return lostFocus
}

func (s *Server) handleSetWindowState(wid, action uint32) {
	shouldFocus := false
	lostFocus := false
	s.mu.Lock()
	win := s.findNormalWindowLocked(wid)
	if win == nil {
		s.mu.Unlock()
		return
	}

	switch action {
	case WindowActionToggleMinimize:
		if win.minimized {
			s.restoreWindowLocked(win)
			shouldFocus = true
		} else {
			lostFocus = s.minimizeWindowLocked(win)
		}
	case WindowActionMinimize:
		lostFocus = s.minimizeWindowLocked(win)
	case WindowActionRestore:
		s.restoreWindowLocked(win)
	case WindowActionToggleMaximize:
		s.maximizeWindowLocked(win)
		shouldFocus = true
	case WindowActionFocus:
		if win.minimized {
			s.restoreWindowLocked(win)
		}
		shouldFocus = true
	case WindowActionClose:
		s.sendClose(win)
	}
	s.mu.Unlock()

	if lostFocus {
		s.sendFocus(win, false)
	}
	if shouldFocus {
		s.focusWindow(win)
		s.raiseWindow(win)
	}
}

func (s *Server) handleCreateLayer(sess *session, layer, anchor uint32, exclusive int, opaqueHint bool, w, h, stride int, shmKey string) (displayapi.WindowCreatedEvent, error) {
	if layer > LayerOverlay {
		return displayapi.WindowCreatedEvent{}, fmt.Errorf("invalid layer %d", layer)
	}

	// Remember original client dimensions for mmap.
	clientW, clientH, clientStride := w, h, stride

	// Compute actual size based on anchoring.
	screenW, screenH := s.fb.Size()
	if anchor&AnchorLeft != 0 && anchor&AnchorRight != 0 {
		w = screenW
	}
	if anchor&AnchorTop != 0 && anchor&AnchorBottom != 0 {
		h = screenH
	}

	// Mmap the client's shm at its original size (may be smaller than final).
	shmSize := clientStride * clientH
	if shmSize <= 0 {
		shmSize = 4 // minimum for mmap
	}
	fd, data, err := openShmRO(shmKey, shmSize)
	if err != nil {
		return displayapi.WindowCreatedEvent{}, fmt.Errorf("mmap layer: %w", err)
	}

	// Create compositor buffer at the stretched size.
	buf := graphics.NewBuffer(w, h)
	// Copy only the overlapping region from the client buffer.
	copyW := clientW
	if copyW > w {
		copyW = w
	}
	copyH := clientH
	if copyH > h {
		copyH = h
	}
	for y := 0; y < copyH; y++ {
		srcOff := y * clientStride
		dstOff := y * buf.Stride
		n := copyW * 4
		if srcOff+n <= len(data) {
			copy(buf.Data[dstOff:dstOff+n], data[srcOff:srcOff+n])
		}
	}

	s.mu.Lock()
	wid := s.nextWID
	s.nextWID++

	win := &window{
		id:         wid,
		width:      w,
		height:     h,
		stride:     w * 4,
		shmFD:      fd,
		shmData:    data,
		shmKey:     shmKey,
		shmStride:  clientStride,
		shmWidth:   clientW,
		shmHeight:  clientH,
		buffer:     buf,
		winType:    WindowLayer,
		layer:      layer,
		anchor:     anchor,
		exclusive:  exclusive,
		opaqueHint: opaqueHint,
		sess:       sess,
	}

	// Compute position from anchor.
	s.computeLayerPosition(win, screenW, screenH)

	sess.windows[wid] = win
	s.layers[layer] = append(s.layers[layer], win)

	// Recompute usable area for normal window placement.
	s.recomputeUsableArea()
	s.mu.Unlock()

	s.markDirty(windowScreenRect(win))

	out := displayapi.WindowCreatedEvent{WindowID: wid, X: win.x, Y: win.y}

	// If the server stretched the surface, tell the client the actual size.
	if w != clientW || h != clientH {
		_ = EmitConfigure(s.service, sess.id, displayapi.ConfigureEvent{WindowID: wid, Width: w, Height: h})
	}
	return out, nil
}

func (s *Server) handleCreatePopup(sess *session, parentID uint32, px, py, w, h, stride int, shmKey string) (displayapi.WindowCreatedEvent, error) {
	s.mu.Lock()
	parent, ok := sess.windows[parentID]
	s.mu.Unlock()
	if !ok {
		return displayapi.WindowCreatedEvent{}, fmt.Errorf("parent window %d not found", parentID)
	}

	fd, data, buf, err := s.mmapAndCopy(shmKey, w, h, stride)
	if err != nil {
		return displayapi.WindowCreatedEvent{}, fmt.Errorf("mmap popup: %w", err)
	}

	s.mu.Lock()
	wid := s.nextWID
	s.nextWID++

	win := &window{
		id:        wid,
		width:     w,
		height:    h,
		stride:    stride,
		shmFD:     fd,
		shmData:   data,
		shmKey:    shmKey,
		shmStride: stride,
		shmWidth:  w,
		shmHeight: h,
		buffer:    buf,
		winType:   WindowPopup,
		parent:    parent,
		popupX:    px,
		popupY:    py,
		sess:      sess,
	}

	// Compute screen position from parent
	sx, sy := win.screenPos()
	win.x = sx
	win.y = sy

	sess.windows[wid] = win
	s.popups = append(s.popups, win)
	s.mu.Unlock()

	s.markDirty(windowScreenRect(win))

	return displayapi.WindowCreatedEvent{WindowID: wid, X: win.x, Y: win.y}, nil
}

func (s *Server) computeLayerPosition(win *window, screenW, screenH int) {
	anchor := win.anchor

	// Horizontal position
	if anchor&AnchorLeft != 0 && anchor&AnchorRight != 0 {
		win.x = 0
	} else if anchor&AnchorHorizontalCenter != 0 {
		win.x = (screenW - win.width) / 2
	} else if anchor&AnchorLeft != 0 {
		win.x = 0
	} else if anchor&AnchorRight != 0 {
		win.x = screenW - win.width
	} else {
		win.x = (screenW - win.width) / 2 // centered
	}

	// Vertical position
	if anchor&AnchorTop != 0 && anchor&AnchorBottom != 0 {
		win.y = 0
	} else if anchor&AnchorVerticalCenter != 0 {
		win.y = (screenH - win.height) / 2
	} else if anchor&AnchorTop != 0 {
		win.y = 0
	} else if anchor&AnchorBottom != 0 {
		win.y = screenH - win.height
	} else {
		win.y = (screenH - win.height) / 2 // centered
	}

	// Keep layers on-screen even if requested size is larger than display.
	if win.width >= screenW {
		win.x = 0
	} else {
		if win.x < 0 {
			win.x = 0
		}
		if win.x+win.width > screenW {
			win.x = screenW - win.width
		}
	}
	if win.height >= screenH {
		win.y = 0
	} else {
		if win.y < 0 {
			win.y = 0
		}
		if win.y+win.height > screenH {
			win.y = screenH - win.height
		}
	}
}

func (s *Server) recomputeUsableArea() {
	screenW, screenH := s.fb.Size()
	area := graphics.Rect{X: 0, Y: 0, W: screenW, H: screenH}

	// Only Top and Bottom layers affect usable area with exclusive zones
	for _, layerList := range []uint32{LayerTop, LayerBottom} {
		for _, win := range s.layers[layerList] {
			if win.exclusive <= 0 {
				continue
			}
			if win.anchor&AnchorTop != 0 && win.anchor&AnchorBottom == 0 {
				// Top edge exclusive
				d := win.exclusive
				if d > area.H {
					d = area.H
				}
				area.Y += d
				area.H -= d
			} else if win.anchor&AnchorBottom != 0 && win.anchor&AnchorTop == 0 {
				// Bottom edge exclusive
				d := win.exclusive
				if d > area.H {
					d = area.H
				}
				area.H -= d
			} else if win.anchor&AnchorLeft != 0 && win.anchor&AnchorRight == 0 {
				// Left edge exclusive
				d := win.exclusive
				if d > area.W {
					d = area.W
				}
				area.X += d
				area.W -= d
			} else if win.anchor&AnchorRight != 0 && win.anchor&AnchorLeft == 0 {
				// Right edge exclusive
				d := win.exclusive
				if d > area.W {
					d = area.W
				}
				area.W -= d
			}
		}
	}

	s.usableArea = area
}

func (s *Server) handleDestroyWindow(sess *session, wid uint32) {
	s.mu.Lock()
	win, ok := sess.windows[wid]
	if !ok {
		s.mu.Unlock()
		return
	}
	s.markDirty(windowScreenRect(win))
	delete(sess.windows, wid)
	s.removeWindowLocked(win)
	needRecompute := win.winType == WindowLayer && win.exclusive > 0
	if needRecompute {
		s.recomputeUsableArea()
	}
	s.mu.Unlock()
	s.cleanupWindow(win)
}

func (s *Server) handleDamage(sess *session, wid uint32, x, y, w, h int) {
	s.mu.Lock()
	win, ok := sess.windows[wid]
	s.mu.Unlock()
	if !ok {
		return
	}

	// Copy damaged region from shm into the compositor buffer in place.
	// All shm field reads must be under bufMu since handleResize modifies them.
	win.bufMu.Lock()
	shmData := win.shmData
	shmStride := win.shmStride
	shmW := win.shmWidth
	shmH := win.shmHeight

	if shmData == nil {
		win.bufMu.Unlock()
		return
	}

	// Clamp to shm bounds.
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > shmW {
		w = shmW - x
	}
	if y+h > shmH {
		h = shmH - y
	}
	if w <= 0 || h <= 0 {
		win.bufMu.Unlock()
		return
	}

	buf := win.buffer
	if buf == nil || buf.Width != win.width || buf.Height != win.height {
		buf = graphics.NewBuffer(win.width, win.height)
		win.buffer = buf
	}
	for dy := y; dy < y+h; dy++ {
		srcOff := dy*shmStride + x*4
		dstOff := dy*buf.Stride + x*4
		n := w * 4
		if srcOff+n <= len(shmData) && dstOff+n <= len(buf.Data) {
			copy(buf.Data[dstOff:dstOff+n], shmData[srcOff:srcOff+n])
		}
	}
	win.bufMu.Unlock()

	// Mark the damaged screen region dirty.
	// Offset damage into screen coordinates (client area origin).
	var ox, oy int
	switch win.winType {
	case WindowNormal:
		ox, oy = win.x, win.y+decorHeight
	case WindowPopup:
		ox, oy = win.screenPos()
	default:
		ox, oy = win.x, win.y
	}
	s.markDirty(graphics.Rect{X: ox + x, Y: oy + y, W: w, H: h})
}

func (s *Server) handleResize(sess *session, wid uint32, w, h, stride int, shmKey string) error {
	s.mu.Lock()
	win, ok := sess.windows[wid]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("window %d not found", wid)
	}

	s.markDirty(windowScreenRect(win))

	size := stride * h
	fd, data, err := openShmRO(shmKey, size)
	if err != nil {
		return fmt.Errorf("mmap resize: %w", err)
	}

	// Swap shm+buffer atomically under the window lock so damage/composite
	// cannot race with unmap/remap during rapid resize.
	win.bufMu.Lock()
	oldFD := win.shmFD
	oldData := win.shmData
	oldKey := win.shmKey

	win.shmFD = fd
	win.shmData = data
	win.shmKey = shmKey
	win.shmStride = stride
	win.shmWidth = w
	win.shmHeight = h
	win.width = w
	win.height = h
	win.stride = stride

	// Full buffer copy
	buf := graphics.NewBuffer(w, h)
	for y := 0; y < h; y++ {
		srcOff := y * stride
		dstOff := y * buf.Stride
		copy(buf.Data[dstOff:dstOff+w*4], data[srcOff:srcOff+w*4])
	}
	win.buffer = buf
	win.bufMu.Unlock()

	if oldData != nil {
		_ = syscall.Munmap(oldData)
	}
	if oldFD >= 0 {
		_ = syscall.Close(oldFD)
	}
	if oldKey != "" && oldKey != shmKey {
		_ = os.Remove(oldKey)
	}

	// Layer surfaces may need position recomputation after size changes
	// (for centered/bottom-right anchored surfaces).
	if win.winType == WindowLayer {
		screenW, screenH := s.fb.Size()
		oldX, oldY := win.x, win.y
		s.computeLayerPosition(win, screenW, screenH)
		if win.x != oldX || win.y != oldY {
			s.sendMoved(win)
		}
		if win.exclusive > 0 {
			s.recomputeUsableArea()
		}
	}

	s.markDirty(windowScreenRect(win))
	return nil
}

// --- Compositing ---

func (s *Server) composite() {
	buf := s.fb.Buffer()
	if buf == nil {
		return
	}

	// FPS counter updates once per second.
	now := time.Now().UnixMilli()
	if now-s.lastFPSMs >= 1000 {
		s.fps = s.frameCount
		s.frameCount = 0
		s.lastFPSMs = now
		s.refreshDiagnostics()
	}

	sw, sh := s.fb.Size()
	damageRects := s.consumeDirtyRects(graphics.Rect{W: sw, H: sh}, 24)
	if len(damageRects) == 0 {
		return
	}

	s.mu.Lock()
	// Snapshot all lists, filtering by active session
	bgLayers := s.filterVisible(s.layers[LayerBackground])
	btmLayers := s.filterVisible(s.layers[LayerBottom])
	wins := s.filterVisible(s.windows)
	pops := s.filterVisible(s.popups)
	topLayers := s.filterVisible(s.layers[LayerTop])
	ovrLayers := s.filterVisible(s.layers[LayerOverlay])
	s.mu.Unlock()

	cursorRect := graphics.Rect{X: s.mouseX, Y: s.mouseY, W: 16, H: 20}
	totalDamage := uint64(0)

	for _, flushRect := range damageRects {
		if flushRect.W <= 0 || flushRect.H <= 0 {
			continue
		}
		totalDamage += uint64(flushRect.W) * uint64(flushRect.H)

		buf.SetClip(flushRect)

		// Re-composite only the clipped dirty region.
		buf.FillRect(flushRect, graphics.DefaultTheme.Background)

		for _, win := range bgLayers {
			if windowScreenRect(win).Intersects(flushRect) {
				s.drawSurface(buf, win, flushRect)
			}
		}
		for _, win := range btmLayers {
			if windowScreenRect(win).Intersects(flushRect) {
				s.drawSurface(buf, win, flushRect)
			}
		}
		for _, win := range wins {
			if win.minimized {
				continue
			}
			if windowScreenRect(win).Intersects(flushRect) {
				s.drawWindow(buf, win, flushRect)
			}
		}
		for _, win := range pops {
			if windowScreenRect(win).Intersects(flushRect) {
				s.drawPopup(buf, win, flushRect)
			}
		}
		for _, win := range topLayers {
			if windowScreenRect(win).Intersects(flushRect) {
				s.drawSurface(buf, win, flushRect)
			}
		}
		for _, win := range ovrLayers {
			if windowScreenRect(win).Intersects(flushRect) {
				s.drawSurface(buf, win, flushRect)
			}
		}

		if cursorRect.Intersects(flushRect) {
			s.drawCursor(buf)
		}
		if s.drawDebug && s.diagRect.Intersects(flushRect) {
			s.drawDiagnostics(buf, flushRect)
		}
		buf.ClearClip()
	}
	flushBackendRects(s.fb, damageRects)

	if totalDamage == 0 {
		return
	}

	s.frameCount++
	s.prevCursorRect = cursorRect
	s.lastDamagePx = totalDamage
	s.damageTick += totalDamage
	s.damageAccum += totalDamage
	s.redrawTick++
	s.redrawAccum++
}

func flushBackendRects(fb graphics.Backend, rects []graphics.Rect) {
	if len(rects) == 0 {
		return
	}
	if bf, ok := fb.(rectBatchFlusher); ok {
		if err := bf.FlushRects(rects); err == nil {
			return
		}
	}
	for _, r := range rects {
		_ = fb.FlushRect(r)
	}
}

func normalizeDamageRectsDisplay(rects []graphics.Rect, bounds graphics.Rect, maxRects int) []graphics.Rect {
	if len(rects) == 0 {
		return nil
	}
	if maxRects < 1 {
		maxRects = 1
	}
	out := make([]graphics.Rect, 0, len(rects))
	for _, r := range rects {
		r = r.Intersection(bounds)
		if r.IsEmpty() {
			continue
		}
		merged := false
		for i := 0; i < len(out); i++ {
			if rectsTouchOrOverlapDisplay(out[i], r) {
				out[i] = out[i].Union(r)
				merged = true
				// Re-merge if this union now overlaps others.
				for j := 0; j < len(out); {
					if j == i {
						j++
						continue
					}
					if rectsTouchOrOverlapDisplay(out[i], out[j]) {
						out[i] = out[i].Union(out[j])
						out = append(out[:j], out[j+1:]...)
						if j < i {
							i--
						}
						continue
					}
					j++
				}
				break
			}
		}
		if !merged {
			out = append(out, r)
		}
		if len(out) > maxRects {
			mergedRect := out[0]
			for _, d := range out[1:] {
				mergedRect = mergedRect.Union(d)
			}
			out = out[:0]
			out = append(out, mergedRect)
		}
	}
	return out
}

func rectsTouchOrOverlapDisplay(a, b graphics.Rect) bool {
	// Expand A by 1px so edge-touching damage rects coalesce.
	ax0 := a.X - 1
	ay0 := a.Y - 1
	ax1 := a.X + a.W + 1
	ay1 := a.Y + a.H + 1
	bx0 := b.X
	by0 := b.Y
	bx1 := b.X + b.W
	by1 := b.Y + b.H
	return ax0 < bx1 && ax1 > bx0 && ay0 < by1 && ay1 > by0
}

func (s *Server) refreshDiagnostics() {
	sw, sh := s.fb.Size()
	if sw <= 0 || sh <= 0 {
		return
	}

	s.damagePerSec = s.damageTick
	s.damageTick = 0
	s.redrawPerSec = s.redrawTick
	s.redrawTick = 0

	if cpuSec, err := readProcessCPUSeconds(); err == nil {
		now := time.Now()
		if s.lastCPUSec > 0 && !s.lastCPUAt.IsZero() && cpuSec >= s.lastCPUSec {
			elapsed := now.Sub(s.lastCPUAt).Seconds()
			if elapsed > 0 {
				deltaCPU := cpuSec - s.lastCPUSec
				s.cpuPct = (deltaCPU / elapsed) * 100.0
			}
		}
		s.lastCPUSec = cpuSec
		s.lastCPUAt = now
	}

	if rss, err := readProcessRSSBytes(); err == nil {
		s.rssBytes = rss
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.heapBytes = ms.HeapAlloc

	oldRect := s.diagRect
	s.diagLines = s.buildDiagnosticsLines(sw, sh)
	s.diagRect = diagnosticsRectFor(sw, s.diagLines, s.diagnosticsFont())
	if oldRect.W > 0 && oldRect.H > 0 {
		s.markDirty(oldRect)
	}
	s.markDirty(s.diagRect)
}

func (s *Server) buildDiagnosticsLines(sw, sh int) []string {
	screenPx := uint64(sw) * uint64(sh)
	damagePct := 0.0
	if screenPx > 0 {
		damagePct = (float64(s.lastDamagePx) / float64(screenPx)) * 100.0
	}
	return []string{
		fmt.Sprintf("FPS %d | CPU(proc) %.2f%% | RSS %s | HEAP %s",
			s.fps, s.cpuPct, formatBytesMiB(s.rssBytes), formatBytesMiB(s.heapBytes)),
		fmt.Sprintf("Damage last %s (%.1f%%) | per sec %s",
			formatPixelCount(s.lastDamagePx), damagePct, formatPixelCount(s.damagePerSec)),
		fmt.Sprintf("Redraws %d/s | Total damage %s",
			s.redrawPerSec, formatPixelCount(s.damageAccum)),
	}
}

func (s *Server) drawDiagnostics(buf *graphics.Buffer, clip graphics.Rect) {
	if len(s.diagLines) == 0 {
		return
	}
	rect := s.diagRect
	if rect.W <= 0 || rect.H <= 0 {
		sw, _ := s.fb.Size()
		rect = diagnosticsRectFor(sw, s.diagLines, s.diagnosticsFont())
	}
	blendFillRoundedRect(buf, rect, 8, graphics.NewColor(9, 14, 25, 172), clip)
	buf.DrawRoundedRect(rect, 8, graphics.NewColor(234, 240, 255, 44))

	font := s.diagnosticsFont()
	if font == nil {
		return
	}
	const pad = 6
	const lineGap = 2
	y := rect.Y + pad
	for _, line := range s.diagLines {
		font.DrawText(buf, line, rect.X+pad, y, graphics.NewColor(234, 240, 255, 240), graphics.Color{})
		y += font.Height + lineGap
	}
}

func (s *Server) diagnosticsFont() *graphics.Font {
	font := graphics.UIFont(graphics.UIFontParagraph)
	if font != nil {
		return font
	}
	return graphics.DefaultFont
}

func diagnosticsRectFor(screenW int, lines []string, font *graphics.Font) graphics.Rect {
	if screenW <= 0 || len(lines) == 0 || font == nil {
		return graphics.Rect{}
	}
	const margin = 6
	const pad = 6
	const lineGap = 2
	maxW := 0
	for _, line := range lines {
		w := font.TextWidth(line)
		if w > maxW {
			maxW = w
		}
	}
	panelW := maxW + pad*2
	panelH := pad*2 + len(lines)*(font.Height+lineGap) - lineGap
	x := screenW - panelW - margin
	if x < margin {
		x = margin
	}
	return graphics.Rect{X: x, Y: margin, W: panelW, H: panelH}
}

func formatBytesMiB(v uint64) string {
	return fmt.Sprintf("%.1fMB", float64(v)/(1024.0*1024.0))
}

func formatPixelCount(v uint64) string {
	const (
		k = 1_000
		m = 1_000_000
		g = 1_000_000_000
	)
	switch {
	case v >= g:
		return fmt.Sprintf("%.2fGpx", float64(v)/g)
	case v >= m:
		return fmt.Sprintf("%.2fMpx", float64(v)/m)
	case v >= k:
		return fmt.Sprintf("%.2fKpx", float64(v)/k)
	default:
		return fmt.Sprintf("%dpx", v)
	}
}

func readProcessCPUSeconds() (float64, error) {
	// Prefer getrusage for microsecond resolution and portability across procfs variants.
	if sec, err := readProcessCPUSecondsRusage(); err == nil && sec >= 0 {
		return sec, nil
	}
	jiffies, err := readProcessCPUJiffies()
	if err != nil {
		return 0, err
	}
	return float64(jiffies) / procClockTicks, nil
}

func readProcessCPUSecondsRusage() (float64, error) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, err
	}
	return timevalToSeconds(ru.Utime) + timevalToSeconds(ru.Stime), nil
}

func readProcessCPUJiffies() (uint64, error) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, err
	}
	raw := strings.TrimSpace(string(data))
	end := strings.LastIndex(raw, ")")
	if end < 0 || end+2 >= len(raw) {
		return 0, fmt.Errorf("unexpected /proc/self/stat format")
	}
	fields := strings.Fields(raw[end+2:])
	if len(fields) < 13 {
		return 0, fmt.Errorf("unexpected /proc/self/stat field count: %d", len(fields))
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, err
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, err
	}
	return utime + stime, nil
}

func timevalToSeconds(tv syscall.Timeval) float64 {
	return float64(tv.Sec) + float64(tv.Usec)/1_000_000.0
}

func readProcessRSSBytes() (uint64, error) {
	// VmRSS from status is explicit and usually the most robust.
	if rss, err := readProcessRSSBytesStatus(); err == nil && rss > 0 {
		return rss, nil
	}
	// Fallback to resident pages from statm.
	if rss, err := readProcessRSSBytesStatm(); err == nil && rss > 0 {
		return rss, nil
	}
	// Last-resort fallback to runtime memory footprint so HUD is never pinned at 0.
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	if ms.Sys > 0 {
		return ms.Sys, nil
	}
	return 0, fmt.Errorf("failed to read process RSS")
}

func readProcessRSSBytesStatus() (uint64, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected VmRSS line: %q", line)
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return kib * 1024, nil
	}
	return 0, fmt.Errorf("VmRSS not found")
}

func readProcessRSSBytesStatm() (uint64, error) {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, fmt.Errorf("unexpected /proc/self/statm format")
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return pages * uint64(os.Getpagesize()), nil
}

func (s *Server) drawWindow(buf *graphics.Buffer, win *window, clip graphics.Rect) {
	frameRect := graphics.Rect{X: win.x, Y: win.y, W: win.width, H: win.height + decorHeight}
	clientRect := graphics.Rect{X: win.x, Y: win.y + decorHeight, W: win.width, H: win.height}

	// Soft shadow around the rounded window.
	drawShadowRounded(buf, frameRect, windowCornerRadius, windowShadowSpread, windowShadowOffsetX, windowShadowOffsetY, windowShadowTopClip, clip)

	// Minimal server-side titlebar with neutral controls.
	bg := graphics.NewColor(248, 249, 252, windowOpacity)
	blendFillRoundedRect(buf, frameRect, windowCornerRadius, bg, clip)

	title := win.title
	if title == "" {
		title = "Untitled"
	}
	titleFont := graphics.UIFont(graphics.UIFontHeading)
	if titleFont == nil {
		titleFont = graphics.DefaultFont
	}
	titleY := win.y + (decorHeight-titleFont.Height)/2
	titleRect := graphics.Rect{X: win.x + titleTextInset, Y: titleY, W: titleFont.TextWidth(title), H: titleFont.Height}
	if titleRect.Intersects(clip) {
		titleFont.DrawText(buf, title, titleRect.X, titleRect.Y,
			graphics.NewColorHex(0x2E3442), graphics.Color{})
	}

	minRect := minimizeButtonRect(win)
	maxRect := maximizeButtonRect(win)
	closeRect := closeButtonRect(win)
	glyph := graphics.NewColorHex(0x4A5161)

	// Minimize glyph: short horizontal bar.
	if minRect.Intersects(clip) {
		buf.DrawLine(minRect.X+4, minRect.Y+minRect.H-6, minRect.X+minRect.W-5, minRect.Y+minRect.H-6, glyph)
	}
	// Maximize glyph: simple square outline.
	maxGlyphRect := graphics.Rect{X: maxRect.X + 4, Y: maxRect.Y + 4, W: maxRect.W - 8, H: maxRect.H - 8}
	if maxGlyphRect.Intersects(clip) {
		buf.DrawRect(maxGlyphRect, glyph)
	}
	// Close glyph: cross.
	if closeRect.Intersects(clip) {
		buf.DrawLine(closeRect.X+5, closeRect.Y+5, closeRect.X+closeRect.W-6, closeRect.Y+closeRect.H-6, glyph)
		buf.DrawLine(closeRect.X+closeRect.W-6, closeRect.Y+5, closeRect.X+5, closeRect.Y+closeRect.H-6, glyph)
	}

	// Subtle separator between titlebar decoration and client content.
	sepY := win.y + decorHeight - 1
	sep := graphics.NewColor(27, 42, 74, 28)
	sepX0 := win.x + 1
	if sepX0 < clip.X {
		sepX0 = clip.X
	}
	sepX1 := win.x + win.width - 1
	if clipRight := clip.X + clip.W; sepX1 > clipRight {
		sepX1 = clipRight
	}
	if sepY >= clip.Y && sepY < clip.Y+clip.H {
		for x := sepX0; x < sepX1; x++ {
			bgpx := buf.GetPixel(x, sepY)
			buf.SetPixel(x, sepY, sep.Blend(bgpx))
		}
	}

	// Client content
	win.bufMu.RLock()
	clientBuf := win.buffer
	if clientBuf == nil {
		win.bufMu.RUnlock()
		return
	}
	// Keep client content alpha as authored by the app (no global attenuation).
	blitWithOpacityRounded(buf, clientBuf, clientRect.X, clientRect.Y, 255, false, frameRect, windowCornerRadius, clip)
	if win.focused {
		buf.DrawRoundedRect(frameRect, windowCornerRadius, graphics.NewColorHex(0xB9C5DD))
	} else {
		buf.DrawRoundedRect(frameRect, windowCornerRadius, graphics.NewColorHex(0xCCD4E4))
	}
	win.bufMu.RUnlock()
}

func closeButtonRect(win *window) graphics.Rect {
	btnW := titleButtonSize
	btnH := titleButtonSize
	top := win.y + (decorHeight-btnH)/2
	return graphics.Rect{
		X: win.x + win.width - btnW - titleButtonRightPad,
		Y: top,
		W: btnW,
		H: btnH,
	}
}

func maximizeButtonRect(win *window) graphics.Rect {
	btnW := titleButtonSize
	btnH := titleButtonSize
	top := win.y + (decorHeight-btnH)/2
	return graphics.Rect{
		X: win.x + win.width - 2*btnW - titleButtonRightPad - titleButtonGap,
		Y: top,
		W: btnW,
		H: btnH,
	}
}

func minimizeButtonRect(win *window) graphics.Rect {
	btnW := titleButtonSize
	btnH := titleButtonSize
	top := win.y + (decorHeight-btnH)/2
	return graphics.Rect{
		X: win.x + win.width - 3*btnW - titleButtonRightPad - 2*titleButtonGap,
		Y: top,
		W: btnW,
		H: btnH,
	}
}

func drawShadowRounded(buf *graphics.Buffer, r graphics.Rect, radius, spread, offsetX, offsetY, topClip int, clip graphics.Rect) {
	minShadowY := r.Y + topClip
	for i := spread; i >= 1; i-- {
		outer := graphics.Rect{
			X: r.X - i + offsetX,
			Y: r.Y - i + offsetY,
			W: r.W + 2*i,
			H: r.H + 2*i,
		}
		inner := graphics.Rect{
			X: r.X - (i - 1) + offsetX,
			Y: r.Y - (i - 1) + offsetY,
			W: r.W + 2*(i-1),
			H: r.H + 2*(i-1),
		}
		step := spread - i + 1
		alpha := uint8(4 + (step*step*40)/(spread*spread))
		blendShadowRing(buf, outer, inner, radius+i, radius+i-1, graphics.NewColor(0, 0, 0, alpha), clip, minShadowY)
	}
}

func blendShadowRing(buf *graphics.Buffer, outer, inner graphics.Rect, outerRadius, innerRadius int, c graphics.Color, clip graphics.Rect, minShadowY int) {
	if outer.W <= 0 || outer.H <= 0 || c.A == 0 {
		return
	}

	if outerRadius < 0 {
		outerRadius = 0
	}
	if outerRadius > outer.W/2 {
		outerRadius = outer.W / 2
	}
	if outerRadius > outer.H/2 {
		outerRadius = outer.H / 2
	}
	if innerRadius < 0 {
		innerRadius = 0
	}

	startX := outer.X
	if startX < 0 {
		startX = 0
	}
	startY := outer.Y
	if startY < 0 {
		startY = 0
	}
	endX := outer.X + outer.W
	if endX > buf.Width {
		endX = buf.Width
	}
	endY := outer.Y + outer.H
	if endY > buf.Height {
		endY = buf.Height
	}
	if startX < clip.X {
		startX = clip.X
	}
	if startY < clip.Y {
		startY = clip.Y
	}
	if startY < minShadowY {
		startY = minShadowY
	}
	if clipX1 := clip.X + clip.W; endX > clipX1 {
		endX = clipX1
	}
	if clipY1 := clip.Y + clip.H; endY > clipY1 {
		endY = clipY1
	}
	if startX >= endX || startY >= endY {
		return
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			outerCov := roundedRectCoverageDisplay(x, y, outer, outerRadius)
			if outerCov <= 0.001 {
				continue
			}
			cov := outerCov
			if inner.W > 0 && inner.H > 0 {
				innerCov := roundedRectCoverageDisplay(x, y, inner, innerRadius)
				cov -= innerCov
				if cov <= 0.001 {
					continue
				}
			}
			if cov > 1 {
				cov = 1
			}
			drawCoverageBlendPixel(buf, x, y, c, cov)
		}
	}
}

func pointInRoundedRect(x, y int, r graphics.Rect, radius int) bool {
	if !r.ContainsXY(x, y) {
		return false
	}
	if radius <= 0 {
		return true
	}

	left := r.X + radius
	right := r.X + r.W - radius
	top := r.Y + radius
	bottom := r.Y + r.H - radius

	if x >= left && x < right {
		return true
	}
	if y >= top && y < bottom {
		return true
	}

	var cx int
	if x < left {
		cx = r.X + radius - 1
	} else {
		cx = r.X + r.W - radius
	}

	var cy int
	if y < top {
		cy = r.Y + radius - 1
	} else {
		cy = r.Y + r.H - radius
	}

	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= radius*radius
}

func fillTopRoundedRect(buf *graphics.Buffer, r graphics.Rect, radius int, c graphics.Color) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if radius <= 0 {
		buf.FillRect(r, c)
		return
	}
	if radius > r.W/2 {
		radius = r.W / 2
	}
	if radius > r.H {
		radius = r.H
	}

	// Center strip and lower body (square bottom corners).
	buf.FillRect(graphics.Rect{X: r.X + radius, Y: r.Y, W: r.W - 2*radius, H: r.H}, c)
	buf.FillRect(graphics.Rect{X: r.X, Y: r.Y + radius, W: r.W, H: r.H - radius}, c)

	// Top rounded corners.
	r2 := radius * radius
	for dy := 0; dy < radius; dy++ {
		for dx := 0; dx < radius; dx++ {
			if dx*dx+dy*dy <= r2 {
				buf.SetPixel(r.X+radius-1-dx, r.Y+radius-1-dy, c)
				buf.SetPixel(r.X+r.W-radius+dx, r.Y+radius-1-dy, c)
			}
		}
	}
}

func blendFillRoundedRect(buf *graphics.Buffer, r graphics.Rect, radius int, c graphics.Color, clip graphics.Rect) {
	if r.W <= 0 || r.H <= 0 || c.A == 0 {
		return
	}
	if radius <= 0 {
		blendFillRect(buf, r, c, clip)
		return
	}
	if radius > r.W/2 {
		radius = r.W / 2
	}
	if radius > r.H/2 {
		radius = r.H / 2
	}

	startX := r.X
	if startX < 0 {
		startX = 0
	}
	startY := r.Y
	if startY < 0 {
		startY = 0
	}
	endX := r.X + r.W
	if endX > buf.Width {
		endX = buf.Width
	}
	endY := r.Y + r.H
	if endY > buf.Height {
		endY = buf.Height
	}
	if startX < clip.X {
		startX = clip.X
	}
	if startY < clip.Y {
		startY = clip.Y
	}
	if clipX1 := clip.X + clip.W; endX > clipX1 {
		endX = clipX1
	}
	if clipY1 := clip.Y + clip.H; endY > clipY1 {
		endY = clipY1
	}
	if startX >= endX || startY >= endY {
		return
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			cov := roundedRectCoverageDisplay(x, y, r, radius)
			if cov <= 0.001 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			drawCoverageBlendPixel(buf, x, y, c, cov)
		}
	}
}

func blendFillRect(buf *graphics.Buffer, r graphics.Rect, c graphics.Color, clip graphics.Rect) {
	if r.W <= 0 || r.H <= 0 || c.A == 0 {
		return
	}
	startX := r.X
	if startX < 0 {
		startX = 0
	}
	startY := r.Y
	if startY < 0 {
		startY = 0
	}
	endX := r.X + r.W
	if endX > buf.Width {
		endX = buf.Width
	}
	endY := r.Y + r.H
	if endY > buf.Height {
		endY = buf.Height
	}
	if startX < clip.X {
		startX = clip.X
	}
	if startY < clip.Y {
		startY = clip.Y
	}
	if clipX1 := clip.X + clip.W; endX > clipX1 {
		endX = clipX1
	}
	if clipY1 := clip.Y + clip.H; endY > clipY1 {
		endY = clipY1
	}
	if startX >= endX || startY >= endY {
		return
	}

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			bg := buf.GetPixel(x, y)
			buf.SetPixel(x, y, c.Blend(bg))
		}
	}
}

func blendFillTopRoundedRect(buf *graphics.Buffer, r graphics.Rect, radius int, c graphics.Color, clip graphics.Rect) {
	if r.W <= 0 || r.H <= 0 || c.A == 0 {
		return
	}
	if radius <= 0 {
		blendFillRect(buf, r, c, clip)
		return
	}
	if radius > r.W/2 {
		radius = r.W / 2
	}
	if radius > r.H {
		radius = r.H
	}

	blendFillRect(buf, graphics.Rect{X: r.X + radius, Y: r.Y, W: r.W - 2*radius, H: r.H}, c, clip)
	blendFillRect(buf, graphics.Rect{X: r.X, Y: r.Y + radius, W: r.W, H: r.H - radius}, c, clip)

	r2 := radius * radius
	for dy := 0; dy < radius; dy++ {
		for dx := 0; dx < radius; dx++ {
			if dx*dx+dy*dy <= r2 {
				px1 := r.X + radius - 1 - dx
				px2 := r.X + r.W - radius + dx
				py := r.Y + radius - 1 - dy
				bg := buf.GetPixel(px1, py)
				buf.SetPixel(px1, py, c.Blend(bg))
				bg = buf.GetPixel(px2, py)
				buf.SetPixel(px2, py, c.Blend(bg))
			}
		}
	}
}

func maskBottomRoundedCorners(buf *graphics.Buffer, r graphics.Rect, radius int, fill graphics.Color) {
	if radius <= 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	if radius > r.W/2 {
		radius = r.W / 2
	}
	if radius > r.H/2 {
		radius = r.H / 2
	}

	r2 := radius * radius
	for dy := 0; dy < radius; dy++ {
		py := r.Y + r.H - radius + dy
		for dx := 0; dx < radius; dx++ {
			if dx*dx+dy*dy <= r2 {
				continue
			}
			buf.SetPixel(r.X+radius-1-dx, py, fill)
			buf.SetPixel(r.X+r.W-radius+dx, py, fill)
		}
	}
}

func maskOutsideRoundedRect(buf *graphics.Buffer, r graphics.Rect, radius int, fill graphics.Color) {
	if radius <= 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	if radius > r.W/2 {
		radius = r.W / 2
	}
	if radius > r.H/2 {
		radius = r.H / 2
	}
	startX := r.X
	if startX < 0 {
		startX = 0
	}
	startY := r.Y
	if startY < 0 {
		startY = 0
	}
	endX := r.X + r.W
	if endX > buf.Width {
		endX = buf.Width
	}
	endY := r.Y + r.H
	if endY > buf.Height {
		endY = buf.Height
	}
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			cov := roundedRectCoverageDisplay(x, y, r, radius)
			if cov <= 0.001 {
				buf.SetPixel(x, y, fill)
				continue
			}
			if cov < 0.999 {
				drawCoverageBlendPixel(buf, x, y, fill, 1-cov)
			}
		}
	}
}

// drawSurface draws a layer surface (no decorations).
func (s *Server) drawSurface(buf *graphics.Buffer, win *window, clip graphics.Rect) {
	win.bufMu.RLock()
	clientBuf := win.buffer
	if clientBuf == nil {
		win.bufMu.RUnlock()
		return
	}
	if win.layer != LayerBackground {
		frameRect := graphics.Rect{X: win.x, Y: win.y, W: win.width, H: win.height}
		drawShadowRounded(buf, frameRect, windowCornerRadius, layerShadowSpread, layerShadowOffsetX, layerShadowOffsetY, layerShadowTopClip, clip)
	}
	// Keep app content alpha as authored; only app pixels with alpha < 255 are translucent.
	blitWithOpacity(buf, clientBuf, win.x, win.y, 255, win.opaqueHint, clip)
	win.bufMu.RUnlock()
}

// drawPopup draws a popup window (no decorations, positioned relative to parent).
func (s *Server) drawPopup(buf *graphics.Buffer, win *window, clip graphics.Rect) {
	win.bufMu.RLock()
	clientBuf := win.buffer
	if clientBuf == nil {
		win.bufMu.RUnlock()
		return
	}
	sx, sy := win.screenPos()
	// Keep popup content alpha as authored by app.
	blitWithOpacity(buf, clientBuf, sx, sy, 255, false, clip)
	win.bufMu.RUnlock()
}

func blitWithOpacity(dst, src *graphics.Buffer, x, y int, opacity uint8, opaqueHint bool, clip graphics.Rect) {
	if dst == nil || src == nil {
		return
	}
	dstRect := graphics.Rect{X: x, Y: y, W: src.Width, H: src.Height}
	drawRect := dstRect.Intersection(clip).Intersection(graphics.Rect{W: dst.Width, H: dst.Height})
	if drawRect.IsEmpty() {
		return
	}

	sx0 := drawRect.X - x
	sy0 := drawRect.Y - y
	sx1 := sx0 + drawRect.W
	sy1 := sy0 + drawRect.H

	if opaqueHint && opacity == 255 &&
		dst.Format == graphics.PixelFormatBGRA && src.Format == graphics.PixelFormatBGRA {
		rowBytes := drawRect.W * 4
		for sy := sy0; sy < sy1; sy++ {
			dy := y + sy
			srcOff := sy*src.Stride + sx0*4
			dstOff := dy*dst.Stride + drawRect.X*4
			simd.CopyBGRAOpaque(
				dst.Data[dstOff:dstOff+rowBytes],
				src.Data[srcOff:srcOff+rowBytes],
				drawRect.W,
			)
		}
		return
	}

	if dst.Format == graphics.PixelFormatBGRA && src.Format == graphics.PixelFormatBGRA {
		rowBytes := drawRect.W * 4
		for sy := sy0; sy < sy1; sy++ {
			dy := y + sy
			srcOff := sy*src.Stride + sx0*4
			dstOff := dy*dst.Stride + drawRect.X*4
			simd.BlendOverBGRA(
				dst.Data[dstOff:dstOff+rowBytes],
				src.Data[srcOff:srcOff+rowBytes],
				drawRect.W,
				opacity,
			)
		}
		return
	}

	for sy := sy0; sy < sy1; sy++ {
		dy := y + sy
		for sx := sx0; sx < sx1; sx++ {
			dx := x + sx
			c := src.GetPixel(sx, sy)
			if opaqueHint {
				c.A = 255
			}
			if c.A == 0 {
				continue
			}
			c.A = uint8((uint16(c.A) * uint16(opacity)) / 255)
			if c.A == 0 {
				continue
			}
			bg := dst.GetPixel(dx, dy)
			dst.SetPixel(dx, dy, c.Blend(bg))
		}
	}
}

func blitWithOpacityRounded(dst, src *graphics.Buffer, x, y int, opacity uint8, opaqueHint bool, roundRect graphics.Rect, radius int, clip graphics.Rect) {
	if dst == nil || src == nil {
		return
	}
	dstRect := graphics.Rect{X: x, Y: y, W: src.Width, H: src.Height}
	drawRect := dstRect.Intersection(clip).Intersection(graphics.Rect{W: dst.Width, H: dst.Height}).Intersection(roundRect)
	if drawRect.IsEmpty() {
		return
	}

	sx0 := drawRect.X - x
	sy0 := drawRect.Y - y
	sx1 := sx0 + drawRect.W
	sy1 := sy0 + drawRect.H

	for sy := sy0; sy < sy1; sy++ {
		dy := y + sy
		for sx := sx0; sx < sx1; sx++ {
			dx := x + sx
			cov := roundedRectCoverageDisplay(dx, dy, roundRect, radius)
			if cov <= 0.001 {
				continue
			}
			c := src.GetPixel(sx, sy)
			if opaqueHint {
				c.A = 255
			}
			if c.A == 0 {
				continue
			}
			c.A = uint8((uint16(c.A) * uint16(opacity)) / 255)
			if cov < 0.999 {
				c.A = uint8(float64(c.A)*cov + 0.5)
			}
			if c.A == 0 {
				continue
			}
			bg := dst.GetPixel(dx, dy)
			dst.SetPixel(dx, dy, c.Blend(bg))
		}
	}
}

func roundedRectCoverageDisplay(x, y int, r graphics.Rect, radius int) float64 {
	if radius <= 0 {
		if r.ContainsXY(x, y) {
			return 1
		}
		return 0
	}
	radius = clampRoundedRadiusDisplay(r, radius)
	if radius <= 0 {
		if r.ContainsXY(x, y) {
			return 1
		}
		return 0
	}

	// Fast-path most pixels and only sample near curved boundaries.
	px := float64(x) + 0.5
	py := float64(y) + 0.5
	sd := roundedRectSignedDistanceDisplay(px, py, r, radius)
	const halfPixelDiag = 0.7071067811865476
	if sd <= -halfPixelDiag {
		return 1
	}
	if sd >= halfPixelDiag {
		return 0
	}

	const samples = 4
	step := 1.0 / float64(samples)
	offset := step / 2.0
	inside := 0
	for sy := 0; sy < samples; sy++ {
		py := float64(y) + offset + float64(sy)*step
		for sx := 0; sx < samples; sx++ {
			px := float64(x) + offset + float64(sx)*step
			if pointInRoundedRectAtDisplayClamped(px, py, r, radius) {
				inside++
			}
		}
	}
	return float64(inside) / float64(samples*samples)
}

func pointInRoundedRectAtDisplay(px, py float64, r graphics.Rect, radius int) bool {
	if radius <= 0 {
		if px < float64(r.X) || px >= float64(r.X+r.W) || py < float64(r.Y) || py >= float64(r.Y+r.H) {
			return false
		}
		return true
	}
	return pointInRoundedRectAtDisplayClamped(px, py, r, clampRoundedRadiusDisplay(r, radius))
}

func pointInRoundedRectAtDisplayClamped(px, py float64, r graphics.Rect, radius int) bool {
	if px < float64(r.X) || px >= float64(r.X+r.W) || py < float64(r.Y) || py >= float64(r.Y+r.H) {
		return false
	}
	if radius <= 0 {
		return true
	}
	rr := float64(radius)
	halfW := float64(r.W) / 2.0
	halfH := float64(r.H) / 2.0
	if rr > halfW {
		rr = halfW
	}
	if rr > halfH {
		rr = halfH
	}
	cx := float64(r.X) + halfW
	cy := float64(r.Y) + halfH
	qx := math.Abs(px-cx) - (halfW - rr)
	qy := math.Abs(py-cy) - (halfH - rr)
	if qx < 0 {
		qx = 0
	}
	if qy < 0 {
		qy = 0
	}
	return (qx*qx + qy*qy) <= (rr * rr)
}

func clampRoundedRadiusDisplay(r graphics.Rect, radius int) int {
	if radius < 0 {
		return 0
	}
	if radius > r.W/2 {
		radius = r.W / 2
	}
	if radius > r.H/2 {
		radius = r.H / 2
	}
	return radius
}

func roundedRectSignedDistanceDisplay(px, py float64, r graphics.Rect, radius int) float64 {
	if r.W <= 0 || r.H <= 0 {
		return 1
	}
	if radius <= 0 {
		dx := math.Max(math.Max(float64(r.X)-px, 0), px-float64(r.X+r.W))
		dy := math.Max(math.Max(float64(r.Y)-py, 0), py-float64(r.Y+r.H))
		if dx > 0 || dy > 0 {
			return math.Hypot(dx, dy)
		}
		inside := math.Min(px-float64(r.X), float64(r.X+r.W)-px)
		insideY := math.Min(py-float64(r.Y), float64(r.Y+r.H)-py)
		if insideY < inside {
			inside = insideY
		}
		return -inside
	}

	halfW := float64(r.W) / 2.0
	halfH := float64(r.H) / 2.0
	rr := float64(radius)
	if rr > halfW {
		rr = halfW
	}
	if rr > halfH {
		rr = halfH
	}

	cx := float64(r.X) + halfW
	cy := float64(r.Y) + halfH
	qx := math.Abs(px-cx) - (halfW - rr)
	qy := math.Abs(py-cy) - (halfH - rr)
	ox := math.Max(qx, 0)
	oy := math.Max(qy, 0)
	outside := math.Hypot(ox, oy)
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - rr
}

func drawCoverageBlendPixel(buf *graphics.Buffer, x, y int, c graphics.Color, coverage float64) {
	if coverage <= 0 {
		return
	}
	a := uint8(float64(c.A)*coverage + 0.5)
	if a == 0 {
		return
	}
	sc := graphics.NewColor(c.R, c.G, c.B, a)
	bg := buf.GetPixel(x, y)
	buf.SetPixel(x, y, sc.Blend(bg))
}

// Cursor bitmap: 16x20, 'B' = black outline, 'W' = white fill, ' ' = transparent.
var cursorBitmap = [20]string{
	"B               ",
	"BB              ",
	"BWB             ",
	"BWWB            ",
	"BWWWB           ",
	"BWWWWB          ",
	"BWWWWWB         ",
	"BWWWWWWB        ",
	"BWWWWWWWB       ",
	"BWWWWWWWWB      ",
	"BWWWWWWWWWB     ",
	"BWWWWWWBBBB     ",
	"BWWWBWWB        ",
	"BWWB BWWB       ",
	"BWB  BWWB       ",
	"BB    BWWB      ",
	"B     BWWB      ",
	"       BWWB     ",
	"       BBB      ",
	"                ",
}

func (s *Server) drawCursor(buf *graphics.Buffer) {
	white := graphics.NewColorRGB(255, 255, 255)
	black := graphics.NewColorRGB(0, 0, 0)
	cx, cy := s.mouseX, s.mouseY
	for row := 0; row < len(cursorBitmap); row++ {
		for col := 0; col < len(cursorBitmap[row]); col++ {
			switch cursorBitmap[row][col] {
			case 'B':
				buf.SetPixel(cx+col, cy+row, black)
			case 'W':
				buf.SetPixel(cx+col, cy+row, white)
			}
		}
	}
}

// --- Input ---

func (s *Server) processInput() {
	var pendingMove *graphics.Event
	flushMove := func() {
		if pendingMove == nil {
			return
		}
		s.handlePointerMotion(pendingMove.X, pendingMove.Y)
		pendingMove = nil
	}

	for {
		ev := s.input.Poll()
		if ev == nil {
			flushMove()
			return
		}
		if ev.Type == graphics.EventQuit {
			flushMove()
			s.Quit()
			return
		}
		switch ev.Type {
		case graphics.EventMouseMove:
			// Coalesce high-frequency move events; only latest position matters.
			pendingMove = ev
		case graphics.EventMouseButtonPress:
			flushMove()
			s.handlePointerButton(ev.MouseButton, true)
		case graphics.EventMouseButtonRelease:
			flushMove()
			s.handlePointerButton(ev.MouseButton, false)
		case graphics.EventKeyPress:
			flushMove()
			s.handleKey(ev, true)
		case graphics.EventKeyRelease:
			flushMove()
			s.handleKey(ev, false)
		}
	}
}

func (s *Server) handlePointerMotion(x, y int) {
	if x == s.mouseX && y == s.mouseY {
		return
	}

	// Dirty the old and new cursor regions.
	s.markDirty(s.prevCursorRect)
	s.mouseX = x
	s.mouseY = y
	cursorRect := graphics.Rect{X: x, Y: y, W: 16, H: 20}
	s.markDirty(cursorRect)

	// Dragging
	if s.dragging && s.dragWin != nil {
		oldRect := windowScreenRect(s.dragWin)
		s.dragWin.x = s.dragWinX + (x - s.dragStartX)
		s.dragWin.y = s.dragWinY + (y - s.dragStartY)
		s.dragWin.maximized = false
		s.markDirty(oldRect)
		s.markDirty(windowScreenRect(s.dragWin))
		s.sendMoved(s.dragWin)
		return
	}

	// Resizing from bottom-right grip.
	if s.resizing && s.resizeWin != nil {
		win := s.resizeWin
		oldRect := windowScreenRect(win)
		newW := s.resizeStartW + (x - s.resizeStartX)
		newH := s.resizeStartH + (y - s.resizeStartY)
		if newW < minWindowWidth {
			newW = minWindowWidth
		}
		if newH < minWindowHeight {
			newH = minWindowHeight
		}
		if newW != win.width || newH != win.height {
			win.width = newW
			win.height = newH
			win.maximized = false
			s.sendConfigure(win)
			s.markDirty(oldRect)
			s.markDirty(windowScreenRect(win))
		}
		return
	}

	win := s.windowAt(x, y)
	if win != s.hovered {
		if s.hovered != nil {
			s.sendPointerLeave(s.hovered)
		}
		s.hovered = win
		if win != nil {
			lx, ly := s.windowLocalCoords(win, x, y)
			s.sendPointerEnter(win, lx, ly)
		}
	} else if win != nil {
		lx, ly := s.windowLocalCoords(win, x, y)
		s.sendPointerMotion(win, lx, ly)
	}
}

// windowLocalCoords converts screen coordinates to window-local coordinates.
func (s *Server) windowLocalCoords(win *window, x, y int) (int, int) {
	switch win.winType {
	case WindowNormal:
		return x - win.x, y - win.y - decorHeight
	case WindowPopup:
		sx, sy := win.screenPos()
		return x - sx, y - sy
	default: // layers
		return x - win.x, y - win.y
	}
}

func (s *Server) handlePointerButton(button graphics.MouseButton, pressed bool) {
	win := s.hovered
	isWheel := button == graphics.MouseButtonWheelUp ||
		button == graphics.MouseButtonWheelDown ||
		button == graphics.MouseButtonWheelLeft ||
		button == graphics.MouseButtonWheelRight

	// Popup dismiss: click outside any popup closes it and removes it.
	if pressed && !isWheel && len(s.popups) > 0 {
		s.mu.Lock()
		var dismissed []*window
		for i := len(s.popups) - 1; i >= 0; i-- {
			pop := s.popups[i]
			sx, sy := pop.screenPos()
			if !(s.mouseX >= sx && s.mouseX < sx+pop.width &&
				s.mouseY >= sy && s.mouseY < sy+pop.height) {
				dismissed = append(dismissed, pop)
			}
		}
		for _, pop := range dismissed {
			s.markDirty(windowScreenRect(pop))
			s.removeWindowLocked(pop)
			delete(pop.sess.windows, pop.id)
		}
		s.mu.Unlock()
		for _, pop := range dismissed {
			s.sendClose(pop)
			s.cleanupWindow(pop)
		}
	}

	if pressed && !isWheel && win != nil {
		if win.winType == WindowNormal {
			// Title bar: close button and drag.
			if s.mouseY >= win.y && s.mouseY < win.y+decorHeight {
				if closeButtonRect(win).ContainsXY(s.mouseX, s.mouseY) {
					s.sendClose(win)
					return
				}
				if maximizeButtonRect(win).ContainsXY(s.mouseX, s.mouseY) {
					s.handleSetWindowState(win.id, WindowActionToggleMaximize)
					return
				}
				if minimizeButtonRect(win).ContainsXY(s.mouseX, s.mouseY) {
					s.handleSetWindowState(win.id, WindowActionToggleMinimize)
					return
				}
				// Clicking titlebar should focus/raise this window before drag begins.
				s.focusWindow(win)
				s.raiseWindow(win)
				if !win.maximized {
					s.dragging = true
					s.dragWin = win
					s.dragStartX = s.mouseX
					s.dragStartY = s.mouseY
					s.dragWinX = win.x
					s.dragWinY = win.y
				}
				return
			}
			// Bottom-right resize grip.
			if s.mouseX >= win.x+win.width-resizeGripSize &&
				s.mouseX < win.x+win.width &&
				s.mouseY >= win.y+decorHeight+win.height-resizeGripSize &&
				s.mouseY < win.y+decorHeight+win.height {
				s.resizing = true
				s.resizeWin = win
				s.resizeStartX = s.mouseX
				s.resizeStartY = s.mouseY
				s.resizeStartW = win.width
				s.resizeStartH = win.height
				return
			}
			// Focus + raise
			s.focusWindow(win)
			s.raiseWindow(win)
		} else if win.winType == WindowLayer || win.winType == WindowPopup {
			// Layers/popups should not steal focus from normal windows.
		}
	}

	if !pressed {
		s.dragging = false
		s.dragWin = nil
		s.resizing = false
		s.resizeWin = nil
	}

	if win != nil {
		buttonCode := 0
		switch button {
		case graphics.MouseButtonLeft:
			buttonCode = 0x110
		case graphics.MouseButtonRight:
			buttonCode = 0x111
		case graphics.MouseButtonMiddle:
			buttonCode = 0x112
		case graphics.MouseButtonWheelUp:
			buttonCode = 4
		case graphics.MouseButtonWheelDown:
			buttonCode = 5
		case graphics.MouseButtonWheelLeft:
			buttonCode = 6
		case graphics.MouseButtonWheelRight:
			buttonCode = 7
		}
		if buttonCode == 0 {
			return
		}
		_ = EmitPointerButton(s.service, win.sess.id, displayapi.PointerButtonEvent{
			WindowID: win.id,
			Button:   buttonCode,
			Pressed:  pressed,
		})
	}
}

func (s *Server) handleKey(ev *graphics.Event, pressed bool) {
	s.mu.Lock()
	target := s.resolveKeyTargetLocked()
	owner, sev, matched := s.matchShortcutLocked(target, ev)
	s.mu.Unlock()
	if matched {
		if pressed {
			_ = EmitShortcut(s.service, owner, sev)
		}
		return
	}

	if target == nil {
		return
	}
	s.sendKey(target, ev, pressed)
}

func (s *Server) sendKey(win *window, ev *graphics.Event, pressed bool) {
	_ = EmitKey(s.service, win.sess.id, displayapi.KeyEvent{
		WindowID: win.id,
		Key:      ev.Key,
		Char:     ev.Rune,
		Pressed:  pressed,
	})
}

func (s *Server) focusWindow(win *window) {
	s.mu.Lock()
	if win == s.focused {
		s.mu.Unlock()
		return
	}
	var old *window
	if s.focused != nil {
		s.focused.focused = false
		old = s.focused
	}
	s.focused = win
	win.focused = true
	s.mu.Unlock()

	if old != nil {
		s.sendFocus(old, false)
		s.markDirty(windowScreenRect(old))
	}
	s.sendFocus(win, true)
	s.markDirty(windowScreenRect(win))
}

func (s *Server) raiseWindow(win *window) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.windows {
		if w == win {
			s.windows = append(s.windows[:i], s.windows[i+1:]...)
			s.windows = append(s.windows, win)
			s.markDirty(windowScreenRect(win))
			return
		}
	}
}

// windowAt finds the topmost window at (x, y) in reverse compositing order.
func (s *Server) windowAt(x, y int) *window {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Overlay layers (topmost)
	for i := len(s.layers[LayerOverlay]) - 1; i >= 0; i-- {
		w := s.layers[LayerOverlay][i]
		if s.isWindowVisible(w) && x >= w.x && x < w.x+w.width && y >= w.y && y < w.y+w.height {
			return w
		}
	}

	// Top layers
	for i := len(s.layers[LayerTop]) - 1; i >= 0; i-- {
		w := s.layers[LayerTop][i]
		if s.isWindowVisible(w) && x >= w.x && x < w.x+w.width && y >= w.y && y < w.y+w.height {
			return w
		}
	}

	// Popups
	for i := len(s.popups) - 1; i >= 0; i-- {
		w := s.popups[i]
		if !s.isWindowVisible(w) {
			continue
		}
		sx, sy := w.screenPos()
		if x >= sx && x < sx+w.width && y >= sy && y < sy+w.height {
			return w
		}
	}

	// Normal windows
	for i := len(s.windows) - 1; i >= 0; i-- {
		w := s.windows[i]
		if w.minimized || !s.isWindowVisible(w) {
			continue
		}
		if x >= w.x && x < w.x+w.width && y >= w.y && y < w.y+w.height+decorHeight {
			return w
		}
	}

	// Bottom layers
	for i := len(s.layers[LayerBottom]) - 1; i >= 0; i-- {
		w := s.layers[LayerBottom][i]
		if s.isWindowVisible(w) && x >= w.x && x < w.x+w.width && y >= w.y && y < w.y+w.height {
			return w
		}
	}

	// Background layers
	for i := len(s.layers[LayerBackground]) - 1; i >= 0; i-- {
		w := s.layers[LayerBackground][i]
		if s.isWindowVisible(w) && x >= w.x && x < w.x+w.width && y >= w.y && y < w.y+w.height {
			return w
		}
	}

	return nil
}

// --- Event sending helpers ---

func (s *Server) sendPointerEnter(win *window, lx, ly int) {
	_ = EmitPointerEnter(s.service, win.sess.id, displayapi.PointerEnterEvent{
		WindowID: win.id,
		X:        lx,
		Y:        ly,
	})
}

func (s *Server) sendPointerLeave(win *window) {
	_ = EmitPointerLeave(s.service, win.sess.id, displayapi.PointerLeaveEvent{WindowID: win.id})
}

func (s *Server) sendPointerMotion(win *window, lx, ly int) {
	_ = EmitPointerMotion(s.service, win.sess.id, displayapi.PointerMotionEvent{
		WindowID: win.id,
		X:        lx,
		Y:        ly,
	})
}

func (s *Server) sendClose(win *window) {
	_ = EmitClose(s.service, win.sess.id, displayapi.CloseEvent{WindowID: win.id})
}

func (s *Server) sendFocus(win *window, focused bool) {
	_ = EmitFocus(s.service, win.sess.id, displayapi.FocusEvent{
		WindowID: win.id,
		Focused:  focused,
	})
}

func (s *Server) sendMoved(win *window) {
	_ = EmitMoved(s.service, win.sess.id, displayapi.MovedEvent{
		WindowID: win.id,
		X:        win.x,
		Y:        win.y,
	})
}

// --- Cleanup ---

func (s *Server) removeSession(sess *session) {
	s.mu.Lock()
	needRecompute := false
	// Snapshot windows for cleanup after lock release.
	windows := make([]*window, 0, len(sess.windows))
	for _, win := range sess.windows {
		windows = append(windows, win)
		s.markDirty(windowScreenRect(win))
		if win.winType == WindowLayer && win.exclusive > 0 {
			needRecompute = true
		}
		s.removeWindowLocked(win)
	}
	delete(s.sessions, sess.id)
	if needRecompute {
		s.recomputeUsableArea()
	}
	s.mu.Unlock()

	for _, win := range windows {
		s.cleanupWindow(win)
	}
}

func (s *Server) removeWindowLocked(win *window) {
	switch win.winType {
	case WindowNormal:
		for i, w := range s.windows {
			if w == win {
				s.windows = append(s.windows[:i], s.windows[i+1:]...)
				break
			}
		}
	case WindowLayer:
		layer := win.layer
		if layer <= LayerOverlay {
			for i, w := range s.layers[layer] {
				if w == win {
					s.layers[layer] = append(s.layers[layer][:i], s.layers[layer][i+1:]...)
					break
				}
			}
		}
	case WindowPopup:
		for i, w := range s.popups {
			if w == win {
				s.popups = append(s.popups[:i], s.popups[i+1:]...)
				break
			}
		}
	}

	if s.focused == win {
		s.focused = nil
	}
	if s.hovered == win {
		s.hovered = nil
	}
	if s.dragWin == win {
		s.dragging = false
		s.dragWin = nil
	}
	if s.resizeWin == win {
		s.resizing = false
		s.resizeWin = nil
	}
}

func (s *Server) cleanupWindow(win *window) {
	if win.shmData != nil {
		syscall.Munmap(win.shmData)
		win.shmData = nil
	}
	if win.shmFD >= 0 {
		syscall.Close(win.shmFD)
		win.shmFD = -1
	}
	if win.shmKey != "" {
		_ = os.Remove(win.shmKey)
		win.shmKey = ""
	}
}
