package decl

import (
	"reflect"
	"strings"

	"avyos.dev/pkg/graphics/parse"
	ui "avyos.dev/pkg/graphics/widget/engine"
)

// componentRegistry holds parsed component definitions keyed by name.
type componentRegistry struct {
	defs map[string]*parse.ComponentDef
}

// newComponentRegistry creates an empty registry.
func newComponentRegistry() *componentRegistry {
	return &componentRegistry{defs: make(map[string]*parse.ComponentDef)}
}

// register adds a component definition.
func (r *componentRegistry) register(def *parse.ComponentDef) {
	r.defs[def.Name] = def
}

// has returns true if the registry has a component with the given name.
func (r *componentRegistry) has(name string) bool {
	_, ok := r.defs[name]
	return ok
}

// instantiate creates an element from a component definition and an AST node.
// The node provides instance-specific property overrides and children.
func (r *componentRegistry) instantiate(name string, node *parse.Node, handler interface{}) (*ui.Element, error) {
	def := r.defs[name]
	if def == nil {
		return nil, &parse.BuildError{
			Line:    node.Line,
			Element: name,
			Message: "unknown component: " + name,
		}
	}

	e := ui.NewElement(name)

	// Apply default attributes from the component definition.
	for _, prop := range def.Defaults {
		e.SetAttribute(prop.Name, prop.Value.Str)
		if prop.Name == "visible" {
			e.SetVisible(parseBool(prop.Value.Str))
		}
	}

	// Apply declared property defaults.
	for _, prop := range def.Properties {
		if !e.HasAttribute(prop.Name) {
			e.SetAttribute(prop.Name, prop.Value.Str)
			if prop.Name == "visible" {
				e.SetVisible(parseBool(prop.Value.Str))
			}
		}
	}

	// Apply instance overrides from the node.
	for _, prop := range node.Properties {
		key := prop.Name
		val := prop.Value.Str

		if applyBuiltinProperty(e, name, key, val, handler) {
			continue
		}

		// Handle signal connections: onXxx -> connect to handler method.
		if strings.HasPrefix(key, "on") && len(key) > 2 && handler != nil {
			sigName := lcFirst(key[2:])
			methodName := val
			connectSignal(e, sigName, handler, methodName)
			continue
		}

		if key == "id" {
			e.SetID(val)
			continue
		}

		e.SetAttribute(key, val)
	}

	if name == "MenuItem" && e.GetAttribute("text") == "" && e.GetAttribute("name") != "" {
		e.SetAttribute("text", e.GetAttribute("name"))
	}

	// Build child elements from the node.
	for _, childNode := range node.Children {
		child, err := r.buildElement(childNode, handler)
		if err != nil {
			return nil, err
		}
		e.AddChild(child)
	}

	// Build template children from the component definition.
	for _, childNode := range def.Children {
		child, err := r.buildElement(childNode, handler)
		if err != nil {
			return nil, err
		}
		e.AddChild(child)
	}

	return e, nil
}

// buildElement builds an element from an AST node, resolving components.
func (r *componentRegistry) buildElement(node *parse.Node, handler interface{}) (*ui.Element, error) {
	if r.has(node.TypeName) {
		return r.instantiate(node.TypeName, node, handler)
	}

	e := ui.NewElement(node.TypeName)

	for _, prop := range node.Properties {
		key := prop.Name
		val := prop.Value.Str

		if applyBuiltinProperty(e, node.TypeName, key, val, handler) {
			continue
		}

		if strings.HasPrefix(key, "on") && len(key) > 2 && handler != nil {
			sigName := lcFirst(key[2:])
			methodName := val
			connectSignal(e, sigName, handler, methodName)
			continue
		}

		if key == "id" {
			e.SetID(val)
			continue
		}

		e.SetAttribute(key, val)
	}

	if node.TypeName == "MenuItem" && e.GetAttribute("text") == "" && e.GetAttribute("name") != "" {
		e.SetAttribute("text", e.GetAttribute("name"))
	}

	for _, childNode := range node.Children {
		child, err := r.buildElement(childNode, handler)
		if err != nil {
			return nil, err
		}
		e.AddChild(child)
	}

	return e, nil
}

// connectSignal connects a signal on an element to a method on the handler.
func connectSignal(e *ui.Element, sigName string, handler interface{}, methodName string) {
	if handler == nil || methodName == "" {
		return
	}
	hv := reflect.ValueOf(handler)
	method := hv.MethodByName(methodName)
	if method.IsValid() && method.Kind() == reflect.Func {
		_ = e.BindSignal(sigName, method.Interface())
	}
}

func applyBuiltinProperty(e *ui.Element, typeName, key, value string, handler interface{}) bool {
	switch key {
	case "visible":
		e.SetAttribute(key, value)
		e.SetVisible(parseBool(value))
		return true
	case "name":
		if typeName == "MenuItem" {
			e.SetAttribute(key, value)
			e.SetAttribute("text", value)
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

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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
