package parse

import (
	"strconv"
	"strings"
)

type parser struct {
	tokens []token
	pos    int
}

// Parse parses a .qml/.ui source into a Document.
func Parse(source string) (*Document, error) {
	tokens, err := lex(source)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens}
	return p.parseDocument()
}

func (p *parser) current() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{typ: tokenEOF}
}

func (p *parser) peek() token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	return token{typ: tokenEOF}
}

func (p *parser) advance() token {
	t := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

func (p *parser) expect(typ tokenType) (token, error) {
	t := p.current()
	if t.typ != typ {
		return t, &ParseError{
			Line:    t.line,
			Column:  t.column,
			Message: "expected " + tokenName(typ) + ", got " + tokenName(t.typ) + " (" + t.value + ")",
		}
	}
	p.advance()
	return t, nil
}

func (p *parser) parseDocument() (*Document, error) {
	doc := &Document{}

	// Parse imports
	for p.current().typ == tokenImport {
		imp, err := p.parseImport()
		if err != nil {
			return nil, err
		}
		doc.Imports = append(doc.Imports, imp)
	}

	// Parse components and root element
	for p.current().typ != tokenEOF {
		if p.current().typ == tokenComponent {
			comp, err := p.parseComponent()
			if err != nil {
				return nil, err
			}
			doc.Components = append(doc.Components, comp)
		} else if p.current().typ == tokenIdent && p.peek().typ == tokenLBrace {
			if doc.Root != nil {
				t := p.current()
				return nil, &ParseError{Line: t.line, Column: t.column, Message: "multiple root elements"}
			}
			root, err := p.parseElement()
			if err != nil {
				return nil, err
			}
			doc.Root = root
		} else {
			t := p.current()
			return nil, &ParseError{Line: t.line, Column: t.column,
				Message: "expected import, component, or root element, got " + tokenName(t.typ)}
		}
	}

	return doc, nil
}

// import "path"
func (p *parser) parseImport() (ImportDecl, error) {
	impTok := p.advance() // consume "import"
	pathTok, err := p.expect(tokenString)
	if err != nil {
		return ImportDecl{}, err
	}
	return ImportDecl{Path: pathTok.value, Line: impTok.line}, nil
}

// component Name { property..., signal..., defaults... }
func (p *parser) parseComponent() (*ComponentDef, error) {
	p.advance() // consume "component"
	nameTok, err := p.expect(tokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenLBrace); err != nil {
		return nil, err
	}

	comp := &ComponentDef{
		Name: nameTok.value,
		Line: nameTok.line,
	}

	for p.current().typ != tokenRBrace && p.current().typ != tokenEOF {
		switch p.current().typ {
		case tokenProperty:
			prop, err := p.parsePropertyDecl()
			if err != nil {
				return nil, err
			}
			comp.Properties = append(comp.Properties, prop)
		case tokenSignal:
			sig, err := p.parseSignalDecl()
			if err != nil {
				return nil, err
			}
			comp.Signals = append(comp.Signals, sig)
		case tokenIdent:
			if p.peek().typ == tokenColon {
				prop, err := p.parsePropertyAssign()
				if err != nil {
					return nil, err
				}
				comp.Defaults = append(comp.Defaults, prop)
			} else if p.peek().typ == tokenLBrace {
				child, err := p.parseElement()
				if err != nil {
					return nil, err
				}
				comp.Children = append(comp.Children, child)
			} else {
				t := p.current()
				return nil, &ParseError{Line: t.line, Column: t.column,
					Message: "expected ':' or '{' after '" + t.value + "'"}
			}
		default:
			t := p.current()
			return nil, &ParseError{Line: t.line, Column: t.column,
				Message: "unexpected token in component: " + tokenName(t.typ)}
		}
	}

	if _, err := p.expect(tokenRBrace); err != nil {
		return nil, err
	}
	return comp, nil
}

// property name: value
func (p *parser) parsePropertyDecl() (*Property, error) {
	p.advance() // consume "property"
	nameTok, err := p.expect(tokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenColon); err != nil {
		return nil, err
	}
	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	return &Property{Name: nameTok.value, Value: val, Line: nameTok.line}, nil
}

// signal name
func (p *parser) parseSignalDecl() (SignalDecl, error) {
	p.advance() // consume "signal"
	nameTok, err := p.expect(tokenIdent)
	if err != nil {
		return SignalDecl{}, err
	}
	return SignalDecl{Name: nameTok.value, Line: nameTok.line}, nil
}

// name: value (regular property assignment)
func (p *parser) parsePropertyAssign() (*Property, error) {
	nameTok := p.advance() // consume name
	p.advance()            // consume ':'
	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	return &Property{Name: nameTok.value, Value: val, Line: nameTok.line}, nil
}

// TypeName '{' body '}'
func (p *parser) parseElement() (*Node, error) {
	nameTok, err := p.expect(tokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenLBrace); err != nil {
		return nil, err
	}

	node := &Node{
		TypeName: nameTok.value,
		Line:     nameTok.line,
	}

	if err := p.parseBody(node); err != nil {
		return nil, err
	}

	if _, err := p.expect(tokenRBrace); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *parser) parseBody(node *Node) error {
	for {
		cur := p.current()
		if cur.typ == tokenRBrace || cur.typ == tokenEOF {
			return nil
		}

		if cur.typ != tokenIdent {
			return &ParseError{Line: cur.line, Column: cur.column,
				Message: "expected property or element, got " + tokenName(cur.typ)}
		}

		next := p.peek()
		if next.typ == tokenLBrace {
			child, err := p.parseElement()
			if err != nil {
				return err
			}
			node.Children = append(node.Children, child)
		} else if next.typ == tokenColon {
			prop, err := p.parsePropertyAssign()
			if err != nil {
				return err
			}
			node.Properties = append(node.Properties, prop)
		} else {
			return &ParseError{
				Line:    next.line,
				Column:  next.column,
				Message: "expected '{' or ':' after '" + cur.value + "', got " + tokenName(next.typ),
			}
		}
	}
}

func (p *parser) parseValue() (Value, error) {
	t := p.current()
	switch t.typ {
	case tokenString:
		p.advance()
		return Value{Kind: ValueString, Str: t.value}, nil
	case tokenNumber:
		p.advance()
		return parseNumber(t.value), nil
	case tokenIdent:
		p.advance()
		if t.value == "true" {
			return Value{Kind: ValueBool, Str: "true", Bool: true}, nil
		}
		if t.value == "false" {
			return Value{Kind: ValueBool, Str: "false", Bool: false}, nil
		}
		return Value{Kind: ValueIdent, Str: t.value}, nil
	default:
		return Value{}, &ParseError{
			Line:    t.line,
			Column:  t.column,
			Message: "expected value, got " + tokenName(t.typ),
		}
	}
}

func parseNumber(s string) Value {
	if strings.Contains(s, ".") {
		f, _ := strconv.ParseFloat(s, 64)
		return Value{Kind: ValueFloat, Str: s, Float: f}
	}
	i, _ := strconv.ParseInt(s, 10, 64)
	return Value{Kind: ValueInt, Str: s, Int: i}
}

func tokenName(typ tokenType) string {
	switch typ {
	case tokenEOF:
		return "EOF"
	case tokenIdent:
		return "identifier"
	case tokenString:
		return "string"
	case tokenNumber:
		return "number"
	case tokenLBrace:
		return "'{'"
	case tokenRBrace:
		return "'}'"
	case tokenColon:
		return "':'"
	case tokenImport:
		return "'import'"
	case tokenComponent:
		return "'component'"
	case tokenProperty:
		return "'property'"
	case tokenSignal:
		return "'signal'"
	}
	return "unknown"
}
