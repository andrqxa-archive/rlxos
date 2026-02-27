package engine

import (
	"reflect"
)

// BuildMenuPopup clones a menu subtree and returns popup content plus dimensions.
func BuildMenuPopup(menu *Element, closeFn func()) (*Element, int, int, bool) {
	if menu == nil {
		return nil, 0, 0, false
	}

	content := cloneElementTree(menu)
	if content == nil {
		return nil, 0, 0, false
	}
	content.SetVisible(true)
	wrapMenuActions(content, closeFn)

	w, h := measureMenuPopup(menu, content)
	return content, w, h, true
}

func measureMenuPopup(src, cloned *Element) (int, int) {
	w := src.AttrInt("menuWidth", 0)
	if w <= 0 {
		w = src.AttrInt("minWidth", 0)
	}
	if w <= 0 {
		w = 220
	}
	if maxW := src.AttrInt("maxWidth", 0); maxW > 0 && w > maxW {
		w = maxW
	}

	h := src.AttrInt("menuHeight", 0)
	if h > 0 {
		return w, h
	}

	top, _, bottom, _ := parsePaddingStr(src.Attr("padding", "8"))
	spacing := src.AttrInt("spacing", 4)
	if spacing < 0 {
		spacing = 0
	}

	items := 0
	h = top + bottom
	for _, child := range cloned.children {
		if !child.visible {
			continue
		}
		rowH := child.AttrInt("minHeight", 34)
		if rowH <= 0 {
			rowH = 34
		}
		h += rowH
		items++
	}
	if items > 1 {
		h += spacing * (items - 1)
	}
	if h < 28 {
		h = 28
	}
	return w, h
}

func cloneElementTree(src *Element) *Element {
	if src == nil {
		return nil
	}
	dst := NewElement(src.tag)
	dst.id = src.id
	dst.visible = src.visible
	dst.minW = src.minW
	dst.minH = src.minH

	for k, v := range src.attrs {
		dst.attrs[k] = v
	}
	for k, fn := range src.signals {
		dst.signals[k] = fn
	}

	for _, child := range src.children {
		clone := cloneElementTree(child)
		if clone != nil {
			dst.AddChild(clone)
		}
	}
	return dst
}

func wrapMenuActions(root *Element, closeFn func()) {
	if root == nil {
		return
	}
	for _, child := range root.children {
		wrapMenuActions(child, closeFn)
	}
	if root.tag != "MenuItem" {
		return
	}
	orig, hasOrig := root.signals["clicked"]
	root.signals["clicked"] = reflect.ValueOf(func() {
		if hasOrig {
			callSignalNoArgs(orig)
		}
		if closeFn != nil {
			closeFn()
		}
	})
}

func callSignalNoArgs(fn reflect.Value) {
	if !fn.IsValid() {
		return
	}
	ft := fn.Type()
	if ft.Kind() != reflect.Func || ft.NumIn() != 0 {
		return
	}
	fn.Call(nil)
}
