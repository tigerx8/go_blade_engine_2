package engine

import (
	"io/ioutil"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompilerStatsHandlerHTTP(t *testing.T) {
	be := NewBladeEngineWithConfig(BladeConfig{
		TemplatesDir:      "./templates",
		CacheEnabled:      false,
		Development:       true,
		TemplateExtension: ".blade.tpl",
	})

	// Simulate some compiler usage
	_ = be.chooseCompilerFor("pages/about.blade.tpl")
	_ = be.chooseCompilerFor("pages/home.gohtml")
	_ = be.chooseCompilerFor("pages/home.gohtml")

	r := httptest.NewRequest("GET", "/compiler-stats", nil)
	w := httptest.NewRecorder()
	h := be.CompilerStatsHTTPHandler()
	h(w, r)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	out := string(body)

	if !strings.Contains(out, "last_used:") {
		t.Fatalf("expected last_used in output, got: %s", out)
	}

	if !strings.Contains(out, "go: 2") {
		t.Fatalf("expected go usage count 2, got: %s", out)
	}

	if !strings.Contains(out, "blade: 1") {
		t.Fatalf("expected blade usage count 1, got: %s", out)
	}
}
