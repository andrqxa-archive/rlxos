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

package app

import (
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"avyos.dev/pkg/graphics"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
)

const cursorSize = 12

// debugFlash represents a fading damage flash overlay.
type debugFlash struct {
	region    graphics.Rect
	remaining int // frames left to display
}

// App represents the main application with a single root widget
// that fills the entire screen.
type App struct {
	backend    graphics.Backend
	input      graphics.InputHandler
	root       graphics.Widget
	focusables []graphics.Widget
	focusIndex int
	running    bool
	background graphics.Color
	title      string
	fps        int
	OnQuit     func()
	OnEscape   func() // If set, called on Escape instead of quitting.

	// Damage tracking
	prevBounds  map[graphics.Widget]graphics.Rect
	damageList  []graphics.Rect
	fullRedraw  bool
	frameSignal bool
	prevCursorX int
	prevCursorY int

	// Debug
	debugDamage  bool
	debugFlashes []debugFlash
	debugFrames  int
	debugColor   graphics.Color
}

type childProvider interface {
	Children() []graphics.Widget
}

type selfDirtyProvider interface {
	SelfDirty() bool
}

type dirtyRectsProvider interface {
	DirtyRects() []graphics.Rect
}

type titleSetter interface {
	SetTitle(title string)
}

type rectBatchFlusher interface {
	FlushRects([]graphics.Rect) error
}

// Options configures a new application.
type Options struct {
	Title         string                // Window title.
	Width         int                   // Window width; 0 = default (800).
	Height        int                   // Window height; 0 = default (600).
	Backend       graphics.Backend      // Display backend (required).
	Input         graphics.InputHandler // Input handler (required).
	FPS           int                   // Target FPS; 0 = default (60).
	Background    graphics.Color        // Background color.
	BackgroundSet bool                  // When true, use Background even if it is transparent (0 alpha).
}

// New creates a new application with the given options.
func New(opts Options) *App {
	if opts.FPS <= 0 {
		opts.FPS = 60
	}
	bg := opts.Background
	if !opts.BackgroundSet && bg == (graphics.Color{}) {
		bg = graphics.DefaultTheme.Background
	}
	return &App{
		backend:    opts.Backend,
		input:      opts.Input,
		title:      opts.Title,
		focusIndex: -1,
		background: bg,
		fps:        opts.FPS,
		fullRedraw: true,
		prevBounds: make(map[graphics.Widget]graphics.Rect),

		debugFrames: 6,
		debugColor:  graphics.NewColor(255, 0, 0, 80),
	}
}

// SetTitle sets the application title (informational only).
func (a *App) SetTitle(title string) {
	a.title = title
	if ts, ok := a.backend.(titleSetter); ok {
		ts.SetTitle(title)
	}
}

// SetBackground sets the background color.
func (a *App) SetBackground(c graphics.Color) {
	a.background = c
	a.fullRedraw = true
}

// SetFPS sets the target frames per second.
func (a *App) SetFPS(fps int) {
	if fps > 0 {
		a.fps = fps
	}
}

// SetDebugDamage enables or disables the damage flash overlay.
// When enabled, repainted regions flash red and fade out over several frames,
// similar to Android's "Show surface updates" developer option.
func (a *App) SetDebugDamage(enabled bool) {
	a.debugDamage = enabled
	a.fullRedraw = true
}

// SetDebugFlashFrames sets how many frames each flash persists (default 6).
func (a *App) SetDebugFlashFrames(frames int) {
	if frames > 0 {
		a.debugFrames = frames
	}
}

// SetDebugFlashColor sets the flash overlay color (default semi-transparent red).
func (a *App) SetDebugFlashColor(c graphics.Color) {
	a.debugColor = c
}

// SetRoot sets the single root widget. It will be resized to fill the
// entire screen when Run() is called.
func (a *App) SetRoot(widget graphics.Widget) {
	a.root = widget
	a.prevBounds = make(map[graphics.Widget]graphics.Rect)
	a.fullRedraw = true
}

// Root returns the current root widget.
func (a *App) Root() graphics.Widget {
	return a.root
}

// AddFocusable registers a widget for keyboard focus (tab navigation).
// These are typically interactive leaf widgets (buttons, text inputs)
// that live somewhere inside the root widget tree.
func (a *App) AddFocusable(widgets ...graphics.Widget) {
	for _, w := range widgets {
		if isNilWidget(w) {
			continue
		}
		a.focusables = append(a.focusables, w)
	}
}

// ClearFocusables removes all registered focusable widgets.
func (a *App) ClearFocusables() {
	a.focusables = a.focusables[:0]
	a.focusIndex = -1
}

// Focus sets keyboard focus to a specific widget.
func (a *App) Focus(widget graphics.Widget) {
	if isNilWidget(widget) {
		return
	}
	for i, w := range a.focusables {
		if isNilWidget(w) {
			continue
		}
		if w == widget {
			a.setFocus(i)
			return
		}
	}
}

// FocusNext moves focus to the next focusable widget.
func (a *App) FocusNext() {
	if len(a.focusables) == 0 {
		return
	}
	next := a.focusIndex + 1
	if next >= len(a.focusables) {
		next = 0
	}
	a.setFocus(next)
}

// FocusPrevious moves focus to the previous focusable widget.
func (a *App) FocusPrevious() {
	if len(a.focusables) == 0 {
		return
	}
	prev := a.focusIndex - 1
	if prev < 0 {
		prev = len(a.focusables) - 1
	}
	a.setFocus(prev)
}

func (a *App) setFocus(index int) {
	if a.focusIndex >= 0 && a.focusIndex < len(a.focusables) {
		if w := a.focusables[a.focusIndex]; !isNilWidget(w) {
			w.SetFocused(false)
		}
	}
	a.focusIndex = index
	if index >= 0 && index < len(a.focusables) {
		if w := a.focusables[index]; !isNilWidget(w) {
			w.SetFocused(true)
		}
	}
}

// Size returns the screen size.
func (a *App) Size() (width, height int) {
	return a.backend.Size()
}

// Redraw marks the entire screen for redraw.
func (a *App) Redraw() {
	a.fullRedraw = true
}

// RequestFrame schedules a render pass without forcing a full-screen redraw.
func (a *App) RequestFrame() {
	a.frameSignal = true
}

// Run starts the main application loop. The root widget is resized
// to fill the entire framebuffer.
func (a *App) Run() error {
	if a.root == nil {
		return fmt.Errorf("no root widget set; call SetRoot() before Run()")
	}

	a.loadConfiguredDefaultFont()

	if a.backend == nil {
		backend := displaybackend.New()
		a.backend = backend
		a.input = backend
	}
	if ts, ok := a.backend.(titleSetter); ok && a.title != "" {
		ts.SetTitle(a.title)
	}

	if err := a.backend.Open(); err != nil {
		return fmt.Errorf("failed to open backend: %w", err)
	}
	defer a.backend.Close()

	// Size root widget to full screen
	w, h := a.backend.Size()
	a.root.SetBounds(graphics.Rect{W: w, H: h})

	if err := a.input.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to open input devices: %v\n", err)
	} else {
		a.input.SetScreenSize(w, h)
		a.input.Start()
		defer a.input.Close()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	a.running = true
	frameTime := time.Second / time.Duration(a.fps)
	ticker := time.NewTicker(frameTime)
	defer ticker.Stop()
	idleDirtyProbeTick := 0
	idleDirtyProbeEvery := 6 // ~10Hz at 60 FPS; catches async UI updates without full frame cost.
	if a.fps > 0 {
		// Keep probe cadence around 10Hz even if app FPS changes.
		v := a.fps / 10
		if v < 1 {
			v = 1
		}
		idleDirtyProbeEvery = v
	}

	fmt.Printf("Graphics app started: %s\n", a.backend.Info())
	if a.debugDamage {
		fmt.Println("Debug: damage flash enabled")
	}

	for a.running {
		select {
		case <-sigChan:
			a.running = false
			continue
		case <-ticker.C:
			hadEvents := a.processEvents()

			needsFrame := a.fullRedraw || a.debugDamage || a.frameSignal
			if !needsFrame && hadEvents {
				needsFrame = true
			}
			if !needsFrame && !a.backend.HasSystemCursor() {
				mx, my := a.input.MousePosition()
				if mx != a.prevCursorX || my != a.prevCursorY {
					needsFrame = true
				}
			}

			// When idle, avoid 60Hz full frame passes across large trees.
			// Probe dirtiness at a lower cadence for async UI updates.
			if !needsFrame {
				idleDirtyProbeTick++
				if idleDirtyProbeTick >= idleDirtyProbeEvery {
					idleDirtyProbeTick = 0
					if a.root != nil && a.root.IsDirty() {
						needsFrame = true
					}
				}
			} else {
				idleDirtyProbeTick = 0
			}

			if needsFrame {
				a.frame()
				a.frameSignal = false
			}
		}
	}

	if a.OnQuit != nil {
		a.OnQuit()
	}
	return nil
}

// Quit stops the application.
func (a *App) Quit() {
	a.running = false
}

func (a *App) processEvents() bool {
	had := false
	for {
		ev := a.input.Poll()
		if ev == nil {
			break
		}
		had = true
		a.handleEvent(*ev)
	}
	return had
}

func (a *App) handleEvent(ev graphics.Event) {
	if ev.Type == graphics.EventQuit {
		a.running = false
		return
	}

	if ev.Type == graphics.EventResize {
		w, h := ev.X, ev.Y
		if a.root != nil && w > 0 && h > 0 {
			a.root.SetBounds(graphics.Rect{W: w, H: h})
			a.fullRedraw = true
		}
		return
	}

	if ev.Type == graphics.EventKeyPress && ev.Key == graphics.KeyEscape {
		if a.OnEscape != nil {
			a.OnEscape()
		} else {
			a.running = false
		}
		return
	}

	// Tab navigation through focusables
	if ev.Type == graphics.EventKeyPress && ev.Key == graphics.KeyTab {
		if ev.IsShift() {
			a.FocusPrevious()
		} else {
			a.FocusNext()
		}
		return
	}

	// Click to focus: find which focusable was clicked
	if ev.Type == graphics.EventMouseButtonPress && ev.MouseButton == graphics.MouseButtonLeft {
		for i, w := range a.focusables {
			if isNilWidget(w) {
				continue
			}
			if w.Bounds().ContainsXY(ev.X, ev.Y) {
				a.setFocus(i)
				break
			}
		}
	}

	// Keyboard events go to the focused widget first.
	if (ev.Type == graphics.EventKeyPress || ev.Type == graphics.EventKeyRelease) &&
		a.focusIndex >= 0 && a.focusIndex < len(a.focusables) {
		w := a.focusables[a.focusIndex]
		if isNilWidget(w) {
			a.focusIndex = -1
			return
		}
		if w.HandleEvent(ev) {
			return
		}
	}

	// Everything else dispatches through the root tree
	if a.root != nil {
		a.root.HandleEvent(ev)
	}
}

func isNilWidget(w graphics.Widget) bool {
	if w == nil {
		return true
	}
	v := reflect.ValueOf(w)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Render pipeline
// ---------------------------------------------------------------------------

// frame runs one render cycle: collect damage, repaint, flush.
func (a *App) frame() {
	a.damageList = a.damageList[:0]
	buf := a.backend.Buffer()
	if buf == nil {
		return
	}
	buf.ClearClip()

	if a.fullRedraw {
		a.renderFull(buf)
		return
	}

	// Collect damage from dirty widgets
	visited := make(map[graphics.Widget]struct{})
	if a.root != nil {
		a.collectDamage(a.root, visited)
	}
	a.prunePrevBounds(visited)

	// Collect damage from cursor movement (only for backends without system cursor)
	if !a.backend.HasSystemCursor() {
		mx, my := a.input.MousePosition()
		if mx != a.prevCursorX || my != a.prevCursorY {
			a.addDamage(cursorRect(a.prevCursorX, a.prevCursorY))
			a.addDamage(cursorRect(mx, my))
		}
	}

	// Fading debug flashes need continuous repaints until they expire
	if a.debugDamage {
		for i := range a.debugFlashes {
			a.addDamage(a.debugFlashes[i].region)
		}
	}

	if len(a.damageList) == 0 {
		return
	}

	sw, sh := a.backend.Size()
	screen := graphics.Rect{W: sw, H: sh}
	damageRects := normalizeDamageRects(a.damageList, screen, 24)
	if len(damageRects) == 0 {
		return
	}

	// Record the new flash before repainting.
	if a.debugDamage {
		merged := damageRects[0]
		for _, d := range damageRects[1:] {
			merged = merged.Union(d)
		}
		a.debugFlashes = append(a.debugFlashes, debugFlash{
			region:    merged,
			remaining: a.debugFrames,
		})
		// Keep debug mode simple: merged redraw with flash overlay.
		buf.FillRect(merged, a.background)
		if a.root != nil && a.root.IsVisible() && a.root.Bounds().Intersects(merged) {
			buf.SetClip(merged)
			a.root.Draw(buf)
			buf.ClearClip()
		}
		if !a.backend.HasSystemCursor() {
			mx, my := a.input.MousePosition()
			buf.SetClip(merged)
			a.drawCursor(buf, mx, my)
			buf.ClearClip()
			a.prevCursorX = mx
			a.prevCursorY = my
		}
		a.renderFlashes(buf, merged)
		if a.root != nil {
			a.root.MarkClean()
		}
		a.backend.FlushRect(merged)
		return
	}

	mx, my := 0, 0
	cursorNow := graphics.Rect{}
	drawCursor := !a.backend.HasSystemCursor()
	if drawCursor {
		mx, my = a.input.MousePosition()
		cursorNow = cursorRect(mx, my)
	}

	for _, r := range damageRects {
		buf.FillRect(r, a.background)
		if a.root != nil && a.root.IsVisible() && a.root.Bounds().Intersects(r) {
			buf.SetClip(r)
			a.root.Draw(buf)
			buf.ClearClip()
		}
		if drawCursor && cursorNow.Intersects(r) {
			buf.SetClip(r)
			a.drawCursor(buf, mx, my)
			buf.ClearClip()
		}
	}
	a.flushDamageRects(damageRects)
	if a.root != nil {
		a.root.MarkClean()
	}
	if drawCursor {
		a.prevCursorX = mx
		a.prevCursorY = my
	}
}

func (a *App) flushDamageRects(rects []graphics.Rect) {
	if len(rects) == 0 {
		return
	}
	if bf, ok := a.backend.(rectBatchFlusher); ok {
		if err := bf.FlushRects(rects); err == nil {
			return
		}
	}
	for _, r := range rects {
		_ = a.backend.FlushRect(r)
	}
}

// renderFull does a complete repaint of the entire screen.
func (a *App) renderFull(buf *graphics.Buffer) {
	buf.ClearClip()
	buf.Clear(a.background)

	if a.root != nil {
		if a.root.IsVisible() {
			a.root.Draw(buf)
		}
		a.root.MarkClean()
		a.prevBounds = make(map[graphics.Widget]graphics.Rect)
		a.syncPrevBounds(a.root)
	}

	// Draw cursor (only for backends without system cursor)
	if !a.backend.HasSystemCursor() {
		mx, my := a.input.MousePosition()
		a.drawCursor(buf, mx, my)
		a.prevCursorX = mx
		a.prevCursorY = my
	}

	if a.debugDamage {
		sw, sh := a.backend.Size()
		screen := graphics.Rect{W: sw, H: sh}
		a.debugFlashes = append(a.debugFlashes, debugFlash{
			region:    screen,
			remaining: a.debugFrames,
		})
		a.renderFlashes(buf, screen)
	}

	a.backend.Flush()
	a.fullRedraw = false
}

func normalizeDamageRects(rects []graphics.Rect, bounds graphics.Rect, maxRects int) []graphics.Rect {
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
			if rectsTouchOrOverlap(out[i], r) {
				out[i] = out[i].Union(r)
				merged = true
				// Re-merge if this union now overlaps others.
				for j := 0; j < len(out); {
					if j == i {
						j++
						continue
					}
					if rectsTouchOrOverlap(out[i], out[j]) {
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
			for _, item := range out[1:] {
				mergedRect = mergedRect.Union(item)
			}
			out = out[:0]
			out = append(out, mergedRect)
		}
	}
	return out
}

func rectsTouchOrOverlap(a, b graphics.Rect) bool {
	// Expand A by 1px to treat edge-touching regions as mergeable.
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

func (a *App) addDamage(r graphics.Rect) {
	if r.IsEmpty() {
		return
	}
	a.damageList = append(a.damageList, r)
}

func (a *App) collectDamage(w graphics.Widget, visited map[graphics.Widget]struct{}) {
	if w == nil {
		return
	}
	visited[w] = struct{}{}

	selfDirty := false
	if sd, ok := w.(selfDirtyProvider); ok {
		selfDirty = sd.SelfDirty()
	} else {
		selfDirty = w.IsDirty()
	}

	if selfDirty {
		addedRect := false
		if dr, ok := w.(dirtyRectsProvider); ok {
			for _, rect := range dr.DirtyRects() {
				clipped := rect.Intersection(w.Bounds())
				if clipped.IsEmpty() {
					continue
				}
				a.addDamage(clipped)
				addedRect = true
			}
		}
		if !addedRect {
			a.addDamage(w.Bounds())
		}
		if prev, ok := a.prevBounds[w]; ok && prev != w.Bounds() {
			a.addDamage(prev)
		}
	}

	a.prevBounds[w] = w.Bounds()

	if cp, ok := w.(childProvider); ok {
		for _, child := range cp.Children() {
			a.collectDamage(child, visited)
		}
	}
}

func (a *App) syncPrevBounds(w graphics.Widget) {
	if w == nil {
		return
	}
	a.prevBounds[w] = w.Bounds()
	if cp, ok := w.(childProvider); ok {
		for _, child := range cp.Children() {
			a.syncPrevBounds(child)
		}
	}
}

func (a *App) prunePrevBounds(visited map[graphics.Widget]struct{}) {
	for w := range a.prevBounds {
		if _, ok := visited[w]; !ok {
			delete(a.prevBounds, w)
		}
	}
}

// renderFlashes draws and ages all active debug flash overlays
// within the given clip region.
func (a *App) renderFlashes(buf *graphics.Buffer, clip graphics.Rect) {
	alive := a.debugFlashes[:0]

	for i := range a.debugFlashes {
		f := &a.debugFlashes[i]

		visible := f.region.Intersection(clip)
		if !visible.IsEmpty() {
			alpha := uint8(int(a.debugColor.A) * f.remaining / a.debugFrames)
			c := graphics.NewColor(a.debugColor.R, a.debugColor.G, a.debugColor.B, alpha)

			for y := visible.Y; y < visible.Y+visible.H; y++ {
				for x := visible.X; x < visible.X+visible.W; x++ {
					bg := buf.GetPixel(x, y)
					buf.SetPixel(x, y, c.Blend(bg))
				}
			}
		}

		f.remaining--
		if f.remaining > 0 {
			alive = append(alive, *f)
		}
	}

	a.debugFlashes = alive
}

// ---------------------------------------------------------------------------
// Cursor
// ---------------------------------------------------------------------------

func cursorRect(x, y int) graphics.Rect {
	return graphics.Rect{X: x, Y: y, W: cursorSize, H: cursorSize}
}

func (a *App) drawCursor(buf *graphics.Buffer, x, y int) {
	fill := []struct{ dx, dy int }{
		{0, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4}, {0, 5}, {0, 6}, {0, 7},
		{0, 8}, {0, 9}, {0, 10}, {0, 11},
		{1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 4}, {1, 5}, {1, 6}, {1, 7},
		{1, 8}, {1, 9}, {1, 10},
		{2, 2}, {2, 3}, {2, 4}, {2, 5}, {2, 6}, {2, 7}, {2, 8}, {2, 9},
		{3, 3}, {3, 4}, {3, 5}, {3, 6}, {3, 7}, {3, 8},
		{4, 4}, {4, 5}, {4, 6}, {4, 7},
		{5, 5}, {5, 6},
	}
	for _, p := range fill {
		buf.SetPixel(x+p.dx, y+p.dy, graphics.ColorWhite)
	}
	outline := []struct{ dx, dy int }{
		{1, 11}, {2, 10}, {3, 9}, {4, 8}, {5, 7}, {6, 6}, {6, 5},
	}
	for _, p := range outline {
		buf.SetPixel(x+p.dx, y+p.dy, graphics.ColorBlack)
	}
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

// Backend returns the display backend.
func (a *App) Backend() graphics.Backend {
	return a.backend
}

// Input returns the input handler.
func (a *App) Input() graphics.InputHandler {
	return a.input
}
