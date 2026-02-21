package ui

import "reflect"

// ComponentRegistry holds parsed component definitions keyed by name.
type ComponentRegistry struct {
	defs map[string]*ComponentDef
}

// NewComponentRegistry creates an empty registry.
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{defs: make(map[string]*ComponentDef)}
}

// Register adds a component definition.
func (r *ComponentRegistry) Register(def *ComponentDef) {
	r.defs[def.Name] = def
}

// Lookup returns a component definition by name, or nil.
func (r *ComponentRegistry) Lookup(name string) *ComponentDef {
	return r.defs[name]
}

// Has returns true if the registry has a component with the given name.
func (r *ComponentRegistry) Has(name string) bool {
	_, ok := r.defs[name]
	return ok
}

// Instantiate creates an Element from a component definition and an AST node.
// The node provides instance-specific property overrides and children.
func (r *ComponentRegistry) Instantiate(name string, node *Node, handler interface{}) (*Element, error) {
	def := r.defs[name]
	if def == nil {
		return nil, &BuildError{
			Line:    node.Line,
			Element: name,
			Message: "unknown component: " + name,
		}
	}

	e := NewElement(name)

	// Apply default attributes from the component definition
	for _, prop := range def.Defaults {
		e.attrs[prop.Name] = prop.Value.Str
	}

	// Apply declared property defaults
	for _, prop := range def.Properties {
		if _, exists := e.attrs[prop.Name]; !exists {
			e.attrs[prop.Name] = prop.Value.Str
		}
	}
	if vis, ok := e.attrs["visible"]; ok && vis != "" {
		e.visible = toBool(vis)
	}

	// Apply instance overrides from the node
	for _, prop := range node.Properties {
		key := prop.Name
		val := prop.Value.Str

		if applyBuiltinProperty(e, name, key, val, handler) {
			continue
		}

		// Handle signal connections: onXxx -> connect to handler method
		if len(key) > 2 && key[:2] == "on" && handler != nil {
			sigName := lcFirst(key[2:])
			methodName := val
			connectSignal(e, sigName, handler, methodName)
			continue
		}

		// Handle id
		if key == "id" {
			e.id = val
			continue
		}

		e.attrs[key] = val
	}
	if name == "MenuItem" && e.attrs["text"] == "" && e.attrs["name"] != "" {
		e.attrs["text"] = e.attrs["name"]
	}

	// Build child elements from the node
	for _, childNode := range node.Children {
		child, err := r.BuildElement(childNode, handler)
		if err != nil {
			return nil, err
		}
		e.AddChild(child)
	}

	// Build template children from the component definition
	for _, childNode := range def.Children {
		child, err := r.BuildElement(childNode, handler)
		if err != nil {
			return nil, err
		}
		e.AddChild(child)
	}

	return e, nil
}

// BuildElement builds an Element from an AST node, resolving components.
func (r *ComponentRegistry) BuildElement(node *Node, handler interface{}) (*Element, error) {
	// Check if this is a registered component
	if r.Has(node.TypeName) {
		return r.Instantiate(node.TypeName, node, handler)
	}

	// Otherwise build a plain element
	e := NewElement(node.TypeName)

	for _, prop := range node.Properties {
		key := prop.Name
		val := prop.Value.Str

		if applyBuiltinProperty(e, node.TypeName, key, val, handler) {
			continue
		}

		if len(key) > 2 && key[:2] == "on" && handler != nil {
			sigName := lcFirst(key[2:])
			methodName := val
			connectSignal(e, sigName, handler, methodName)
			continue
		}

		if key == "id" {
			e.id = val
			continue
		}

		e.attrs[key] = val
	}
	if node.TypeName == "MenuItem" && e.attrs["text"] == "" && e.attrs["name"] != "" {
		e.attrs["text"] = e.attrs["name"]
	}

	for _, childNode := range node.Children {
		child, err := r.BuildElement(childNode, handler)
		if err != nil {
			return nil, err
		}
		e.AddChild(child)
	}

	return e, nil
}

// connectSignal connects a signal on an element to a method on the handler.
func connectSignal(e *Element, sigName string, handler interface{}, methodName string) {
	if handler == nil || methodName == "" {
		return
	}
	hv := reflect.ValueOf(handler)
	method := hv.MethodByName(methodName)
	if method.IsValid() {
		e.signals[sigName] = method
	}
}

func applyBuiltinProperty(e *Element, typeName, key, value string, handler interface{}) bool {
	switch key {
	case "visible":
		e.attrs[key] = value
		e.visible = toBool(value)
		return true
	case "name":
		if typeName == "MenuItem" {
			e.attrs[key] = value
			e.attrs["text"] = value
			return true
		}
	case "action":
		if typeName == "MenuItem" {
			if handler != nil && value != "" {
				connectSignal(e, "clicked", handler, value)
			}
			return true
		}
	}
	return false
}

// lcFirst lowercases the first character of a string.
func lcFirst(s string) string {
	if s == "" {
		return s
	}
	first := s[0]
	if first >= 'A' && first <= 'Z' {
		return string(first+32) + s[1:]
	}
	return s
}
