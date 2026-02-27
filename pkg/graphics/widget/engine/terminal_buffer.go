package engine

import (
	"time"

	gfxfont "avyos.dev/pkg/graphics/font"
	graphics "avyos.dev/pkg/graphics/input"
)

// TerminalCell holds one terminal glyph with resolved foreground/background colors.
type TerminalCell struct {
	Char      rune
	Fg        graphics.Color
	Bg        graphics.Color
	Underline bool
}

// SetTerminalBuffer updates a multiline element with terminal-styled cells.
// It keeps the text-area scroll state while replacing its backing lines.
func (e *Element) SetTerminalBuffer(lines [][]TerminalCell, cursorCol, cursorRow int, showCursor bool) {
	if e == nil || !e.AttrBool("editable", false) || !e.AttrBool("multiline", false) {
		return
	}

	st := e.getTextAreaState()
	oldLines := st.terminalLines
	oldCursorCol := st.terminalCursorCol
	oldCursorRow := st.terminalCursorRow
	oldShowCursor := st.terminalShowCursor
	oldScrollRow := st.scrollRow
	oldScrollCol := st.scrollCol
	oldMaxCols := terminalMaxCols(oldLines)

	st.terminalLines = lines
	st.terminalCursorCol = cursorCol
	st.terminalCursorRow = cursorRow
	st.terminalShowCursor = showCursor

	if len(lines) == 0 {
		st.lines = [][]rune{{}}
		st.cursorRow = 0
		st.cursorCol = 0
		st.scrollRow = 0
		st.scrollCol = 0
		st.cursorBlink = true
		st.lastBlink = time.Now()
		e.clearDirtyRects()
		e.dirty = true
		return
	}

	st.lines = make([][]rune, len(lines))
	for i := range lines {
		row := make([]rune, len(lines[i]))
		for j := range lines[i] {
			ch := lines[i][j].Char
			if ch == 0 {
				ch = ' '
			}
			row[j] = ch
		}
		st.lines[i] = row
	}

	st.cursorRow = cursorRow
	st.cursorCol = cursorCol
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

	dirtyRects, fullDirty := e.terminalDirtyRects(
		oldLines,
		lines,
		oldCursorCol,
		oldCursorRow,
		oldShowCursor,
		cursorCol,
		cursorRow,
		showCursor,
		oldScrollRow,
		oldScrollCol,
		st.scrollRow,
		st.scrollCol,
		oldMaxCols,
	)
	if fullDirty {
		e.clearDirtyRects()
		e.dirty = true
		return
	}
	if len(dirtyRects) == 0 {
		return
	}
	e.clearDirtyRects()
	for _, rect := range dirtyRects {
		e.addDirtyRect(rect)
	}
}

func terminalNormalizeCell(cell TerminalCell) TerminalCell {
	if cell.Char == 0 {
		cell.Char = ' '
	}
	if cell.Bg.A == 0 {
		cell.Bg = graphics.ColorTransparent
	}
	return cell
}

func terminalCellAt(lines [][]TerminalCell, row, col int) TerminalCell {
	if row < 0 || row >= len(lines) {
		return TerminalCell{Char: ' ', Bg: graphics.ColorTransparent}
	}
	line := lines[row]
	if col < 0 || col >= len(line) {
		return TerminalCell{Char: ' ', Bg: graphics.ColorTransparent}
	}
	return terminalNormalizeCell(line[col])
}

func terminalMaxCols(lines [][]TerminalCell) int {
	maxCols := 0
	for _, line := range lines {
		if len(line) > maxCols {
			maxCols = len(line)
		}
	}
	return maxCols
}

func terminalMaxScroll(totalRows, visRows int) int {
	maxScroll := totalRows - visRows
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func terminalTextWidth(e *Element, content graphics.Rect, maxScroll int) (int, graphics.Rect) {
	textW := content.W
	if textW <= 0 {
		return 0, graphics.Rect{}
	}

	var trackRect graphics.Rect
	if e.AttrBool("showScrollbar", false) && maxScroll > 0 {
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
		trackRect = graphics.Rect{
			X: content.X + content.W - barW - margin,
			Y: content.Y + margin,
			W: barW,
			H: content.H - margin*2,
		}
		if trackRect.X > content.X && trackRect.H >= 8 {
			textW = trackRect.X - content.X - 1
			if textW < 1 {
				textW = 1
			}
		} else {
			trackRect = graphics.Rect{}
		}
	}

	return textW, trackRect
}

func terminalCursorRect(
	textArea graphics.Rect,
	textRect graphics.Rect,
	cellW, cellH, scrollCol, scrollRow, cursorCol, cursorRow int,
) graphics.Rect {
	if cursorRow < scrollRow {
		return graphics.Rect{}
	}
	relRow := cursorRow - scrollRow
	if relRow < 0 {
		return graphics.Rect{}
	}
	relCol := cursorCol - scrollCol
	if relCol < 0 {
		return graphics.Rect{}
	}
	cx := textArea.X + relCol*cellW
	cy := textArea.Y + relRow*cellH
	rect := graphics.Rect{X: cx, Y: cy, W: 2, H: cellH}
	return rect.Intersection(textRect)
}

func (e *Element) terminalDirtyRects(
	oldLines [][]TerminalCell,
	newLines [][]TerminalCell,
	oldCursorCol, oldCursorRow int,
	oldShowCursor bool,
	newCursorCol, newCursorRow int,
	newShowCursor bool,
	oldScrollRow, oldScrollCol int,
	newScrollRow, newScrollCol int,
	oldMaxCols int,
) ([]graphics.Rect, bool) {
	if e.AttrBool("wrap", false) {
		return nil, true
	}

	font := elementFont(e, gfxfont.UIFontParagraph)
	if font == nil {
		return nil, true
	}

	cellH := font.Height
	if cellH <= 0 {
		cellH = font.TextHeight("W")
	}
	if cellH <= 0 {
		cellH = 16
	}
	cellW := font.Width
	if cellW <= 0 {
		cellW = font.TextWidth("W")
	}
	if cellW <= 0 {
		cellW = 8
	}

	textArea := e.contentArea()
	if textArea.W <= 0 || textArea.H <= 0 {
		return nil, true
	}
	visRows := textArea.H / cellH
	if visRows <= 0 {
		return nil, true
	}

	oldMaxScroll := terminalMaxScroll(len(oldLines), visRows)
	newMaxScroll := terminalMaxScroll(len(newLines), visRows)
	oldTextW, oldScrollTrack := terminalTextWidth(e, textArea, oldMaxScroll)
	newTextW, newScrollTrack := terminalTextWidth(e, textArea, newMaxScroll)
	if oldTextW <= 0 || newTextW <= 0 {
		return nil, true
	}
	if oldTextW != newTextW {
		return nil, true
	}
	if oldScrollRow != newScrollRow || oldScrollCol != newScrollCol {
		return nil, true
	}

	visCols := newTextW / cellW
	if visCols <= 0 {
		return nil, true
	}

	textH := visRows * cellH
	if textH > textArea.H {
		textH = textArea.H
	}
	textRect := graphics.Rect{X: textArea.X, Y: textArea.Y, W: newTextW, H: textH}

	if len(oldLines) == 0 || oldMaxCols == 0 {
		dirty := []graphics.Rect{textRect}
		if !oldScrollTrack.IsEmpty() || !newScrollTrack.IsEmpty() {
			dirty = append(dirty, oldScrollTrack, newScrollTrack)
		}
		return dirty, false
	}

	dirty := make([]graphics.Rect, 0, 24)
	for row := 0; row < visRows; row++ {
		lineIdx := newScrollRow + row
		segStart := -1
		for col := 0; col < visCols; col++ {
			cellIdx := newScrollCol + col
			oldCell := terminalCellAt(oldLines, lineIdx, cellIdx)
			newCell := terminalCellAt(newLines, lineIdx, cellIdx)
			if oldCell != newCell {
				if segStart < 0 {
					segStart = col
				}
				continue
			}
			if segStart >= 0 {
				dirty = append(dirty, graphics.Rect{
					X: textArea.X + segStart*cellW,
					Y: textArea.Y + row*cellH,
					W: (col - segStart) * cellW,
					H: cellH,
				}.Intersection(textRect))
				segStart = -1
			}
		}
		if segStart >= 0 {
			dirty = append(dirty, graphics.Rect{
				X: textArea.X + segStart*cellW,
				Y: textArea.Y + row*cellH,
				W: (visCols - segStart) * cellW,
				H: cellH,
			}.Intersection(textRect))
		}
	}

	if oldShowCursor {
		if r := terminalCursorRect(textArea, textRect, cellW, cellH, oldScrollCol, oldScrollRow, oldCursorCol, oldCursorRow); !r.IsEmpty() {
			dirty = append(dirty, r)
		}
	}
	if newShowCursor {
		if r := terminalCursorRect(textArea, textRect, cellW, cellH, newScrollCol, newScrollRow, newCursorCol, newCursorRow); !r.IsEmpty() {
			dirty = append(dirty, r)
		}
	}

	if !oldScrollTrack.IsEmpty() || !newScrollTrack.IsEmpty() {
		if oldMaxScroll != newMaxScroll {
			dirty = append(dirty, oldScrollTrack, newScrollTrack)
		}
	}

	out := make([]graphics.Rect, 0, len(dirty))
	for _, rect := range dirty {
		if !rect.IsEmpty() {
			out = append(out, rect)
		}
	}
	return out, false
}
