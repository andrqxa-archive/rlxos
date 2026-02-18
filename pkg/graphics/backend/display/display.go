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

package displaybackend

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	display "avyos.dev/api/display"

	"avyos.dev/pkg/graphics"
)

// Popup represents a popup window managed by the display backend.
type Popup struct {
	win     *display.ClientWindow
	widget  graphics.Widget
	onClose func()
	backend *Backend
	mouseX  int // last known pointer position (window-local)
	mouseY  int
	mu      sync.RWMutex
	closed  bool
}

// Redraw re-renders the popup widget into the popup buffer.
func (p *Popup) Redraw() {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return
	}
	buf := p.win.Buffer()
	p.mu.RUnlock()
	if buf == nil {
		return
	}
	// Popups frequently use translucent backgrounds and shadows; clear first
	// to avoid cumulative darkening across hover-triggered redraws.
	buf.ClearClip()
	buf.Clear(graphics.ColorTransparent)
	p.widget.Draw(buf)
	p.widget.MarkClean()
	p.win.DamageAll()
}

// Backend implements graphics.Backend and graphics.InputHandler
// using the custom display protocol client.
type Backend struct {
	client *display.DisplayClient
	window *display.ClientWindow
	events chan graphics.Event
	width  int
	height int
	title  string

	// Layer surface options (call SetLayer before Open).
	isLayer         bool
	layer           uint32
	anchor          uint32
	exclusive       int
	layerOpaqueHint bool

	// Popup windows.
	popups  map[uint32]*Popup
	popupMu sync.Mutex

	mouseX    int
	mouseY    int
	modifiers graphics.Modifiers
	capsLock  bool
	pendingW  int
	pendingH  int
	pending   bool

	running atomic.Bool
	mu      sync.Mutex
	quit    chan struct{}
	wg      sync.WaitGroup
}

// New creates a display backend with default 800x600 size.
func New() *Backend {
	return &Backend{
		width:  800,
		height: 600,
		events: make(chan graphics.Event, 256),
		quit:   make(chan struct{}),
		popups: make(map[uint32]*Popup),
	}
}

// SetTitle sets the window title (call before Open).
func (b *Backend) SetTitle(title string) {
	b.mu.Lock()
	b.title = title
	win := b.window
	b.mu.Unlock()
	if win != nil {
		if err := win.SetTitle(title); err != nil {
			log.Printf("display backend: failed to set title: %v", err)
		}
	}
}

// SetSize sets the window size (call before Open).
func (b *Backend) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// Resize changes the main surface size at runtime.
// For layer surfaces this allows dynamic content-sized bars/docks.
func (b *Backend) Resize(width, height int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if width <= 0 || height <= 0 {
		return nil
	}

	b.width = width
	b.height = height

	if b.window == nil {
		return nil
	}

	if err := b.window.Resize(width, height); err != nil {
		return err
	}

	// Notify the app loop to relayout immediately.
	b.emit(graphics.Event{
		Type: graphics.EventResize,
		X:    width,
		Y:    height,
	})
	return nil
}

// ListWindows returns current managed normal windows from the display server.
func (b *Backend) ListWindows() ([]display.WindowInfo, error) {
	b.mu.Lock()
	cl := b.client
	b.mu.Unlock()
	if cl == nil {
		return nil, fmt.Errorf("display backend not open")
	}
	return cl.ListWindows()
}

// SetWindowState updates a managed window state (minimize/maximize/restore/focus).
func (b *Backend) SetWindowState(windowID, action uint32) error {
	b.mu.Lock()
	cl := b.client
	b.mu.Unlock()
	if cl == nil {
		return fmt.Errorf("display backend not open")
	}
	return cl.SetWindowState(windowID, action)
}

// RegisterShortcut registers a compositor shortcut for this client.
// Use ShortcutScopeGlobal for compositor-wide shortcuts and ShortcutScopeClient
// for shortcuts scoped to this client (optionally a specific window ID).
func (b *Backend) RegisterShortcut(shortcutID, windowID, scope uint32, key graphics.Key, modifiers graphics.Modifiers) error {
	b.mu.Lock()
	cl := b.client
	b.mu.Unlock()
	if cl == nil {
		return fmt.Errorf("display backend not open")
	}
	return cl.RegisterShortcut(shortcutID, windowID, scope, key, modifiers)
}

// UnregisterShortcut removes a previously registered shortcut.
func (b *Backend) UnregisterShortcut(shortcutID uint32) error {
	b.mu.Lock()
	cl := b.client
	b.mu.Unlock()
	if cl == nil {
		return fmt.Errorf("display backend not open")
	}
	return cl.UnregisterShortcut(shortcutID)
}

// SetLayer configures this backend as a layer surface.
// Call before Open(). Layer surfaces are anchored to screen edges
// and can reserve exclusive zones (e.g. 32px for a panel).
func (b *Backend) SetLayer(layer, anchor uint32, exclusive int) {
	b.isLayer = true
	b.layer = layer
	b.anchor = anchor
	b.exclusive = exclusive
}

// ReconfigureLayer updates anchor/exclusive for an already-open layer surface.
// The main layer surface is recreated in-place to apply new positioning.
func (b *Backend) ReconfigureLayer(anchor uint32, exclusive int) error {
	b.mu.Lock()
	if !b.isLayer {
		b.mu.Unlock()
		return fmt.Errorf("backend is not configured as layer surface")
	}
	b.anchor = anchor
	b.exclusive = exclusive
	cl := b.client
	oldWin := b.window
	layer := b.layer
	width := b.width
	height := b.height
	opaque := b.layerOpaqueHint
	b.mu.Unlock()

	// Not opened yet; new layer config will apply on Open.
	if cl == nil || oldWin == nil {
		return nil
	}

	// Recreate any active popups with the next interaction.
	b.popupMu.Lock()
	for _, p := range b.popups {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			continue
		}
		p.closed = true
		p.mu.Unlock()
		_ = p.win.Destroy()
		if p.onClose != nil {
			p.onClose()
		}
	}
	b.popups = make(map[uint32]*Popup)
	b.popupMu.Unlock()

	newWin, err := cl.CreateLayer(layer, anchor, exclusive, width, height, opaque)
	if err != nil {
		return err
	}

	// Swap to the new window before destroying old one to avoid close->quit.
	b.mu.Lock()
	b.window = newWin
	b.width = newWin.Width
	b.height = newWin.Height
	b.pending = false
	b.mu.Unlock()

	_ = oldWin.Destroy()

	b.emit(graphics.Event{
		Type: graphics.EventResize,
		X:    newWin.Width,
		Y:    newWin.Height,
	})
	return nil
}

// SetLayerOpaqueHint hints that the layer content is fully opaque.
func (b *Backend) SetLayerOpaqueHint(opaque bool) {
	b.layerOpaqueHint = opaque
}

// Open connects to the display server and creates a window or layer surface.
func (b *Backend) Open() error {
	if b.client != nil {
		return nil // already opened
	}

	cl, err := display.Dial()
	if err != nil {
		return fmt.Errorf("display connect: %w", err)
	}
	b.client = cl

	var win *display.ClientWindow
	if b.isLayer {
		win, err = cl.CreateLayer(b.layer, b.anchor, b.exclusive, b.width, b.height, b.layerOpaqueHint)
	} else {
		win, err = cl.CreateWindow(b.width, b.height)
	}
	if err != nil {
		cl.Close()
		b.client = nil
		return fmt.Errorf("display create surface: %w", err)
	}
	b.window = win

	// Server may have stretched the layer surface.
	b.mu.Lock()
	b.width = win.Width
	b.height = win.Height
	b.mu.Unlock()

	if !b.isLayer {
		title := b.title
		if title == "" {
			title = "Graphics App"
		}
		_ = win.SetTitle(title)
	}

	return nil
}

// Close stops the event loop and disconnects.
func (b *Backend) Close() error {
	b.running.Store(false)
	select {
	case <-b.quit:
	default:
		close(b.quit)
	}
	b.wg.Wait()

	// Close all popups.
	b.popupMu.Lock()
	for _, p := range b.popups {
		p.win.Destroy()
	}
	b.popups = make(map[uint32]*Popup)
	b.popupMu.Unlock()

	if b.client != nil {
		err := b.client.Close()
		b.client = nil
		b.window = nil
		return err
	}
	return nil
}

// Size returns the window dimensions.
func (b *Backend) Size() (int, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.width, b.height
}

// Buffer returns the shared memory pixel buffer.
func (b *Backend) Buffer() *graphics.Buffer {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.window == nil {
		return nil
	}
	if b.pending {
		if err := b.window.Resize(b.pendingW, b.pendingH); err == nil {
			b.width = b.pendingW
			b.height = b.pendingH
		}
		b.pending = false
	}
	return b.window.Buffer()
}

// Flush marks the entire window as damaged.
func (b *Backend) Flush() error {
	b.mu.Lock()
	win := b.window
	b.mu.Unlock()
	if win == nil {
		return nil
	}
	return win.DamageAll()
}

// FlushRect marks a region as damaged.
func (b *Backend) FlushRect(r graphics.Rect) error {
	b.mu.Lock()
	win := b.window
	b.mu.Unlock()
	if win == nil {
		return nil
	}
	return win.Damage(r)
}

// Info returns backend information.
func (b *Backend) Info() string {
	kind := "Window"
	if b.isLayer {
		kind = "Layer"
	}
	return fmt.Sprintf("Display: %s %dx%d, BGRA", kind, b.width, b.height)
}

// HasSystemCursor returns true since the server draws the cursor.
func (b *Backend) HasSystemCursor() bool {
	return true
}

// --- Popup management ---

// OpenPopup creates a popup window positioned relative to the main window.
// The content widget is rendered into the popup buffer and receives events.
// onClose is called when the popup is dismissed (e.g. click outside).
func (b *Backend) OpenPopup(x, y, w, h int, content graphics.Widget, onClose func()) *Popup {
	b.mu.Lock()
	cl := b.client
	mainWin := b.window
	b.mu.Unlock()
	if cl == nil || mainWin == nil {
		return nil
	}

	win, err := cl.CreatePopup(mainWin, x, y, w, h)
	if err != nil {
		return nil
	}

	p := &Popup{
		win:     win,
		widget:  content,
		onClose: onClose,
		backend: b,
	}

	// Set widget bounds and do initial render.
	content.SetBounds(graphics.Rect{W: w, H: h})
	p.Redraw()

	b.popupMu.Lock()
	b.popups[win.ID] = p
	b.popupMu.Unlock()

	return p
}

// ClosePopup destroys a popup window. Safe to call multiple times.
func (b *Backend) ClosePopup(p *Popup) {
	if p == nil {
		return
	}

	b.popupMu.Lock()
	_, exists := b.popups[p.win.ID]
	if exists {
		delete(b.popups, p.win.ID)
	}
	b.popupMu.Unlock()

	if !exists {
		return // already closed by server dismiss
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	p.win.Destroy()
	if p.onClose != nil {
		p.onClose()
	}
}

// --- InputHandler implementation ---

// Start begins the event loop goroutine.
func (b *Backend) Start() {
	b.running.Store(true)
	b.wg.Add(1)
	go b.eventLoop()
}

// Poll returns the next event or nil.
func (b *Backend) Poll() *graphics.Event {
	select {
	case ev := <-b.events:
		return &ev
	default:
		return nil
	}
}

// MousePosition returns the last known pointer position.
func (b *Backend) MousePosition() (int, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mouseX, b.mouseY
}

// SetScreenSize is a no-op for window-based backends.
func (b *Backend) SetScreenSize(_, _ int) {}

func (b *Backend) eventLoop() {
	defer b.wg.Done()
	const (
		idlePollMin = 2 * time.Millisecond
		idlePollMax = 32 * time.Millisecond
	)
	idlePollSleep := idlePollMin
	for b.running.Load() {
		ev := b.client.Poll()
		if ev == nil {
			select {
			case <-b.quit:
				return
			default:
			}
			time.Sleep(idlePollSleep)
			if idlePollSleep < idlePollMax {
				idlePollSleep *= 2
				if idlePollSleep > idlePollMax {
					idlePollSleep = idlePollMax
				}
			}
			continue
		}
		idlePollSleep = idlePollMin

		// Check if this event belongs to a popup.
		b.popupMu.Lock()
		popup, isPopup := b.popups[ev.WindowID]
		b.popupMu.Unlock()

		if isPopup {
			b.handlePopupEvent(popup, ev)
		} else {
			b.translateEvent(ev)
		}
	}
}

func (b *Backend) handlePopupEvent(p *Popup, ev *display.Event) {
	switch ev.Type {
	case display.ClientEventPointerMotion, display.ClientEventPointerEnter:
		p.mouseX = ev.X
		p.mouseY = ev.Y
		gev := graphics.Event{Type: graphics.EventMouseMove, X: ev.X, Y: ev.Y}
		if p.widget.HandleEvent(gev) {
			if p.widget.IsDirty() {
				p.Redraw()
			}
		}

	case display.ClientEventPointerLeave:
		p.mouseX = -1
		p.mouseY = -1
		gev := graphics.Event{Type: graphics.EventMouseMove, X: -1, Y: -1}
		if p.widget.HandleEvent(gev) {
			if p.widget.IsDirty() {
				p.Redraw()
			}
		}

	case display.ClientEventPointerButton:
		if btn, ok := translateScrollButton(ev.Button); ok && ev.Pressed {
			gev := graphics.Event{
				Type:        graphics.EventMouseButtonPress,
				X:           p.mouseX,
				Y:           p.mouseY,
				MouseButton: btn,
			}
			p.widget.HandleEvent(gev)
			if p.widget.IsDirty() {
				p.Redraw()
			}
			return
		}
		btn := translateButton(ev.Button)
		evType := graphics.EventMouseButtonRelease
		if ev.Pressed {
			evType = graphics.EventMouseButtonPress
		}
		// EvtPointerButton has no X/Y; use last known position from motion events.
		gev := graphics.Event{Type: evType, X: p.mouseX, Y: p.mouseY, MouseButton: btn}
		p.widget.HandleEvent(gev)
		if p.widget.IsDirty() {
			p.Redraw()
		}

	case display.ClientEventClose:
		// Server dismissed the popup (click outside).
		b.popupMu.Lock()
		_, exists := b.popups[p.win.ID]
		if exists {
			delete(b.popups, p.win.ID)
		}
		b.popupMu.Unlock()
		if !exists {
			return // already closed by ClosePopup
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		p.closed = true
		p.mu.Unlock()
		p.win.Destroy()
		if p.onClose != nil {
			p.onClose()
		}
	}
}

func (b *Backend) translateEvent(ev *display.Event) {
	switch ev.Type {
	case display.ClientEventPointerMotion, display.ClientEventPointerEnter:
		b.mu.Lock()
		b.mouseX = ev.X
		b.mouseY = ev.Y
		b.mu.Unlock()
		b.emit(graphics.Event{
			Type: graphics.EventMouseMove,
			X:    ev.X,
			Y:    ev.Y,
		})

	case display.ClientEventPointerButton:
		if btn, ok := translateScrollButton(ev.Button); ok && ev.Pressed {
			b.mu.Lock()
			mx, my := b.mouseX, b.mouseY
			b.mu.Unlock()
			b.emit(graphics.Event{
				Type:        graphics.EventMouseButtonPress,
				X:           mx,
				Y:           my,
				MouseButton: btn,
			})
			return
		}
		btn := translateButton(ev.Button)
		evType := graphics.EventMouseButtonRelease
		if ev.Pressed {
			evType = graphics.EventMouseButtonPress
		}
		b.mu.Lock()
		mx, my := b.mouseX, b.mouseY
		b.mu.Unlock()
		b.emit(graphics.Event{
			Type:        evType,
			X:           mx,
			Y:           my,
			MouseButton: btn,
		})

	case display.ClientEventKey:
		b.updateModifiers(ev.Key, ev.Pressed)
		evType := graphics.EventKeyRelease
		if ev.Pressed {
			evType = graphics.EventKeyPress
		}
		b.mu.Lock()
		mods := b.modifiers
		b.mu.Unlock()
		b.emit(graphics.Event{
			Type:      evType,
			Key:       ev.Key,
			Rune:      ev.Char,
			Modifiers: mods,
		})

	case display.ClientEventFocus:
		if ev.Focused {
			b.emit(graphics.Event{Type: graphics.EventFocusIn})
		} else {
			b.emit(graphics.Event{Type: graphics.EventFocusOut})
		}

	case display.ClientEventConfigure:
		b.mu.Lock()
		oldW, oldH := b.width, b.height
		b.width = ev.Width
		b.height = ev.Height
		if b.window != nil && ev.WindowID == b.window.ID &&
			(ev.Width != oldW || ev.Height != oldH) {
			b.pendingW = ev.Width
			b.pendingH = ev.Height
			b.pending = true
		}
		b.mu.Unlock()
		// Apply layout resize now; backing shm resize is deferred to Buffer()
		// so rendering and buffer remap stay on the render thread.
		if ev.Width != oldW || ev.Height != oldH {
			b.emit(graphics.Event{
				Type: graphics.EventResize,
				X:    ev.Width,
				Y:    ev.Height,
			})
		}

	case display.ClientEventClose:
		// Only quit if the main window is closed.
		b.mu.Lock()
		isMain := b.window != nil && ev.WindowID == b.window.ID
		b.mu.Unlock()
		if isMain {
			b.emit(graphics.Event{Type: graphics.EventQuit})
		}
	}
}

func (b *Backend) updateModifiers(key graphics.Key, pressed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch key {
	case graphics.KeyLeftShift, graphics.KeyRightShift:
		if pressed {
			b.modifiers |= graphics.ModShift
		} else {
			b.modifiers &^= graphics.ModShift
		}
	case graphics.KeyLeftCtrl, graphics.KeyRightCtrl:
		if pressed {
			b.modifiers |= graphics.ModCtrl
		} else {
			b.modifiers &^= graphics.ModCtrl
		}
	case graphics.KeyLeftAlt, graphics.KeyRightAlt:
		if pressed {
			b.modifiers |= graphics.ModAlt
		} else {
			b.modifiers &^= graphics.ModAlt
		}
	case graphics.KeyCapsLock:
		if pressed {
			b.capsLock = !b.capsLock
			if b.capsLock {
				b.modifiers |= graphics.ModCapsLock
			} else {
				b.modifiers &^= graphics.ModCapsLock
			}
		}
	}
}

func (b *Backend) emit(ev graphics.Event) {
	select {
	case b.events <- ev:
	default:
		log.Printf("display backend: event queue full, dropped event type %d", ev.Type)
	}
}

func translateButton(code int) graphics.MouseButton {
	switch code {
	case 0x110:
		return graphics.MouseButtonLeft
	case 0x111:
		return graphics.MouseButtonRight
	case 0x112:
		return graphics.MouseButtonMiddle
	default:
		return graphics.MouseButtonNone
	}
}

func translateScrollButton(code int) (graphics.MouseButton, bool) {
	switch code {
	case 4, 0x113:
		return graphics.MouseButtonWheelUp, true
	case 5, 0x114:
		return graphics.MouseButtonWheelDown, true
	case 6:
		return graphics.MouseButtonWheelLeft, true
	case 7:
		return graphics.MouseButtonWheelRight, true
	default:
		return graphics.MouseButtonNone, false
	}
}
