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

package evdev

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics"
)

// Event types
const (
	evSyn = 0x00
	evKey = 0x01
	evRel = 0x02
	evAbs = 0x03
)

// Relative axes
const (
	relX      = 0x00
	relY      = 0x01
	relHWheel = 0x06
	relWheel  = 0x08
)

// Mouse button codes
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
)

// inputEvent mirrors the Linux input_event structure.
type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

const inputEventSize = int(unsafe.Sizeof(inputEvent{}))

// Device represents an evdev input device.
type Device struct {
	file *os.File
	name string
	path string
}

// Handler manages evdev input devices.
type Handler struct {
	devices   []*Device
	events    chan graphics.Event
	quit      chan struct{}
	wg        sync.WaitGroup
	layout    KeyboardLayout
	modifiers graphics.Modifiers
	capsLock  bool
	mouseX    int
	mouseY    int
	maxX      int
	maxY      int
	mu        sync.Mutex
}

// NewHandler creates a new evdev input handler.
func NewHandler() *Handler {
	return &Handler{
		events: make(chan graphics.Event, 100),
		quit:   make(chan struct{}),
		layout: LayoutUS,
	}
}

// SetLayout sets the keyboard layout.
func (h *Handler) SetLayout(layout KeyboardLayout) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.layout = layout
}

// SetScreenSize sets the screen size for mouse bounds.
func (h *Handler) SetScreenSize(width, height int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.maxX = width
	h.maxY = height
	if h.mouseX >= width {
		h.mouseX = width - 1
	}
	if h.mouseY >= height {
		h.mouseY = height - 1
	}
}

// Open scans for and opens input devices.
func (h *Handler) Open() error {
	matches, err := filepath.Glob(fs.Resolve("device:input/event*"))
	if err != nil {
		return fmt.Errorf("failed to scan input devices: %w", err)
	}

	for _, path := range matches {
		dev, err := h.openDevice(path)
		if err != nil {
			continue // Skip devices we can't open
		}
		h.devices = append(h.devices, dev)
	}

	if len(h.devices) == 0 {
		return fmt.Errorf("no input devices found")
	}

	return nil
}

func (h *Handler) openDevice(path string) (*Device, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	// Get device name
	name := make([]byte, 256)
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		file.Fd(),
		uintptr(0x80FF4506), // EVIOCGNAME(256)
		uintptr(unsafe.Pointer(&name[0])),
	)
	if errno != 0 {
		file.Close()
		return nil, errno
	}

	nameStr := strings.TrimRight(string(name), "\x00")
	return &Device{
		file: file,
		name: nameStr,
		path: path,
	}, nil
}

// Start begins reading events from all devices.
func (h *Handler) Start() {
	for _, dev := range h.devices {
		h.wg.Add(1)
		go h.readDevice(dev)
	}
}

// Stop stops reading events.
func (h *Handler) Stop() {
	close(h.quit)
	h.wg.Wait()
}

// Close closes all devices.
func (h *Handler) Close() error {
	h.Stop()
	for _, dev := range h.devices {
		dev.file.Close()
	}
	h.devices = nil
	close(h.events)
	return nil
}

// Events returns the event channel.
func (h *Handler) Events() <-chan graphics.Event {
	return h.events
}

// Poll returns the next event or nil if none available.
func (h *Handler) Poll() *graphics.Event {
	select {
	case ev := <-h.events:
		return &ev
	default:
		return nil
	}
}

// Wait waits for the next event with a timeout.
func (h *Handler) Wait(timeout time.Duration) *graphics.Event {
	select {
	case ev := <-h.events:
		return &ev
	case <-time.After(timeout):
		return nil
	}
}

func (h *Handler) readDevice(dev *Device) {
	defer h.wg.Done()

	buf := make([]byte, inputEventSize*64)

	for {
		select {
		case <-h.quit:
			return
		default:
		}

		// Use select with timeout for non-blocking read
		var readfds syscall.FdSet
		fd := int(dev.file.Fd())
		readfds.Bits[fd/64] |= 1 << (uint(fd) % 64)

		tv := syscall.Timeval{Sec: 0, Usec: 50000} // 50ms timeout
		n, err := syscall.Select(fd+1, &readfds, nil, nil, &tv)
		if err != nil || n == 0 {
			continue
		}

		bytesRead, err := dev.file.Read(buf)
		if err != nil {
			continue
		}

		for i := 0; i+inputEventSize <= bytesRead; i += inputEventSize {
			var ev inputEvent
			ev.Type = binary.LittleEndian.Uint16(buf[i+16:])
			ev.Code = binary.LittleEndian.Uint16(buf[i+18:])
			ev.Value = int32(binary.LittleEndian.Uint32(buf[i+20:]))

			if event := h.processEvent(&ev); event != nil {
				select {
				case h.events <- *event:
				default:
					// Drop event if channel is full
				}
			}
		}
	}
}

func (h *Handler) processEvent(ev *inputEvent) *graphics.Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch ev.Type {
	case evKey:
		return h.processKeyEvent(ev)
	case evRel:
		return h.processRelEvent(ev)
	}
	return nil
}

func (h *Handler) processKeyEvent(ev *inputEvent) *graphics.Event {
	// Handle mouse buttons
	switch ev.Code {
	case btnLeft, btnRight, btnMiddle:
		return h.processMouseButton(ev)
	}

	// Handle keyboard
	layout, ok := layoutMaps[h.layout]
	if !ok {
		layout = usLayout
	}

	mapping, ok := layout[ev.Code]
	if !ok {
		return nil
	}

	// Update modifiers
	switch mapping.key {
	case graphics.KeyLeftShift, graphics.KeyRightShift:
		if ev.Value == 1 {
			h.modifiers |= graphics.ModShift
		} else if ev.Value == 0 {
			h.modifiers &^= graphics.ModShift
		}
	case graphics.KeyLeftCtrl, graphics.KeyRightCtrl:
		if ev.Value == 1 {
			h.modifiers |= graphics.ModCtrl
		} else if ev.Value == 0 {
			h.modifiers &^= graphics.ModCtrl
		}
	case graphics.KeyLeftAlt, graphics.KeyRightAlt:
		if ev.Value == 1 {
			h.modifiers |= graphics.ModAlt
		} else if ev.Value == 0 {
			h.modifiers &^= graphics.ModAlt
		}
	case graphics.KeyCapsLock:
		if ev.Value == 1 {
			h.capsLock = !h.capsLock
			if h.capsLock {
				h.modifiers |= graphics.ModCapsLock
			} else {
				h.modifiers &^= graphics.ModCapsLock
			}
		}
	}

	// Determine rune
	var r rune
	shifted := h.modifiers&graphics.ModShift != 0
	if h.capsLock && mapping.normal >= 'a' && mapping.normal <= 'z' {
		shifted = !shifted
	}
	if shifted {
		r = mapping.shifted
	} else {
		r = mapping.normal
	}

	eventType := graphics.EventKeyPress
	if ev.Value == 0 {
		eventType = graphics.EventKeyRelease
	} else if ev.Value == 2 {
		// Key repeat - treat as press
		eventType = graphics.EventKeyPress
	}

	return &graphics.Event{
		Type:      eventType,
		Key:       mapping.key,
		Rune:      r,
		Modifiers: h.modifiers,
	}
}

func (h *Handler) processMouseButton(ev *inputEvent) *graphics.Event {
	var btn graphics.MouseButton
	switch ev.Code {
	case btnLeft:
		btn = graphics.MouseButtonLeft
	case btnRight:
		btn = graphics.MouseButtonRight
	case btnMiddle:
		btn = graphics.MouseButtonMiddle
	}

	eventType := graphics.EventMouseButtonPress
	if ev.Value == 0 {
		eventType = graphics.EventMouseButtonRelease
	}

	return &graphics.Event{
		Type:        eventType,
		X:           h.mouseX,
		Y:           h.mouseY,
		MouseButton: btn,
		Modifiers:   h.modifiers,
	}
}

func (h *Handler) processRelEvent(ev *inputEvent) *graphics.Event {
	switch ev.Code {
	case relX:
		if ev.Value == 0 {
			return nil
		}
		oldX := h.mouseX
		h.mouseX += int(ev.Value)
		if h.mouseX < 0 {
			h.mouseX = 0
		}
		if h.maxX > 0 && h.mouseX >= h.maxX {
			h.mouseX = h.maxX - 1
		}
		if h.mouseX == oldX {
			return nil
		}
	case relY:
		if ev.Value == 0 {
			return nil
		}
		oldY := h.mouseY
		h.mouseY += int(ev.Value)
		if h.mouseY < 0 {
			h.mouseY = 0
		}
		if h.maxY > 0 && h.mouseY >= h.maxY {
			h.mouseY = h.maxY - 1
		}
		if h.mouseY == oldY {
			return nil
		}
	case relWheel:
		if ev.Value > 0 {
			return &graphics.Event{
				Type:        graphics.EventMouseButtonPress,
				X:           h.mouseX,
				Y:           h.mouseY,
				MouseButton: graphics.MouseButtonWheelUp,
				Modifiers:   h.modifiers,
			}
		}
		if ev.Value < 0 {
			return &graphics.Event{
				Type:        graphics.EventMouseButtonPress,
				X:           h.mouseX,
				Y:           h.mouseY,
				MouseButton: graphics.MouseButtonWheelDown,
				Modifiers:   h.modifiers,
			}
		}
		return nil
	case relHWheel:
		if ev.Value > 0 {
			return &graphics.Event{
				Type:        graphics.EventMouseButtonPress,
				X:           h.mouseX,
				Y:           h.mouseY,
				MouseButton: graphics.MouseButtonWheelLeft,
				Modifiers:   h.modifiers,
			}
		}
		if ev.Value < 0 {
			return &graphics.Event{
				Type:        graphics.EventMouseButtonPress,
				X:           h.mouseX,
				Y:           h.mouseY,
				MouseButton: graphics.MouseButtonWheelRight,
				Modifiers:   h.modifiers,
			}
		}
		return nil
	default:
		return nil
	}

	return &graphics.Event{
		Type:      graphics.EventMouseMove,
		X:         h.mouseX,
		Y:         h.mouseY,
		Modifiers: h.modifiers,
	}
}

// MousePosition returns the current mouse position.
func (h *Handler) MousePosition() (x, y int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.mouseX, h.mouseY
}

// Devices returns information about opened devices.
func (h *Handler) Devices() []string {
	result := make([]string, len(h.devices))
	for i, dev := range h.devices {
		result[i] = fmt.Sprintf("%s: %s", dev.path, dev.name)
	}
	return result
}
