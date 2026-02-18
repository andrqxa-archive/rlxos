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

import "avyos.dev/pkg/graphics"

// KeyboardLayout represents a keyboard layout.
type KeyboardLayout string

const (
	LayoutUS KeyboardLayout = "us"
	LayoutUK KeyboardLayout = "uk"
	LayoutDE KeyboardLayout = "de"
)

// keyMapping maps evdev keycodes to Key constants and runes.
type keyMapping struct {
	key     graphics.Key
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
	keyEsc:        {graphics.KeyEscape, 0, 0},
	key1:          {graphics.KeyNone, '1', '!'},
	key2:          {graphics.KeyNone, '2', '@'},
	key3:          {graphics.KeyNone, '3', '#'},
	key4:          {graphics.KeyNone, '4', '$'},
	key5:          {graphics.KeyNone, '5', '%'},
	key6:          {graphics.KeyNone, '6', '^'},
	key7:          {graphics.KeyNone, '7', '&'},
	key8:          {graphics.KeyNone, '8', '*'},
	key9:          {graphics.KeyNone, '9', '('},
	key0:          {graphics.KeyNone, '0', ')'},
	keyMinus:      {graphics.KeyNone, '-', '_'},
	keyEqual:      {graphics.KeyNone, '=', '+'},
	keyBackspace:  {graphics.KeyBackspace, 0, 0},
	keyTab:        {graphics.KeyTab, '\t', '\t'},
	keyQ:          {graphics.KeyNone, 'q', 'Q'},
	keyW:          {graphics.KeyNone, 'w', 'W'},
	keyE:          {graphics.KeyNone, 'e', 'E'},
	keyR:          {graphics.KeyNone, 'r', 'R'},
	keyT:          {graphics.KeyNone, 't', 'T'},
	keyY:          {graphics.KeyNone, 'y', 'Y'},
	keyU:          {graphics.KeyNone, 'u', 'U'},
	keyI:          {graphics.KeyNone, 'i', 'I'},
	keyO:          {graphics.KeyNone, 'o', 'O'},
	keyP:          {graphics.KeyNone, 'p', 'P'},
	keyLeftBrace:  {graphics.KeyNone, '[', '{'},
	keyRightBrace: {graphics.KeyNone, ']', '}'},
	keyEnter:      {graphics.KeyEnter, '\n', '\n'},
	keyLeftCtrl:   {graphics.KeyLeftCtrl, 0, 0},
	keyA:          {graphics.KeyNone, 'a', 'A'},
	keyS:          {graphics.KeyNone, 's', 'S'},
	keyD:          {graphics.KeyNone, 'd', 'D'},
	keyF:          {graphics.KeyNone, 'f', 'F'},
	keyG:          {graphics.KeyNone, 'g', 'G'},
	keyH:          {graphics.KeyNone, 'h', 'H'},
	keyJ:          {graphics.KeyNone, 'j', 'J'},
	keyK:          {graphics.KeyNone, 'k', 'K'},
	keyL:          {graphics.KeyNone, 'l', 'L'},
	keySemicolon:  {graphics.KeyNone, ';', ':'},
	keyApostrophe: {graphics.KeyNone, '\'', '"'},
	keyGrave:      {graphics.KeyNone, '`', '~'},
	keyLeftShift:  {graphics.KeyLeftShift, 0, 0},
	keyBackslash:  {graphics.KeyNone, '\\', '|'},
	keyZ:          {graphics.KeyNone, 'z', 'Z'},
	keyX:          {graphics.KeyNone, 'x', 'X'},
	keyC:          {graphics.KeyNone, 'c', 'C'},
	keyV:          {graphics.KeyNone, 'v', 'V'},
	keyB:          {graphics.KeyNone, 'b', 'B'},
	keyN:          {graphics.KeyNone, 'n', 'N'},
	keyM:          {graphics.KeyNone, 'm', 'M'},
	keyComma:      {graphics.KeyNone, ',', '<'},
	keyDot:        {graphics.KeyNone, '.', '>'},
	keySlash:      {graphics.KeyNone, '/', '?'},
	keyRightShift: {graphics.KeyRightShift, 0, 0},
	keyLeftAlt:    {graphics.KeyLeftAlt, 0, 0},
	keySpace:      {graphics.KeySpace, ' ', ' '},
	keyCapsLock:   {graphics.KeyCapsLock, 0, 0},
	keyF1:         {graphics.KeyF1, 0, 0},
	keyF2:         {graphics.KeyF2, 0, 0},
	keyF3:         {graphics.KeyF3, 0, 0},
	keyF4:         {graphics.KeyF4, 0, 0},
	keyF5:         {graphics.KeyF5, 0, 0},
	keyF6:         {graphics.KeyF6, 0, 0},
	keyF7:         {graphics.KeyF7, 0, 0},
	keyF8:         {graphics.KeyF8, 0, 0},
	keyF9:         {graphics.KeyF9, 0, 0},
	keyF10:        {graphics.KeyF10, 0, 0},
	keyF11:        {graphics.KeyF11, 0, 0},
	keyF12:        {graphics.KeyF12, 0, 0},
	keyNumLock:    {graphics.KeyNumLock, 0, 0},
	keyScrollLock: {graphics.KeyScrollLock, 0, 0},
	keyRightCtrl:  {graphics.KeyRightCtrl, 0, 0},
	keyRightAlt:   {graphics.KeyRightAlt, 0, 0},
	keyHome:       {graphics.KeyHome, 0, 0},
	keyUp:         {graphics.KeyUp, 0, 0},
	keyPageUp:     {graphics.KeyPageUp, 0, 0},
	keyLeft:       {graphics.KeyLeft, 0, 0},
	keyRight:      {graphics.KeyRight, 0, 0},
	keyEnd:        {graphics.KeyEnd, 0, 0},
	keyDown:       {graphics.KeyDown, 0, 0},
	keyPageDown:   {graphics.KeyPageDown, 0, 0},
	keyInsert:     {graphics.KeyInsert, 0, 0},
	keyDelete:     {graphics.KeyDelete, 0, 0},
}

// UK layout - same as US but with some differences
var ukLayout = func() map[uint16]keyMapping {
	layout := make(map[uint16]keyMapping)
	for k, v := range usLayout {
		layout[k] = v
	}
	// UK-specific differences
	layout[key2] = keyMapping{graphics.KeyNone, '2', '"'}
	layout[key3] = keyMapping{graphics.KeyNone, '3', '£'}
	layout[keyApostrophe] = keyMapping{graphics.KeyNone, '\'', '@'}
	layout[keyGrave] = keyMapping{graphics.KeyNone, '`', '¬'}
	return layout
}()

// German layout
var deLayout = func() map[uint16]keyMapping {
	layout := make(map[uint16]keyMapping)
	for k, v := range usLayout {
		layout[k] = v
	}
	// German-specific differences
	layout[keyY] = keyMapping{graphics.KeyNone, 'z', 'Z'}
	layout[keyZ] = keyMapping{graphics.KeyNone, 'y', 'Y'}
	layout[keyMinus] = keyMapping{graphics.KeyNone, 'ß', '?'}
	layout[keyEqual] = keyMapping{graphics.KeyNone, '´', '`'}
	layout[keyLeftBrace] = keyMapping{graphics.KeyNone, 'ü', 'Ü'}
	layout[keyRightBrace] = keyMapping{graphics.KeyNone, '+', '*'}
	layout[keySemicolon] = keyMapping{graphics.KeyNone, 'ö', 'Ö'}
	layout[keyApostrophe] = keyMapping{graphics.KeyNone, 'ä', 'Ä'}
	layout[keyGrave] = keyMapping{graphics.KeyNone, '^', '°'}
	layout[keyBackslash] = keyMapping{graphics.KeyNone, '#', '\''}
	layout[keySlash] = keyMapping{graphics.KeyNone, '-', '_'}
	return layout
}()
