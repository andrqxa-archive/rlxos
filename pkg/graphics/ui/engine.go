package ui

import "os"

// Engine loads .ui files and builds Element trees.
type Engine struct {
	Registry *ComponentRegistry
}

// NewEngine creates a new Engine with the standard library loaded.
func NewEngine() (*Engine, error) {
	reg := NewComponentRegistry()
	if err := loadStdLibrary(reg); err != nil {
		return nil, err
	}
	return &Engine{Registry: reg}, nil
}

// LoadString parses a .ui source string and builds the root Element.
// The handler receives signal connections (onXxx: MethodName).
func (eng *Engine) LoadString(source string, handler interface{}) (*Element, error) {
	doc, err := Parse(source)
	if err != nil {
		return nil, err
	}

	// Process imports
	for _, imp := range doc.Imports {
		if err := eng.loadImport(imp); err != nil {
			return nil, err
		}
	}

	// Register inline component definitions
	for _, comp := range doc.Components {
		eng.Registry.Register(comp)
	}

	// Build root element
	if doc.Root == nil {
		return nil, &BuildError{Message: "no root element in UI source"}
	}

	return eng.Registry.BuildElement(doc.Root, handler)
}

// LoadFile loads a .ui file from disk and builds the root Element.
func (eng *Engine) LoadFile(path string, handler interface{}) (*Element, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &BuildError{Message: "failed to read UI file: " + err.Error()}
	}
	return eng.LoadString(string(data), handler)
}

// loadImport resolves an import path and registers its components.
func (eng *Engine) loadImport(imp ImportDecl) error {
	// Check standard library
	if src, ok := stdSources[imp.Path]; ok {
		doc, err := Parse(src)
		if err != nil {
			return &BuildError{
				Line:    imp.Line,
				Message: "failed to parse import " + imp.Path + ": " + err.Error(),
			}
		}
		for _, comp := range doc.Components {
			eng.Registry.Register(comp)
		}
		return nil
	}

	// Try loading from filesystem (relative path)
	data, err := os.ReadFile(imp.Path)
	if err != nil {
		return &BuildError{
			Line:    imp.Line,
			Message: "failed to load import " + imp.Path + ": " + err.Error(),
		}
	}
	doc, err := Parse(string(data))
	if err != nil {
		return &BuildError{
			Line:    imp.Line,
			Message: "failed to parse import " + imp.Path + ": " + err.Error(),
		}
	}
	for _, comp := range doc.Components {
		eng.Registry.Register(comp)
	}
	return nil
}

// FindElement searches the tree for an element by id.
func FindElement(root *Element, id string) *Element {
	if root == nil {
		return nil
	}
	if root.id == id {
		return root
	}
	return root.FindChild(id)
}

// CollectFocusable collects all elements with focusable=true.
func CollectFocusable(root *Element) []*Element {
	var result []*Element
	collectFocusableRec(root, &result)
	return result
}

func collectFocusableRec(e *Element, result *[]*Element) {
	if e == nil {
		return
	}
	if e.AttrBool("focusable", false) {
		*result = append(*result, e)
	}
	for _, child := range e.children {
		collectFocusableRec(child, result)
	}
}
