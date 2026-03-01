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

import gfxinput "avyos.dev/pkg/graphics/input"

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
	key     gfxinput.Key
	normal  rune
	shifted rune
}

// keyMap is the US keyboard layout mapping (evdev keycodes).
var keyMap = map[uint32]keyMapping{
	evKeyEsc:        {gfxinput.KeyEscape, 0, 0},
	evKey1:          {gfxinput.KeyNone, '1', '!'},
	evKey2:          {gfxinput.KeyNone, '2', '@'},
	evKey3:          {gfxinput.KeyNone, '3', '#'},
	evKey4:          {gfxinput.KeyNone, '4', '$'},
	evKey5:          {gfxinput.KeyNone, '5', '%'},
	evKey6:          {gfxinput.KeyNone, '6', '^'},
	evKey7:          {gfxinput.KeyNone, '7', '&'},
	evKey8:          {gfxinput.KeyNone, '8', '*'},
	evKey9:          {gfxinput.KeyNone, '9', '('},
	evKey0:          {gfxinput.KeyNone, '0', ')'},
	evKeyMinus:      {gfxinput.KeyNone, '-', '_'},
	evKeyEqual:      {gfxinput.KeyNone, '=', '+'},
	evKeyBackspace:  {gfxinput.KeyBackspace, 0, 0},
	evKeyTab:        {gfxinput.KeyTab, '\t', '\t'},
	evKeyQ:          {gfxinput.KeyNone, 'q', 'Q'},
	evKeyW:          {gfxinput.KeyNone, 'w', 'W'},
	evKeyE:          {gfxinput.KeyNone, 'e', 'E'},
	evKeyR:          {gfxinput.KeyNone, 'r', 'R'},
	evKeyT:          {gfxinput.KeyNone, 't', 'T'},
	evKeyY:          {gfxinput.KeyNone, 'y', 'Y'},
	evKeyU:          {gfxinput.KeyNone, 'u', 'U'},
	evKeyI:          {gfxinput.KeyNone, 'i', 'I'},
	evKeyO:          {gfxinput.KeyNone, 'o', 'O'},
	evKeyP:          {gfxinput.KeyNone, 'p', 'P'},
	evKeyLeftBrace:  {gfxinput.KeyNone, '[', '{'},
	evKeyRightBrace: {gfxinput.KeyNone, ']', '}'},
	evKeyEnter:      {gfxinput.KeyEnter, '\n', '\n'},
	evKeyLeftCtrl:   {gfxinput.KeyLeftCtrl, 0, 0},
	evKeyA:          {gfxinput.KeyNone, 'a', 'A'},
	evKeyS:          {gfxinput.KeyNone, 's', 'S'},
	evKeyD:          {gfxinput.KeyNone, 'd', 'D'},
	evKeyF:          {gfxinput.KeyNone, 'f', 'F'},
	evKeyG:          {gfxinput.KeyNone, 'g', 'G'},
	evKeyH:          {gfxinput.KeyNone, 'h', 'H'},
	evKeyJ:          {gfxinput.KeyNone, 'j', 'J'},
	evKeyK:          {gfxinput.KeyNone, 'k', 'K'},
	evKeyL:          {gfxinput.KeyNone, 'l', 'L'},
	evKeySemicolon:  {gfxinput.KeyNone, ';', ':'},
	evKeyApostrophe: {gfxinput.KeyNone, '\'', '"'},
	evKeyGrave:      {gfxinput.KeyNone, '`', '~'},
	evKeyLeftShift:  {gfxinput.KeyLeftShift, 0, 0},
	evKeyBackslash:  {gfxinput.KeyNone, '\\', '|'},
	evKeyZ:          {gfxinput.KeyNone, 'z', 'Z'},
	evKeyX:          {gfxinput.KeyNone, 'x', 'X'},
	evKeyC:          {gfxinput.KeyNone, 'c', 'C'},
	evKeyV:          {gfxinput.KeyNone, 'v', 'V'},
	evKeyB:          {gfxinput.KeyNone, 'b', 'B'},
	evKeyN:          {gfxinput.KeyNone, 'n', 'N'},
	evKeyM:          {gfxinput.KeyNone, 'm', 'M'},
	evKeyComma:      {gfxinput.KeyNone, ',', '<'},
	evKeyDot:        {gfxinput.KeyNone, '.', '>'},
	evKeySlash:      {gfxinput.KeyNone, '/', '?'},
	evKeyRightShift: {gfxinput.KeyRightShift, 0, 0},
	evKeyLeftAlt:    {gfxinput.KeyLeftAlt, 0, 0},
	evKeySpace:      {gfxinput.KeySpace, ' ', ' '},
	evKeyCapsLock:   {gfxinput.KeyCapsLock, 0, 0},
	evKeyF1:         {gfxinput.KeyF1, 0, 0},
	evKeyF2:         {gfxinput.KeyF2, 0, 0},
	evKeyF3:         {gfxinput.KeyF3, 0, 0},
	evKeyF4:         {gfxinput.KeyF4, 0, 0},
	evKeyF5:         {gfxinput.KeyF5, 0, 0},
	evKeyF6:         {gfxinput.KeyF6, 0, 0},
	evKeyF7:         {gfxinput.KeyF7, 0, 0},
	evKeyF8:         {gfxinput.KeyF8, 0, 0},
	evKeyF9:         {gfxinput.KeyF9, 0, 0},
	evKeyF10:        {gfxinput.KeyF10, 0, 0},
	evKeyF11:        {gfxinput.KeyF11, 0, 0},
	evKeyF12:        {gfxinput.KeyF12, 0, 0},
	evKeyNumLock:    {gfxinput.KeyNumLock, 0, 0},
	evKeyScrollLock: {gfxinput.KeyScrollLock, 0, 0},
	evKeyRightCtrl:  {gfxinput.KeyRightCtrl, 0, 0},
	evKeyRightAlt:   {gfxinput.KeyRightAlt, 0, 0},
	evKeyHome:       {gfxinput.KeyHome, 0, 0},
	evKeyUp:         {gfxinput.KeyUp, 0, 0},
	evKeyPageUp:     {gfxinput.KeyPageUp, 0, 0},
	evKeyLeft:       {gfxinput.KeyLeft, 0, 0},
	evKeyRight:      {gfxinput.KeyRight, 0, 0},
	evKeyEnd:        {gfxinput.KeyEnd, 0, 0},
	evKeyDown:       {gfxinput.KeyDown, 0, 0},
	evKeyPageDown:   {gfxinput.KeyPageDown, 0, 0},
	evKeyInsert:     {gfxinput.KeyInsert, 0, 0},
	evKeyDelete:     {gfxinput.KeyDelete, 0, 0},
}

// seatHandler manages wl_pointer and wl_keyboard events.
type seatHandler struct {
	cl         *client
	pointerID  uint32
	keyboardID uint32
	events     chan gfxinput.Event
	mouseX     int
	mouseY     int
	modifiers  gfxinput.Modifiers
	capsLock   bool
}

// newSeatHandler creates a seat handler with the given event channel.
func newSeatHandler(cl *client, events chan gfxinput.Event) *seatHandler {
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
			s.emit(gfxinput.Event{
				Type:      gfxinput.EventMouseMove,
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

			var btn gfxinput.MouseButton
			switch button {
			case evBtnLeft:
				btn = gfxinput.MouseButtonLeft
			case evBtnRight:
				btn = gfxinput.MouseButtonRight
			case evBtnMiddle:
				btn = gfxinput.MouseButtonMiddle
			}

			evType := gfxinput.EventMouseButtonPress
			if state == 0 {
				evType = gfxinput.EventMouseButtonRelease
			}

			s.emit(gfxinput.Event{
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
			var btn gfxinput.MouseButton
			switch axis {
			case pointerAxisVertical:
				if val > 0 {
					btn = gfxinput.MouseButtonWheelDown
				} else {
					btn = gfxinput.MouseButtonWheelUp
				}
			case pointerAxisHorizontal:
				if val > 0 {
					btn = gfxinput.MouseButtonWheelRight
				} else {
					btn = gfxinput.MouseButtonWheelLeft
				}
			default:
				return
			}
			s.emit(gfxinput.Event{
				Type:        gfxinput.EventMouseButtonPress,
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
			shifted := s.modifiers&gfxinput.ModShift != 0
			if s.capsLock && mapping.normal >= 'a' && mapping.normal <= 'z' {
				shifted = !shifted
			}
			if shifted {
				r = mapping.shifted
			} else {
				r = mapping.normal
			}

			evType := gfxinput.EventKeyPress
			if state == 0 {
				evType = gfxinput.EventKeyRelease
			}

			s.emit(gfxinput.Event{
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
				s.modifiers |= gfxinput.ModShift
			}
			if depressed&4 != 0 { // Control
				s.modifiers |= gfxinput.ModCtrl
			}
			if depressed&8 != 0 { // Alt/Mod1
				s.modifiers |= gfxinput.ModAlt
			}
			s.capsLock = locked&2 != 0
			if s.capsLock {
				s.modifiers |= gfxinput.ModCapsLock
			}
		}

	default:
		closeFDs(fds)
	}
}

func (s *seatHandler) updateModifiers(key gfxinput.Key, state uint32) {
	switch key {
	case gfxinput.KeyLeftShift, gfxinput.KeyRightShift:
		if state == 1 {
			s.modifiers |= gfxinput.ModShift
		} else {
			s.modifiers &^= gfxinput.ModShift
		}
	case gfxinput.KeyLeftCtrl, gfxinput.KeyRightCtrl:
		if state == 1 {
			s.modifiers |= gfxinput.ModCtrl
		} else {
			s.modifiers &^= gfxinput.ModCtrl
		}
	case gfxinput.KeyLeftAlt, gfxinput.KeyRightAlt:
		if state == 1 {
			s.modifiers |= gfxinput.ModAlt
		} else {
			s.modifiers &^= gfxinput.ModAlt
		}
	case gfxinput.KeyCapsLock:
		if state == 1 {
			s.capsLock = !s.capsLock
			if s.capsLock {
				s.modifiers |= gfxinput.ModCapsLock
			} else {
				s.modifiers &^= gfxinput.ModCapsLock
			}
		}
	}
}

func (s *seatHandler) emit(ev gfxinput.Event) {
	select {
	case s.events <- ev:
	default:
		// drop if full
	}
}
