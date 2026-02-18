package ui

import (
	"reflect"
	"strings"
	"time"

	"avyos.dev/pkg/graphics"
)

// Element is the universal DOM-like node. Every UI element is an Element.
// It implements graphics.Widget and renders entirely from attributes.
// Based on its attributes, it can behave as any widget: button, text input,
// checkbox, slider, container, etc.
type Element struct {
	tag      string
	id       string
	children []*Element
	parent   *Element

	// Attributes drive all visual and behavioral properties
	attrs map[string]string

	// State managed by the engine
	hovered bool
	pressed bool
	// rightPressed tracks secondary-button press/release for context-menu signals.
	rightPressed bool
	focused      bool
	visible      bool
	dirty        bool
	dirtyRects   []graphics.Rect
	bounds       graphics.Rect
	minW         int
	minH         int

	// Internal mutable state (lazily initialized for editable/slidable elements)
	state interface{}

	// Signals: signal name -> reflected method
	signals map[string]reflect.Value
}

// NewElement creates a new Element with the given tag.
func NewElement(tag string) *Element {
	return &Element{
		tag:     tag,
		attrs:   make(map[string]string),
		signals: make(map[string]reflect.Value),
		visible: true,
		dirty:   true,
	}
}

// --- Attribute access ---

// SetAttribute sets an attribute value and marks dirty.
func (e *Element) SetAttribute(name string, value interface{}) {
	s := toString(value)
	if e.attrs[name] != s {
		e.attrs[name] = s
		if name == "text" {
			e.syncEditableTextState(s)
		}
		e.clearDirtyRects()
		e.dirty = true
	}
}

func (e *Element) syncEditableTextState(text string) {
	switch st := e.state.(type) {
	case *textInputState:
		st.text = []rune(text)
		if st.cursorPos < 0 {
			st.cursorPos = 0
		}
		if st.cursorPos > len(st.text) {
			st.cursorPos = len(st.text)
		}
		if st.scrollOffset < 0 {
			st.scrollOffset = 0
		}
		e.ensureTextInputVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()

	case *textAreaState:
		st.terminalLines = nil
		st.terminalCursorCol = 0
		st.terminalCursorRow = 0
		st.terminalShowCursor = false

		raw := strings.Split(text, "\n")
		if len(raw) == 0 {
			raw = []string{""}
		}
		st.lines = make([][]rune, len(raw))
		for i, line := range raw {
			st.lines[i] = []rune(line)
		}
		clampTextAreaCursor(st)

		visRows, visCols := e.textAreaVisibleRowsCols()
		if visRows > 0 {
			maxScrollRow := len(st.lines) - visRows
			if maxScrollRow < 0 {
				maxScrollRow = 0
			}
			if st.scrollRow > maxScrollRow {
				st.scrollRow = maxScrollRow
			}
		}
		if st.scrollRow < 0 {
			st.scrollRow = 0
		}

		if visCols > 0 {
			maxCols := 0
			for _, line := range st.lines {
				if len(line) > maxCols {
					maxCols = len(line)
				}
			}
			maxScrollCol := maxCols - visCols
			if maxScrollCol < 0 {
				maxScrollCol = 0
			}
			if st.scrollCol > maxScrollCol {
				st.scrollCol = maxScrollCol
			}
		}
		if st.scrollCol < 0 {
			st.scrollCol = 0
		}

		st.cursorBlink = true
		st.lastBlink = time.Now()
	}
}

// GetAttribute returns an attribute value.
func (e *Element) GetAttribute(name string) string {
	return e.attrs[name]
}

// Attr returns the string attribute or default.
func (e *Element) Attr(name, def string) string {
	if v, ok := e.attrs[name]; ok && v != "" {
		return v
	}
	return def
}

// AttrInt returns an integer attribute or default.
func (e *Element) AttrInt(name string, def int) int {
	if v, ok := e.attrs[name]; ok && v != "" {
		return toInt(v)
	}
	return def
}

// AttrFloat returns a float attribute or default.
func (e *Element) AttrFloat(name string, def float64) float64 {
	if v, ok := e.attrs[name]; ok && v != "" {
		return toFloat(v)
	}
	return def
}

// AttrBool returns a boolean attribute or default.
func (e *Element) AttrBool(name string, def bool) bool {
	if v, ok := e.attrs[name]; ok && v != "" {
		return toBool(v)
	}
	return def
}

// AttrColor returns a color attribute or default.
func (e *Element) AttrColor(name string, def graphics.Color) graphics.Color {
	if v, ok := e.attrs[name]; ok && v != "" {
		value := strings.TrimSpace(v)
		if strings.EqualFold(value, "transparent") {
			return graphics.ColorTransparent
		}
		if looksHexColorLiteral(value) {
			return parseColorStr(value)
		}
		c := parseColorStr(value)
		if c != (graphics.Color{}) {
			return c
		}
	}
	return def
}

func (e *Element) hasAttr(name string) bool {
	_, ok := e.attrs[name]
	return ok
}

func (e *Element) overflowMode() string {
	if !e.hasAttr("overflow") {
		if e.hasAttr("scrollable") && e.AttrBool("scrollable", false) {
			return "auto"
		}
		return "visible"
	}
	mode := strings.ToLower(strings.TrimSpace(e.Attr("overflow", "")))
	switch mode {
	case "visible":
		return "visible"
	case "hidden", "clip":
		return "hidden"
	case "scroll":
		return "scroll"
	case "auto":
		return "auto"
	default:
		return "visible"
	}
}

func (e *Element) overflowScrollable() bool {
	if len(e.children) == 0 {
		return false
	}
	if e.hasAttr("scrollable") {
		return e.AttrBool("scrollable", false)
	}
	mode := e.overflowMode()
	return mode == "auto" || mode == "scroll"
}

func (e *Element) overflowClipped() bool {
	return e.overflowMode() != "visible"
}

func looksHexColorLiteral(s string) bool {
	if !strings.HasPrefix(s, "#") {
		return false
	}
	hex := s[1:]
	switch len(hex) {
	case 3, 6, 8:
	default:
		return false
	}
	for _, ch := range hex {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return true
}

// Tag returns the element's tag name.
func (e *Element) Tag() string { return e.tag }

// ID returns the element's id.
func (e *Element) ID() string { return e.id }

// ChildElements returns child elements.
func (e *Element) ChildElements() []*Element { return e.children }

// AddChild adds a child element.
func (e *Element) AddChild(child *Element) {
	child.parent = e
	e.children = append(e.children, child)
	e.clearDirtyRects()
	e.dirty = true
	if e.bounds.W > 0 && e.bounds.H > 0 {
		e.layoutChildren()
	}
}

// ClearChildren removes all child elements.
func (e *Element) ClearChildren() {
	if len(e.children) == 0 {
		return
	}
	for _, child := range e.children {
		child.parent = nil
	}
	e.children = nil
	e.clearDirtyRects()
	e.dirty = true
	if e.bounds.W > 0 && e.bounds.H > 0 {
		e.layoutChildren()
	}
}

// SetID updates the element id.
func (e *Element) SetID(id string) {
	if e.id != id {
		e.id = id
		e.clearDirtyRects()
		e.dirty = true
	}
}

// BindSignal binds a runtime signal handler (for example "clicked").
func (e *Element) BindSignal(name string, fn interface{}) bool {
	if fn == nil {
		return false
	}
	v := reflect.ValueOf(fn)
	if !v.IsValid() || v.Kind() != reflect.Func {
		return false
	}
	e.signals[name] = v
	return true
}

// EmitSignal emits a signal by name with optional args.
func (e *Element) EmitSignal(name string, args ...interface{}) {
	fn, ok := e.signals[name]
	if !ok || !fn.IsValid() {
		return
	}
	ft := fn.Type()
	if ft.NumIn() == 0 || len(args) == 0 {
		fn.Call(nil)
		return
	}
	// Pass args that match
	callArgs := make([]reflect.Value, 0, len(args))
	for i, arg := range args {
		if i >= ft.NumIn() {
			break
		}
		callArgs = append(callArgs, reflect.ValueOf(arg))
	}
	fn.Call(callArgs)
}

// --- graphics.Widget interface ---

func (e *Element) Draw(buf *graphics.Buffer) {
	if !e.visible {
		return
	}
	drawElement(e, buf)
}

func (e *Element) Bounds() graphics.Rect {
	return e.bounds
}

func (e *Element) SetBounds(r graphics.Rect) {
	if e.bounds != r {
		e.bounds = r
		e.clearDirtyRects()
		e.dirty = true
		e.layoutChildren()
	}
}

func (e *Element) MinSize() graphics.Point {
	w := e.minW
	h := e.minH
	if mw := e.AttrInt("width", 0); mw > w {
		w = mw
	}
	if mh := e.AttrInt("height", 0); mh > h {
		h = mh
	}
	if mw := e.AttrInt("minWidth", 0); mw > w {
		w = mw
	}
	if mh := e.AttrInt("minHeight", 0); mh > h {
		h = mh
	}

	// Widget-specific min sizes based on attributes
	if e.AttrBool("editable", false) {
		ms := e.editableMinSize()
		if ms.X > w {
			w = ms.X
		}
		if ms.Y > h {
			h = ms.Y
		}
	}
	if e.AttrBool("checkable", false) {
		ms := e.checkableMinSize()
		if ms.X > w {
			w = ms.X
		}
		if ms.Y > h {
			h = ms.Y
		}
	}
	if e.AttrBool("toggleable", false) {
		ms := e.toggleMinSize()
		if ms.X > w {
			w = ms.X
		}
		if ms.Y > h {
			h = ms.Y
		}
	}
	if e.AttrBool("listView", false) {
		ms := e.listViewMinSize()
		if ms.X > w {
			w = ms.X
		}
		if ms.Y > h {
			h = ms.Y
		}
	}
	if e.AttrBool("slidable", false) {
		ms := e.slidableMinSize()
		if ms.X > w {
			w = ms.X
		}
		if ms.Y > h {
			h = ms.Y
		}
	}
	if e.AttrBool("showProgress", false) {
		ms := e.progressMinSize()
		if ms.X > w {
			w = ms.X
		}
		if ms.Y > h {
			h = ms.Y
		}
	}
	// Optional intrinsic sizing for Image widgets.
	// Keep this opt-in so decoded SVG sizes (often large for crisp scaling)
	// do not blow up layout metrics for icon-like usage.
	if strings.EqualFold(e.Tag(), "image") && e.AttrBool("intrinsicSize", false) {
		if st := e.getImageState(); st != nil && st.source != nil {
			if st.source.Width > w {
				w = st.source.Width
			}
			if st.source.Height > h {
				h = st.source.Height
			}
		}
	}
	if e.Attr("title", "") != "" {
		ms := e.frameMinSize()
		if ms.X > w {
			w = ms.X
		}
		if ms.Y > h {
			h = ms.Y
		}
	}

	// Compute min size from text
	if text := e.Attr("text", ""); text != "" {
		font := elementFont(e, graphics.UIFontParagraph)
		tw := font.TextWidth(text)
		th := font.TextHeight(text)
		pt, pr, pb, pl := e.padding()
		if tw+pl+pr > w {
			w = tw + pl + pr
		}
		if th+pt+pb > h {
			h = th + pt + pb
		}
	}

	// Compute min size from children
	if len(e.children) > 0 {
		cw := 0
		ch := 0
		if e.overflowScrollable() {
			cw, ch = e.childrenCrossMinSize()
		} else {
			cw, ch = e.childrenMinSize()
		}
		pt, pr, pb, pl := e.padding()
		if cw+pl+pr > w {
			w = cw + pl + pr
		}
		if ch+pt+pb > h {
			h = ch + pt + pb
		}
	}

	if mw := e.AttrInt("maxWidth", 0); mw > 0 && w > mw {
		w = mw
	}
	if mh := e.AttrInt("maxHeight", 0); mh > 0 && h > mh {
		h = mh
	}

	return graphics.Point{X: w, Y: h}
}

// childrenCrossMinSize returns only cross-axis constraints for scrollable containers.
// This avoids large content forcing parent overflow while preserving useful cross-axis sizing.
func (e *Element) childrenCrossMinSize() (w, h int) {
	if len(e.children) == 0 {
		return 0, 0
	}
	if e.isRow() {
		maxH := 0
		for _, child := range e.children {
			if !child.visible {
				continue
			}
			ms := child.MinSize()
			if ms.Y > maxH {
				maxH = ms.Y
			}
		}
		return 0, maxH
	}
	maxW := 0
	for _, child := range e.children {
		if !child.visible {
			continue
		}
		ms := child.MinSize()
		if ms.X > maxW {
			maxW = ms.X
		}
	}
	return maxW, 0
}

func (e *Element) HandleEvent(ev graphics.Event) bool {
	if !e.visible {
		return false
	}

	// Wheel events should target deepest child under pointer, then bubble to scrollable parents.
	if ev.Type == graphics.EventMouseButtonPress && isWheelButton(ev.MouseButton) {
		for i := len(e.children) - 1; i >= 0; i-- {
			child := e.children[i]
			if !child.visible || !child.Bounds().ContainsXY(ev.X, ev.Y) {
				continue
			}
			if child.HandleEvent(ev) {
				return true
			}
		}
		if e.isInteractive() && e.interactHandleEvent(ev) {
			return true
		}
		return false
	}

	// Interactive handling based on attributes
	if e.isInteractive() {
		if e.interactHandleEvent(ev) {
			return true
		}
	}

	// Propagate to children in reverse z-order
	for i := len(e.children) - 1; i >= 0; i-- {
		if e.children[i].HandleEvent(ev) {
			return true
		}
	}
	return false
}

func isWheelButton(btn graphics.MouseButton) bool {
	switch btn {
	case graphics.MouseButtonWheelUp, graphics.MouseButtonWheelDown, graphics.MouseButtonWheelLeft, graphics.MouseButtonWheelRight:
		return true
	default:
		return false
	}
}

func (e *Element) SetFocused(focused bool) {
	if e.focused != focused {
		e.focused = focused
		e.clearDirtyRects()
		e.dirty = true
	}
}

func (e *Element) IsFocused() bool { return e.focused }

func (e *Element) IsDirty() bool {
	if e.dirty {
		return true
	}
	for _, child := range e.children {
		if child.IsDirty() {
			return true
		}
	}
	return false
}

func (e *Element) SelfDirty() bool { return e.dirty }

// DirtyRects returns optional subregions that should be redrawn for this widget.
// Nil means repaint the full widget bounds.
func (e *Element) DirtyRects() []graphics.Rect {
	if len(e.dirtyRects) == 0 {
		return nil
	}
	out := make([]graphics.Rect, len(e.dirtyRects))
	copy(out, e.dirtyRects)
	return out
}

func (e *Element) MarkClean() {
	e.dirty = false
	e.clearDirtyRects()
	for _, child := range e.children {
		child.MarkClean()
	}
}

func (e *Element) MarkDirty() {
	e.clearDirtyRects()
	e.dirty = true
}

func (e *Element) SetVisible(visible bool) {
	if e.visible != visible {
		e.visible = visible
		e.clearDirtyRects()
		e.dirty = true
		if e.parent != nil {
			e.parent.dirty = true
			if e.parent.bounds.W > 0 && e.parent.bounds.H > 0 {
				e.parent.layoutChildren()
			}
		}
	}
}

func (e *Element) IsVisible() bool { return e.visible }

// Children returns child widgets (for app.App damage tracking).
func (e *Element) Children() []graphics.Widget {
	out := make([]graphics.Widget, len(e.children))
	for i, c := range e.children {
		out[i] = c
	}
	return out
}

func (e *Element) clearDirtyRects() {
	e.dirtyRects = e.dirtyRects[:0]
}

func (e *Element) addDirtyRect(r graphics.Rect) {
	r = r.Intersection(e.bounds)
	if r.IsEmpty() {
		return
	}
	e.dirty = true
	e.dirtyRects = append(e.dirtyRects, r)
	if len(e.dirtyRects) > 128 {
		merged := e.dirtyRects[0]
		for _, rect := range e.dirtyRects[1:] {
			merged = merged.Union(rect)
		}
		e.dirtyRects = e.dirtyRects[:1]
		e.dirtyRects[0] = merged
	}
}

// --- Helpers ---

func (e *Element) padding() (top, right, bottom, left int) {
	if v, ok := e.attrs["padding"]; ok && v != "" {
		return parsePaddingStr(v)
	}
	return 0, 0, 0, 0
}

func (e *Element) contentArea() graphics.Rect {
	pt, pr, pb, pl := e.padding()
	return graphics.Rect{
		X: e.bounds.X + pl,
		Y: e.bounds.Y + pt,
		W: e.bounds.W - pl - pr,
		H: e.bounds.H - pt - pb,
	}
}

func (e *Element) isRow() bool {
	orientation := strings.ToLower(strings.TrimSpace(e.Attr("orientation", "")))
	switch orientation {
	case "row", "horizontal":
		return true
	case "column", "vertical":
		return false
	}
	return strings.ToLower(strings.TrimSpace(e.Attr("direction", "column"))) == "row"
}

// FindChild finds a descendant by id (depth-first).
func (e *Element) FindChild(id string) *Element {
	for _, child := range e.children {
		if child.id == id {
			return child
		}
		if found := child.FindChild(id); found != nil {
			return found
		}
	}
	return nil
}
