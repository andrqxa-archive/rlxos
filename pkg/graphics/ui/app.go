package ui

import (
	"os"

	"avyos.dev/pkg/graphics"
	gapp "avyos.dev/pkg/graphics/app"
)

// App is the base struct for UI applications. Embed this in your app struct
// and implement handler methods that get bound to UI signals.
//
// Usage:
//
//	type MyApp struct {
//	    ui.App
//	}
//
//	func (a *MyApp) HandleClick() { ... }
//
//	func main() {
//	    app := &MyApp{}
//	    app.LoadFile("ui/demo.ui")
//	    app.Run()
//	}
type App struct {
	engine      *Engine
	rootElement *Element
	app         *gapp.App
	opts        gapp.Options
	configure   []func(*gapp.App)
	focusables  []graphics.Widget
	focusTarget graphics.Widget
}

// LoadFile loads a .ui file and builds the element tree.
// The receiver (typically the embedding struct) is used as the signal handler.
func (a *App) LoadFile(path string, handler interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return &BuildError{Message: "failed to read UI file: " + err.Error()}
	}
	return a.LoadString(string(data), handler)
}

// LoadString loads UI from a string source.
func (a *App) LoadString(source string, handler interface{}) error {
	eng, err := NewEngine()
	if err != nil {
		return err
	}
	a.engine = eng

	root, err := eng.LoadString(source, handler)
	if err != nil {
		return err
	}
	a.rootElement = root
	return nil
}

// Root returns the root element.
func (a *App) Root() *Element {
	return a.rootElement
}

// FindElement searches the element tree by id.
func (a *App) FindElement(id string) *Element {
	return FindElement(a.rootElement, id)
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
func (a *App) AddFocusable(widgets ...graphics.Widget) {
	if a.app != nil {
		a.app.AddFocusable(widgets...)
		return
	}
	a.focusables = append(a.focusables, widgets...)
}

// Focus sets initial or current focus.
func (a *App) Focus(widget graphics.Widget) {
	if a.app != nil {
		a.app.Focus(widget)
		return
	}
	a.focusTarget = widget
}

// Run starts the application main loop.
func (a *App) Run() error {
	if a.rootElement == nil {
		return &BuildError{Message: "no UI loaded; call LoadFile or LoadString first"}
	}

	opts := a.opts
	if opts.Title == "" {
		if a.rootElement != nil {
			opts.Title = a.rootElement.Attr("title", "UI App")
		} else {
			opts.Title = "UI App"
		}
	}
	if !opts.BackgroundSet && opts.Background == (graphics.Color{}) {
		opts.Background = graphics.ColorTransparent
		opts.BackgroundSet = true
	}

	a.app = gapp.New(opts)
	a.app.SetRoot(a.rootElement)

	// Register focusable elements from declarative UI elements.
	if a.rootElement != nil {
		for _, f := range CollectFocusable(a.rootElement) {
			a.app.AddFocusable(f)
		}
	}

	// Register explicit focusables.
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
