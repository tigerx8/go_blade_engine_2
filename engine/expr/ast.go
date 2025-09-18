package expr

import (
	"fmt"
	"strings"
)

// Minimal AST node types for expressions we care about
type Expr interface{}

type Ident struct{ Name string }
type DollarIdent struct{ Name string }
type StringLit struct{ Val string }
type NumberLit struct{ Val string }

type DotAccess struct {
	Base  Expr
	Field string
}
type IndexAccess struct {
	Base Expr
	Key  Expr
}
type CallExpr struct {
	Fn   Expr
	Args []Expr
}
type PipeExpr struct {
	Left  Expr
	Right Expr
}

// Current represents the '.' root context in templates
type Current struct{}

// Helper to check for simple dollar-based variable like $name or $user.Name
func IsSimpleDollarVariable(e Expr) bool {
	// walk down DotAccess/IndexAccess to see if ultimate base is DollarIdent
	cur := e
	for {
		switch v := cur.(type) {
		case *DollarIdent:
			return true
		case *DotAccess:
			cur = v.Base
			continue
		case *IndexAccess:
			cur = v.Base
			continue
		default:
			return false
		}
	}
}

// ToTemplate converts an AST Expr back into a Go template expression string.
// It also converts DollarIdent into dot-based access (e.g. $x -> .x) when serializing.
func ToTemplate(e Expr) (string, error) {
	switch v := e.(type) {
	case *DollarIdent:
		return "." + v.Name, nil
	case *Current:
		return ".", nil
	case *Ident:
		return v.Name, nil
	case *StringLit:
		// re-quote
		return `"` + v.Val + `"`, nil
	case *NumberLit:
		return v.Val, nil
	case *DotAccess:
		baseS, err := ToTemplate(v.Base)
		if err != nil {
			return "", err
		}
		// if base is current '.' then avoid producing '..Field'
		if _, ok := v.Base.(*Current); ok {
			return "." + v.Field, nil
		}
		return baseS + "." + v.Field, nil
	case *IndexAccess:
		baseS, err := ToTemplate(v.Base)
		if err != nil {
			return "", err
		}
		keyS, err := ToTemplate(v.Key)
		if err != nil {
			return "", err
		}
		return "(index " + baseS + " " + keyS + ")", nil
	case *CallExpr:
		fnS, err := ToTemplate(v.Fn)
		if err != nil {
			return "", err
		}
		parts := []string{fnS}
		for _, a := range v.Args {
			as, err := ToTemplate(a)
			if err != nil {
				return "", err
			}
			parts = append(parts, as)
		}
		return strings.Join(parts, " "), nil
	case *PipeExpr:
		leftS, err := ToTemplate(v.Left)
		if err != nil {
			return "", err
		}
		rightS, err := ToTemplate(v.Right)
		if err != nil {
			return "", err
		}
		return leftS + " | " + rightS, nil
	default:
		return "", fmt.Errorf("unsupported expr type %T", e)
	}
}
