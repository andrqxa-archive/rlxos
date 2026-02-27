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

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	graphics "avyos.dev/pkg/graphics/input"
)

// minimalXKBKeymap is a minimal but valid XKB keymap that maps evdev keycodes
// to basic keysyms. Wayland requires sending this to clients via wl_keyboard.keymap.
// Most clients use libxkbcommon to parse this.
const minimalXKBKeymap = `xkb_keymap {
  xkb_keycodes "minimal" {
    minimum = 8;
    maximum = 255;
    <ESC>  = 9;
    <AE01> = 10;  <AE02> = 11;  <AE03> = 12;  <AE04> = 13;
    <AE05> = 14;  <AE06> = 15;  <AE07> = 16;  <AE08> = 17;
    <AE09> = 18;  <AE10> = 19;  <AE11> = 20;  <AE12> = 21;
    <BKSP> = 22;  <TAB>  = 23;
    <AD01> = 24;  <AD02> = 25;  <AD03> = 26;  <AD04> = 27;
    <AD05> = 28;  <AD06> = 29;  <AD07> = 30;  <AD08> = 31;
    <AD09> = 32;  <AD10> = 33;  <AD11> = 34;  <AD12> = 35;
    <RTRN> = 36;  <LCTL> = 37;
    <AC01> = 38;  <AC02> = 39;  <AC03> = 40;  <AC04> = 41;
    <AC05> = 42;  <AC06> = 43;  <AC07> = 44;  <AC08> = 45;
    <AC09> = 46;  <AC10> = 47;  <AC11> = 48;  <TLDE> = 49;
    <LFSH> = 50;  <BKSL> = 51;
    <AB01> = 52;  <AB02> = 53;  <AB03> = 54;  <AB04> = 55;
    <AB05> = 56;  <AB06> = 57;  <AB07> = 58;  <AB08> = 59;
    <AB09> = 60;  <AB10> = 61;  <RTSH> = 62;
    <LALT> = 64;  <SPCE> = 65;  <CAPS> = 66;
    <FK01> = 67;  <FK02> = 68;  <FK03> = 69;  <FK04> = 70;
    <FK05> = 71;  <FK06> = 72;  <FK07> = 73;  <FK08> = 74;
    <FK09> = 75;  <FK10> = 76;
    <FK11> = 95;  <FK12> = 96;
    <RCTL> = 105; <RALT> = 108;
    <HOME> = 110; <UP>   = 111; <PGUP> = 112;
    <LEFT> = 113; <RGHT> = 114; <END>  = 115;
    <DOWN> = 116; <PGDN> = 117;
    <INS>  = 118; <DELE> = 119;
  };

  xkb_types "minimal" {
    virtual_modifiers NumLock;
    type "ONE_LEVEL" {
      modifiers= none;
      level_name[Level1]= "Any";
    };
    type "TWO_LEVEL" {
      modifiers= Shift;
      map[Shift]= Level2;
      level_name[Level1]= "Base";
      level_name[Level2]= "Shift";
    };
    type "ALPHABETIC" {
      modifiers= Shift+Lock;
      map[Shift]= Level2;
      map[Lock]= Level2;
      level_name[Level1]= "Base";
      level_name[Level2]= "Caps";
    };
  };

  xkb_compatibility "minimal" {
    interpret Any+AnyOf(all) { action= SetMods(modifiers=modMapMods,clearLocks); };
    interpret Shift_L+AnyOf(all) { action= SetMods(modifiers=Shift,clearLocks); };
    interpret Shift_R+AnyOf(all) { action= SetMods(modifiers=Shift,clearLocks); };
    interpret Control_L+AnyOf(all) { action= SetMods(modifiers=Control,clearLocks); };
    interpret Control_R+AnyOf(all) { action= SetMods(modifiers=Control,clearLocks); };
    interpret Alt_L+AnyOf(all) { action= SetMods(modifiers=Mod1,clearLocks); };
    interpret Alt_R+AnyOf(all) { action= SetMods(modifiers=Mod1,clearLocks); };
    interpret Caps_Lock+AnyOf(all) { action= LockMods(modifiers=Lock); };
  };

  xkb_symbols "minimal" {
    name[group1]="English (US)";
    key <ESC>  { [ Escape ] };
    key <AE01> { [ 1, exclam ] };
    key <AE02> { [ 2, at ] };
    key <AE03> { [ 3, numbersign ] };
    key <AE04> { [ 4, dollar ] };
    key <AE05> { [ 5, percent ] };
    key <AE06> { [ 6, asciicircum ] };
    key <AE07> { [ 7, ampersand ] };
    key <AE08> { [ 8, asterisk ] };
    key <AE09> { [ 9, parenleft ] };
    key <AE10> { [ 0, parenright ] };
    key <AE11> { [ minus, underscore ] };
    key <AE12> { [ equal, plus ] };
    key <BKSP> { [ BackSpace ] };
    key <TAB>  { [ Tab ] };
    key <AD01> { [ q, Q ] };
    key <AD02> { [ w, W ] };
    key <AD03> { [ e, E ] };
    key <AD04> { [ r, R ] };
    key <AD05> { [ t, T ] };
    key <AD06> { [ y, Y ] };
    key <AD07> { [ u, U ] };
    key <AD08> { [ i, I ] };
    key <AD09> { [ o, O ] };
    key <AD10> { [ p, P ] };
    key <AD11> { [ bracketleft, braceleft ] };
    key <AD12> { [ bracketright, braceright ] };
    key <RTRN> { [ Return ] };
    key <LCTL> { [ Control_L ] };
    key <AC01> { [ a, A ] };
    key <AC02> { [ s, S ] };
    key <AC03> { [ d, D ] };
    key <AC04> { [ f, F ] };
    key <AC05> { [ g, G ] };
    key <AC06> { [ h, H ] };
    key <AC07> { [ j, J ] };
    key <AC08> { [ k, K ] };
    key <AC09> { [ l, L ] };
    key <AC10> { [ semicolon, colon ] };
    key <AC11> { [ apostrophe, quotedbl ] };
    key <TLDE> { [ grave, asciitilde ] };
    key <LFSH> { [ Shift_L ] };
    key <BKSL> { [ backslash, bar ] };
    key <AB01> { [ z, Z ] };
    key <AB02> { [ x, X ] };
    key <AB03> { [ c, C ] };
    key <AB04> { [ v, V ] };
    key <AB05> { [ b, B ] };
    key <AB06> { [ n, N ] };
    key <AB07> { [ m, M ] };
    key <AB08> { [ comma, less ] };
    key <AB09> { [ period, greater ] };
    key <AB10> { [ slash, question ] };
    key <RTSH> { [ Shift_R ] };
    key <LALT> { [ Alt_L ] };
    key <SPCE> { [ space ] };
    key <CAPS> { [ Caps_Lock ] };
    key <FK01> { [ F1 ] };
    key <FK02> { [ F2 ] };
    key <FK03> { [ F3 ] };
    key <FK04> { [ F4 ] };
    key <FK05> { [ F5 ] };
    key <FK06> { [ F6 ] };
    key <FK07> { [ F7 ] };
    key <FK08> { [ F8 ] };
    key <FK09> { [ F9 ] };
    key <FK10> { [ F10 ] };
    key <FK11> { [ F11 ] };
    key <FK12> { [ F12 ] };
    key <RCTL> { [ Control_R ] };
    key <RALT> { [ Alt_R ] };
    key <HOME> { [ Home ] };
    key <UP>   { [ Up ] };
    key <PGUP> { [ Prior ] };
    key <LEFT> { [ Left ] };
    key <RGHT> { [ Right ] };
    key <END>  { [ End ] };
    key <DOWN> { [ Down ] };
    key <PGDN> { [ Next ] };
    key <INS>  { [ Insert ] };
    key <DELE> { [ Delete ] };
    modifier_map Shift { <LFSH>, <RTSH> };
    modifier_map Lock { <CAPS> };
    modifier_map Control { <LCTL>, <RCTL> };
    modifier_map Mod1 { <LALT>, <RALT> };
  };
};
`

// memfdCreate creates an anonymous file via the memfd_create syscall.
func memfdCreate(name string) (int, error) {
	nameBytes := append([]byte(name), 0)

	var sysNum uintptr
	switch runtime.GOARCH {
	case "amd64":
		sysNum = 319
	case "arm64":
		sysNum = 279
	case "386":
		sysNum = 356
	default:
		sysNum = 319
	}

	fd, _, errno := syscall.Syscall(
		sysNum,
		uintptr(unsafe.Pointer(&nameBytes[0])),
		1, // MFD_CLOEXEC
		0,
	)
	if errno != 0 {
		return -1, fmt.Errorf("memfd_create: %w", errno)
	}
	return int(fd), nil
}

// createKeymapFD creates a memfd containing the XKB keymap, returns (fd, size).
func createKeymapFD() (int, int, error) {
	fd, err := memfdCreate("wl-keymap")
	if err != nil {
		return -1, 0, err
	}

	data := []byte(minimalXKBKeymap)
	// XKB keymap must be NUL-terminated
	data = append(data, 0)

	if err := syscall.Ftruncate(fd, int64(len(data))); err != nil {
		syscall.Close(fd)
		return -1, 0, err
	}

	mapped, err := syscall.Mmap(fd, 0, len(data), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return -1, 0, err
	}
	copy(mapped, data)
	syscall.Munmap(mapped)

	return fd, len(data), nil
}

// keyToEvdev converts a graphics.Key to an evdev keycode.
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

func keyToEvdev(key graphics.Key) int {
	if code, ok := keyToEvdevMap[key]; ok {
		return code
	}
	return 0
}
