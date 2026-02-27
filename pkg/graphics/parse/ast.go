package parse

// ValueKind distinguishes the type of a parsed value.
type ValueKind int

const (
	ValueString ValueKind = iota
	ValueInt
	ValueFloat
	ValueBool
	ValueIdent
)

// Value is a parsed property value.
type Value struct {
	Kind  ValueKind
	Str   string
	Int   int64
	Float float64
	Bool  bool
}

// Property is a key-value pair in a Node.
type Property struct {
	Name  string
	Value Value
	Line  int
}

// Node represents an element in the UI tree.
type Node struct {
	TypeName   string
	Properties []*Property
	Children   []*Node
	Line       int
}

// PropValue returns the Value for a named property, or zero Value and false.
func (n *Node) PropValue(name string) (Value, bool) {
	for _, p := range n.Properties {
		if p.Name == name {
			return p.Value, true
		}
	}
	return Value{}, false
}

// PropString returns the string representation of a property.
func (n *Node) PropString(name string) string {
	if v, ok := n.PropValue(name); ok {
		return v.Str
	}
	return ""
}

// PropInt returns the int value of a property, or the default.
func (n *Node) PropInt(name string, def int) int {
	if v, ok := n.PropValue(name); ok {
		if v.Kind == ValueInt {
			return int(v.Int)
		}
	}
	return def
}

// PropFloat returns the float value of a property, or the default.
func (n *Node) PropFloat(name string, def float64) float64 {
	if v, ok := n.PropValue(name); ok {
		switch v.Kind {
		case ValueFloat:
			return v.Float
		case ValueInt:
			return float64(v.Int)
		}
	}
	return def
}

// PropBool returns the bool value of a property, or the default.
func (n *Node) PropBool(name string, def bool) bool {
	if v, ok := n.PropValue(name); ok {
		if v.Kind == ValueBool {
			return v.Bool
		}
	}
	return def
}

// ImportDecl represents an import statement.
type ImportDecl struct {
	Path string
	Line int
}

// SignalDecl represents a signal declaration inside a component.
type SignalDecl struct {
	Name string
	Line int
}

// ComponentDef represents a component definition.
type ComponentDef struct {
	Name       string
	Properties []*Property  // property declarations with defaults
	Signals    []SignalDecl // signal declarations
	Defaults   []*Property  // default attribute values
	Children   []*Node      // child element templates (future)
	Line       int
}

// Document is the top-level parse result.
type Document struct {
	Imports    []ImportDecl
	Components []*ComponentDef
	Root       *Node // the root element (Window, VBox, etc.)
}
