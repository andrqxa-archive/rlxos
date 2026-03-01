package app

import (
	"strings"

	displayapi "avyos.dev/api/display"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	gfxinput "avyos.dev/pkg/graphics/input"
	core "avyos.dev/pkg/graphics/pixmap"
	uidecl "avyos.dev/pkg/graphics/widget/decl"
	ui "avyos.dev/pkg/graphics/widget/engine"
)

// LoadFile loads a .ui file and builds the element tree.
func (a *App) LoadFile(path string, handler interface{}) error {
	root, err := uidecl.LoadFile(path, handler)
	if err != nil {
		return err
	}
	a.rootElement = root
	return nil
}

// LoadString loads UI from a string source.
func (a *App) LoadString(source string, handler interface{}) error {
	root, err := uidecl.LoadString(source, handler)
	if err != nil {
		return err
	}
	a.rootElement = root
	return nil
}

// UIRoot returns the declarative root element, if loaded.
func (a *App) UIRoot() *ui.Element {
	root, _ := a.rootElement.(*ui.Element)
	return root
}

// FindElement searches the loaded declarative tree by id.
func (a *App) FindElement(id string) *ui.Element {
	return uidecl.FindElement(a.UIRoot(), id)
}

// SetOptions configures app startup options on this instance.
func (a *App) SetOptions(opts Options) {
	base := New(opts)

	// Preserve declarative state and callbacks across option updates.
	rootElement := a.rootElement
	configure := a.configure
	focusables := a.focusables
	focusTarget := a.focusTarget
	menuPopup := a.menuPopup

	*a = *base
	a.rootElement = rootElement
	a.configure = configure
	a.focusables = focusables
	a.focusTarget = focusTarget
	a.menuPopup = menuPopup
}

// Configure registers a callback invoked once before Run starts.
func (a *App) Configure(fn func(*App)) {
	if fn != nil {
		a.configure = append(a.configure, fn)
	}
}

// RegisterGlobalShortcut registers a global shortcut callback.
func (a *App) RegisterGlobalShortcut(shortcutID uint32, key gfxinput.Key, ch rune, modifiers gfxinput.Modifiers, handler func(gfxinput.Event)) error {
	return a.RegisterShortcut(shortcutID, 0, displayapi.ShortcutScopeGlobal, key, ch, modifiers, handler)
}

// RegisterClientShortcut registers a shortcut scoped to this client.
// Pass windowID=0 to match any window from this client.
func (a *App) RegisterClientShortcut(shortcutID, windowID uint32, key gfxinput.Key, ch rune, modifiers gfxinput.Modifiers, handler func(gfxinput.Event)) error {
	return a.RegisterShortcut(shortcutID, windowID, displayapi.ShortcutScopeClient, key, ch, modifiers, handler)
}

// OpenMenu opens a popup menu at window-local coordinates (x, y) using
// the menu definition found by menuID in the current UI tree.
func (a *App) OpenMenu(menuID string, x, y int) bool {
	root := a.UIRoot()
	if root == nil || strings.TrimSpace(menuID) == "" {
		return false
	}

	db := a.displayBackend()
	if db == nil {
		return false
	}

	a.CloseMenu()

	var popup *displaybackend.Popup
	closeFn := func() { a.CloseMenu() }

	menu := uidecl.FindElement(root, menuID)
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
	if b, ok := a.backend.(*displaybackend.Backend); ok {
		return b
	}
	return nil
}

// prepareDeclarativeRoot converts loaded declarative trees into runtime root state.
func (a *App) prepareDeclarativeRoot() {
	root := a.UIRoot()
	if root == nil || !isNilWidget(a.root) {
		return
	}

	if a.title == "" {
		a.title = root.Attr("title", "UI App")
	}
	if !a.backgroundSet && a.backgroundAuto {
		a.background = core.ColorTransparent
		a.backgroundSet = true
	}

	a.SetRoot(root)
	for _, f := range uidecl.CollectFocusable(root) {
		a.AddFocusable(f)
	}
}
