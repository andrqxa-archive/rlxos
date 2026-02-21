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

// EventType represents the type of an event.
type EventType int

const (
	EventNone EventType = iota
	EventMouseMove
	EventMouseButtonPress
	EventMouseButtonRelease
	EventKeyPress
	EventKeyRelease
	EventFocusIn
	EventFocusOut
	EventQuit
	EventResize
	EventShortcut
)

// MouseButton represents a mouse button.
type MouseButton int

const (
	MouseButtonNone MouseButton = iota
	MouseButtonLeft
	MouseButtonMiddle
	MouseButtonRight
	MouseButtonWheelUp
	MouseButtonWheelDown
	MouseButtonWheelLeft
	MouseButtonWheelRight
)

// Key represents a keyboard key.
type Key int

const (
	KeyNone Key = iota
	KeyEscape
	KeyBackspace
	KeyTab
	KeyEnter
	KeySpace
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyLeftShift
	KeyRightShift
	KeyLeftCtrl
	KeyRightCtrl
	KeyLeftAlt
	KeyRightAlt
	KeyCapsLock
	KeyNumLock
	KeyScrollLock
)

// Modifiers represents keyboard modifiers.
type Modifiers uint8

const (
	ModShift Modifiers = 1 << iota
	ModCtrl
	ModAlt
	ModCapsLock
)

// Event represents an input event.
type Event struct {
	Type EventType

	// Mouse events
	X, Y        int
	MouseButton MouseButton

	// Keyboard events
	Key       Key
	Rune      rune
	Modifiers Modifiers

	// Shortcut events
	ShortcutID uint32
	WindowID   uint32
	Scope      uint32
}
