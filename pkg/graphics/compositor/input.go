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
	"time"

	graphics "avyos.dev/pkg/graphics/input"
)

// inputState tracks the compositor's input routing state.
type inputState struct {
	comp *Compositor

	// Pointer state
	mouseX, mouseY int
	hoveredWindow  *Window
	hoveredSession *clientSession

	// Keyboard state
	focusedWindow  *Window
	focusedSession *clientSession

	// Drag state
	dragging   bool
	dragWindow *Window
	dragStartX int
	dragStartY int
	dragWinX   int
	dragWinY   int

	// Track pressed keys for keyboard enter
	pressedKeys []uint32
}

func newInputState(comp *Compositor) *inputState {
	return &inputState{comp: comp}
}

// processEvent routes an evdev event to the appropriate Wayland client.
func (inp *inputState) processEvent(ev *graphics.Event) {
	switch ev.Type {
	case graphics.EventMouseMove:
		inp.handlePointerMotion(ev.X, ev.Y)

	case graphics.EventMouseButtonPress:
		inp.handlePointerButton(ev.MouseButton, true)

	case graphics.EventMouseButtonRelease:
		inp.handlePointerButton(ev.MouseButton, false)

	case graphics.EventKeyPress:
		inp.handleKey(ev, true)

	case graphics.EventKeyRelease:
		inp.handleKey(ev, false)
	}
}

// handlePointerMotion processes mouse movement.
func (inp *inputState) handlePointerMotion(x, y int) {
	inp.mouseX = x
	inp.mouseY = y

	// Handle window drag
	if inp.dragging && inp.dragWindow != nil {
		dx := x - inp.dragStartX
		dy := y - inp.dragStartY
		inp.dragWindow.x = inp.dragWinX + dx
		inp.dragWindow.y = inp.dragWinY + dy
		inp.comp.requestRedraw()
		return
	}

	// Find window under cursor
	win := inp.comp.windowAt(x, y)

	if win != inp.hoveredWindow {
		// Pointer left previous window
		if inp.hoveredWindow != nil && inp.hoveredSession != nil && inp.hoveredSession.pointerID != 0 {
			serial := inp.hoveredSession.serial()
			p := make([]byte, 8)
			putUint32(p, 0, serial)
			putUint32(p, 4, inp.hoveredWindow.surface.id)
			inp.hoveredSession.conn.sendMsg(inp.hoveredSession.pointerID, pointerLeaveEvent, p)
			inp.sendPointerFrame(inp.hoveredSession)
		}

		inp.hoveredWindow = win
		if win != nil {
			inp.hoveredSession = win.session
		} else {
			inp.hoveredSession = nil
		}

		// Pointer entered new window
		if win != nil && inp.hoveredSession != nil && inp.hoveredSession.pointerID != 0 {
			lx, ly := win.surfaceLocal(x, y)
			serial := inp.hoveredSession.serial()
			p := make([]byte, 16)
			putUint32(p, 0, serial)
			putUint32(p, 4, win.surface.id)
			putInt32(p, 8, intToFixed(lx))
			putInt32(p, 12, intToFixed(ly))
			inp.hoveredSession.conn.sendMsg(inp.hoveredSession.pointerID, pointerEnterEvent, p)
			inp.sendPointerFrame(inp.hoveredSession)
		}
	} else if win != nil && inp.hoveredSession != nil && inp.hoveredSession.pointerID != 0 {
		// Motion within same window
		lx, ly := win.surfaceLocal(x, y)
		ms := uint32(time.Now().UnixMilli())
		p := make([]byte, 12)
		putUint32(p, 0, ms)
		putInt32(p, 4, intToFixed(lx))
		putInt32(p, 8, intToFixed(ly))
		inp.hoveredSession.conn.sendMsg(inp.hoveredSession.pointerID, pointerMotionEvent, p)
		inp.sendPointerFrame(inp.hoveredSession)
	}
}

// handlePointerButton processes a mouse button press or release.
func (inp *inputState) handlePointerButton(button graphics.MouseButton, pressed bool) {
	// Map to Linux evdev button code
	var code uint32
	switch button {
	case graphics.MouseButtonLeft:
		code = evBtnLeft
	case graphics.MouseButtonRight:
		code = evBtnRight
	case graphics.MouseButtonMiddle:
		code = evBtnMiddle
	default:
		return
	}

	win := inp.hoveredWindow

	if pressed && win != nil {
		// Check if click is in the title bar (decoration area)
		if inp.mouseY >= win.y && inp.mouseY < win.y+win.decorH {
			// Check close button (right side of title bar)
			closeX := win.x + win.width - win.decorH
			if inp.mouseX >= closeX && inp.mouseX < win.x+win.width {
				// Close button clicked — send xdg_toplevel.close
				if win.toplevel != nil {
					win.session.conn.sendMsg(win.toplevel.id, xdgToplevelCloseEvent, nil)
				}
				return
			}

			// Start dragging
			inp.dragging = true
			inp.dragWindow = win
			inp.dragStartX = inp.mouseX
			inp.dragStartY = inp.mouseY
			inp.dragWinX = win.x
			inp.dragWinY = win.y
			return
		}

		// Click on client area — focus the window
		inp.focusWindow(win)

		// Raise to top
		inp.comp.raiseWindow(win)
	}

	if !pressed {
		inp.dragging = false
		inp.dragWindow = nil
	}

	// Forward to client
	if win != nil && inp.hoveredSession != nil && inp.hoveredSession.pointerID != 0 {
		serial := inp.hoveredSession.serial()
		ms := uint32(time.Now().UnixMilli())
		var state uint32
		if pressed {
			state = pointerButtonPressed
		}
		p := make([]byte, 16)
		putUint32(p, 0, serial)
		putUint32(p, 4, ms)
		putUint32(p, 8, code)
		putUint32(p, 12, state)
		inp.hoveredSession.conn.sendMsg(inp.hoveredSession.pointerID, pointerButtonEvent, p)
		inp.sendPointerFrame(inp.hoveredSession)
	}
}

// handleKey processes a keyboard event.
func (inp *inputState) handleKey(ev *graphics.Event, pressed bool) {
	win := inp.focusedWindow
	if win == nil || inp.focusedSession == nil || inp.focusedSession.keyboardID == 0 {
		return
	}

	// Convert graphics.Key back to evdev keycode
	keycode := keyToEvdev(ev.Key)
	if keycode == 0 {
		return
	}

	// Track pressed keys
	if pressed {
		inp.pressedKeys = append(inp.pressedKeys, uint32(keycode))
	} else {
		for i, k := range inp.pressedKeys {
			if k == uint32(keycode) {
				inp.pressedKeys = append(inp.pressedKeys[:i], inp.pressedKeys[i+1:]...)
				break
			}
		}
	}

	serial := inp.focusedSession.serial()
	ms := uint32(time.Now().UnixMilli())
	var state uint32
	if pressed {
		state = keyboardKeyPressed
	}

	p := make([]byte, 16)
	putUint32(p, 0, serial)
	putUint32(p, 4, ms)
	putUint32(p, 8, uint32(keycode))
	putUint32(p, 12, state)
	inp.focusedSession.conn.sendMsg(inp.focusedSession.keyboardID, keyboardKeyEvent, p)

	// Send modifiers if changed
	inp.sendModifiers(ev.Modifiers)
}

// focusWindow changes keyboard focus.
func (inp *inputState) focusWindow(win *Window) {
	if win == inp.focusedWindow {
		return
	}

	// Leave old window
	if inp.focusedWindow != nil && inp.focusedSession != nil && inp.focusedSession.keyboardID != 0 {
		serial := inp.focusedSession.serial()
		p := make([]byte, 8)
		putUint32(p, 0, serial)
		putUint32(p, 4, inp.focusedWindow.surface.id)
		inp.focusedSession.conn.sendMsg(inp.focusedSession.keyboardID, keyboardLeaveEvent, p)
	}

	inp.focusedWindow = win
	if win != nil {
		inp.focusedSession = win.session
	} else {
		inp.focusedSession = nil
	}

	// Enter new window
	if win != nil && inp.focusedSession != nil && inp.focusedSession.keyboardID != 0 {
		serial := inp.focusedSession.serial()
		// keyboard.enter: serial, surface, keys(wl_array)
		keysArraySize := len(inp.pressedKeys) * 4
		p := make([]byte, 8+4+keysArraySize)
		putUint32(p, 0, serial)
		putUint32(p, 4, win.surface.id)
		putUint32(p, 8, uint32(keysArraySize))
		for i, k := range inp.pressedKeys {
			putUint32(p, 12+i*4, k)
		}
		inp.focusedSession.conn.sendMsg(inp.focusedSession.keyboardID, keyboardEnterEvent, p)
	}

	inp.comp.requestRedraw()
}

// sendModifiers sends wl_keyboard.modifiers to the focused client.
func (inp *inputState) sendModifiers(mods graphics.Modifiers) {
	if inp.focusedSession == nil || inp.focusedSession.keyboardID == 0 {
		return
	}

	var depressed uint32
	if mods&graphics.ModShift != 0 {
		depressed |= 1 // Shift
	}
	if mods&graphics.ModCtrl != 0 {
		depressed |= 4 // Control
	}
	if mods&graphics.ModAlt != 0 {
		depressed |= 8 // Mod1
	}

	var locked uint32
	if mods&graphics.ModCapsLock != 0 {
		locked |= 2 // Lock
	}

	serial := inp.focusedSession.serial()
	p := make([]byte, 20)
	putUint32(p, 0, serial)
	putUint32(p, 4, depressed)
	putUint32(p, 8, 0) // latched
	putUint32(p, 12, locked)
	putUint32(p, 16, 0) // group
	inp.focusedSession.conn.sendMsg(inp.focusedSession.keyboardID, keyboardModifiersEvent, p)
}

func (inp *inputState) sendPointerFrame(session *clientSession) {
	if session.pointerID != 0 {
		session.conn.sendMsg(session.pointerID, pointerFrameEvent, nil)
	}
}

// keyToEvdev converts a graphics.Key to an evdev keycode.
// These are the raw evdev keycodes as used in wl_keyboard.key events.
var keyToEvdevMap = map[graphics.Key]int{
	graphics.KeyEscape:     1,
	graphics.KeyBackspace:  14,
	graphics.KeyTab:        15,
	graphics.KeyEnter:      28,
	graphics.KeySpace:      57,
	graphics.KeyLeft:       105,
	graphics.KeyRight:      106,
	graphics.KeyUp:         103,
	graphics.KeyDown:       108,
	graphics.KeyHome:       102,
	graphics.KeyEnd:        107,
	graphics.KeyPageUp:     104,
	graphics.KeyPageDown:   109,
	graphics.KeyInsert:     110,
	graphics.KeyDelete:     111,
	graphics.KeyF1:         59,
	graphics.KeyF2:         60,
	graphics.KeyF3:         61,
	graphics.KeyF4:         62,
	graphics.KeyF5:         63,
	graphics.KeyF6:         64,
	graphics.KeyF7:         65,
	graphics.KeyF8:         66,
	graphics.KeyF9:         67,
	graphics.KeyF10:        68,
	graphics.KeyF11:        87,
	graphics.KeyF12:        88,
	graphics.KeyLeftShift:  42,
	graphics.KeyRightShift: 54,
	graphics.KeyLeftCtrl:   29,
	graphics.KeyRightCtrl:  97,
	graphics.KeyLeftAlt:    56,
	graphics.KeyRightAlt:   100,
	graphics.KeyCapsLock:   58,
}

// runeToEvdev maps printable runes to evdev keycodes (US layout).
var runeToEvdevMap = map[rune]int{
	'1': 2, '2': 3, '3': 4, '4': 5, '5': 6,
	'6': 7, '7': 8, '8': 9, '9': 10, '0': 11,
	'-': 12, '=': 13,
	'q': 16, 'w': 17, 'e': 18, 'r': 19, 't': 20,
	'y': 21, 'u': 22, 'i': 23, 'o': 24, 'p': 25,
	'[': 26, ']': 27,
	'a': 30, 's': 31, 'd': 32, 'f': 33, 'g': 34,
	'h': 35, 'j': 36, 'k': 37, 'l': 38,
	';': 39, '\'': 40, '`': 41,
	'\\': 43,
	'z':  44, 'x': 45, 'c': 46, 'v': 47, 'b': 48,
	'n': 49, 'm': 50,
	',': 51, '.': 52, '/': 53,
}

func keyToEvdev(key graphics.Key) int {
	if code, ok := keyToEvdevMap[key]; ok {
		return code
	}
	return 0
}
