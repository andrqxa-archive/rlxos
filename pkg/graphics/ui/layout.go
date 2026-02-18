package ui

import (
	"fmt"

	"avyos.dev/pkg/graphics"
)

type flowItem struct {
	child *Element
	x     int
	y     int
	w     int
	h     int
}

func (e *Element) layoutChildren() {
	if len(e.children) == 0 {
		return
	}

	layoutMode := e.Attr("layout", "")
	switch layoutMode {
	case "grid":
		e.layoutGrid()
	case "stack":
		e.layoutStack()
	case "flow":
		e.layoutFlow()
	default:
		e.layoutFlex()
	}
}

// layoutFlex arranges children in a row or column (CSS flexbox-like).
func (e *Element) layoutFlex() {
	content := e.contentArea()
	if content.W <= 0 || content.H <= 0 {
		return
	}

	visible := make([]*Element, 0, len(e.children))
	for _, child := range e.children {
		if child.visible {
			visible = append(visible, child)
		}
	}
	if len(visible) == 0 {
		return
	}

	spacing := e.AttrInt("spacing", 0)
	totalSpacing := spacing * (len(visible) - 1)
	isRow := e.isRow()
	alignment := e.Attr("alignment", "stretch")

	var mainAvail, crossAvail int
	if isRow {
		mainAvail = content.W
		crossAvail = content.H
	} else {
		mainAvail = content.H
		crossAvail = content.W
	}

	mainSizes := make([]int, len(visible))
	crossMins := make([]int, len(visible))
	weights := make([]int, len(visible))
	fixedTotal := 0
	totalWeight := 0

	for i, child := range visible {
		ms := child.MinSize()
		mainMin := axisSize(ms, isRow)
		crossMins[i] = crossAxisSize(ms, isRow)

		if w := child.AttrInt("flex", 0); w > 0 {
			weights[i] = w
		} else if child.AttrBool("expand", false) {
			weights[i] = 1
		}

		if weights[i] <= 0 {
			mainSizes[i] = mainMin
			fixedTotal += mainSizes[i]
			continue
		}

		if mainMin < 0 {
			mainMin = 0
		}
		mainSizes[i] = mainMin
		totalWeight += weights[i]
	}

	remaining := mainAvail - totalSpacing - fixedTotal
	if remaining < 0 {
		remaining = 0
	}

	if totalWeight > 0 {
		used := 0
		for i := range visible {
			if weights[i] <= 0 {
				continue
			}
			share := remaining * weights[i] / totalWeight
			if share > mainSizes[i] {
				mainSizes[i] = share
			}
			used += share
		}

		remainder := remaining - used
		for i := range visible {
			if remainder <= 0 {
				break
			}
			if weights[i] <= 0 {
				continue
			}
			mainSizes[i]++
			remainder--
		}
	}

	contentMain := totalSpacing
	for _, v := range mainSizes {
		contentMain += v
	}

	scrollY := 0
	if !isRow && e.overflowScrollable() {
		maxScroll := contentMain - mainAvail
		if maxScroll < 0 {
			maxScroll = 0
		}
		scrollY = e.AttrInt("scrollY", 0)
		if scrollY < 0 {
			scrollY = 0
		}
		if scrollY > maxScroll {
			scrollY = maxScroll
		}
		e.attrs["scrollY"] = fmt.Sprintf("%d", scrollY)
	} else if e.AttrInt("scrollY", 0) != 0 {
		e.attrs["scrollY"] = "0"
	}

	pos := 0
	if !isRow {
		pos = -scrollY
	}

	for i, child := range visible {
		mainSize := mainSizes[i]
		crossSize := alignCross(alignment, crossAvail, crossMins[i])

		explicitCross := explicitCrossSize(child, isRow)
		if explicitCross > 0 {
			crossSize = explicitCross
		}

		minCross := minCrossSize(child, isRow)
		if minCross > 0 && crossSize < minCross {
			crossSize = minCross
		}
		maxCross := maxCrossSize(child, isRow)
		if maxCross > 0 && crossSize > maxCross {
			crossSize = maxCross
		}

		crossPos := alignCrossPos(alignment, crossAvail, crossSize)
		if isRow {
			child.SetBounds(graphics.Rect{
				X: content.X + pos,
				Y: content.Y + crossPos,
				W: mainSize,
				H: crossSize,
			})
		} else {
			child.SetBounds(graphics.Rect{
				X: content.X + crossPos,
				Y: content.Y + pos,
				W: crossSize,
				H: mainSize,
			})
		}
		pos += mainSize + spacing
	}
}

func alignCross(alignment string, available, minSize int) int {
	if alignment == "stretch" {
		return available
	}
	if minSize > 0 {
		return minSize
	}
	return available
}

func alignCrossPos(alignment string, available, size int) int {
	switch alignment {
	case "center":
		return (available - size) / 2
	case "end":
		return available - size
	default:
		return 0
	}
}

// layoutFlow arranges children left-to-right and wraps into rows as needed.
func (e *Element) layoutFlow() {
	content := e.contentArea()
	if content.W <= 0 || content.H <= 0 {
		return
	}

	colSpacing := e.AttrInt("colSpacing", e.AttrInt("spacing", 0))
	rowSpacing := e.AttrInt("rowSpacing", e.AttrInt("spacing", 0))

	items := make([]flowItem, 0, len(e.children))
	x := content.X
	y := content.Y
	rowHeight := 0
	lineRight := content.X + content.W

	for _, child := range e.children {
		if !child.visible {
			continue
		}

		ms := child.MinSize()
		w := ms.X
		h := ms.Y
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}

		if x > content.X && x+w > lineRight {
			x = content.X
			y += rowHeight + rowSpacing
			rowHeight = 0
		}

		items = append(items, flowItem{
			child: child,
			x:     x,
			y:     y,
			w:     w,
			h:     h,
		})

		if h > rowHeight {
			rowHeight = h
		}
		x += w + colSpacing
	}

	contentMain := 0
	if len(items) > 0 {
		contentMain = (y - content.Y) + rowHeight
	}

	scrollY := 0
	if e.overflowScrollable() {
		maxScroll := contentMain - content.H
		if maxScroll < 0 {
			maxScroll = 0
		}
		scrollY = e.AttrInt("scrollY", 0)
		if scrollY < 0 {
			scrollY = 0
		}
		if scrollY > maxScroll {
			scrollY = maxScroll
		}
		e.attrs["scrollY"] = fmt.Sprintf("%d", scrollY)
	} else if e.AttrInt("scrollY", 0) != 0 {
		e.attrs["scrollY"] = "0"
	}

	for _, item := range items {
		item.child.SetBounds(graphics.Rect{
			X: item.x,
			Y: item.y - scrollY,
			W: item.w,
			H: item.h,
		})
	}
}

// layoutGrid arranges children in a grid based on row/col attributes.
func (e *Element) layoutGrid() {
	content := e.contentArea()
	if content.W <= 0 || content.H <= 0 {
		return
	}

	colSpacing := e.AttrInt("colSpacing", e.AttrInt("spacing", 0))
	rowSpacing := e.AttrInt("rowSpacing", e.AttrInt("spacing", 0))

	// Find max row and col
	maxRow, maxCol := 0, 0
	for _, child := range e.children {
		r := child.AttrInt("row", 0)
		c := child.AttrInt("col", 0)
		rs := child.AttrInt("rowSpan", 1)
		cs := child.AttrInt("colSpan", 1)
		if r+rs > maxRow {
			maxRow = r + rs
		}
		if c+cs > maxCol {
			maxCol = c + cs
		}
	}
	if maxRow == 0 || maxCol == 0 {
		return
	}

	cellW := (content.W - colSpacing*(maxCol-1)) / maxCol
	cellH := (content.H - rowSpacing*(maxRow-1)) / maxRow

	for _, child := range e.children {
		if !child.visible {
			continue
		}
		r := child.AttrInt("row", 0)
		c := child.AttrInt("col", 0)
		rs := child.AttrInt("rowSpan", 1)
		cs := child.AttrInt("colSpan", 1)

		x := content.X + c*(cellW+colSpacing)
		y := content.Y + r*(cellH+rowSpacing)
		w := cellW*cs + colSpacing*(cs-1)
		h := cellH*rs + rowSpacing*(rs-1)

		child.SetBounds(graphics.Rect{X: x, Y: y, W: w, H: h})
	}
}

// layoutStack shows only the active child (all children fill the content area).
func (e *Element) layoutStack() {
	content := e.contentArea()
	active := e.AttrInt("active", 0)

	for i, child := range e.children {
		child.SetBounds(content)
		child.SetVisible(i == active)
	}
}

// childrenMinSize computes the aggregate min size of all visible children.
func (e *Element) childrenMinSize() (w, h int) {
	visible := make([]*Element, 0, len(e.children))
	for _, child := range e.children {
		if child.visible {
			visible = append(visible, child)
		}
	}
	if len(visible) == 0 {
		return 0, 0
	}

	spacing := e.AttrInt("spacing", 0)
	totalSpacing := spacing * (len(visible) - 1)
	isRow := e.isRow()

	if isRow {
		totalW := totalSpacing
		maxH := 0
		for _, child := range visible {
			ms := child.MinSize()
			totalW += ms.X
			if ms.Y > maxH {
				maxH = ms.Y
			}
		}
		return totalW, maxH
	}

	maxW := 0
	totalH := totalSpacing
	for _, child := range visible {
		ms := child.MinSize()
		totalH += ms.Y
		if ms.X > maxW {
			maxW = ms.X
		}
	}
	return maxW, totalH
}

func axisSize(ms graphics.Point, isRow bool) int {
	if isRow {
		return ms.X
	}
	return ms.Y
}

func crossAxisSize(ms graphics.Point, isRow bool) int {
	if isRow {
		return ms.Y
	}
	return ms.X
}

func explicitCrossSize(child *Element, isRow bool) int {
	if isRow {
		return child.AttrInt("height", 0)
	}
	return child.AttrInt("width", 0)
}

func minCrossSize(child *Element, isRow bool) int {
	if isRow {
		return child.AttrInt("minHeight", 0)
	}
	return child.AttrInt("minWidth", 0)
}

func maxCrossSize(child *Element, isRow bool) int {
	if isRow {
		return child.AttrInt("maxHeight", 0)
	}
	return child.AttrInt("maxWidth", 0)
}
