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

package wayland

import (
	"fmt"
	"sync"

	"avyos.dev/pkg/graphics"
)

// Backend implements graphics.Backend and graphics.InputHandler for Wayland.
type Backend struct {
	cl      *client
	xdg     *xdgShell
	pool    *shmPool
	seat    *seatHandler
	events  chan graphics.Event
	width   int
	height  int
	title   string
	running bool
	mu      sync.Mutex
	quit    chan struct{}
	wg      sync.WaitGroup
}

// New creates a new Wayland backend.
func New() *Backend {
	return &Backend{
		width:  800,
		height: 600,
		events: make(chan graphics.Event, 256),
		quit:   make(chan struct{}),
	}
}

// SetTitle sets the window title (must be called before Open).
func (b *Backend) SetTitle(title string) {
	b.title = title
}

// Open connects to the Wayland compositor and creates a window.
func (b *Backend) Open() error {
	cl, err := newClient()
	if err != nil {
		return fmt.Errorf("wayland connect: %w", err)
	}
	b.cl = cl

	// Discover globals
	if err := cl.bindGlobals(); err != nil {
		cl.close()
		return fmt.Errorf("wayland globals: %w", err)
	}

	// Set up seat handler
	b.seat = newSeatHandler(cl, b.events)
	b.seat.bind()

	// Do a roundtrip to get seat capabilities
	if err := cl.roundtrip(); err != nil {
		cl.close()
		return fmt.Errorf("wayland seat roundtrip: %w", err)
	}

	// Create surface
	if err := cl.createSurface(); err != nil {
		cl.close()
		return fmt.Errorf("wayland surface: %w", err)
	}

	// Create XDG shell
	xdg, err := newXdgShell(cl)
	if err != nil {
		cl.close()
		return fmt.Errorf("wayland xdg: %w", err)
	}
	b.xdg = xdg

	// Set title
	title := b.title
	if title == "" {
		title = "Graphics App"
	}
	xdg.setTitle(title)
	xdg.setAppID("graphics-app")

	// Handle configure events
	xdg.onConfigure = func(w, h int) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if w > 0 && h > 0 && (w != b.width || h != b.height) {
			b.width = w
			b.height = h
			if b.pool != nil {
				b.pool.resize(w, h)
			}
		}
	}

	xdg.onClose = func() {
		b.emit(graphics.Event{Type: graphics.EventQuit})
	}

	// Initial commit to trigger configure
	if err := xdg.initialCommit(); err != nil {
		cl.close()
		return fmt.Errorf("wayland initial commit: %w", err)
	}

	// Wait for first configure
	if err := xdg.waitConfigure(); err != nil {
		cl.close()
		return fmt.Errorf("wayland configure: %w", err)
	}

	// Create shared memory buffers
	pool, err := newShmPool(cl, b.xdg.width, b.xdg.height)
	if err != nil {
		cl.close()
		return fmt.Errorf("wayland shm: %w", err)
	}
	b.pool = pool
	b.width = b.xdg.width
	b.height = b.xdg.height

	return nil
}

// Close cleans up the Wayland connection.
func (b *Backend) Close() error {
	b.running = false
	select {
	case <-b.quit:
	default:
		close(b.quit)
	}
	b.wg.Wait()

	if b.pool != nil {
		b.pool.destroy()
	}
	if b.cl != nil {
		return b.cl.close()
	}
	return nil
}

// Size returns the window dimensions.
func (b *Backend) Size() (int, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.width, b.height
}

// Buffer returns the back buffer for drawing.
func (b *Backend) Buffer() *graphics.Buffer {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pool == nil {
		return nil
	}
	return b.pool.backBuffer().buf
}

// Flush copies the entire back buffer to the Wayland surface.
func (b *Backend) Flush() error {
	return b.FlushRect(graphics.Rect{W: b.width, H: b.height})
}

// FlushRect commits a damaged region to the Wayland surface.
func (b *Backend) FlushRect(r graphics.Rect) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pool == nil || b.cl == nil {
		return nil
	}

	buf := b.pool.backBuffer()

	// Attach buffer
	if err := b.cl.surfaceAttach(buf.id, 0, 0); err != nil {
		return err
	}

	// Mark damage
	if err := b.cl.surfaceDamage(int32(r.X), int32(r.Y), int32(r.W), int32(r.H)); err != nil {
		return err
	}

	// Commit
	if err := b.cl.surfaceCommit(); err != nil {
		return err
	}

	buf.released = false
	b.pool.swap()

	// Copy contents to new back buffer so incremental drawing works
	newBack := b.pool.backBuffer()
	copy(newBack.buf.Data, buf.buf.Data)

	return nil
}

// Info returns information about the Wayland backend.
func (b *Backend) Info() string {
	return fmt.Sprintf("Wayland: %dx%d, ARGB8888", b.width, b.height)
}

// HasSystemCursor returns true since Wayland compositors provide cursors.
func (b *Backend) HasSystemCursor() bool {
	return true
}

// --- InputHandler interface ---
// Open() is shared with Backend interface (above).
// Close() is shared with Backend interface (above).

// Start begins the event dispatch goroutine.
func (b *Backend) Start() {
	b.running = true
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

// MousePosition returns the last known mouse position.
func (b *Backend) MousePosition() (int, int) {
	if b.seat != nil {
		return b.seat.mouseX, b.seat.mouseY
	}
	return 0, 0
}

// SetScreenSize is a no-op for Wayland (window-based, not screen-based).
func (b *Backend) SetScreenSize(_, _ int) {}

func (b *Backend) eventLoop() {
	defer b.wg.Done()
	for b.running {
		select {
		case <-b.quit:
			return
		default:
			if err := b.cl.dispatch(); err != nil {
				b.emit(graphics.Event{Type: graphics.EventQuit})
				return
			}
		}
	}
}

func (b *Backend) emit(ev graphics.Event) {
	select {
	case b.events <- ev:
	default:
	}
}
