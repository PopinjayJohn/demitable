package formula

import (
	"fmt"
	"strings"
	"unicode"
)

type tokKind int

const (
	tkEOF tokKind = iota
	tkNumber
	tkString
	tkIdent
	tkOp
	tkPunct
)

type token struct {
	kind tokKind
	val  string
	pos  int
}

type lexer struct {
	src string
	i   int
}

func (l *lexer) lexAll() ([]token, error) {
	var out []token
	for {
		l.skipWS()
		if l.i >= len(l.src) {
			out = append(out, token{kind: tkEOF, pos: l.i})
			return out, nil
		}
		c := l.src[l.i]
		switch {
		case unicode.IsDigit(rune(c)) || (c == '.' && l.i+1 < len(l.src) && unicode.IsDigit(rune(l.src[l.i+1]))):
			tok, err := l.lexNumber()
			if err != nil {
				return nil, err
			}
			out = append(out, tok)
		case c == '"' || c == '\'':
			tok, err := l.lexString()
			if err != nil {
				return nil, err
			}
			out = append(out, tok)
		case isIdentStart(rune(c)):
			out = append(out, l.lexIdent())
		default:
			// multi-char operators
			if l.i+1 < len(l.src) {
				two := l.src[l.i : l.i+2]
				switch two {
				case "==", "!=", "<=", ">=", "&&", "||":
					out = append(out, token{kind: tkOp, val: two, pos: l.i})
					l.i += 2
					continue
				}
			}
			switch c {
			case '+', '-', '*', '/', '%', '<', '>', '?', ':', '.', ',', '(', ')', '[', ']', '{', '}':
				k := tkPunct
				if c == '+' || c == '-' || c == '*' || c == '/' || c == '%' || c == '<' || c == '>' || c == '?' || c == ':' || c == '.' {
					// treat operators as op when appropriate; keep simple: mark arithmetic/comparison as op
				}
				_ = k
				out = append(out, token{kind: tkPunct, val: string(c), pos: l.i})
				l.i++
			default:
				return nil, fmt.Errorf("formula: unexpected character %q at %d", c, l.i)
			}
		}
	}
}

func (l *lexer) skipWS() {
	for l.i < len(l.src) {
		r := rune(l.src[l.i])
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			l.i++
			continue
		}
		break
	}
}

func (l *lexer) lexNumber() (token, error) {
	start := l.i
	hasDot := false
	for l.i < len(l.src) {
		c := l.src[l.i]
		if unicode.IsDigit(rune(c)) {
			l.i++
			continue
		}
		if c == '.' && !hasDot {
			hasDot = true
			l.i++
			continue
		}
		break
	}
	return token{kind: tkNumber, val: l.src[start:l.i], pos: start}, nil
}

func (l *lexer) lexString() (token, error) {
	start := l.i
	q := l.src[l.i]
	l.i++
	var b strings.Builder
	for l.i < len(l.src) {
		c := l.src[l.i]
		if c == byte(q) {
			l.i++
			return token{kind: tkString, val: b.String(), pos: start}, nil
		}
		if c == '\\' {
			l.i++
			if l.i >= len(l.src) {
				return token{}, fmt.Errorf("formula: unterminated string")
			}
			esc := l.src[l.i]
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"', '\'', '\\':
				b.WriteByte(esc)
			default:
				b.WriteByte('\\')
				b.WriteByte(esc)
			}
			l.i++
			continue
		}
		b.WriteByte(c)
		l.i++
	}
	return token{}, fmt.Errorf("formula: unterminated string")
}

func (l *lexer) lexIdent() token {
	start := l.i
	for l.i < len(l.src) && isIdentPart(rune(l.src[l.i])) {
		l.i++
	}
	return token{kind: tkIdent, val: l.src[start:l.i], pos: start}
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
