package engine

import (
	"fmt"
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

	fmt.Println("=== ORIGINAL ===")
	fmt.Println(original)

	content, layout, err := c.processExtends(original)
	if err != nil {
		t.Fatalf("processExtends error: %v", err)
	}
	fmt.Println("=== AFTER extends (layout=", layout, ") ===")
	fmt.Println(content)

	content2, err := c.processAllDirectives(content, path)
	if err != nil {
		t.Fatalf("processAllDirectives error: %v", err)
	}
	fmt.Println("=== AFTER directives ===")
	fmt.Println(content2)

	if layout != "" {
		combined, err := c.combineWithLayout(content2, layout)
		if err != nil {
			t.Fatalf("combineWithLayout error: %v", err)
		}
		fmt.Println("=== AFTER combineWithLayout ===")
		fmt.Println(combined)
	}
}
