package expr

import (
	"unicode"
)

type TokenType int

const (
	TokEOF TokenType = iota
	TokIdent
	TokDollarIdent
	TokDot
	TokDotSpaced
	TokLBracket
	TokRBracket
	TokString
	TokNumber
	TokLParen
	TokRParen
	TokPipe
	TokComma
	TokOp
	TokOther
)

type Token struct {
	Typ TokenType
	Val string
}

type Lexer struct {
	input []rune
	pos   int
}

func NewLexer(s string) *Lexer {
	return &Lexer{input: []rune(s), pos: 0}
}

func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	r := l.input[l.pos]
	l.pos++
	return r
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) emitToken(typ TokenType, val string) Token {
	return Token{Typ: typ, Val: val}
}

func (l *Lexer) NextToken() Token {
	hadSpace := false
	for {
		ch := l.peek()
		if ch == 0 {
			return l.emitToken(TokEOF, "")
		}
		// whitespace
		if unicode.IsSpace(ch) {
			hadSpace = true
			l.next()
			continue
		}
		switch ch {
		case '|':
			l.next()
			return l.emitToken(TokPipe, "|")
		case '(':
			l.next()
			return l.emitToken(TokLParen, "(")
		case ')':
			l.next()
			return l.emitToken(TokRParen, ")")
		case '.':
			// single dot token; if there was whitespace before the dot, emit TokDotSpaced
			l.next()
			if hadSpace {
				return l.emitToken(TokDotSpaced, ".")
			}
			return l.emitToken(TokDot, ".")
		case '[':
			l.next()
			return l.emitToken(TokLBracket, "[")
		case ']':
			l.next()
			return l.emitToken(TokRBracket, "]")
		case ',':
			l.next()
			return l.emitToken(TokComma, ",")
		case '$':
			l.next()
			// dollar identifier
			var buf []rune
			for unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_' {
				buf = append(buf, l.next())
			}
			return l.emitToken(TokDollarIdent, string(buf))
		case '"', '\'':
			q := l.next()
			var buf []rune
			for {
				r := l.next()
				if r == 0 {
					break
				}
				if r == '\\' {
					// include escaped char
					nxt := l.next()
					buf = append(buf, r, nxt)
					continue
				}
				if r == q {
					break
				}
				buf = append(buf, r)
			}
			return l.emitToken(TokString, string(buf))
		default:
			if unicode.IsDigit(ch) {
				var buf []rune
				for unicode.IsDigit(l.peek()) || l.peek() == '.' {
					buf = append(buf, l.next())
				}
				return l.emitToken(TokNumber, string(buf))
			}
			if unicode.IsLetter(ch) {
				var buf []rune
				for unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_' || l.peek() == '.' {
					buf = append(buf, l.next())
				}
				return l.emitToken(TokIdent, string(buf))
			}
			// operators
			if stringsContainsAny(string(ch), "+-*/%&><=") {
				l.next()
				return l.emitToken(TokOp, string(ch))
			}
			// fallback
			l.next()
			return l.emitToken(TokOther, string(ch))
		}
	}
}

func stringsContainsAny(s, chars string) bool {
	for _, c := range s {
		for _, t := range chars {
			if c == t {
				return true
			}
		}
	}
	return false
}
