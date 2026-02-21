package ui

import (
	"reflect"
	"strings"

	displayapi "avyos.dev/api/display"
	"avyos.dev/pkg/graphics"
	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
)

// RegisterShortcut registers a compositor shortcut callback.
// For rune-based shortcuts use key=graphics.KeyNone and set ch.
func (a *App) RegisterShortcut(shortcutID, windowID, scope uint32, key graphics.Key, ch rune, modifiers graphics.Modifiers, handler func(graphics.Event)) error {
	if a.app != nil {
		return a.app.RegisterShortcut(shortcutID, windowID, scope, key, ch, modifiers, handler)
	}
	a.Configure(func(core *gapp.App) {
		_ = core.RegisterShortcut(shortcutID, windowID, scope, key, ch, modifiers, handler)
	})
	return nil
}

// RegisterGlobalShortcut registers a global shortcut callback.
func (a *App) RegisterGlobalShortcut(shortcutID uint32, key graphics.Key, ch rune, modifiers graphics.Modifiers, handler func(graphics.Event)) error {
	return a.RegisterShortcut(shortcutID, 0, displayapi.ShortcutScopeGlobal, key, ch, modifiers, handler)
}

// RegisterClientShortcut registers a shortcut scoped to this client.
// Pass windowID=0 to match any window from this client.
func (a *App) RegisterClientShortcut(shortcutID, windowID uint32, key graphics.Key, ch rune, modifiers graphics.Modifiers, handler func(graphics.Event)) error {
	return a.RegisterShortcut(shortcutID, windowID, displayapi.ShortcutScopeClient, key, ch, modifiers, handler)
}

// UnregisterShortcut removes a previously registered shortcut callback.
func (a *App) UnregisterShortcut(shortcutID uint32) error {
	if a.app != nil {
		return a.app.UnregisterShortcut(shortcutID)
	}
	a.Configure(func(core *gapp.App) {
		_ = core.UnregisterShortcut(shortcutID)
	})
	return nil
}

// OpenMenu opens a popup menu at window-local coordinates (x, y) using
// the menu definition found by menuID in the current UI tree.
func (a *App) OpenMenu(menuID string, x, y int) bool {
	if a.rootElement == nil || strings.TrimSpace(menuID) == "" {
		return false
	}
	db := a.displayBackend()
	if db == nil {
		return false
	}
	menu := FindElement(a.rootElement, menuID)
	if menu == nil {
		return false
	}

	content := cloneElementTree(menu)
	if content == nil {
		return false
	}
	content.SetVisible(true)

	w, h := measureMenuPopup(menu, content)
	screenW, screenH := db.Size()
	if w > screenW {
		w = screenW
	}
	if h > screenH {
		h = screenH
	}
	if x+w > screenW {
		x = screenW - w
	}
	if y+h > screenH {
		y = screenH - h
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	a.CloseMenu()
	var popup *displaybackend.Popup
	closeFn := func() { a.CloseMenu() }
	wrapMenuActions(content, closeFn)

	popup = db.OpenPopup(x, y, w, h, content, func() {
		if a.menuPopup == popup {
			a.menuPopup = nil
		}
	})
	if popup == nil {
		return false
	}
	a.menuPopup = popup
	return true
}

// CloseMenu closes the currently open popup menu, if any.
func (a *App) CloseMenu() {
	p := a.menuPopup
	a.menuPopup = nil
	if p == nil {
		return
	}
	if db := a.displayBackend(); db != nil {
		db.ClosePopup(p)
	}
}

func (a *App) displayBackend() *displaybackend.Backend {
	if a.app != nil {
		if b, ok := a.app.Backend().(*displaybackend.Backend); ok {
			return b
		}
	}
	if b, ok := a.opts.Backend.(*displaybackend.Backend); ok {
		return b
	}
	return nil
}

func measureMenuPopup(src, cloned *Element) (int, int) {
	w := src.AttrInt("menuWidth", 0)
	if w <= 0 {
		w = src.AttrInt("minWidth", 0)
	}
	if w <= 0 {
		w = 220
	}
	if maxW := src.AttrInt("maxWidth", 0); maxW > 0 && w > maxW {
		w = maxW
	}

	h := src.AttrInt("menuHeight", 0)
	if h > 0 {
		return w, h
	}

	top, _, bottom, _ := parsePaddingStr(src.Attr("padding", "8"))
	spacing := src.AttrInt("spacing", 4)
	if spacing < 0 {
		spacing = 0
	}

	items := 0
	h = top + bottom
	for _, child := range cloned.children {
		if !child.visible {
			continue
		}
		rowH := child.AttrInt("minHeight", 34)
		if rowH <= 0 {
			rowH = 34
		}
		h += rowH
		items++
	}
	if items > 1 {
		h += spacing * (items - 1)
	}
	if h < 28 {
		h = 28
	}
	return w, h
}

func cloneElementTree(src *Element) *Element {
	if src == nil {
		return nil
	}
	dst := NewElement(src.tag)
	dst.id = src.id
	dst.visible = src.visible
	dst.minW = src.minW
	dst.minH = src.minH

	for k, v := range src.attrs {
		dst.attrs[k] = v
	}
	for k, fn := range src.signals {
		dst.signals[k] = fn
	}

	for _, child := range src.children {
		clone := cloneElementTree(child)
		if clone != nil {
			dst.AddChild(clone)
		}
	}
	return dst
}

func wrapMenuActions(root *Element, closeFn func()) {
	if root == nil {
		return
	}
	for _, child := range root.children {
		wrapMenuActions(child, closeFn)
	}
	if root.tag != "MenuItem" {
		return
	}
	orig, hasOrig := root.signals["clicked"]
	root.signals["clicked"] = reflect.ValueOf(func() {
		if hasOrig {
			callSignalNoArgs(orig)
		}
		if closeFn != nil {
			closeFn()
		}
	})
}

func callSignalNoArgs(fn reflect.Value) {
	if !fn.IsValid() {
		return
	}
	ft := fn.Type()
	if ft.Kind() != reflect.Func || ft.NumIn() != 0 {
		return
	}
	fn.Call(nil)
}
