package expr

import (
	"fmt"
)

type Parser struct {
	lex *Lexer
	cur Token
}

func NewParser(input string) *Parser {
	l := NewLexer(input)
	p := &Parser{lex: l}
	p.cur = p.lex.NextToken()
	return p
}

func (p *Parser) next() Token {
	t := p.cur
	p.cur = p.lex.NextToken()
	return t
}

func (p *Parser) expect(typ TokenType) (Token, error) {
	if p.cur.Typ == typ {
		return p.next(), nil
	}
	return Token{}, fmt.Errorf("expected token %v, got %v", typ, p.cur)
}

func (p *Parser) Parse() (Expr, error) {
	return p.parsePipe()
}

// parsePipe: for now treat pipe as binary right-associative
func (p *Parser) parsePipe() (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if p.cur.Typ == TokPipe {
		p.next()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &PipeExpr{Left: left, Right: right}, nil
	}
	return left, nil
}

func (p *Parser) parsePrimary() (Expr, error) {
	var base Expr
	switch p.cur.Typ {
	case TokDollarIdent:
		t := p.next()
		base = &DollarIdent{Name: t.Val}
	case TokIdent:
		t := p.next()
		base = &Ident{Name: t.Val}
	case TokDot:
		// current context '.' followed by optional ident chain
		p.next()
		base = &Current{}
		// if next is ident, parse as field chain
		if p.cur.Typ == TokIdent {
			fld := p.next().Val
			base = &DotAccess{Base: base, Field: fld}
		}
	case TokDotSpaced:
		// treat as current context as well
		p.next()
		base = &Current{}
		if p.cur.Typ == TokIdent {
			fld := p.next().Val
			base = &DotAccess{Base: base, Field: fld}
		}
	case TokString:
		t := p.next()
		return &StringLit{Val: t.Val}, nil
	case TokNumber:
		t := p.next()
		return &NumberLit{Val: t.Val}, nil
	case TokLParen:
		p.next()
		e, err := p.Parse()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokRParen); err != nil {
			return nil, err
		}
		return e, nil
	default:
		return nil, fmt.Errorf("unexpected token %v", p.cur)
	}

	// follow dot/index chain
	var err error
	base, err = p.parseFieldIndexChain(base)
	if err != nil {
		return nil, err
	}

	// Support space-separated call args after a base (e.g. index .Map "key")
	switch p.cur.Typ {
	case TokDollarIdent, TokIdent, TokString, TokNumber, TokLParen, TokDot, TokDotSpaced:
		var args []Expr
		for p.cur.Typ != TokPipe && p.cur.Typ != TokRParen && p.cur.Typ != TokEOF {
			if p.cur.Typ == TokComma {
				p.next()
				continue
			}
			arg, err := p.Parse()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return &CallExpr{Fn: base, Args: args}, nil
	default:
		return base, nil
	}
}

func (p *Parser) parseFieldIndexChain(base Expr) (Expr, error) {
	for {
		if p.cur.Typ == TokDot {
			p.next()
			if p.cur.Typ != TokIdent {
				return nil, fmt.Errorf("expected ident after dot, got %v", p.cur)
			}
			fld := p.next().Val
			base = &DotAccess{Base: base, Field: fld}
			continue
		}
		if p.cur.Typ == TokLBracket {
			p.next()
			// accept string, number or ident or dollarident
			var key Expr
			switch p.cur.Typ {
			case TokString:
				key = &StringLit{Val: p.next().Val}
			case TokNumber:
				key = &NumberLit{Val: p.next().Val}
			case TokDollarIdent:
				key = &DollarIdent{Name: p.next().Val}
			case TokIdent:
				key = &Ident{Name: p.next().Val}
			default:
				return nil, fmt.Errorf("unexpected token in index: %v", p.cur)
			}
			if _, err := p.expect(TokRBracket); err != nil {
				return nil, err
			}
			base = &IndexAccess{Base: base, Key: key}
			continue
		}
		// function call on base? e.g. mapKeys $m -> not supported here unless ident
		break
	}
	return base, nil
}
