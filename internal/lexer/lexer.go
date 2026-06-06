package lexer

import (
	"strconv"
	"strings"

	"github.com/buildkite/conditional/internal/token"
)

type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()
	l.skipComment()

	// log.Printf("Tok: %c %q", l.ch, l.ch)

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.EQ, Literal: literal}
		} else if l.peekChar() == '~' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.RE_EQ, Literal: literal}
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NOT_EQ, Literal: literal}
		} else if l.peekChar() == '~' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.RE_NOT_EQ, Literal: literal}
		} else {
			tok = newToken(token.BANG, l.ch)
		}
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.AND, Literal: literal}
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.OR, Literal: literal}
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	case '@':
		tok = newToken(token.ILLEGAL, l.ch)
	case '.':
		tok = newToken(token.DOT, l.ch)
	case '?':
		tok = newToken(token.QUESTION, l.ch)
	case ':':
		tok = newToken(token.COLON, l.ch)
	case '$':
		tok.Type = token.SHELL
		var ok bool
		tok.Literal, ok = l.readShell()
		if !ok {
			tok.Type = token.ILLEGAL
		}
		return tok
	case '"':
		tok.Type = token.STRING
		var terminated bool
		tok.Literal, tok.Raw, terminated = l.readString('"')
		tok.Flags = `"`
		if !terminated {
			tok.Type = token.ILLEGAL
			tok.Literal = tok.Raw
			tok.Flags = ""
		}
	case '\'':
		tok.Type = token.STRING
		var terminated bool
		tok.Literal, tok.Raw, terminated = l.readString('\'')
		tok.Flags = `'`
		if !terminated {
			tok.Type = token.ILLEGAL
			tok.Literal = tok.Raw
			tok.Flags = ""
		}
	case '/':
		tok.Type = token.REGEXP
		var terminated bool
		tok.Literal, tok.Flags, terminated = l.readRegex()
		if !terminated {
			tok.Type = token.ILLEGAL
			tok.Flags = ""
		}
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case '[':
		tok = newToken(token.LBRACKET, l.ch)
	case ']':
		tok = newToken(token.RBRACKET, l.ch)
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			tok.Type = token.INT
			tok.Literal = l.readNumber()
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipComment() {
	if l.ch != '/' || l.peekChar() != '/' {
		return
	}
	for {
		l.readChar()

		if l.ch == '\n' || l.ch == 0 {
			l.skipWhitespace()
			break
		}
	}

	// keep ignoring whitespace and comments
	l.skipComment()
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition += 1
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPosition]
	}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isIdentPart(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readString(quote byte) (string, string, bool) {
	position := l.position + 1
	var out strings.Builder
	for {
		l.readChar()
		if l.ch == 0 {
			return out.String(), l.input[position:l.position], false
		}
		if l.ch == quote {
			return out.String(), l.input[position:l.position], true
		}
		if l.ch != '\\' {
			out.WriteByte(l.ch)
			continue
		}

		if quote == '\'' {
			if !l.readSingleQuotedEscape(&out) {
				return out.String(), l.input[position:l.position], false
			}
			continue
		}
		if !l.readStringEscape(&out) {
			return out.String(), l.input[position:l.position], false
		}
	}
}

func (l *Lexer) readSingleQuotedEscape(out *strings.Builder) bool {
	l.readChar()
	if l.ch == 0 {
		return false
	}
	switch l.ch {
	case '\\', '\'':
		out.WriteByte(l.ch)
	default:
		out.WriteByte('\\')
		out.WriteByte(l.ch)
	}
	return true
}

func (l *Lexer) readStringEscape(out *strings.Builder) bool {
	l.readChar()
	if l.ch == 0 {
		return false
	}

	switch l.ch {
	case 'n':
		out.WriteByte('\n')
	case 's':
		out.WriteByte(' ')
	case 'r':
		out.WriteByte('\r')
	case 't':
		out.WriteByte('\t')
	case 'v':
		out.WriteByte('\v')
	case 'f':
		out.WriteByte('\f')
	case 'b':
		out.WriteByte('\b')
	case 'a':
		out.WriteByte('\a')
	case 'e':
		out.WriteByte('\x1b')
	case '\\', '"':
		out.WriteByte(l.ch)
	case 'x':
		if l.readHexEscape(out) {
			return true
		}
		out.WriteByte('x')
	default:
		if isOctalDigit(l.ch) {
			return l.readOctalEscape(out)
		}
		out.WriteByte(l.ch)
	}
	return true
}

func (l *Lexer) readHexEscape(out *strings.Builder) bool {
	if l.readPosition+1 >= len(l.input) {
		return false
	}
	if !isHexDigit(l.input[l.readPosition]) || !isHexDigit(l.input[l.readPosition+1]) {
		return false
	}

	digits := l.input[l.readPosition : l.readPosition+2]
	value, err := strconv.ParseInt(digits, 16, 32)
	if err != nil {
		return false
	}
	l.readChar()
	l.readChar()
	out.WriteByte(byte(value))
	return true
}

func (l *Lexer) readOctalEscape(out *strings.Builder) bool {
	digits := []byte{l.ch}
	for len(digits) < 3 && l.readPosition < len(l.input) && isOctalDigit(l.input[l.readPosition]) {
		l.readChar()
		digits = append(digits, l.ch)
	}

	value, err := strconv.ParseInt(string(digits), 8, 32)
	if err != nil || value > 0xff {
		return false
	}
	out.WriteByte(byte(value))
	return true
}

func (l *Lexer) readShell() (string, bool) {
	position := l.position
	l.readChar()

	if isIdentStart(l.ch) {
		for isIdentPart(l.ch) {
			l.readChar()
		}
		return l.input[position:l.position], true
	}

	if l.ch != '{' {
		l.readChar()
		return l.input[position:l.position], false
	}

	depth := 1
	escaped := false
	for {
		l.readChar()
		if l.ch == 0 {
			return l.input[position:l.position], false
		}

		switch {
		case l.ch == '\\' && !escaped:
			escaped = true
			continue
		case (l.ch == '"' || l.ch == '\'') && !escaped:
			if !l.skipRawQuotedString(l.ch) {
				return l.input[position:l.position], false
			}
		case l.ch == '{' && !escaped:
			depth++
		case l.ch == '}' && !escaped:
			depth--
			if depth == 0 {
				l.readChar()
				return l.input[position:l.position], true
			}
		}

		escaped = false
	}
}

func (l *Lexer) skipRawQuotedString(quote byte) bool {
	escaped := false
	for {
		l.readChar()
		if l.ch == 0 {
			return false
		}
		if l.ch == quote && !escaped {
			return true
		}
		escaped = l.ch == '\\' && !escaped
		if l.ch != '\\' {
			escaped = false
		}
	}
}

func (l *Lexer) readRegex() (string, string, bool) {
	position := l.position + 1
	escaped := false
	terminated := false
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == '/' && !escaped {
			terminated = true
			break
		}

		if l.ch == '\\' && !escaped {
			escaped = true
		} else {
			escaped = false
		}
	}

	literal := l.input[position:l.position]
	if !terminated {
		return literal, "", false
	}

	flagsPosition := l.readPosition
	for l.readPosition < len(l.input) && isLetter(l.input[l.readPosition]) {
		l.readChar()
	}

	return literal, l.input[flagsPosition:l.readPosition], true
}

func isIdentStart(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isLetter(ch byte) bool {
	return isIdentStart(ch)
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch) || ch == '.'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isOctalDigit(ch byte) bool {
	return '0' <= ch && ch <= '7'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}
