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

package graphics

import (
	"strings"
	"sync"
)

const (
	UIFontParagraph  = "paragraph"
	UIFontSubheading = "subheading"
	UIFontHeading    = "heading"
	UIFontTitle      = "title"
)

var (
	uiFontsMu sync.RWMutex
	uiFonts   = map[string]*Font{
		UIFontParagraph:  DefaultFont,
		UIFontSubheading: DefaultFont,
		UIFontHeading:    DefaultFont,
		UIFontTitle:      DefaultFont,
	}
)

// SetUIFont sets a font for a logical UI text role.
func SetUIFont(role string, font *Font) {
	role = normalizeUIFontRole(role)
	if role == "" || font == nil {
		return
	}
	uiFontsMu.Lock()
	uiFonts[role] = font
	uiFontsMu.Unlock()
}

// SetUIFonts sets multiple UI role fonts at once.
func SetUIFonts(fonts map[string]*Font) {
	uiFontsMu.Lock()
	defer uiFontsMu.Unlock()
	for role, font := range fonts {
		nr := normalizeUIFontRole(role)
		if nr == "" || font == nil {
			continue
		}
		uiFonts[nr] = font
	}
}

// UIFont returns the font for a UI role, falling back to the default font.
func UIFont(role string) *Font {
	role = normalizeUIFontRole(role)
	uiFontsMu.RLock()
	defer uiFontsMu.RUnlock()
	if role != "" {
		if f := uiFonts[role]; f != nil {
			return f
		}
	}
	if f := uiFonts[UIFontParagraph]; f != nil {
		return f
	}
	if DefaultFont != nil {
		return DefaultFont
	}
	return nil
}

func normalizeUIFontRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", "paragraph", "parag", "body", "text":
		return UIFontParagraph
	case "subheading", "sub-head", "subtitle":
		return UIFontSubheading
	case "heading", "header", "h2":
		return UIFontHeading
	case "title", "h1":
		return UIFontTitle
	default:
		return ""
	}
}
