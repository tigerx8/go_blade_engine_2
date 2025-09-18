package engine

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to write temporary template files under a temp templates dir
func writeTempTemplate(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}
	if err := ioutil.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write template %s: %v", p, err)
	}
	return name
}

func TestEndToEndRender_BladeAndGo(t *testing.T) {
	// Create a temporary templates dir
	tDir, err := ioutil.TempDir("", "blade_e2e")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tDir)

	// Write a Blade template that uses a simple variable and an if-block
	bladeTpl := `Hello {{ $user.Name }}
@if($user.IsAdmin)
Admin
@endif
`
	writeTempTemplate(t, tDir, "pages/user.blade.tpl", bladeTpl)

	// Write a Go template
	goTpl := `Hello {{ .user.Name }}
{{ if .user.IsAdmin }}Admin
{{ end }}`
	writeTempTemplate(t, tDir, "pages/user.gohtml", goTpl)

	// Engine configured with auto-mode extensions
	be := NewBladeEngineWithConfig(BladeConfig{
		TemplatesDir:       tDir,
		CacheEnabled:       false,
		Development:        true,
		TemplateExtension:  ".blade.tpl",
		AutoModeExtensions: []string{".gohtml", ".html"},
	})

	// Render Blade template
	out1, err := be.RenderString("pages/user.blade.tpl", map[string]interface{}{"user": map[string]interface{}{"Name": "Alice", "IsAdmin": true}})
	if err != nil {
		t.Fatalf("render blade template failed: %v", err)
	}
	if !strings.Contains(out1, "Hello Alice") || !strings.Contains(out1, "Admin") {
		t.Fatalf("unexpected blade output: %s", out1)
	}

	// Render Go template
	out2, err := be.RenderString("pages/user.gohtml", map[string]interface{}{"user": map[string]interface{}{"Name": "Bob", "IsAdmin": false}})
	if err != nil {
		t.Fatalf("render go template failed: %v", err)
	}
	if !strings.Contains(out2, "Hello Bob") || strings.Contains(out2, "Admin") {
		t.Fatalf("unexpected go output: %s", out2)
	}

	// Check last used compiler and counts
	last, counts := be.LastUsedCompiler()
	if last == "" {
		t.Fatalf("expected last used compiler to be set")
	}
	if counts["go"] < 1 || counts["blade"] < 1 {
		t.Fatalf("expected both compilers to have usage > 0, got: %v", counts)
	}
}
