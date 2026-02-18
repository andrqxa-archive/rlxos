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

package input

import "avyos.dev/pkg/graphics/core"

// Backend is the display backend interface.
// Implementations include framebuffer (Linux /dev/fb0) and Wayland.
type Backend interface {
	Open() error
	Close() error
	Size() (width, height int)
	Buffer() *core.Buffer
	Flush() error
	FlushRect(core.Rect) error
	Info() string
	HasSystemCursor() bool
}

// InputHandler is the input handler interface.
// Implementations include evdev (Linux raw input) and Wayland seat.
type InputHandler interface {
	Open() error
	Start()
	Close() error
	Poll() *Event
	MousePosition() (x, y int)
	SetScreenSize(width, height int)
}
