package declapp

import (
	"errors"

	gapp "avyos.dev/pkg/graphics/app"
	displaybackend "avyos.dev/pkg/graphics/backend/display"
	graphics "avyos.dev/pkg/graphics/input"
	uidecl "avyos.dev/pkg/graphics/widget/decl"
	ui "avyos.dev/pkg/graphics/widget/engine"
)

// App is a declarative UI lifecycle that runs engine.Element trees on app.App.
type App struct {
	rootElement *ui.Element
	app         *gapp.App
	menuPopup   *displaybackend.Popup
	opts        gapp.Options
	configure   []func(*gapp.App)
	focusables  []gapp.Widget
	focusTarget gapp.Widget
}

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

// Root returns the root element.
func (a *App) Root() *ui.Element {
	return a.rootElement
}

// FindElement searches the element tree by id.
func (a *App) FindElement(id string) *ui.Element {
	return uidecl.FindElement(a.rootElement, id)
}

// SetOptions sets app.App options used when Run starts the app.
func (a *App) SetOptions(opts gapp.Options) {
	a.opts = opts
}

// Configure registers a callback invoked after app construction and before Run.
func (a *App) Configure(fn func(*gapp.App)) {
	if fn != nil {
		a.configure = append(a.configure, fn)
	}
}

// AddFocusable registers focusable widgets.
func (a *App) AddFocusable(widgets ...gapp.Widget) {
	if a.app != nil {
		for _, w := range widgets {
			a.app.AddFocusable(w)
		}
		return
	}
	a.focusables = append(a.focusables, widgets...)
}

// Focus sets initial or current focus.
func (a *App) Focus(widget gapp.Widget) {
	if a.app != nil {
		a.app.Focus(widget)
		return
	}
	a.focusTarget = widget
}

// Run starts the application main loop.
func (a *App) Run() error {
	if a.rootElement == nil {
		return errors.New("no UI loaded; call LoadFile or LoadString first")
	}

	opts := a.opts
	if opts.Title == "" {
		opts.Title = a.rootElement.Attr("title", "UI App")
	}
	if !opts.BackgroundSet && opts.Background == (graphics.Color{}) {
		opts.Background = graphics.ColorTransparent
		opts.BackgroundSet = true
	}

	a.app = gapp.New(opts)
	a.app.SetRoot(a.rootElement)

	if a.rootElement != nil {
		for _, f := range uidecl.CollectFocusable(a.rootElement) {
			a.app.AddFocusable(f)
		}
	}

	for _, f := range a.focusables {
		a.app.AddFocusable(f)
	}

	if a.focusTarget != nil {
		a.app.Focus(a.focusTarget)
	}
	for _, fn := range a.configure {
		fn(a.app)
	}

	return a.app.Run()
}

// Quit stops the application.
func (a *App) Quit() {
	if a.app != nil {
		a.app.Quit()
	}
}

// Redraw forces a full screen redraw.
func (a *App) Redraw() {
	if a.app != nil {
		a.app.Redraw()
	}
}

// RequestFrame schedules a render pass using incremental damage when possible.
func (a *App) RequestFrame() {
	if a.app != nil {
		a.app.RequestFrame()
	}
}

// InternalApp returns the underlying app.App for advanced use.
func (a *App) InternalApp() *gapp.App {
	return a.app
}
