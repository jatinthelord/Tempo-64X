// Package lexer implements the Tempo language lexer.
// It tokenizes source files written in Tempo syntax.
package lexer

import (
	"strings"

	"github.com/jatu/tempo/token"
)

// Lexer holds the lexer state
type Lexer struct {
	input   string
	pos     int
	readPos int
	ch      byte
	line    int
	col     int
}

// New creates a new Lexer for the given input string
func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, col: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.col++
	if l.ch == '\n' {
		l.line++
		l.col = 0
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) makeToken(t token.Type, lit string) token.Token {
	return token.Token{Type: t, Literal: lit, Line: l.line, Col: l.col}
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() token.Token {
	l.skipWhitespaceAndComments()

	if l.ch == 0 {
		return l.makeToken(token.EOF, "")
	}

	switch l.ch {
	case '\n':
		tok := l.makeToken(token.NEWLINE, "\n")
		l.readChar()
		return tok
	case ',':
		tok := l.makeToken(token.COMMA, ",")
		l.readChar()
		return tok
	case ':':
		tok := l.makeToken(token.COLON, ":")
		l.readChar()
		return tok
	case ';':
		// inline comment — skip to end of line
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
		return l.NextToken()
	case '[':
		tok := l.makeToken(token.LBRACKET, "[")
		l.readChar()
		return tok
	case ']':
		tok := l.makeToken(token.RBRACKET, "]")
		l.readChar()
		return tok
	case '(':
		tok := l.makeToken(token.LPAREN, "(")
		l.readChar()
		return tok
	case ')':
		tok := l.makeToken(token.RPAREN, ")")
		l.readChar()
		return tok
	case '*':
		tok := l.makeToken(token.STAR, "*")
		l.readChar()
		return tok
	case '$':
		// could be $-msg expression
		tok := l.makeToken(token.DOLLAR, "$")
		l.readChar()
		return tok
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			tok := l.makeToken(token.ALIAS, "==")
			l.readChar()
			return tok
		}
		tok := l.makeToken(token.EQUALS, "=")
		l.readChar()
		return tok
	case '-':
		tok := l.makeToken(token.MINUS, "-")
		l.readChar()
		return tok
	case '.':
		// .data, .text, .bytes
		return l.readDotIdentifier()
	case '%':
		return l.readPercentToken()
	case '\'', '"':
		return l.readString()
	case '0':
		if l.peekChar() == 'x' || l.peekChar() == 'X' {
			return l.readHexLiteral()
		}
		if l.peekChar() == 'b' || l.peekChar() == 'B' {
			return l.readBinLiteral()
		}
		return l.readNumber()
	default:
		if isDigit(l.ch) {
			return l.readNumber()
		}
		if isLetter(l.ch) {
			return l.readIdentifierOrKeyword()
		}
		tok := l.makeToken(token.ILLEGAL, string(l.ch))
		l.readChar()
		return tok
	}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
	// Skip ; line comments
	if l.ch == ';' {
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
	}
	// Skip // comments
	if l.ch == '/' && l.peekChar() == '/' {
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
	}
}

func (l *Lexer) readIdentifierOrKeyword() token.Token {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '%' {
		l.readChar()
	}
	lit := l.input[start:l.pos]
	lower := strings.ToLower(lit)

	// Check section keywords
	if lower == "section" {
		// peek next non-space word
		l.skipWhitespaceAndComments()
		sectionStart := l.pos
		for isLetter(l.ch) || l.ch == '.' {
			l.readChar()
		}
		sectionName := strings.ToLower(l.input[sectionStart:l.pos])
		switch sectionName {
		case ".data", "data":
			return l.makeToken(token.SECTION_DATA, "Section .data")
		case ".text", "text":
			return l.makeToken(token.SECTION_TEXT, "Section .text")
		}
		return l.makeToken(token.IDENTIFIER, lit)
	}

	// Check %rmpi% full token
	if lit == "%rmpi%" {
		return l.makeToken(token.RMPI, lit)
	}

	// Register check
	if token.IsRegister(lower) {
		return l.makeToken(token.REGISTER, lower)
	}

	// Keyword table
	tt := token.LookupIdentifier(lit)
	if tt == token.IDENTIFIER {
		tt = token.LookupIdentifier(lower)
	}
	return l.makeToken(tt, lit)
}

func (l *Lexer) readDotIdentifier() token.Token {
	start := l.pos
	l.readChar() // consume '.'
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	lit := l.input[start:l.pos]
	lower := strings.ToLower(lit)
	switch lower {
	case ".data":
		return l.makeToken(token.SECTION_DATA, lit)
	case ".text":
		return l.makeToken(token.SECTION_TEXT, lit)
	case ".bytes":
		return l.makeToken(token.BYTES_KEYWORD, lit)
	default:
		return l.makeToken(token.IDENTIFIER, lit)
	}
}

func (l *Lexer) readPercentToken() token.Token {
	start := l.pos
	l.readChar() // consume '%'
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	if l.ch == '%' {
		l.readChar() // consume closing '%'
	}
	lit := l.input[start:l.pos]
	switch strings.ToLower(lit) {
	case "%rmpi%":
		return l.makeToken(token.RMPI, lit)
	case "%rdi%":
		return l.makeToken(token.REGISTER, "rdi")
	case "%rdmi%":
		return l.makeToken(token.REGISTER, "rdx")
	default:
		return l.makeToken(token.IDENTIFIER, lit)
	}
}

func (l *Lexer) readString() token.Token {
	quote := l.ch
	l.readChar() // consume opening quote
	start := l.pos
	for l.ch != quote && l.ch != 0 && l.ch != '\n' {
		l.readChar()
	}
	lit := l.input[start:l.pos]
	if l.ch == quote {
		l.readChar()
	}
	return l.makeToken(token.STRING, lit)
}

func (l *Lexer) readNumber() token.Token {
	start := l.pos
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.makeToken(token.NUMBER, l.input[start:l.pos])
}

func (l *Lexer) readHexLiteral() token.Token {
	start := l.pos
	l.readChar() // '0'
	l.readChar() // 'x'
	for isHexDigit(l.ch) {
		l.readChar()
	}
	return l.makeToken(token.HEX_LITERAL, l.input[start:l.pos])
}

func (l *Lexer) readBinLiteral() token.Token {
	start := l.pos
	l.readChar() // '0'
	l.readChar() // 'b'
	for l.ch == '0' || l.ch == '1' {
		l.readChar()
	}
	return l.makeToken(token.BIN_LITERAL, l.input[start:l.pos])
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
