package engine

import (
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	gfxfont "avyos.dev/pkg/graphics/fonts"
	gfxinput "avyos.dev/pkg/graphics/input"
	core "avyos.dev/pkg/graphics/pixmap"
)

// --- Event dispatch ---

// isInteractive returns true if the element handles events.
func (e *Element) isInteractive() bool {
	return e.AttrBool("interactive", false) ||
		e.AttrBool("editable", false) ||
		e.AttrBool("toggleable", false) ||
		e.AttrBool("checkable", false) ||
		(e.overflowScrollable() && !e.isRow()) ||
		e.AttrBool("listView", false) ||
		e.AttrBool("slidable", false) ||
		len(e.signals) > 0
}

// interactHandleEvent dispatches events based on element attributes.
func (e *Element) interactHandleEvent(ev gfxinput.Event) bool {
	if e.focused {
		if ev.Type == gfxinput.EventKeyPress {
			if _, ok := e.signals["keyPressed"]; ok {
				e.EmitSignal("keyPressed", ev)
				return true
			}
		}
		if ev.Type == gfxinput.EventKeyRelease {
			if _, ok := e.signals["keyReleased"]; ok {
				e.EmitSignal("keyReleased", ev)
				return true
			}
		}
	}

	if ev.Type == gfxinput.EventMouseButtonPress && isWheelButton(ev.MouseButton) {
		if e.AttrBool("listView", false) {
			return e.handleListViewEvent(ev)
		}
		if e.AttrBool("editable", false) && e.AttrBool("multiline", false) {
			return e.handleTextAreaEvent(ev)
		}
		if e.overflowScrollable() && !e.isRow() {
			return e.handleScrollableWheel(ev)
		}
	}

	if e.AttrBool("editable", false) {
		if e.AttrBool("multiline", false) {
			return e.handleTextAreaEvent(ev)
		}
		return e.handleTextInputEvent(ev)
	}
	if e.AttrBool("toggleable", false) {
		return e.handleToggleEvent(ev)
	}
	if e.AttrBool("slidable", false) {
		return e.handleSliderEvent(ev)
	}
	if e.AttrBool("listView", false) {
		return e.handleListViewEvent(ev)
	}
	if e.AttrBool("checkable", false) {
		return e.handleCheckableEvent(ev)
	}
	if e.AttrBool("interactive", false) || len(e.signals) > 0 {
		return e.handleClickEvent(ev)
	}
	return false
}

// --- Click handling (buttons, interactive elements) ---

func (e *Element) handleClickEvent(ev gfxinput.Event) bool {
	bounds := e.Bounds()
	switch ev.Type {
	case gfxinput.EventMouseMove:
		inside := core.RectContainsXY(bounds, ev.X, ev.Y)
		if inside != e.hovered {
			e.hovered = inside
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonPress:
		if ev.MouseButton == gfxinput.MouseButtonLeft && core.RectContainsXY(bounds, ev.X, ev.Y) {
			e.pressed = true
			e.dirty = true
			return true
		}
		if ev.MouseButton == gfxinput.MouseButtonRight && core.RectContainsXY(bounds, ev.X, ev.Y) {
			e.rightPressed = true
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonRelease:
		if ev.MouseButton == gfxinput.MouseButtonLeft && e.pressed {
			inside := core.RectContainsXY(bounds, ev.X, ev.Y)
			e.hovered = inside
			if inside {
				e.EmitSignal("clicked")
			}
			e.pressed = false
			e.dirty = true
			return true
		}
		if ev.MouseButton == gfxinput.MouseButtonRight && e.rightPressed {
			inside := core.RectContainsXY(bounds, ev.X, ev.Y)
			e.hovered = inside
			if inside {
				e.EmitSignal("secondaryClicked")
			}
			e.rightPressed = false
			e.dirty = true
			return true
		}
	case gfxinput.EventKeyPress:
		if !e.focused {
			return false
		}
		if _, ok := e.signals["keyPressed"]; ok {
			e.EmitSignal("keyPressed", ev)
			return true
		}
		if ev.Key == gfxinput.KeyEnter || ev.Key == gfxinput.KeySpace {
			e.EmitSignal("clicked")
			return true
		}
	case gfxinput.EventKeyRelease:
		if !e.focused {
			return false
		}
		if _, ok := e.signals["keyReleased"]; ok {
			e.EmitSignal("keyReleased", ev)
			return true
		}
	}
	return false
}

// --- Checkable handling ---

func (e *Element) handleCheckableEvent(ev gfxinput.Event) bool {
	bounds := e.Bounds()
	switch ev.Type {
	case gfxinput.EventMouseMove:
		inside := core.RectContainsXY(bounds, ev.X, ev.Y)
		if inside != e.hovered {
			e.hovered = inside
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonPress:
		if ev.MouseButton == gfxinput.MouseButtonLeft && core.RectContainsXY(bounds, ev.X, ev.Y) {
			checked := !e.AttrBool("checked", false)
			e.attrs["checked"] = fmt.Sprintf("%v", checked)
			e.dirty = true
			e.EmitSignal("changed", checked)
			return true
		}
	case gfxinput.EventKeyPress:
		if e.focused && (ev.Key == gfxinput.KeyEnter || ev.Key == gfxinput.KeySpace) {
			checked := !e.AttrBool("checked", false)
			e.attrs["checked"] = fmt.Sprintf("%v", checked)
			e.dirty = true
			e.EmitSignal("changed", checked)
			return true
		}
	}
	return false
}

func (e *Element) handleToggleEvent(ev gfxinput.Event) bool {
	bounds := e.Bounds()
	switch ev.Type {
	case gfxinput.EventMouseMove:
		inside := core.RectContainsXY(bounds, ev.X, ev.Y)
		if inside != e.hovered {
			e.hovered = inside
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonPress:
		if ev.MouseButton == gfxinput.MouseButtonLeft && core.RectContainsXY(bounds, ev.X, ev.Y) {
			e.pressed = true
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonRelease:
		if ev.MouseButton == gfxinput.MouseButtonLeft && e.pressed {
			inside := core.RectContainsXY(bounds, ev.X, ev.Y)
			e.hovered = inside
			e.pressed = false
			if inside {
				checked := !e.AttrBool("checked", false)
				e.attrs["checked"] = fmt.Sprintf("%v", checked)
				e.EmitSignal("changed", checked)
			}
			e.dirty = true
			return true
		}
	case gfxinput.EventKeyPress:
		if e.focused && (ev.Key == gfxinput.KeyEnter || ev.Key == gfxinput.KeySpace) {
			checked := !e.AttrBool("checked", false)
			e.attrs["checked"] = fmt.Sprintf("%v", checked)
			e.dirty = true
			e.EmitSignal("changed", checked)
			return true
		}
	}
	return false
}

func (e *Element) handleListViewEvent(ev gfxinput.Event) bool {
	bounds := e.Bounds()
	itemsRaw := e.Attr("items", "")
	if itemsRaw == "" {
		return false
	}
	items := strings.Split(itemsRaw, "|")
	rowH := e.AttrInt("rowHeight", 34)
	if rowH <= 0 {
		rowH = 34
	}
	visibleRows := bounds.Dy() / rowH
	if visibleRows < 1 {
		visibleRows = 1
	}
	scrollIndex := e.AttrInt("scrollIndex", 0)
	maxScroll := len(items) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollIndex < 0 {
		scrollIndex = 0
	}
	if scrollIndex > maxScroll {
		scrollIndex = maxScroll
	}

	rowAt := func(x, y int) int {
		if !core.RectContainsXY(bounds, x, y) {
			return -1
		}
		idx := (y-bounds.Min.Y)/rowH + scrollIndex
		if idx < 0 || idx >= len(items) {
			return -1
		}
		return idx
	}

	switch ev.Type {
	case gfxinput.EventMouseMove:
		idx := rowAt(ev.X, ev.Y)
		prev := e.AttrInt("hoveredIndex", -1)
		if idx != prev {
			e.attrs["hoveredIndex"] = fmt.Sprintf("%d", idx)
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonPress:
		if isWheelButton(ev.MouseButton) {
			if !core.RectContainsXY(bounds, ev.X, ev.Y) {
				return false
			}
			delta := 0
			switch ev.MouseButton {
			case gfxinput.MouseButtonWheelUp:
				delta = -1
			case gfxinput.MouseButtonWheelDown:
				delta = 1
			default:
				return false
			}
			scrollIndex += delta
			if scrollIndex < 0 {
				scrollIndex = 0
			}
			if scrollIndex > maxScroll {
				scrollIndex = maxScroll
			}
			e.attrs["scrollIndex"] = fmt.Sprintf("%d", scrollIndex)
			e.dirty = true
			return true
		}
		if ev.MouseButton == gfxinput.MouseButtonRight {
			if !core.RectContainsXY(bounds, ev.X, ev.Y) {
				return false
			}
			idx := rowAt(ev.X, ev.Y)
			e.attrs["pressedIndex"] = "-1"
			e.attrs["hoveredIndex"] = fmt.Sprintf("%d", idx)
			if idx >= 0 {
				e.attrs["selected"] = fmt.Sprintf("%d", idx)
			}
			e.dirty = true
			e.EmitSignal("secondaryClicked")
			return true
		}
		if ev.MouseButton == gfxinput.MouseButtonLeft {
			if !core.RectContainsXY(bounds, ev.X, ev.Y) {
				return false
			}
			e.pressed = true
			e.attrs["pressedIndex"] = fmt.Sprintf("%d", rowAt(ev.X, ev.Y))
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonRelease:
		if ev.MouseButton == gfxinput.MouseButtonLeft && e.pressed {
			e.pressed = false
			releasedIdx := rowAt(ev.X, ev.Y)
			pressedIdx := e.AttrInt("pressedIndex", -1)
			e.attrs["pressedIndex"] = "-1"
			e.attrs["hoveredIndex"] = fmt.Sprintf("%d", releasedIdx)
			if releasedIdx >= 0 && releasedIdx == pressedIdx {
				e.attrs["selected"] = fmt.Sprintf("%d", releasedIdx)
				e.dirty = true
				e.EmitSignal("changed", releasedIdx, strings.TrimSpace(items[releasedIdx]))
				return true
			}
			e.dirty = true
			return true
		}
	case gfxinput.EventKeyPress:
		if !e.focused {
			return false
		}
		selected := e.AttrInt("selected", 0)
		if selected < 0 {
			selected = 0
		}
		if selected >= len(items) {
			selected = len(items) - 1
		}
		switch ev.Key {
		case gfxinput.KeyUp:
			if selected > 0 {
				selected--
			}
		case gfxinput.KeyDown:
			if selected < len(items)-1 {
				selected++
			}
		case gfxinput.KeyPageUp:
			selected -= visibleRows
			if selected < 0 {
				selected = 0
			}
		case gfxinput.KeyPageDown:
			selected += visibleRows
			if selected > len(items)-1 {
				selected = len(items) - 1
			}
		case gfxinput.KeyHome:
			selected = 0
		case gfxinput.KeyEnd:
			selected = len(items) - 1
		default:
			return false
		}
		if selected < scrollIndex {
			scrollIndex = selected
		}
		if selected >= scrollIndex+visibleRows {
			scrollIndex = selected - visibleRows + 1
		}
		if scrollIndex < 0 {
			scrollIndex = 0
		}
		if scrollIndex > maxScroll {
			scrollIndex = maxScroll
		}
		e.attrs["selected"] = fmt.Sprintf("%d", selected)
		e.attrs["scrollIndex"] = fmt.Sprintf("%d", scrollIndex)
		e.dirty = true
		e.EmitSignal("changed", selected, strings.TrimSpace(items[selected]))
		return true
	}
	return false
}

// --- Text input handling ---

func (e *Element) handleTextInputEvent(ev gfxinput.Event) bool {
	bounds := e.Bounds()
	content := e.contentArea()
	st := e.getTextInputState()
	font := elementFont(e, gfxfont.UIFontParagraph)

	switch ev.Type {
	case gfxinput.EventMouseButtonPress:
		if ev.MouseButton == gfxinput.MouseButtonLeft && core.RectContainsXY(bounds, ev.X, ev.Y) {
			display := textInputDisplayRunes(st, e.AttrBool("password", false))
			start, end := textInputVisibleRange(font, display, st.scrollOffset, content.Dx())
			relX := ev.X - content.Min.X
			if relX < 0 {
				relX = 0
			}
			st.cursorPos = textInputCursorFromX(font, display, start, end, relX)
			e.ensureTextInputVisible(st)
			st.cursorBlink = true
			st.lastBlink = time.Now()
			e.dirty = true
			return true
		}
	case gfxinput.EventKeyPress:
		if !e.focused {
			return false
		}
		return e.handleTextInputKey(st, ev)
	}
	return false
}

func (e *Element) handleTextInputKey(st *textInputState, ev gfxinput.Event) bool {
	switch ev.Key {
	case gfxinput.KeyLeft:
		if st.cursorPos > 0 {
			st.cursorPos--
			e.ensureTextInputVisible(st)
			st.cursorBlink = true
			st.lastBlink = time.Now()
			e.dirty = true
		}
		return true
	case gfxinput.KeyRight:
		if st.cursorPos < len(st.text) {
			st.cursorPos++
			e.ensureTextInputVisible(st)
			st.cursorBlink = true
			st.lastBlink = time.Now()
			e.dirty = true
		}
		return true
	case gfxinput.KeyHome:
		st.cursorPos = 0
		e.ensureTextInputVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyEnd:
		st.cursorPos = len(st.text)
		e.ensureTextInputVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyBackspace:
		if st.cursorPos > 0 {
			st.text = append(st.text[:st.cursorPos-1], st.text[st.cursorPos:]...)
			st.cursorPos--
			e.syncTextInput(st)
			e.ensureTextInputVisible(st)
			st.cursorBlink = true
			st.lastBlink = time.Now()
			e.dirty = true
		}
		return true
	case gfxinput.KeyDelete:
		if st.cursorPos < len(st.text) {
			st.text = append(st.text[:st.cursorPos], st.text[st.cursorPos+1:]...)
			e.syncTextInput(st)
			e.dirty = true
		}
		return true
	case gfxinput.KeyEnter:
		e.EmitSignal("submitted", string(st.text))
		return true
	default:
		if ev.Rune != 0 && ev.Rune >= 32 && !ev.IsCtrl() {
			newText := make([]rune, len(st.text)+1)
			copy(newText, st.text[:st.cursorPos])
			newText[st.cursorPos] = ev.Rune
			copy(newText[st.cursorPos+1:], st.text[st.cursorPos:])
			st.text = newText
			st.cursorPos++
			e.syncTextInput(st)
			e.ensureTextInputVisible(st)
			st.cursorBlink = true
			st.lastBlink = time.Now()
			e.dirty = true
			return true
		}
	}
	return false
}

func (e *Element) syncTextInput(st *textInputState) {
	e.attrs["text"] = string(st.text)
	e.EmitSignal("changed", string(st.text))
}

func textInputDisplayRunes(st *textInputState, password bool) []rune {
	if st == nil {
		return nil
	}
	if !password {
		return st.text
	}
	if len(st.text) == 0 {
		return nil
	}
	out := make([]rune, len(st.text))
	for i := range out {
		out[i] = '*'
	}
	return out
}

func textInputSliceWidth(font *gfxfont.Font, display []rune, start, end int) int {
	if font == nil {
		return 0
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(display) {
		start = len(display)
	}
	if end > len(display) {
		end = len(display)
	}
	if start >= end {
		return 0
	}
	return font.TextWidth(string(display[start:end]))
}

func textInputVisibleRange(font *gfxfont.Font, display []rune, start, maxWidth int) (int, int) {
	if start < 0 {
		start = 0
	}
	if start > len(display) {
		start = len(display)
	}
	if maxWidth <= 0 {
		return start, start
	}
	end := start
	for end < len(display) {
		if textInputSliceWidth(font, display, start, end+1) > maxWidth {
			break
		}
		end++
	}
	return start, end
}

func textInputCursorFromX(font *gfxfont.Font, display []rune, start, end, relX int) int {
	if relX <= 0 || start >= end {
		return start
	}
	prevW := 0
	for i := start; i < end; i++ {
		nextW := textInputSliceWidth(font, display, start, i+1)
		if relX < (prevW+nextW)/2 {
			return i
		}
		prevW = nextW
	}
	return end
}

func (e *Element) ensureTextInputVisible(st *textInputState) {
	font := elementFont(e, gfxfont.UIFontParagraph)
	content := e.contentArea()
	if content.Dx() <= 0 {
		return
	}

	display := textInputDisplayRunes(st, e.AttrBool("password", false))
	if st.cursorPos < 0 {
		st.cursorPos = 0
	}
	if st.cursorPos > len(display) {
		st.cursorPos = len(display)
	}
	if st.scrollOffset < 0 {
		st.scrollOffset = 0
	}
	if st.scrollOffset > len(display) {
		st.scrollOffset = len(display)
	}
	if st.cursorPos < st.scrollOffset {
		st.scrollOffset = st.cursorPos
	}

	for st.scrollOffset < st.cursorPos &&
		textInputSliceWidth(font, display, st.scrollOffset, st.cursorPos) > content.Dx() {
		st.scrollOffset++
	}

	for st.scrollOffset > 0 &&
		textInputSliceWidth(font, display, st.scrollOffset-1, st.cursorPos) <= content.Dx() {
		st.scrollOffset--
	}
}

// --- Text area handling ---

func (e *Element) handleTextAreaEvent(ev gfxinput.Event) bool {
	bounds := e.Bounds()
	content := e.contentArea()
	st := e.getTextAreaState()
	font := elementFont(e, gfxfont.UIFontParagraph)
	textW := e.textAreaTextWidth(content)

	switch ev.Type {
	case gfxinput.EventMouseButtonPress:
		if isWheelButton(ev.MouseButton) {
			visRows, _ := e.textAreaVisibleRowsCols()
			if visRows < 1 {
				visRows = 1
			}
			step := int(math.Max(1, float64(visRows/3)))
			switch ev.MouseButton {
			case gfxinput.MouseButtonWheelUp:
				st.scrollRow -= step
			case gfxinput.MouseButtonWheelDown:
				st.scrollRow += step
			default:
				return false
			}
			maxScroll := e.textAreaMaxScroll(st)
			if st.scrollRow < 0 {
				st.scrollRow = 0
			}
			if st.scrollRow > maxScroll {
				st.scrollRow = maxScroll
			}
			e.dirty = true
			return true
		}
		if ev.MouseButton == gfxinput.MouseButtonLeft && core.RectContainsXY(bounds, ev.X, ev.Y) {
			relX := ev.X - content.Min.X
			relY := ev.Y - content.Min.Y
			if relX < 0 {
				relX = 0
			}
			if relY < 0 {
				relY = 0
			}

			if e.AttrBool("wrap", false) {
				segments := textAreaWrappedSegments(st.lines, font, textW)
				if len(segments) == 0 {
					return false
				}
				visualRow := relY/font.Height + st.scrollRow
				if visualRow < 0 {
					visualRow = 0
				}
				if visualRow >= len(segments) {
					visualRow = len(segments) - 1
				}
				seg := segments[visualRow]
				line := st.lines[seg.row]
				col := textAreaCursorFromX(font, line, seg.start, relX, textW)
				if col < seg.start {
					col = seg.start
				}
				if col > seg.end {
					col = seg.end
				}
				st.cursorRow = seg.row
				st.cursorCol = col
			} else {
				row := relY/font.Height + st.scrollRow
				if row < 0 {
					row = 0
				}
				if row >= len(st.lines) {
					row = len(st.lines) - 1
				}
				line := st.lines[row]
				col := textAreaCursorFromX(font, line, st.scrollCol, relX, textW)
				st.cursorRow = row
				st.cursorCol = col
			}
			clampTextAreaCursor(st)
			e.ensureTextAreaVisible(st)
			st.cursorBlink = true
			st.lastBlink = time.Now()
			e.dirty = true
			return true
		}
	case gfxinput.EventKeyPress:
		if !e.focused {
			return false
		}
		return e.handleTextAreaKey(st, ev)
	}
	return false
}

func (e *Element) handleScrollableWheel(ev gfxinput.Event) bool {
	step := e.AttrInt("scrollStep", 40)
	if step < 8 {
		step = 8
	}
	scrollY := e.AttrInt("scrollY", 0)
	maxScroll := e.maxScrollableY()
	if maxScroll <= 0 {
		return false
	}
	start := scrollY
	switch ev.MouseButton {
	case gfxinput.MouseButtonWheelUp:
		scrollY -= step
	case gfxinput.MouseButtonWheelDown:
		scrollY += step
	default:
		return false
	}

	if scrollY < 0 {
		scrollY = 0
	}
	if scrollY > maxScroll {
		scrollY = maxScroll
	}
	if scrollY == start {
		return false
	}
	e.attrs["scrollY"] = fmt.Sprintf("%d", scrollY)
	e.layoutChildren()
	e.dirty = true
	return true
}

func (e *Element) maxScrollableY() int {
	content := e.contentArea()
	scrollY := e.AttrInt("scrollY", 0)
	baseY := content.Min.Y - scrollY
	maxBottom := baseY
	for _, child := range e.children {
		if !child.visible {
			continue
		}
		bottom := child.maxVisibleBottom()
		if bottom > maxBottom {
			maxBottom = bottom
		}
	}
	contentH := maxBottom - baseY
	maxScroll := contentH - content.Dy()
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (e *Element) maxVisibleBottom() int {
	if e == nil || !e.visible {
		return 0
	}
	bottom := e.bounds.Min.Y + e.bounds.Dy()
	if e.overflowClipped() {
		return bottom
	}
	for _, child := range e.children {
		if !child.visible {
			continue
		}
		cb := child.maxVisibleBottom()
		if cb > bottom {
			bottom = cb
		}
	}
	return bottom
}

func (e *Element) handleTextAreaKey(st *textAreaState, ev gfxinput.Event) bool {
	readOnly := e.AttrBool("readOnly", false)

	switch ev.Key {
	case gfxinput.KeyLeft:
		if st.cursorCol > 0 {
			st.cursorCol--
		} else if st.cursorRow > 0 {
			st.cursorRow--
			st.cursorCol = len(st.lines[st.cursorRow])
		}
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyRight:
		if st.cursorRow < len(st.lines) && st.cursorCol < len(st.lines[st.cursorRow]) {
			st.cursorCol++
		} else if st.cursorRow < len(st.lines)-1 {
			st.cursorRow++
			st.cursorCol = 0
		}
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyUp:
		if e.AttrBool("wrap", false) {
			e.moveWrappedCursorVertical(st, -1)
		} else if st.cursorRow > 0 {
			st.cursorRow--
			if st.cursorCol > len(st.lines[st.cursorRow]) {
				st.cursorCol = len(st.lines[st.cursorRow])
			}
		}
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyDown:
		if e.AttrBool("wrap", false) {
			e.moveWrappedCursorVertical(st, 1)
		} else if st.cursorRow < len(st.lines)-1 {
			st.cursorRow++
			if st.cursorCol > len(st.lines[st.cursorRow]) {
				st.cursorCol = len(st.lines[st.cursorRow])
			}
		}
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyHome:
		if readOnly {
			st.scrollRow = 0
			e.dirty = true
			return true
		}
		st.cursorCol = 0
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyEnd:
		if readOnly {
			st.scrollRow = e.textAreaMaxScroll(st)
			e.dirty = true
			return true
		}
		st.cursorCol = len(st.lines[st.cursorRow])
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyBackspace:
		if readOnly {
			return true
		}
		if st.cursorCol > 0 {
			line := st.lines[st.cursorRow]
			st.lines[st.cursorRow] = append(line[:st.cursorCol-1], line[st.cursorCol:]...)
			st.cursorCol--
		} else if st.cursorRow > 0 {
			prevLen := len(st.lines[st.cursorRow-1])
			st.lines[st.cursorRow-1] = append(st.lines[st.cursorRow-1], st.lines[st.cursorRow]...)
			st.lines = append(st.lines[:st.cursorRow], st.lines[st.cursorRow+1:]...)
			st.cursorRow--
			st.cursorCol = prevLen
		}
		e.syncTextArea(st)
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyDelete:
		if readOnly {
			return true
		}
		line := st.lines[st.cursorRow]
		if st.cursorCol < len(line) {
			st.lines[st.cursorRow] = append(line[:st.cursorCol], line[st.cursorCol+1:]...)
		} else if st.cursorRow < len(st.lines)-1 {
			st.lines[st.cursorRow] = append(st.lines[st.cursorRow], st.lines[st.cursorRow+1]...)
			st.lines = append(st.lines[:st.cursorRow+1], st.lines[st.cursorRow+2:]...)
		}
		e.syncTextArea(st)
		e.dirty = true
		return true
	case gfxinput.KeyEnter:
		if readOnly {
			return true
		}
		line := st.lines[st.cursorRow]
		before := make([]rune, st.cursorCol)
		copy(before, line[:st.cursorCol])
		after := make([]rune, len(line)-st.cursorCol)
		copy(after, line[st.cursorCol:])
		st.lines[st.cursorRow] = before
		newLines := make([][]rune, len(st.lines)+1)
		copy(newLines, st.lines[:st.cursorRow+1])
		newLines[st.cursorRow+1] = after
		copy(newLines[st.cursorRow+2:], st.lines[st.cursorRow+1:])
		st.lines = newLines
		st.cursorRow++
		st.cursorCol = 0
		e.syncTextArea(st)
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyPageUp:
		visRows, _ := e.textAreaVisibleRowsCols()
		if visRows < 1 {
			visRows = 1
		}
		if readOnly {
			st.scrollRow -= visRows
			if st.scrollRow < 0 {
				st.scrollRow = 0
			}
			e.dirty = true
			return true
		}
		st.cursorRow -= visRows
		if st.cursorRow < 0 {
			st.cursorRow = 0
		}
		if st.cursorCol > len(st.lines[st.cursorRow]) {
			st.cursorCol = len(st.lines[st.cursorRow])
		}
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	case gfxinput.KeyPageDown:
		visRows, _ := e.textAreaVisibleRowsCols()
		if visRows < 1 {
			visRows = 1
		}
		if readOnly {
			maxScroll := e.textAreaMaxScroll(st)
			st.scrollRow += visRows
			if st.scrollRow > maxScroll {
				st.scrollRow = maxScroll
			}
			e.dirty = true
			return true
		}
		st.cursorRow += visRows
		if st.cursorRow >= len(st.lines) {
			st.cursorRow = len(st.lines) - 1
		}
		if st.cursorCol > len(st.lines[st.cursorRow]) {
			st.cursorCol = len(st.lines[st.cursorRow])
		}
		e.ensureTextAreaVisible(st)
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.dirty = true
		return true
	default:
		if ev.Rune != 0 && ev.Rune >= 32 && !ev.IsCtrl() && !readOnly {
			line := st.lines[st.cursorRow]
			newLine := make([]rune, len(line)+1)
			copy(newLine, line[:st.cursorCol])
			newLine[st.cursorCol] = ev.Rune
			copy(newLine[st.cursorCol+1:], line[st.cursorCol:])
			st.lines[st.cursorRow] = newLine
			st.cursorCol++
			e.syncTextArea(st)
			e.ensureTextAreaVisible(st)
			st.cursorBlink = true
			st.lastBlink = time.Now()
			e.dirty = true
			return true
		}
	}
	return false
}

func (e *Element) textAreaVisibleRowsCols() (int, int) {
	font := elementFont(e, gfxfont.UIFontParagraph)
	content := e.contentArea()
	visRows := content.Dy() / font.Height
	textW := e.textAreaTextWidth(content)
	if textW < 0 {
		textW = 0
	}
	charW := font.Width
	if charW <= 0 {
		charW = font.TextWidth("W")
	}
	if charW <= 0 {
		charW = 8
	}
	visCols := textW / charW
	return visRows, visCols
}

func (e *Element) textAreaTextWidth(content image.Rectangle) int {
	textW := content.Dx()
	if textW <= 0 {
		return textW
	}
	if !e.AttrBool("showScrollbar", false) {
		return textW
	}

	barW := e.AttrInt("scrollbarWidth", 6)
	if barW < 4 {
		barW = 4
	}
	if barW > 14 {
		barW = 14
	}
	margin := e.AttrInt("scrollbarMargin", 2)
	if margin < 0 {
		margin = 0
	}
	trackX := content.Min.X + content.Dx() - barW - margin
	if trackX > content.Min.X {
		textW = trackX - content.Min.X - 1
		if textW < 1 {
			textW = 1
		}
	}
	return textW
}

func (e *Element) textAreaMaxScroll(st *textAreaState) int {
	visRows, _ := e.textAreaVisibleRowsCols()
	if visRows < 1 {
		visRows = 1
	}
	totalRows := len(st.lines)
	if e.AttrBool("wrap", false) {
		font := elementFont(e, gfxfont.UIFontParagraph)
		content := e.contentArea()
		totalRows = len(textAreaWrappedSegments(st.lines, font, e.textAreaTextWidth(content)))
	}
	if totalRows < 1 {
		totalRows = 1
	}
	maxScroll := totalRows - visRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (e *Element) moveWrappedCursorVertical(st *textAreaState, delta int) {
	if st == nil || delta == 0 {
		return
	}
	font := elementFont(e, gfxfont.UIFontParagraph)
	content := e.contentArea()
	textW := e.textAreaTextWidth(content)
	segments := textAreaWrappedSegments(st.lines, font, textW)
	if len(segments) == 0 {
		return
	}

	currentVisual := textAreaWrapVisualRowForCursor(segments, st.cursorRow, st.cursorCol)
	targetVisual := currentVisual + delta
	if targetVisual < 0 {
		targetVisual = 0
	}
	if targetVisual >= len(segments) {
		targetVisual = len(segments) - 1
	}

	currentSeg := segments[currentVisual]
	currentLine := st.lines[currentSeg.row]
	currentCol := st.cursorCol
	if currentCol < currentSeg.start {
		currentCol = currentSeg.start
	}
	if currentCol > currentSeg.end {
		currentCol = currentSeg.end
	}
	relX := textAreaLineSliceWidth(font, currentLine, currentSeg.start, currentCol)

	targetSeg := segments[targetVisual]
	targetLine := st.lines[targetSeg.row]
	targetCol := textAreaCursorFromX(font, targetLine, targetSeg.start, relX, textW)
	if targetCol < targetSeg.start {
		targetCol = targetSeg.start
	}
	if targetCol > targetSeg.end {
		targetCol = targetSeg.end
	}

	st.cursorRow = targetSeg.row
	st.cursorCol = targetCol
}

// TextAreaAtBottom reports whether the current multiline viewport is at the end.
func (e *Element) TextAreaAtBottom(threshold int) bool {
	if e == nil || !e.AttrBool("editable", false) || !e.AttrBool("multiline", false) {
		return true
	}
	if threshold < 0 {
		threshold = 0
	}
	st := e.getTextAreaState()
	maxScroll := e.textAreaMaxScroll(st)
	return st.scrollRow >= maxScroll-threshold
}

// ScrollTextAreaToBottom moves the multiline viewport to the newest lines.
func (e *Element) ScrollTextAreaToBottom() {
	if e == nil || !e.AttrBool("editable", false) || !e.AttrBool("multiline", false) {
		return
	}
	st := e.getTextAreaState()
	maxScroll := e.textAreaMaxScroll(st)
	if st.scrollRow != maxScroll {
		st.scrollRow = maxScroll
		e.clearDirtyRects()
		e.dirty = true
	}
}

func (e *Element) syncTextArea(st *textAreaState) {
	parts := make([]string, len(st.lines))
	for i, l := range st.lines {
		parts[i] = string(l)
	}
	e.attrs["text"] = strings.Join(parts, "\n")
	e.EmitSignal("changed", e.attrs["text"])
}

func clampTextAreaCursor(st *textAreaState) {
	if st.cursorRow < 0 {
		st.cursorRow = 0
	}
	if st.cursorRow >= len(st.lines) {
		st.cursorRow = len(st.lines) - 1
	}
	if st.cursorCol < 0 {
		st.cursorCol = 0
	}
	if st.cursorRow >= 0 && st.cursorRow < len(st.lines) {
		if st.cursorCol > len(st.lines[st.cursorRow]) {
			st.cursorCol = len(st.lines[st.cursorRow])
		}
	}
}

func textAreaLineSliceWidth(font *gfxfont.Font, line []rune, start, end int) int {
	if font == nil {
		return 0
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(line) {
		start = len(line)
	}
	if end > len(line) {
		end = len(line)
	}
	if start >= end {
		return 0
	}
	return font.TextWidth(string(line[start:end]))
}

func textAreaVisibleEndCol(font *gfxfont.Font, line []rune, start, maxWidth int) int {
	if start < 0 {
		start = 0
	}
	if start > len(line) {
		start = len(line)
	}
	if maxWidth <= 0 {
		return start
	}
	end := start
	for end < len(line) {
		if textAreaLineSliceWidth(font, line, start, end+1) > maxWidth {
			break
		}
		end++
	}
	return end
}

func textAreaCursorFromX(font *gfxfont.Font, line []rune, start, relX, maxWidth int) int {
	if start < 0 {
		start = 0
	}
	if start > len(line) {
		start = len(line)
	}
	if relX <= 0 {
		return start
	}
	end := textAreaVisibleEndCol(font, line, start, maxWidth)
	if start >= end {
		return start
	}
	prevW := 0
	for i := start; i < end; i++ {
		nextW := textAreaLineSliceWidth(font, line, start, i+1)
		if relX < (prevW+nextW)/2 {
			return i
		}
		prevW = nextW
	}
	return end
}

type textAreaWrapSegment struct {
	row   int
	start int
	end   int
}

func textAreaWrappedSegments(lines [][]rune, font *gfxfont.Font, maxWidth int) []textAreaWrapSegment {
	if len(lines) == 0 {
		return []textAreaWrapSegment{{row: 0, start: 0, end: 0}}
	}
	if maxWidth < 1 {
		maxWidth = 1
	}

	out := make([]textAreaWrapSegment, 0, len(lines))
	for row, line := range lines {
		if len(line) == 0 {
			out = append(out, textAreaWrapSegment{row: row, start: 0, end: 0})
			continue
		}
		start := 0
		for start < len(line) {
			end := textAreaVisibleEndCol(font, line, start, maxWidth)
			if end <= start {
				end = start + 1
			}
			if end > len(line) {
				end = len(line)
			}
			out = append(out, textAreaWrapSegment{row: row, start: start, end: end})
			start = end
		}
	}
	if len(out) == 0 {
		return []textAreaWrapSegment{{row: 0, start: 0, end: 0}}
	}
	return out
}

func textAreaWrapVisualRowForCursor(segments []textAreaWrapSegment, row, col int) int {
	if len(segments) == 0 {
		return 0
	}
	first := -1
	last := -1
	for i, seg := range segments {
		if seg.row != row {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
		if col >= seg.start && col <= seg.end {
			return i
		}
	}
	if first < 0 {
		return 0
	}
	if col < segments[first].start {
		return first
	}
	return last
}

func (e *Element) ensureTextAreaVisible(st *textAreaState) {
	font := elementFont(e, gfxfont.UIFontParagraph)
	content := e.contentArea()
	visRows := content.Dy() / font.Height
	textW := e.textAreaTextWidth(content)
	if visRows <= 0 || textW <= 0 {
		return
	}
	clampTextAreaCursor(st)

	if e.AttrBool("wrap", false) {
		segments := textAreaWrappedSegments(st.lines, font, textW)
		if len(segments) == 0 {
			st.scrollRow = 0
			st.scrollCol = 0
			return
		}
		cursorVisual := textAreaWrapVisualRowForCursor(segments, st.cursorRow, st.cursorCol)
		if cursorVisual < st.scrollRow {
			st.scrollRow = cursorVisual
		} else if cursorVisual >= st.scrollRow+visRows {
			st.scrollRow = cursorVisual - visRows + 1
		}
		maxScroll := len(segments) - visRows
		if maxScroll < 0 {
			maxScroll = 0
		}
		if st.scrollRow < 0 {
			st.scrollRow = 0
		}
		if st.scrollRow > maxScroll {
			st.scrollRow = maxScroll
		}
		st.scrollCol = 0
		return
	}

	if st.cursorRow < st.scrollRow {
		st.scrollRow = st.cursorRow
	} else if st.cursorRow >= st.scrollRow+visRows {
		st.scrollRow = st.cursorRow - visRows + 1
	}
	line := st.lines[st.cursorRow]
	if st.scrollCol > len(line) {
		st.scrollCol = len(line)
	}
	if st.cursorCol < st.scrollCol {
		st.scrollCol = st.cursorCol
	}
	for st.scrollCol < st.cursorCol &&
		textAreaLineSliceWidth(font, line, st.scrollCol, st.cursorCol) > textW {
		st.scrollCol++
	}
	for st.scrollCol > 0 &&
		textAreaLineSliceWidth(font, line, st.scrollCol-1, st.cursorCol) <= textW {
		st.scrollCol--
	}
	if st.scrollRow < 0 {
		st.scrollRow = 0
	}
	if st.scrollCol < 0 {
		st.scrollCol = 0
	}
}

// --- Slider handling ---

const (
	sliderThumbW = 12
	sliderThumbH = 20
	sliderTrackH = 4
)

func (e *Element) handleSliderEvent(ev gfxinput.Event) bool {
	bounds := e.Bounds()
	st := e.getSliderState()

	switch ev.Type {
	case gfxinput.EventMouseMove:
		if st.dragging {
			e.sliderUpdateFromMouse(ev.X, ev.Y, bounds)
			return true
		}
		inside := e.sliderThumbContains(ev.X, ev.Y, bounds)
		if inside != e.hovered {
			e.hovered = inside
			e.dirty = true
			return true
		}
	case gfxinput.EventMouseButtonPress:
		if ev.MouseButton == gfxinput.MouseButtonLeft && core.RectContainsXY(bounds, ev.X, ev.Y) {
			st.dragging = true
			e.sliderUpdateFromMouse(ev.X, ev.Y, bounds)
			return true
		}
	case gfxinput.EventMouseButtonRelease:
		if ev.MouseButton == gfxinput.MouseButtonLeft && st.dragging {
			st.dragging = false
			e.dirty = true
			return true
		}
	case gfxinput.EventKeyPress:
		if e.focused {
			min := e.AttrFloat("min", 0)
			max := e.AttrFloat("max", 100)
			step := e.AttrFloat("step", 0)
			delta := (max - min) / 20
			if step > 0 {
				delta = step
			}
			val := e.AttrFloat("value", min)
			switch ev.Key {
			case gfxinput.KeyLeft, gfxinput.KeyDown:
				e.sliderSetValue(val - delta)
				return true
			case gfxinput.KeyRight, gfxinput.KeyUp:
				e.sliderSetValue(val + delta)
				return true
			case gfxinput.KeyHome:
				e.sliderSetValue(min)
				return true
			case gfxinput.KeyEnd:
				e.sliderSetValue(max)
				return true
			}
		}
	}
	return false
}

func (e *Element) sliderSetValue(v float64) {
	min := e.AttrFloat("min", 0)
	max := e.AttrFloat("max", 100)
	step := e.AttrFloat("step", 0)
	v = clampSnap(v, min, max, step)
	e.attrs["value"] = formatFloat(v)
	e.dirty = true
	e.EmitSignal("changed", v)
}

func (e *Element) sliderUpdateFromMouse(mx, my int, bounds image.Rectangle) {
	min := e.AttrFloat("min", 0)
	max := e.AttrFloat("max", 100)
	step := e.AttrFloat("step", 0)
	var ratio float64
	if e.Attr("orientation", "horizontal") == "vertical" {
		trackLen := bounds.Dy() - sliderThumbW
		if trackLen <= 0 {
			return
		}
		ratio = 1.0 - float64(my-bounds.Min.Y-sliderThumbW/2)/float64(trackLen)
	} else {
		trackLen := bounds.Dx() - sliderThumbW
		if trackLen <= 0 {
			return
		}
		ratio = float64(mx-bounds.Min.X-sliderThumbW/2) / float64(trackLen)
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	v := clampSnap(min+ratio*(max-min), min, max, step)
	e.attrs["value"] = formatFloat(v)
	e.dirty = true
	e.EmitSignal("changed", v)
}

func (e *Element) sliderThumbContains(mx, my int, bounds image.Rectangle) bool {
	if e.Attr("orientation", "horizontal") == "vertical" {
		thumbY := e.sliderValueToPixelV(bounds)
		return core.RectContainsXY(core.RectXYWH(bounds.Min.X+(bounds.Dx()-sliderThumbH)/2, thumbY, sliderThumbH, sliderThumbW), mx, my)
	}
	thumbX := e.sliderValueToPixelH(bounds)
	return core.RectContainsXY(core.RectXYWH(thumbX, bounds.Min.Y+(bounds.Dy()-sliderThumbH)/2, sliderThumbW, sliderThumbH), mx, my)
}

func (e *Element) sliderValueToPixelH(bounds image.Rectangle) int {
	min := e.AttrFloat("min", 0)
	max := e.AttrFloat("max", 100)
	val := e.AttrFloat("value", min)
	trackLen := bounds.Dx() - sliderThumbW
	if max <= min || trackLen <= 0 {
		return bounds.Min.X
	}
	return bounds.Min.X + int((val-min)/(max-min)*float64(trackLen))
}

func (e *Element) sliderValueToPixelV(bounds image.Rectangle) int {
	min := e.AttrFloat("min", 0)
	max := e.AttrFloat("max", 100)
	val := e.AttrFloat("value", min)
	trackLen := bounds.Dy() - sliderThumbW
	if max <= min || trackLen <= 0 {
		return bounds.Min.Y
	}
	return bounds.Min.Y + int((max-val)/(max-min)*float64(trackLen))
}

// --- Min size helpers ---

func (e *Element) editableMinSize() image.Point {
	font := elementFont(e, gfxfont.UIFontParagraph)
	pad := 8
	if e.AttrBool("multiline", false) {
		return image.Point{X: font.Width*20 + pad, Y: font.Height*4 + pad}
	}
	return image.Point{X: font.Width*15 + pad, Y: font.Height + pad}
}

func (e *Element) checkableMinSize() image.Point {
	font := elementFont(e, gfxfont.UIFontParagraph)
	boxSize := font.Height
	gap := 6
	text := e.Attr("text", "")
	textW := font.TextWidth(text)
	textH := font.TextHeight(text)
	h := boxSize
	if textH > h {
		h = textH
	}
	return image.Point{X: boxSize + gap + textW, Y: h}
}

func (e *Element) toggleMinSize() image.Point {
	font := elementFont(e, gfxfont.UIFontParagraph)
	text := e.Attr("text", "")
	textW := font.TextWidth(text)
	w := 42
	if textW > 0 {
		w += 8 + textW
	}
	h := 24
	if font.Height > h {
		h = font.Height
	}
	return image.Point{X: w, Y: h}
}

func (e *Element) listViewMinSize() image.Point {
	font := elementFont(e, gfxfont.UIFontParagraph)
	items := strings.Split(e.Attr("items", ""), "|")
	rowH := e.AttrInt("rowHeight", 34)
	if rowH < font.Height+8 {
		rowH = font.Height + 8
	}
	rows := len(items)
	if rows < 3 {
		rows = 3
	}
	maxW := 120
	for _, it := range items {
		w := font.TextWidth(strings.TrimSpace(it)) + 24
		if w > maxW {
			maxW = w
		}
	}
	return image.Point{X: maxW, Y: rows * rowH}
}

func (e *Element) slidableMinSize() image.Point {
	if e.Attr("orientation", "horizontal") == "vertical" {
		return image.Point{X: sliderThumbH + 4, Y: 80}
	}
	return image.Point{X: 80, Y: sliderThumbH + 4}
}

func (e *Element) progressMinSize() image.Point {
	font := elementFont(e, gfxfont.UIFontParagraph)
	return image.Point{X: 100, Y: font.Height + 8}
}

func (e *Element) frameMinSize() image.Point {
	font := elementTitleFont(e)
	title := e.Attr("title", "")
	titleGap := 4

	titleH := 0
	titleW := 0
	if title != "" {
		titleH = font.Height / 2
		titleW = font.TextWidth(title) + titleGap*4
	}

	cw, ch := e.childrenMinSize()
	pt, pr, pb, pl := e.padding()

	w := cw + pl + pr + 2
	if titleW+2 > w {
		w = titleW + 2
	}
	h := ch + pt + pb + 2 + titleH

	return image.Point{X: w, Y: h}
}

// --- Utility ---

func clampSnap(v, min, max, step float64) float64 {
	if step > 0 {
		v = min + math.Round((v-min)/step)*step
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%d", int(f))
	}
	return fmt.Sprintf("%g", f)
}
