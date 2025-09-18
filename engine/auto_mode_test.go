package engine

import (
	"testing"
)

func TestChooseCompilerForExtensions(t *testing.T) {
	be := NewBladeEngineWithConfig(BladeConfig{
		TemplatesDir: "templates",
		CacheEnabled: false,
		Mode:        "blade",
	})

	// default should be blade
	comp := be.chooseCompilerFor("pages/about.blade.tpl")
	if comp != be.compiler {
		t.Fatalf("expected blade compiler for .blade.tpl, got %v", comp)
	}

	// .gohtml should select go compiler
	comp = be.chooseCompilerFor("pages/home.gohtml")
	if comp != be.goCompiler {
		t.Fatalf("expected go compiler for .gohtml, got %v", comp)
	}

	// double-check instrumentation
	last, counts := be.LastUsedCompiler()
	if last != "go" {
		t.Fatalf("expected last used compiler to be 'go', got %s", last)
	}
	if counts["go"] == 0 {
		t.Fatalf("expected usageCounts for 'go' to be > 0")
	}
}
