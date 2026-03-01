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
	"fmt"
	"image"
	"image/color"
	"log"
	"net"
	"sync"
	"time"

	gfxdisplay "avyos.dev/pkg/graphics/backend"
	gfxfont "avyos.dev/pkg/graphics/fonts"
	gfxinput "avyos.dev/pkg/graphics/input"
	core "avyos.dev/pkg/graphics/pixmap"
)

const (
	decorHeight   = 24
	cursorSize    = 12
	bgColor       = 0x2E3440 // Nord dark background
	decorActive   = 0x5E81AC // Nord blue
	decorInactive = 0x4C566A // Nord gray
	decorText     = 0xECEFF4 // Nord white
	closeBtn      = 0xBF616A // Nord red
)

// Window represents a client window managed by the compositor.
type Window struct {
	surface  *surfaceState
	toplevel *xdgToplevelState
	session  *clientSession
	x, y     int
	width    int
	height   int
	decorH   int
}

// surfaceLocal converts global coordinates to surface-local coordinates.
func (w *Window) surfaceLocal(gx, gy int) (int, int) {
	return gx - w.x, gy - w.y - w.decorH
}

// bounds returns the total window bounds including decorations.
func (w *Window) bounds() image.Rectangle {
	return core.RectXYWH(w.x, w.y, w.width, w.height+w.decorH)
}

// containsPoint checks if a point is within the window bounds.
func (w *Window) containsPoint(x, y int) bool {
	return core.RectContainsXY(w.bounds(), x, y)
}

// Compositor is the main Wayland compositor.
type Compositor struct {
	fb       gfxdisplay.Backend
	input    gfxinput.Handler
	listener *net.UnixListener
	display  string

	sessions []*clientSession
	windows  []*Window // stack order: back to front

	// Layer surfaces by layer (background, bottom, top, overlay)
	layers [4][]*LayerSurface

	// Foreign toplevel management
	foreign *foreignManagerList

	inp *inputState

	mu      sync.Mutex
	quit    chan struct{}
	redraw  chan struct{}
	dirty   bool
	startMs int64

	// FPS tracking
	frameCount int
	fps        int
	lastFPSMs  int64
}

// New creates a new compositor.
func New(fb gfxdisplay.Backend, input gfxinput.Handler) *Compositor {
	c := &Compositor{
		fb:      fb,
		input:   input,
		foreign: newForeignManagerList(),
		quit:    make(chan struct{}),
		redraw:  make(chan struct{}, 1),
		startMs: time.Now().UnixMilli(),
	}
	c.inp = newInputState(c)
	return c
}

// Run starts the compositor. Blocks until quit.
func (c *Compositor) Run() error {
	// Open display backend
	if err := c.fb.Open(); err != nil {
		return fmt.Errorf("open framebuffer: %w", err)
	}
	defer c.fb.Close()

	// Open input
	if err := c.input.Open(); err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	w, h := c.fb.Size()
	c.input.SetScreenSize(w, h)
	c.input.Start()
	defer c.input.Close()

	// Create server socket
	ln, name, err := listen()
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	c.listener = ln
	c.display = name
	defer ln.Close()

	log.Printf("Compositor listening on %s", name)

	// Accept loop
	go c.acceptLoop()

	// Initial draw
	c.dirty = true

	// Main loop — drain all pending events, then composite once if dirty
	for {
		// Check quit
		select {
		case <-c.quit:
			return nil
		default:
		}

		dirty := false

		// Drain all pending input events before compositing
		for {
			ev := c.input.Poll()
			if ev == nil {
				break
			}
			if ev.Type == gfxinput.EventQuit {
				return nil
			}
			if ev.Type == gfxinput.EventMouseMove {
				mx, my := c.input.MousePosition()
				ev.X = mx
				ev.Y = my
			}
			c.inp.processEvent(ev)
			dirty = true
		}

		// Drain redraw signal from client commits
		select {
		case <-c.redraw:
			dirty = true
		default:
		}

		// Also check the compositor-level dirty flag (set by client commits, focus changes, etc.)
		if c.dirty {
			dirty = true
			c.dirty = false
		}

		if dirty {
			c.composite()
		} else {
			// Nothing to do — block until quit, client redraw, or ~60fps timeout
			select {
			case <-c.quit:
				return nil
			case <-c.redraw:
				c.dirty = true
			case <-time.After(16 * time.Millisecond):
				// Idle timeout — re-check for input
			}
		}
	}
}

// Quit stops the compositor.
func (c *Compositor) Quit() {
	select {
	case <-c.quit:
	default:
		close(c.quit)
	}
}

// Display returns the WAYLAND_DISPLAY name.
func (c *Compositor) Display() string {
	return c.display
}

// acceptLoop accepts new client connections.
func (c *Compositor) acceptLoop() {
	for {
		uc, err := c.listener.AcceptUnix()
		if err != nil {
			select {
			case <-c.quit:
				return
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}

		log.Printf("New client connected")

		session := newClientSession(uc, c)
		c.mu.Lock()
		c.sessions = append(c.sessions, session)
		c.mu.Unlock()

		go session.run()
	}
}

// composite renders all windows onto the framebuffer.
func (c *Compositor) composite() {
	buf := c.fb.Buffer()
	if buf == nil {
		return
	}

	c.mu.Lock()
	windows := make([]*Window, len(c.windows))
	copy(windows, c.windows)
	// Copy layer surfaces
	var layersCopy [4][]*LayerSurface
	for i := 0; i < 4; i++ {
		layersCopy[i] = make([]*LayerSurface, len(c.layers[i]))
		copy(layersCopy[i], c.layers[i])
	}
	c.mu.Unlock()

	// Clear background
	bg := core.NewColorHex(bgColor)
	buf.Clear(bg)

	// Layer: background
	for _, ls := range layersCopy[layerBackground] {
		drawLayerSurface(buf, ls)
	}

	// Layer: bottom
	for _, ls := range layersCopy[layerBottom] {
		drawLayerSurface(buf, ls)
	}

	// Draw each window (back to front)
	for _, win := range windows {
		c.drawWindow(buf, win)
	}

	// Layer: top
	for _, ls := range layersCopy[layerTop] {
		drawLayerSurface(buf, ls)
	}

	// Layer: overlay
	for _, ls := range layersCopy[layerOverlay] {
		drawLayerSurface(buf, ls)
	}

	// Draw cursor
	c.drawCursor(buf, c.inp.mouseX, c.inp.mouseY)

	// FPS counter (top-right corner)
	c.frameCount++
	now := time.Now().UnixMilli()
	if now-c.lastFPSMs >= 1000 {
		c.fps = c.frameCount
		c.frameCount = 0
		c.lastFPSMs = now
	}
	fpsText := fmt.Sprintf("FPS: %d", c.fps)
	w, _ := c.fb.Size()
	fpsX := w - len(fpsText)*8 - 4
	gfxfont.DefaultFont.DrawText(buf, fpsText, fpsX, 4, core.NewColorHex(0xECEFF4), core.NewColorHex(0x2E3440))

	c.fb.Flush()
}

// drawWindow draws a single window with decorations.
func (c *Compositor) drawWindow(buf *core.Buffer, win *Window) {
	if win.surface == nil {
		return
	}
	// Snapshot the buffer pointer (thread-safe) — the compositor goroutine
	// reads this while the client session goroutine may swap in a new buffer.
	clientBuf := win.surface.getBuffer()
	if clientBuf == nil {
		return
	}

	focused := win == c.inp.focusedWindow

	// Title bar
	var titleColor color.NRGBA
	if focused {
		titleColor = core.NewColorHex(decorActive)
	} else {
		titleColor = core.NewColorHex(decorInactive)
	}

	titleRect := core.RectXYWH(win.x, win.y, win.width, win.decorH)
	buf.FillRect(titleRect, titleColor)

	// Window title text
	title := "Untitled"
	if win.toplevel != nil && win.toplevel.title != "" {
		title = win.toplevel.title
	}
	textColor := core.NewColorHex(decorText)
	textX := win.x + 6
	textY := win.y + (win.decorH-16)/2
	gfxfont.DefaultFont.DrawText(buf, title, textX, textY, textColor, color.NRGBA{})

	// Close button [X]
	closeBtnRect := core.RectXYWH(win.x+win.width-win.decorH, win.y, win.decorH, win.decorH)
	buf.FillRect(closeBtnRect, core.NewColorHex(closeBtn))
	xTextX := closeBtnRect.Min.X + (closeBtnRect.Dx()-8)/2
	xTextY := closeBtnRect.Min.Y + (closeBtnRect.Dy()-16)/2
	gfxfont.DefaultFont.DrawText(buf, "X", xTextX, xTextY, textColor, color.NRGBA{})

	// Window border
	borderRect := core.RectXYWH(win.x-1, win.y-1, win.width+2, win.height+win.decorH+2)
	if focused {
		buf.DrawRect(borderRect, core.NewColorHex(decorActive))
	} else {
		buf.DrawRect(borderRect, core.NewColorHex(decorInactive))
	}

	// Client surface content (use BlitOpaque since client may use XRGB with undefined alpha)
	buf.BlitOpaque(clientBuf, win.x, win.y+win.decorH)
}

// drawCursor draws a simple software cursor.
func (c *Compositor) drawCursor(buf *core.Buffer, x, y int) {
	white := core.NewColorRGB(255, 255, 255)
	black := core.NewColorRGB(0, 0, 0)

	// Simple arrow cursor
	for i := 0; i < cursorSize; i++ {
		// Left edge
		buf.SetPixel(x, y+i, white)
		// Fill
		for j := 1; j < i && j < cursorSize-2; j++ {
			buf.SetPixel(x+j, y+i, white)
		}
		// Right edge
		if i > 0 && i < cursorSize-1 {
			buf.SetPixel(x+i, y+i, black)
		}
	}
	// Bottom edge
	for i := 0; i < cursorSize; i++ {
		buf.SetPixel(x+i, y+cursorSize-1, black)
	}
}

// requestRedraw signals that the screen needs to be redrawn.
func (c *Compositor) requestRedraw() {
	select {
	case c.redraw <- struct{}{}:
	default:
	}
}

// timestamp returns milliseconds since compositor start.
func (c *Compositor) timestamp() uint32 {
	return uint32(time.Now().UnixMilli() - c.startMs)
}

// windowAt returns the topmost window containing the given point.
func (c *Compositor) windowAt(x, y int) *Window {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Search from top (front) to bottom (back)
	for i := len(c.windows) - 1; i >= 0; i-- {
		if c.windows[i].containsPoint(x, y) {
			return c.windows[i]
		}
	}
	return nil
}

// raiseWindow moves a window to the top of the stack.
func (c *Compositor) raiseWindow(win *Window) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, w := range c.windows {
		if w == win {
			c.windows = append(c.windows[:i], c.windows[i+1:]...)
			c.windows = append(c.windows, win)
			break
		}
	}
}

// addWindow registers a new window for compositing.
func (c *Compositor) addWindow(win *Window) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Position new windows with a small offset
	offset := len(c.windows) * 30
	w, h := c.fb.Size()
	win.x = 50 + offset
	win.y = 50 + offset

	// Keep on screen
	if win.x+win.width > w {
		win.x = 50
	}
	if win.y+win.height+win.decorH > h {
		win.y = 50
	}

	c.windows = append(c.windows, win)
}

// removeWindow removes a window from compositing.
func (c *Compositor) removeWindow(toplevel *xdgToplevelState) {
	c.mu.Lock()
	for i, w := range c.windows {
		if w.toplevel == toplevel {
			c.windows = append(c.windows[:i], c.windows[i+1:]...)

			// Clear focus if this was focused
			if c.inp.focusedWindow == w {
				c.inp.focusedWindow = nil
				c.inp.focusedSession = nil
			}
			if c.inp.hoveredWindow == w {
				c.inp.hoveredWindow = nil
				c.inp.hoveredSession = nil
			}
			break
		}
	}
	c.mu.Unlock()

	// Notify foreign managers
	c.foreign.notifyToplevelClosed(toplevel)
}

// removeSurface removes all windows associated with a surface.
func (c *Compositor) removeSurface(surf *surfaceState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < len(c.windows); i++ {
		if c.windows[i].surface == surf {
			w := c.windows[i]
			c.windows = append(c.windows[:i], c.windows[i+1:]...)
			i--

			if c.inp.focusedWindow == w {
				c.inp.focusedWindow = nil
				c.inp.focusedSession = nil
			}
			if c.inp.hoveredWindow == w {
				c.inp.hoveredWindow = nil
				c.inp.hoveredSession = nil
			}
		}
	}
}

// removeSession removes all data for a disconnected client.
func (c *Compositor) removeSession(session *clientSession) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove windows belonging to this session
	for i := 0; i < len(c.windows); i++ {
		if c.windows[i].session == session {
			w := c.windows[i]
			c.windows = append(c.windows[:i], c.windows[i+1:]...)
			i--

			if c.inp.focusedWindow == w {
				c.inp.focusedWindow = nil
				c.inp.focusedSession = nil
			}
			if c.inp.hoveredWindow == w {
				c.inp.hoveredWindow = nil
				c.inp.hoveredSession = nil
			}
		}
	}

	// Remove session
	for i, s := range c.sessions {
		if s == session {
			c.sessions = append(c.sessions[:i], c.sessions[i+1:]...)
			break
		}
	}

	c.requestRedraw()
}

// isFocusedToplevel checks if a toplevel is the focused window.
func (c *Compositor) isFocusedToplevel(toplevel *xdgToplevelState) bool {
	if c.inp.focusedWindow == nil {
		return false
	}
	return c.inp.focusedWindow.toplevel == toplevel
}

// updateWindowSize updates a window's dimensions when its buffer changes.
func (c *Compositor) updateWindowSize(surf *surfaceState, width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, w := range c.windows {
		if w.surface == surf {
			w.width = width
			w.height = height
			return
		}
	}
}

// registerToplevel is called when a client creates an xdg_toplevel.
// It creates a Window and adds it to the compositor.
func (c *Compositor) registerToplevel(session *clientSession, surface *surfaceState, toplevel *xdgToplevelState) {
	win := &Window{
		surface:  surface,
		toplevel: toplevel,
		session:  session,
		width:    800,
		height:   600,
		decorH:   decorHeight,
	}

	c.addWindow(win)

	// Auto-focus the new window
	c.inp.focusWindow(win)

	// Notify foreign toplevel managers
	c.foreign.notifyNewToplevel(toplevel, session)
}

// --- Layer surface management ---

// addLayerSurface registers a layer surface and sends its initial configure.
func (c *Compositor) addLayerSurface(ls *layerSurfaceState) {
	screenW, screenH := c.fb.Size()

	layer := ls.layer
	if layer > 3 {
		layer = layerTop
	}

	lsurf := &LayerSurface{
		state:   ls,
		session: ls.session,
	}

	c.mu.Lock()
	c.layers[layer] = append(c.layers[layer], lsurf)
	c.mu.Unlock()

	// Compute geometry and send configure
	lsurf.computeGeometry(screenW, screenH, [4]int32{})

	ls.session.sendLayerSurfaceConfigure(ls, lsurf.width, lsurf.height)
}

// removeLayerSurface removes a layer surface.
func (c *Compositor) removeLayerSurface(ls *layerSurfaceState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < 4; i++ {
		for j := 0; j < len(c.layers[i]); j++ {
			if c.layers[i][j].state == ls {
				c.layers[i] = append(c.layers[i][:j], c.layers[i][j+1:]...)
				j--
			}
		}
	}
}

// --- Foreign toplevel management ---

// sendExistingToplevels sends all existing toplevels to a newly bound foreign manager.
func (c *Compositor) sendExistingToplevels(fm *foreignManager) {
	c.mu.Lock()
	windows := make([]*Window, len(c.windows))
	copy(windows, c.windows)
	c.mu.Unlock()

	for _, win := range windows {
		if win.toplevel != nil {
			fm.sendToplevel(win.toplevel)
		}
	}
}

// activateToplevel raises and focuses a window (called by foreign handle).
func (c *Compositor) activateToplevel(toplevel *xdgToplevelState) {
	c.mu.Lock()
	var target *Window
	for _, w := range c.windows {
		if w.toplevel == toplevel {
			target = w
			break
		}
	}
	c.mu.Unlock()

	if target != nil {
		c.raiseWindow(target)
		c.inp.focusWindow(target)
		c.requestRedraw()
	}
}

// closeToplevel sends a close event to the toplevel's owning client.
func (c *Compositor) closeToplevel(toplevel *xdgToplevelState) {
	if toplevel.xdgSurf != nil && toplevel.xdgSurf.session != nil {
		toplevel.xdgSurf.session.conn.sendMsg(toplevel.id, xdgToplevelCloseEvent, nil)
	}
}
