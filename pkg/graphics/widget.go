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

// TextAlign represents text alignment.
type TextAlign int

const (
	AlignLeft TextAlign = iota
	AlignCenter
	AlignRight
)

// Widget is the interface that all widgets must implement.
type Widget interface {
	Draw(buf *Buffer)
	Bounds() Rect
	SetBounds(r Rect)
	MinSize() Point
	HandleEvent(ev Event) bool
	SetFocused(focused bool)
	IsFocused() bool
	IsDirty() bool
	MarkClean()
	SetVisible(visible bool)
	IsVisible() bool
}

// BaseWidget provides common functionality for widgets.
type BaseWidget struct {
	bounds  Rect
	focused bool
	dirty   bool
	visible bool
	minSize Point
}

// NewBaseWidget creates a new base widget.
func NewBaseWidget() BaseWidget {
	return BaseWidget{
		visible: true,
		dirty:   true,
	}
}

// Bounds returns the widget's bounds.
func (w *BaseWidget) Bounds() Rect {
	return w.bounds
}

// SetBounds sets the widget's bounds.
func (w *BaseWidget) SetBounds(r Rect) {
	if w.bounds != r {
		w.bounds = r
		w.dirty = true
	}
}

// MinSize returns the minimum size the widget needs.
func (w *BaseWidget) MinSize() Point {
	return w.minSize
}

// SetMinSize sets the minimum size for the widget.
func (w *BaseWidget) SetMinSize(size Point) {
	w.minSize = size
}

// IsFocused returns true if the widget has focus.
func (w *BaseWidget) IsFocused() bool {
	return w.focused
}

// SetFocused sets the focus state of the widget.
func (w *BaseWidget) SetFocused(focused bool) {
	if w.focused != focused {
		w.focused = focused
		w.dirty = true
	}
}

// IsDirty returns true if the widget needs to be redrawn.
func (w *BaseWidget) IsDirty() bool {
	return w.dirty
}

// SelfDirty returns true if the widget itself needs a redraw.
// Containers can use this to distinguish their own dirty state from children.
func (w *BaseWidget) SelfDirty() bool {
	return w.dirty
}

// MarkClean marks the widget as not needing a redraw.
func (w *BaseWidget) MarkClean() {
	w.dirty = false
}

// MarkDirty marks the widget as needing a redraw.
func (w *BaseWidget) MarkDirty() {
	w.dirty = true
}

// IsVisible returns true if the widget is visible.
func (w *BaseWidget) IsVisible() bool {
	return w.visible
}

// SetVisible sets the visibility of the widget.
func (w *BaseWidget) SetVisible(visible bool) {
	if w.visible != visible {
		w.visible = visible
		w.dirty = true
	}
}
