package engine

import (
	"strings"
	"testing"
)

// Table-driven tests for expression handling in processVariables/processRawVariables
func TestExpressionEdgeCases(t *testing.T) {
	c := NewCompilerWithOptions("./", "blade", nil)

	cases := []struct {
		name         string
		input        string
		wantContains []string
	}{
		{"nested function calls and pipeline", `{{ trim (upper $user.Name) | html }}`, []string{"trim", "upper", ".user.Name"}},
		{"bracket indexing with quoted key", `{{ $m["key"] }}`, []string{"m", "key"}},
		{"bracket indexing with number", `{{ $arr[0] }}`, []string{"arr", "0"}},
		{"raw variable preserved", `{!! $html !!}`, []string{"raw", "html"}},
		{"mixed whitespace and trim markers", `{{- $name -}}`, []string{"escape", "name"}},
		{"complex with function call and brackets", `{{ join "," (mapKeys $m["nested"]) }}`, []string{"mapKeys", "m"}},
		{"dollar in non-variable context", `{{ printf "$%s" $value }}`, []string{"printf", "value"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.processAllDirectives(tc.input, "test")
			if err != nil {
				t.Fatalf("processing error: %v", err)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Fatalf("case %s: expected compiled output to contain %q, got: %s", tc.name, want, got)
				}
			}
		})
	}
}

// A small fuzz-like generator covering bracket/key permutations
func TestExpressionBracketFuzz(t *testing.T) {
	c := NewCompilerWithOptions("./", "blade", nil)
	keys := []string{"a", "b-c", "123", "with space"}
	for _, k := range keys {
		in := "{{ $m[\"" + k + "\"] }}"
		got, err := c.processAllDirectives(in, "fuzz")
		if err != nil {
			t.Fatalf("processAllDirectives error for key %q: %v", k, err)
		}
		// Accept either converted index() form or a normalized bracket access; assert presence of m and key
		if !(strings.Contains(got, "(index .m") || strings.Contains(got, "m[\"") || strings.Contains(got, ".m[\"")) {
			t.Fatalf("expected index/bracket form for key %q, got: %s", k, got)
		}
	}
}
