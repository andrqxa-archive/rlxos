package parse

import "fmt"

// ParseError represents an error during UI parsing with location info.
type ParseError struct {
	Line    int
	Column  int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, col %d: %s", e.Line, e.Column, e.Message)
}

// BuildError represents an error during element tree construction.
type BuildError struct {
	Line    int
	Element string
	Message string
}

func (e *BuildError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d <%s>: %s", e.Line, e.Element, e.Message)
	}
	return fmt.Sprintf("<%s>: %s", e.Element, e.Message)
}
