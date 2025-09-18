package engine

import (
	"strings"
	"testing"
)

func TestProcessVariablesPreserveGoDirectives(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `{{ range .items }}{{ raw .HTMLContent }}{{ end }}`
	out, err := c.CompileString(input, "templates/pages/test.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	// The processed string should still contain '{{ raw .HTMLContent }}' and '{{range .items}}' (or similar)
	if !contains(out, "raw .HTMLContent") {
		t.Fatalf("expected raw .HTMLContent preserved in output, got: %s", out)
	}
	if !contains(out, "range .items") {
		t.Fatalf("expected range .items preserved in output, got: %s", out)
	}
}

func TestProcessVariablesEscapesPlainVars(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `Hello {{ $name }}`
	out, err := c.CompileString(input, "templates/pages/test2.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	if !contains(out, "{{escape .name}}") {
		t.Fatalf("expected $name to be converted to {{escape .name}}, got: %s", out)
	}
}

// tiny helper to avoid importing strings in tests repeatedly
func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func TestTrimMarkersAndPipelinesPreserved(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `{{- range .items }}{{ .Name | printf "%s" }}{{- end }}`
	out, err := c.CompileString(input, "templates/pages/test3.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	if !contains(out, "range .items") {
		t.Fatalf("expected range .items preserved in output, got: %s", out)
	}
	if !contains(out, "| printf") {
		t.Fatalf("expected pipeline preserved in output, got: %s", out)
	}
}

func TestFunctionCallsPreserved(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `{{ index .Map "key" }} {{ call .Func "arg" }}`
	out, err := c.CompileString(input, "templates/pages/test4.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	if !contains(out, "index .Map") {
		t.Fatalf("expected index .Map preserved in output, got: %s", out)
	}
	if !contains(out, "call .Func") {
		t.Fatalf("expected call .Func preserved in output, got: %s", out)
	}
}

func TestNestedDirectivesAndCustomFunctions(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `{{ if (isset .User "name") }}Hello {{ $user.Name }}{{ else }}No user{{ end }}`
	out, err := c.CompileString(input, "templates/pages/test5.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	// isset is a custom function; the if should be preserved and $user.Name should be rewritten
	if !contains(out, "isset .User \"name\"") {
		t.Fatalf("expected isset call preserved, got: %s", out)
	}
	if !contains(out, "{{escape .user.Name}}") {
		t.Fatalf("expected $user.Name rewritten to {{escape .user.Name}}, got: %s", out)
	}
}

func TestComplexPipelinesAndFunctionCalls(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `{{- range $i, $v := .List }}{{ $v.Name | printf "%s" | html }}{{- end }}`
	out, err := c.CompileString(input, "templates/pages/test6.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	// The pipeline and range should be preserved as complex expressions
	if !contains(out, "range") {
		t.Fatalf("expected range preserved, got: %s", out)
	}
	if !contains(out, "| printf") || !contains(out, "| html") {
		t.Fatalf("expected pipeline preserved, got: %s", out)
	}
}

func TestBracketIndexingAndQuotedKeys(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `Value: {{ $m["key"] }} and nested: {{ $user.Profile[0].Name }}`
	out, err := c.CompileString(input, "templates/pages/test7.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	// bracket indexed variables should be rewritten into index(...) form
	if !contains(out, "{{escape (index .m \"key\")}}") {
		t.Fatalf("expected bracket indexed var rewritten to index(...), got: %s", out)
	}
	if !contains(out, "{{escape (index .user.Profile 0).Name}}") {
		t.Fatalf("expected chained dot with index rewritten into index(...), got: %s", out)
	}
}

func TestMixedPipelinesAndFunctionsEdgeCases(t *testing.T) {
	c := NewCompilerWithOptions("templates", "blade", nil)

	input := `{{ printf "%s" (join "," $list) }}`
	out, err := c.CompileString(input, "templates/pages/test8.blade.tpl")
	if err != nil {
		t.Fatalf("CompileString error: %v", err)
	}

	// This is complex: function call with pipeline-like behavior and parens — should be preserved
	if !contains(out, "printf") || !contains(out, "join") || !contains(out, ".list") {
		t.Fatalf("expected complex expression preserved (with normalized .list), got: %s", out)
	}
}
