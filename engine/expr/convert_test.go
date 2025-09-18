package expr

import (
	"testing"
)

func TestParseSimpleDollar(t *testing.T) {
	p := NewParser("$name")
	e, err := p.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !IsSimpleDollarVariable(e) {
		t.Fatalf("expected simple dollar variable, got %#v", e)
	}
}

func TestParseDotAccess(t *testing.T) {
	p := NewParser("$user.Name")
	e, err := p.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !IsSimpleDollarVariable(e) {
		t.Fatalf("expected simple dollar variable chain, got %#v", e)
	}
}

func TestParseIndexAccess(t *testing.T) {
	p := NewParser("$m[\"key\"]")
	e, err := p.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !IsSimpleDollarVariable(e) {
		t.Fatalf("expected simple dollar index access, got %#v", e)
	}
}
