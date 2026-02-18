package ui

import (
	"strings"
	"time"
)

// textInputState holds mutable state for single-line text editing.
type textInputState struct {
	text         []rune
	cursorPos    int
	scrollOffset int
	cursorBlink  bool
	lastBlink    time.Time
}

// textAreaState holds mutable state for multiline text editing.
type textAreaState struct {
	lines       [][]rune
	cursorRow   int
	cursorCol   int
	scrollRow   int
	scrollCol   int
	cursorBlink bool
	lastBlink   time.Time

	terminalLines      [][]TerminalCell
	terminalCursorRow  int
	terminalCursorCol  int
	terminalShowCursor bool
}

// sliderState holds mutable state for slider drag tracking.
type sliderState struct {
	dragging bool
}

func (e *Element) getTextInputState() *textInputState {
	if st, ok := e.state.(*textInputState); ok {
		return st
	}
	st := &textInputState{cursorBlink: true, lastBlink: time.Now()}
	if t := e.Attr("text", ""); t != "" {
		st.text = []rune(t)
	}
	e.state = st
	return st
}

func (e *Element) getTextAreaState() *textAreaState {
	if st, ok := e.state.(*textAreaState); ok {
		return st
	}
	st := &textAreaState{
		lines:       [][]rune{{}},
		cursorBlink: true,
		lastBlink:   time.Now(),
	}
	if t := e.Attr("text", ""); t != "" {
		rawLines := strings.Split(t, "\n")
		st.lines = make([][]rune, len(rawLines))
		for i, l := range rawLines {
			st.lines[i] = []rune(l)
		}
	}
	e.state = st
	return st
}

func (e *Element) getSliderState() *sliderState {
	if st, ok := e.state.(*sliderState); ok {
		return st
	}
	st := &sliderState{}
	e.state = st
	return st
}
