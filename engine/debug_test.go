package engine

import (
	"os"
	"testing"
)

func TestDebugCompileHome(t *testing.T) {
	c := NewCompilerWithOptions("../templates", "blade", nil)
	path := "../templates/pages/home.blade.tpl"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original := string(data)

	t.Log("=== ORIGINAL ===")
	t.Log(original)

	content, layout, err := c.processExtends(original)
	if err != nil {
		t.Fatalf("processExtends error: %v", err)
	}
	t.Logf("=== AFTER extends (layout=%s) ===", layout)
	t.Log(content)

	content2, err := c.processAllDirectives(content, path)
	if err != nil {
		t.Fatalf("processAllDirectives error: %v", err)
	}
	t.Log("=== AFTER directives ===")
	t.Log(content2)

	if layout != "" {
		combined, err := c.combineWithLayout(content2, layout)
		if err != nil {
			t.Fatalf("combineWithLayout error: %v", err)
		}
		t.Log("=== AFTER combineWithLayout ===")
		t.Log(combined)
	}
}
