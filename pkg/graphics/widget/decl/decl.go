package decl

import (
	"os"

	"avyos.dev/pkg/graphics/parse"
	ui "avyos.dev/pkg/graphics/widget/engine"
)

// LoadString parses declarative UI source and builds an element tree.
func LoadString(source string, handler interface{}) (*ui.Element, error) {
	ld, err := newLoader()
	if err != nil {
		return nil, err
	}
	return ld.loadString(source, handler)
}

// LoadFile parses a declarative UI file and builds an element tree.
func LoadFile(path string, handler interface{}) (*ui.Element, error) {
	ld, err := newLoader()
	if err != nil {
		return nil, err
	}
	return ld.loadFile(path, handler)
}

type loader struct {
	registry *componentRegistry
}

func newLoader() (*loader, error) {
	reg := newComponentRegistry()
	if err := loadStdLibrary(reg); err != nil {
		return nil, err
	}
	return &loader{registry: reg}, nil
}

func (ld *loader) loadString(source string, handler interface{}) (*ui.Element, error) {
	doc, err := parse.Parse(source)
	if err != nil {
		return nil, err
	}

	for _, imp := range doc.Imports {
		if err := ld.loadImport(imp); err != nil {
			return nil, err
		}
	}

	for _, comp := range doc.Components {
		ld.registry.register(comp)
	}

	if doc.Root == nil {
		return nil, &parse.BuildError{Message: "no root element in UI source"}
	}

	return ld.registry.buildElement(doc.Root, handler)
}

func (ld *loader) loadFile(path string, handler interface{}) (*ui.Element, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &parse.BuildError{Message: "failed to read UI file: " + err.Error()}
	}
	return ld.loadString(string(data), handler)
}

func (ld *loader) loadImport(imp parse.ImportDecl) error {
	if src, ok := stdSources[imp.Path]; ok {
		doc, err := parse.Parse(src)
		if err != nil {
			return &parse.BuildError{
				Line:    imp.Line,
				Message: "failed to parse import " + imp.Path + ": " + err.Error(),
			}
		}
		for _, comp := range doc.Components {
			ld.registry.register(comp)
		}
		return nil
	}

	data, err := os.ReadFile(imp.Path)
	if err != nil {
		return &parse.BuildError{
			Line:    imp.Line,
			Message: "failed to load import " + imp.Path + ": " + err.Error(),
		}
	}
	doc, err := parse.Parse(string(data))
	if err != nil {
		return &parse.BuildError{
			Line:    imp.Line,
			Message: "failed to parse import " + imp.Path + ": " + err.Error(),
		}
	}
	for _, comp := range doc.Components {
		ld.registry.register(comp)
	}
	return nil
}

// FindElement searches the tree for an element by id.
func FindElement(root *ui.Element, id string) *ui.Element {
	if root == nil {
		return nil
	}
	if root.ID() == id {
		return root
	}
	return root.FindChild(id)
}

// CollectFocusable collects all elements with focusable=true.
func CollectFocusable(root *ui.Element) []*ui.Element {
	var result []*ui.Element
	collectFocusableRec(root, &result)
	return result
}

func collectFocusableRec(e *ui.Element, result *[]*ui.Element) {
	if e == nil {
		return
	}
	if e.AttrBool("focusable", false) {
		*result = append(*result, e)
	}
	for _, child := range e.ChildElements() {
		collectFocusableRec(child, result)
	}
}
