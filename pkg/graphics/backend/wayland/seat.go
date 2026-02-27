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

import graphics "avyos.dev/pkg/graphics/input"

// wl_seat opcodes (client → server)
const (
	seatGetPointerOp  = 0
	seatGetKeyboardOp = 1
)

// wl_seat events
const (
	seatCapabilitiesEvent = 0
	seatNameEvent         = 1
)

// Seat capability bits
const (
	seatCapPointer  = 1
	seatCapKeyboard = 2
)

// wl_pointer events
const (
	pointerEnterEvent  = 0
	pointerLeaveEvent  = 1
	pointerMotionEvent = 2
	pointerButtonEvent = 3
	pointerAxisEvent   = 4
	pointerFrameEvent  = 5
)

const (
	pointerAxisVertical   = 0
	pointerAxisHorizontal = 1
)

// wl_keyboard events
const (
	keyboardKeymapEvent    = 0
	keyboardEnterEvent     = 1
	keyboardLeaveEvent     = 2
	keyboardKeyEvent       = 3
	keyboardModifiersEvent = 4
)

// Linux evdev key codes (same as in input/evdev/keymap.go)
const (
	evKeyEsc        = 1
	evKey1          = 2
	evKey2          = 3
	evKey3          = 4
	evKey4          = 5
	evKey5          = 6
	evKey6          = 7
	evKey7          = 8
	evKey8          = 9
	evKey9          = 10
	evKey0          = 11
	evKeyMinus      = 12
	evKeyEqual      = 13
	evKeyBackspace  = 14
	evKeyTab        = 15
	evKeyQ          = 16
	evKeyW          = 17
	evKeyE          = 18
	evKeyR          = 19
	evKeyT          = 20
	evKeyY          = 21
	evKeyU          = 22
	evKeyI          = 23
	evKeyO          = 24
	evKeyP          = 25
	evKeyLeftBrace  = 26
	evKeyRightBrace = 27
	evKeyEnter      = 28
	evKeyLeftCtrl   = 29
	evKeyA          = 30
	evKeyS          = 31
	evKeyD          = 32
	evKeyF          = 33
	evKeyG          = 34
	evKeyH          = 35
	evKeyJ          = 36
	evKeyK          = 37
	evKeyL          = 38
	evKeySemicolon  = 39
	evKeyApostrophe = 40
	evKeyGrave      = 41
	evKeyLeftShift  = 42
	evKeyBackslash  = 43
	evKeyZ          = 44
	evKeyX          = 45
	evKeyC          = 46
	evKeyV          = 47
	evKeyB          = 48
	evKeyN          = 49
	evKeyM          = 50
	evKeyComma      = 51
	evKeyDot        = 52
	evKeySlash      = 53
	evKeyRightShift = 54
	evKeyLeftAlt    = 56
	evKeySpace      = 57
	evKeyCapsLock   = 58
	evKeyF1         = 59
	evKeyF2         = 60
	evKeyF3         = 61
	evKeyF4         = 62
	evKeyF5         = 63
	evKeyF6         = 64
	evKeyF7         = 65
	evKeyF8         = 66
	evKeyF9         = 67
	evKeyF10        = 68
	evKeyNumLock    = 69
	evKeyScrollLock = 70
	evKeyF11        = 87
	evKeyF12        = 88
	evKeyRightCtrl  = 97
	evKeyRightAlt   = 100
	evKeyHome       = 102
	evKeyUp         = 103
	evKeyPageUp     = 104
	evKeyLeft       = 105
	evKeyRight      = 106
	evKeyEnd        = 107
	evKeyDown       = 108
	evKeyPageDown   = 109
	evKeyInsert     = 110
	evKeyDelete     = 111
)

// Mouse button codes (evdev)
const (
	evBtnLeft   = 0x110
	evBtnRight  = 0x111
	evBtnMiddle = 0x112
)

// keyMapping maps an evdev keycode to a Key constant and runes.
type keyMapping struct {
	key     graphics.Key
	normal  rune
	shifted rune
}

// keyMap is the US keyboard layout mapping (evdev keycodes).
var keyMap = map[uint32]keyMapping{
	evKeyEsc:        {graphics.KeyEscape, 0, 0},
	evKey1:          {graphics.KeyNone, '1', '!'},
	evKey2:          {graphics.KeyNone, '2', '@'},
	evKey3:          {graphics.KeyNone, '3', '#'},
	evKey4:          {graphics.KeyNone, '4', '$'},
	evKey5:          {graphics.KeyNone, '5', '%'},
	evKey6:          {graphics.KeyNone, '6', '^'},
	evKey7:          {graphics.KeyNone, '7', '&'},
	evKey8:          {graphics.KeyNone, '8', '*'},
	evKey9:          {graphics.KeyNone, '9', '('},
	evKey0:          {graphics.KeyNone, '0', ')'},
	evKeyMinus:      {graphics.KeyNone, '-', '_'},
	evKeyEqual:      {graphics.KeyNone, '=', '+'},
	evKeyBackspace:  {graphics.KeyBackspace, 0, 0},
	evKeyTab:        {graphics.KeyTab, '\t', '\t'},
	evKeyQ:          {graphics.KeyNone, 'q', 'Q'},
	evKeyW:          {graphics.KeyNone, 'w', 'W'},
	evKeyE:          {graphics.KeyNone, 'e', 'E'},
	evKeyR:          {graphics.KeyNone, 'r', 'R'},
	evKeyT:          {graphics.KeyNone, 't', 'T'},
	evKeyY:          {graphics.KeyNone, 'y', 'Y'},
	evKeyU:          {graphics.KeyNone, 'u', 'U'},
	evKeyI:          {graphics.KeyNone, 'i', 'I'},
	evKeyO:          {graphics.KeyNone, 'o', 'O'},
	evKeyP:          {graphics.KeyNone, 'p', 'P'},
	evKeyLeftBrace:  {graphics.KeyNone, '[', '{'},
	evKeyRightBrace: {graphics.KeyNone, ']', '}'},
	evKeyEnter:      {graphics.KeyEnter, '\n', '\n'},
	evKeyLeftCtrl:   {graphics.KeyLeftCtrl, 0, 0},
	evKeyA:          {graphics.KeyNone, 'a', 'A'},
	evKeyS:          {graphics.KeyNone, 's', 'S'},
	evKeyD:          {graphics.KeyNone, 'd', 'D'},
	evKeyF:          {graphics.KeyNone, 'f', 'F'},
	evKeyG:          {graphics.KeyNone, 'g', 'G'},
	evKeyH:          {graphics.KeyNone, 'h', 'H'},
	evKeyJ:          {graphics.KeyNone, 'j', 'J'},
	evKeyK:          {graphics.KeyNone, 'k', 'K'},
	evKeyL:          {graphics.KeyNone, 'l', 'L'},
	evKeySemicolon:  {graphics.KeyNone, ';', ':'},
	evKeyApostrophe: {graphics.KeyNone, '\'', '"'},
	evKeyGrave:      {graphics.KeyNone, '`', '~'},
	evKeyLeftShift:  {graphics.KeyLeftShift, 0, 0},
	evKeyBackslash:  {graphics.KeyNone, '\\', '|'},
	evKeyZ:          {graphics.KeyNone, 'z', 'Z'},
	evKeyX:          {graphics.KeyNone, 'x', 'X'},
	evKeyC:          {graphics.KeyNone, 'c', 'C'},
	evKeyV:          {graphics.KeyNone, 'v', 'V'},
	evKeyB:          {graphics.KeyNone, 'b', 'B'},
	evKeyN:          {graphics.KeyNone, 'n', 'N'},
	evKeyM:          {graphics.KeyNone, 'm', 'M'},
	evKeyComma:      {graphics.KeyNone, ',', '<'},
	evKeyDot:        {graphics.KeyNone, '.', '>'},
	evKeySlash:      {graphics.KeyNone, '/', '?'},
	evKeyRightShift: {graphics.KeyRightShift, 0, 0},
	evKeyLeftAlt:    {graphics.KeyLeftAlt, 0, 0},
	evKeySpace:      {graphics.KeySpace, ' ', ' '},
	evKeyCapsLock:   {graphics.KeyCapsLock, 0, 0},
	evKeyF1:         {graphics.KeyF1, 0, 0},
	evKeyF2:         {graphics.KeyF2, 0, 0},
	evKeyF3:         {graphics.KeyF3, 0, 0},
	evKeyF4:         {graphics.KeyF4, 0, 0},
	evKeyF5:         {graphics.KeyF5, 0, 0},
	evKeyF6:         {graphics.KeyF6, 0, 0},
	evKeyF7:         {graphics.KeyF7, 0, 0},
	evKeyF8:         {graphics.KeyF8, 0, 0},
	evKeyF9:         {graphics.KeyF9, 0, 0},
	evKeyF10:        {graphics.KeyF10, 0, 0},
	evKeyF11:        {graphics.KeyF11, 0, 0},
	evKeyF12:        {graphics.KeyF12, 0, 0},
	evKeyNumLock:    {graphics.KeyNumLock, 0, 0},
	evKeyScrollLock: {graphics.KeyScrollLock, 0, 0},
	evKeyRightCtrl:  {graphics.KeyRightCtrl, 0, 0},
	evKeyRightAlt:   {graphics.KeyRightAlt, 0, 0},
	evKeyHome:       {graphics.KeyHome, 0, 0},
	evKeyUp:         {graphics.KeyUp, 0, 0},
	evKeyPageUp:     {graphics.KeyPageUp, 0, 0},
	evKeyLeft:       {graphics.KeyLeft, 0, 0},
	evKeyRight:      {graphics.KeyRight, 0, 0},
	evKeyEnd:        {graphics.KeyEnd, 0, 0},
	evKeyDown:       {graphics.KeyDown, 0, 0},
	evKeyPageDown:   {graphics.KeyPageDown, 0, 0},
	evKeyInsert:     {graphics.KeyInsert, 0, 0},
	evKeyDelete:     {graphics.KeyDelete, 0, 0},
}

// seatHandler manages wl_pointer and wl_keyboard events.
type seatHandler struct {
	cl         *client
	pointerID  uint32
	keyboardID uint32
	events     chan graphics.Event
	mouseX     int
	mouseY     int
	modifiers  graphics.Modifiers
	capsLock   bool
}

// newSeatHandler creates a seat handler with the given event channel.
func newSeatHandler(cl *client, events chan graphics.Event) *seatHandler {
	return &seatHandler{
		cl:     cl,
		events: events,
	}
}

// bind replaces the default seat handler and requests pointer/keyboard.
// We request both unconditionally because the capabilities event was likely
// already consumed by the no-op handler during bindGlobals().
func (s *seatHandler) bind() {
	s.cl.setHandler(s.cl.seat, s.handleSeat)

	// Request pointer
	s.pointerID = s.cl.allocID()
	s.cl.setHandler(s.pointerID, s.handlePointer)
	p := make([]byte, 4)
	putUint32(p, 0, s.pointerID)
	s.cl.conn.sendMsg(s.cl.seat, seatGetPointerOp, p)

	// Request keyboard
	s.keyboardID = s.cl.allocID()
	s.cl.setHandler(s.keyboardID, s.handleKeyboard)
	p = make([]byte, 4)
	putUint32(p, 0, s.keyboardID)
	s.cl.conn.sendMsg(s.cl.seat, seatGetKeyboardOp, p)
}

func (s *seatHandler) handleSeat(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	if opcode == seatCapabilitiesEvent && len(payload) >= 4 {
		caps := getUint32(payload, 0)

		if caps&seatCapPointer != 0 && s.pointerID == 0 {
			s.pointerID = s.cl.allocID()
			s.cl.setHandler(s.pointerID, s.handlePointer)
			p := make([]byte, 4)
			putUint32(p, 0, s.pointerID)
			s.cl.conn.sendMsg(s.cl.seat, seatGetPointerOp, p)
		}

		if caps&seatCapKeyboard != 0 && s.keyboardID == 0 {
			s.keyboardID = s.cl.allocID()
			s.cl.setHandler(s.keyboardID, s.handleKeyboard)
			p := make([]byte, 4)
			putUint32(p, 0, s.keyboardID)
			s.cl.conn.sendMsg(s.cl.seat, seatGetKeyboardOp, p)
		}
	}
}

func (s *seatHandler) handlePointer(_ uint32, opcode uint16, payload []byte, fds []int) {
	closeFDs(fds)
	switch opcode {
	case pointerMotionEvent:
		if len(payload) >= 12 {
			// payload: uint32 time, fixed x, fixed y
			sx := fixedToInt(getInt32(payload, 4))
			sy := fixedToInt(getInt32(payload, 8))
			s.mouseX = sx
			s.mouseY = sy
			s.emit(graphics.Event{
				Type:      graphics.EventMouseMove,
				X:         sx,
				Y:         sy,
				Modifiers: s.modifiers,
			})
		}

	case pointerButtonEvent:
		if len(payload) >= 12 {
			// payload: uint32 serial, uint32 time, uint32 button, uint32 state
			button := getUint32(payload, 8)
			state := getUint32(payload, 12)

			var btn graphics.MouseButton
			switch button {
			case evBtnLeft:
				btn = graphics.MouseButtonLeft
			case evBtnRight:
				btn = graphics.MouseButtonRight
			case evBtnMiddle:
				btn = graphics.MouseButtonMiddle
			}

			evType := graphics.EventMouseButtonPress
			if state == 0 {
				evType = graphics.EventMouseButtonRelease
			}

			s.emit(graphics.Event{
				Type:        evType,
				X:           s.mouseX,
				Y:           s.mouseY,
				MouseButton: btn,
				Modifiers:   s.modifiers,
			})
		}

	case pointerEnterEvent:
		if len(payload) >= 16 {
			sx := fixedToInt(getInt32(payload, 8))
			sy := fixedToInt(getInt32(payload, 12))
			s.mouseX = sx
			s.mouseY = sy
		}

	case pointerAxisEvent:
		if len(payload) >= 12 {
			axis := getUint32(payload, 4)
			val := fixedToInt(getInt32(payload, 8))
			if val == 0 {
				return
			}
			var btn graphics.MouseButton
			switch axis {
			case pointerAxisVertical:
				if val > 0 {
					btn = graphics.MouseButtonWheelDown
				} else {
					btn = graphics.MouseButtonWheelUp
				}
			case pointerAxisHorizontal:
				if val > 0 {
					btn = graphics.MouseButtonWheelRight
				} else {
					btn = graphics.MouseButtonWheelLeft
				}
			default:
				return
			}
			s.emit(graphics.Event{
				Type:        graphics.EventMouseButtonPress,
				X:           s.mouseX,
				Y:           s.mouseY,
				MouseButton: btn,
				Modifiers:   s.modifiers,
			})
		}

	case pointerLeaveEvent:
		// pointer left our surface
	}
}

func (s *seatHandler) handleKeyboard(_ uint32, opcode uint16, payload []byte, fds []int) {
	switch opcode {
	case keyboardKeymapEvent:
		// We receive an xkb keymap fd - close it since we use our own mapping
		closeFDs(fds)

	case keyboardKeyEvent:
		closeFDs(fds)
		if len(payload) >= 12 {
			// payload: uint32 serial, uint32 time, uint32 key, uint32 state
			key := getUint32(payload, 8)
			state := getUint32(payload, 12)

			// Wayland key = evdev keycode (no +8 offset in wl_keyboard.key)
			evKey := key

			mapping, ok := keyMap[evKey]
			if !ok {
				return
			}

			// Update modifier state
			s.updateModifiers(mapping.key, state)

			// Determine rune
			var r rune
			shifted := s.modifiers&graphics.ModShift != 0
			if s.capsLock && mapping.normal >= 'a' && mapping.normal <= 'z' {
				shifted = !shifted
			}
			if shifted {
				r = mapping.shifted
			} else {
				r = mapping.normal
			}

			evType := graphics.EventKeyPress
			if state == 0 {
				evType = graphics.EventKeyRelease
			}

			s.emit(graphics.Event{
				Type:      evType,
				Key:       mapping.key,
				Rune:      r,
				Modifiers: s.modifiers,
			})
		}

	case keyboardModifiersEvent:
		closeFDs(fds)
		if len(payload) >= 16 {
			// payload: uint32 serial, uint32 mods_depressed, uint32 mods_latched, uint32 mods_locked
			depressed := getUint32(payload, 4)
			locked := getUint32(payload, 12)

			s.modifiers = 0
			if depressed&1 != 0 { // Shift
				s.modifiers |= graphics.ModShift
			}
			if depressed&4 != 0 { // Control
				s.modifiers |= graphics.ModCtrl
			}
			if depressed&8 != 0 { // Alt/Mod1
				s.modifiers |= graphics.ModAlt
			}
			s.capsLock = locked&2 != 0
			if s.capsLock {
				s.modifiers |= graphics.ModCapsLock
			}
		}

	default:
		closeFDs(fds)
	}
}

func (s *seatHandler) updateModifiers(key graphics.Key, state uint32) {
	switch key {
	case graphics.KeyLeftShift, graphics.KeyRightShift:
		if state == 1 {
			s.modifiers |= graphics.ModShift
		} else {
			s.modifiers &^= graphics.ModShift
		}
	case graphics.KeyLeftCtrl, graphics.KeyRightCtrl:
		if state == 1 {
			s.modifiers |= graphics.ModCtrl
		} else {
			s.modifiers &^= graphics.ModCtrl
		}
	case graphics.KeyLeftAlt, graphics.KeyRightAlt:
		if state == 1 {
			s.modifiers |= graphics.ModAlt
		} else {
			s.modifiers &^= graphics.ModAlt
		}
	case graphics.KeyCapsLock:
		if state == 1 {
			s.capsLock = !s.capsLock
			if s.capsLock {
				s.modifiers |= graphics.ModCapsLock
			} else {
				s.modifiers &^= graphics.ModCapsLock
			}
		}
	}
}

func (s *seatHandler) emit(ev graphics.Event) {
	select {
	case s.events <- ev:
	default:
		// drop if full
	}
}
