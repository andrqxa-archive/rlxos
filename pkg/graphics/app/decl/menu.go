package declapp

import (
	"strings"

	displayapi "avyos.dev/api/display"
	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	graphics "avyos.dev/pkg/graphics/input"
	uidecl "avyos.dev/pkg/graphics/widget/decl"
	ui "avyos.dev/pkg/graphics/widget/engine"
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

	a.CloseMenu()

	var popup *displaybackend.Popup
	closeFn := func() { a.CloseMenu() }

	menu := uidecl.FindElement(a.rootElement, menuID)
	content, w, h, ok := ui.BuildMenuPopup(menu, closeFn)
	if !ok {
		return false
	}

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
