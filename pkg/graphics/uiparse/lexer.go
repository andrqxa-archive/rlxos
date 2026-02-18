package uiparse

import "unicode"

type tokenType int

const (
	tokenEOF tokenType = iota
	tokenIdent
	tokenString
	tokenNumber
	tokenLBrace
	tokenRBrace
	tokenColon
	tokenImport    // "import" keyword
	tokenComponent // "component" keyword
	tokenProperty  // "property" keyword
	tokenSignal    // "signal" keyword
)

type token struct {
	typ    tokenType
	value  string
	line   int
	column int
}

type lexer struct {
	src    []rune
	pos    int
	line   int
	col    int
	tokens []token
}

func lex(source string) ([]token, error) {
	l := &lexer{
		src:  []rune(source),
		line: 1,
		col:  1,
	}
	if err := l.run(); err != nil {
		return nil, err
	}
	l.tokens = append(l.tokens, token{typ: tokenEOF, line: l.line, column: l.col})
	return l.tokens, nil
}

func (l *lexer) run() error {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		if unicode.IsSpace(ch) {
			l.advance()
			continue
		}

		// Comments
		if ch == '/' && l.pos+1 < len(l.src) {
			next := l.src[l.pos+1]
			if next == '/' {
				l.skipLineComment()
				continue
			}
			if next == '*' {
				if err := l.skipBlockComment(); err != nil {
					return err
				}
				continue
			}
		}

		switch ch {
		case '{':
			l.emit(tokenLBrace, "{")
			l.advance()
		case '}':
			l.emit(tokenRBrace, "}")
			l.advance()
		case ':':
			l.emit(tokenColon, ":")
			l.advance()
		case '"':
			if err := l.readString(); err != nil {
				return err
			}
		case '#':
			l.readColor()
		default:
			if ch == '-' || ch == '+' || (ch >= '0' && ch <= '9') {
				l.readNumber()
			} else if isIdentStart(ch) {
				l.readIdent()
			} else {
				return &ParseError{Line: l.line, Column: l.col, Message: "unexpected character: " + string(ch)}
			}
		}
	}
	return nil
}

func (l *lexer) advance() {
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *lexer) emit(typ tokenType, value string) {
	l.tokens = append(l.tokens, token{typ: typ, value: value, line: l.line, column: l.col})
}

func (l *lexer) skipLineComment() {
	l.advance()
	l.advance()
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
}

func (l *lexer) skipBlockComment() error {
	startLine, startCol := l.line, l.col
	l.advance()
	l.advance()
	for l.pos < len(l.src) {
		if l.src[l.pos] == '*' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
			l.advance()
			l.advance()
			return nil
		}
		l.advance()
	}
	return &ParseError{Line: startLine, Column: startCol, Message: "unterminated block comment"}
}

func (l *lexer) readString() error {
	startLine, startCol := l.line, l.col
	l.advance() // skip opening "
	var buf []rune
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\\' && l.pos+1 < len(l.src) {
			l.advance()
			esc := l.src[l.pos]
			switch esc {
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case '\\':
				buf = append(buf, '\\')
			case '"':
				buf = append(buf, '"')
			default:
				buf = append(buf, '\\', esc)
			}
			l.advance()
			continue
		}
		if ch == '"' {
			l.emit(tokenString, string(buf))
			l.advance()
			return nil
		}
		if ch == '\n' {
			return &ParseError{Line: l.line, Column: l.col, Message: "unterminated string"}
		}
		buf = append(buf, ch)
		l.advance()
	}
	return &ParseError{Line: startLine, Column: startCol, Message: "unterminated string"}
}

func (l *lexer) readColor() {
	startCol := l.col
	start := l.pos
	l.advance()
	for l.pos < len(l.src) && isHexDigit(l.src[l.pos]) {
		l.advance()
	}
	l.tokens = append(l.tokens, token{
		typ:    tokenIdent,
		value:  string(l.src[start:l.pos]),
		line:   l.line,
		column: startCol,
	})
}

func (l *lexer) readNumber() {
	startCol := l.col
	start := l.pos
	if l.src[l.pos] == '-' || l.src[l.pos] == '+' {
		l.advance()
	}
	for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
		l.advance()
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		l.advance()
		for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
			l.advance()
		}
	}
	l.tokens = append(l.tokens, token{
		typ:    tokenNumber,
		value:  string(l.src[start:l.pos]),
		line:   l.line,
		column: startCol,
	})
}

func (l *lexer) readIdent() {
	startCol := l.col
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.advance()
	}
	value := string(l.src[start:l.pos])
	typ := classifyIdent(value)
	l.tokens = append(l.tokens, token{
		typ:    typ,
		value:  value,
		line:   l.line,
		column: startCol,
	})
}

func classifyIdent(s string) tokenType {
	switch s {
	case "import":
		return tokenImport
	case "component":
		return tokenComponent
	case "property":
		return tokenProperty
	case "signal":
		return tokenSignal
	default:
		return tokenIdent
	}
}

func isIdentStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isIdentPart(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '.'
}

func isHexDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
