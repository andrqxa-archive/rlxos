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

import gfxinput "avyos.dev/pkg/graphics/input"

// KeyboardLayout represents a keyboard layout.
type KeyboardLayout string

const (
	LayoutUS KeyboardLayout = "us"
	LayoutUK KeyboardLayout = "uk"
	LayoutDE KeyboardLayout = "de"
)

// keyMapping maps evdev keycodes to Key constants and runes.
type keyMapping struct {
	key     gfxinput.Key
	normal  rune
	shifted rune
}

// layoutMaps contains the keycode mappings for different layouts.
var layoutMaps = map[KeyboardLayout]map[uint16]keyMapping{
	LayoutUS: usLayout,
	LayoutUK: ukLayout,
	LayoutDE: deLayout,
}

// Linux evdev key codes
const (
	keyEsc        = 1
	key1          = 2
	key2          = 3
	key3          = 4
	key4          = 5
	key5          = 6
	key6          = 7
	key7          = 8
	key8          = 9
	key9          = 10
	key0          = 11
	keyMinus      = 12
	keyEqual      = 13
	keyBackspace  = 14
	keyTab        = 15
	keyQ          = 16
	keyW          = 17
	keyE          = 18
	keyR          = 19
	keyT          = 20
	keyY          = 21
	keyU          = 22
	keyI          = 23
	keyO          = 24
	keyP          = 25
	keyLeftBrace  = 26
	keyRightBrace = 27
	keyEnter      = 28
	keyLeftCtrl   = 29
	keyA          = 30
	keyS          = 31
	keyD          = 32
	keyF          = 33
	keyG          = 34
	keyH          = 35
	keyJ          = 36
	keyK          = 37
	keyL          = 38
	keySemicolon  = 39
	keyApostrophe = 40
	keyGrave      = 41
	keyLeftShift  = 42
	keyBackslash  = 43
	keyZ          = 44
	keyX          = 45
	keyC          = 46
	keyV          = 47
	keyB          = 48
	keyN          = 49
	keyM          = 50
	keyComma      = 51
	keyDot        = 52
	keySlash      = 53
	keyRightShift = 54
	keyKpAsterisk = 55
	keyLeftAlt    = 56
	keySpace      = 57
	keyCapsLock   = 58
	keyF1         = 59
	keyF2         = 60
	keyF3         = 61
	keyF4         = 62
	keyF5         = 63
	keyF6         = 64
	keyF7         = 65
	keyF8         = 66
	keyF9         = 67
	keyF10        = 68
	keyNumLock    = 69
	keyScrollLock = 70
	keyF11        = 87
	keyF12        = 88
	keyRightCtrl  = 97
	keyRightAlt   = 100
	keyHome       = 102
	keyUp         = 103
	keyPageUp     = 104
	keyLeft       = 105
	keyRight      = 106
	keyEnd        = 107
	keyDown       = 108
	keyPageDown   = 109
	keyInsert     = 110
	keyDelete     = 111
)

var usLayout = map[uint16]keyMapping{
	keyEsc:        {gfxinput.KeyEscape, 0, 0},
	key1:          {gfxinput.KeyNone, '1', '!'},
	key2:          {gfxinput.KeyNone, '2', '@'},
	key3:          {gfxinput.KeyNone, '3', '#'},
	key4:          {gfxinput.KeyNone, '4', '$'},
	key5:          {gfxinput.KeyNone, '5', '%'},
	key6:          {gfxinput.KeyNone, '6', '^'},
	key7:          {gfxinput.KeyNone, '7', '&'},
	key8:          {gfxinput.KeyNone, '8', '*'},
	key9:          {gfxinput.KeyNone, '9', '('},
	key0:          {gfxinput.KeyNone, '0', ')'},
	keyMinus:      {gfxinput.KeyNone, '-', '_'},
	keyEqual:      {gfxinput.KeyNone, '=', '+'},
	keyBackspace:  {gfxinput.KeyBackspace, 0, 0},
	keyTab:        {gfxinput.KeyTab, '\t', '\t'},
	keyQ:          {gfxinput.KeyNone, 'q', 'Q'},
	keyW:          {gfxinput.KeyNone, 'w', 'W'},
	keyE:          {gfxinput.KeyNone, 'e', 'E'},
	keyR:          {gfxinput.KeyNone, 'r', 'R'},
	keyT:          {gfxinput.KeyNone, 't', 'T'},
	keyY:          {gfxinput.KeyNone, 'y', 'Y'},
	keyU:          {gfxinput.KeyNone, 'u', 'U'},
	keyI:          {gfxinput.KeyNone, 'i', 'I'},
	keyO:          {gfxinput.KeyNone, 'o', 'O'},
	keyP:          {gfxinput.KeyNone, 'p', 'P'},
	keyLeftBrace:  {gfxinput.KeyNone, '[', '{'},
	keyRightBrace: {gfxinput.KeyNone, ']', '}'},
	keyEnter:      {gfxinput.KeyEnter, '\n', '\n'},
	keyLeftCtrl:   {gfxinput.KeyLeftCtrl, 0, 0},
	keyA:          {gfxinput.KeyNone, 'a', 'A'},
	keyS:          {gfxinput.KeyNone, 's', 'S'},
	keyD:          {gfxinput.KeyNone, 'd', 'D'},
	keyF:          {gfxinput.KeyNone, 'f', 'F'},
	keyG:          {gfxinput.KeyNone, 'g', 'G'},
	keyH:          {gfxinput.KeyNone, 'h', 'H'},
	keyJ:          {gfxinput.KeyNone, 'j', 'J'},
	keyK:          {gfxinput.KeyNone, 'k', 'K'},
	keyL:          {gfxinput.KeyNone, 'l', 'L'},
	keySemicolon:  {gfxinput.KeyNone, ';', ':'},
	keyApostrophe: {gfxinput.KeyNone, '\'', '"'},
	keyGrave:      {gfxinput.KeyNone, '`', '~'},
	keyLeftShift:  {gfxinput.KeyLeftShift, 0, 0},
	keyBackslash:  {gfxinput.KeyNone, '\\', '|'},
	keyZ:          {gfxinput.KeyNone, 'z', 'Z'},
	keyX:          {gfxinput.KeyNone, 'x', 'X'},
	keyC:          {gfxinput.KeyNone, 'c', 'C'},
	keyV:          {gfxinput.KeyNone, 'v', 'V'},
	keyB:          {gfxinput.KeyNone, 'b', 'B'},
	keyN:          {gfxinput.KeyNone, 'n', 'N'},
	keyM:          {gfxinput.KeyNone, 'm', 'M'},
	keyComma:      {gfxinput.KeyNone, ',', '<'},
	keyDot:        {gfxinput.KeyNone, '.', '>'},
	keySlash:      {gfxinput.KeyNone, '/', '?'},
	keyRightShift: {gfxinput.KeyRightShift, 0, 0},
	keyLeftAlt:    {gfxinput.KeyLeftAlt, 0, 0},
	keySpace:      {gfxinput.KeySpace, ' ', ' '},
	keyCapsLock:   {gfxinput.KeyCapsLock, 0, 0},
	keyF1:         {gfxinput.KeyF1, 0, 0},
	keyF2:         {gfxinput.KeyF2, 0, 0},
	keyF3:         {gfxinput.KeyF3, 0, 0},
	keyF4:         {gfxinput.KeyF4, 0, 0},
	keyF5:         {gfxinput.KeyF5, 0, 0},
	keyF6:         {gfxinput.KeyF6, 0, 0},
	keyF7:         {gfxinput.KeyF7, 0, 0},
	keyF8:         {gfxinput.KeyF8, 0, 0},
	keyF9:         {gfxinput.KeyF9, 0, 0},
	keyF10:        {gfxinput.KeyF10, 0, 0},
	keyF11:        {gfxinput.KeyF11, 0, 0},
	keyF12:        {gfxinput.KeyF12, 0, 0},
	keyNumLock:    {gfxinput.KeyNumLock, 0, 0},
	keyScrollLock: {gfxinput.KeyScrollLock, 0, 0},
	keyRightCtrl:  {gfxinput.KeyRightCtrl, 0, 0},
	keyRightAlt:   {gfxinput.KeyRightAlt, 0, 0},
	keyHome:       {gfxinput.KeyHome, 0, 0},
	keyUp:         {gfxinput.KeyUp, 0, 0},
	keyPageUp:     {gfxinput.KeyPageUp, 0, 0},
	keyLeft:       {gfxinput.KeyLeft, 0, 0},
	keyRight:      {gfxinput.KeyRight, 0, 0},
	keyEnd:        {gfxinput.KeyEnd, 0, 0},
	keyDown:       {gfxinput.KeyDown, 0, 0},
	keyPageDown:   {gfxinput.KeyPageDown, 0, 0},
	keyInsert:     {gfxinput.KeyInsert, 0, 0},
	keyDelete:     {gfxinput.KeyDelete, 0, 0},
}

// UK layout - same as US but with some differences
var ukLayout = func() map[uint16]keyMapping {
	layout := make(map[uint16]keyMapping)
	for k, v := range usLayout {
		layout[k] = v
	}
	// UK-specific differences
	layout[key2] = keyMapping{gfxinput.KeyNone, '2', '"'}
	layout[key3] = keyMapping{gfxinput.KeyNone, '3', '£'}
	layout[keyApostrophe] = keyMapping{gfxinput.KeyNone, '\'', '@'}
	layout[keyGrave] = keyMapping{gfxinput.KeyNone, '`', '¬'}
	return layout
}()

// German layout
var deLayout = func() map[uint16]keyMapping {
	layout := make(map[uint16]keyMapping)
	for k, v := range usLayout {
		layout[k] = v
	}
	// German-specific differences
	layout[keyY] = keyMapping{gfxinput.KeyNone, 'z', 'Z'}
	layout[keyZ] = keyMapping{gfxinput.KeyNone, 'y', 'Y'}
	layout[keyMinus] = keyMapping{gfxinput.KeyNone, 'ß', '?'}
	layout[keyEqual] = keyMapping{gfxinput.KeyNone, '´', '`'}
	layout[keyLeftBrace] = keyMapping{gfxinput.KeyNone, 'ü', 'Ü'}
	layout[keyRightBrace] = keyMapping{gfxinput.KeyNone, '+', '*'}
	layout[keySemicolon] = keyMapping{gfxinput.KeyNone, 'ö', 'Ö'}
	layout[keyApostrophe] = keyMapping{gfxinput.KeyNone, 'ä', 'Ä'}
	layout[keyGrave] = keyMapping{gfxinput.KeyNone, '^', '°'}
	layout[keyBackslash] = keyMapping{gfxinput.KeyNone, '#', '\''}
	layout[keySlash] = keyMapping{gfxinput.KeyNone, '-', '_'}
	return layout
}()
