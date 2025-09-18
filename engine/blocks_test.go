package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Nested blocks: layout defines a block that includes another block; page overrides inner block
func TestNestedBlocksAndSectionOverride(t *testing.T) {
	tmp := t.TempDir()
	layoutsDir := filepath.Join(tmp, "layouts")
	pagesDir := filepath.Join(tmp, "pages")
	_ = os.MkdirAll(layoutsDir, 0755)
	_ = os.MkdirAll(pagesDir, 0755)

	// layout with nested blocks
	layoutPath := filepath.Join(layoutsDir, "nested.blade.tpl")
	layoutContent := `{{block "outer" .}}{{block "inner" .}}Inner Default{{end}}{{end}}<div>{{template "content" .}}</div>`
	if err := os.WriteFile(layoutPath, []byte(layoutContent), 0644); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	// page overrides inner block only
	pagePath := filepath.Join(pagesDir, "nested_home.blade.tpl")
	pageContent := `@extends('layouts/nested.blade.tpl')
@section('inner')Overridden Inner@endsection
@section('content')<p>Body</p>@endsection`
	if err := os.WriteFile(pagePath, []byte(pageContent), 0644); err != nil {
		t.Fatalf("write page: %v", err)
	}

	c := NewCompilerWithOptions(tmp, "blade", nil)
	compiled, err := c.Compile(pagePath)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// inner override should appear in compiled output
	if !strings.Contains(compiled, "Overridden Inner") {
		t.Fatalf("expected inner block overridden, got: %s", compiled)
	}
}

// Multiple sections: page provides only some sections, layout default preserved for others
func TestMultipleSectionsAndDefaultPreservation(t *testing.T) {
	tmp := t.TempDir()
	layoutsDir := filepath.Join(tmp, "layouts")
	pagesDir := filepath.Join(tmp, "pages")
	_ = os.MkdirAll(layoutsDir, 0755)
	_ = os.MkdirAll(pagesDir, 0755)

	layoutPath := filepath.Join(layoutsDir, "multi.blade.tpl")
	layoutContent := `<html><head><title>{{block "title" .}}Default Title{{end}}</title></head><body><header>{{block "header" .}}Default Header{{end}}</header><main>{{template "content" .}}</main></body></html>`
	if err := os.WriteFile(layoutPath, []byte(layoutContent), 0644); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	pagePath := filepath.Join(pagesDir, "multi_home.blade.tpl")
	pageContent := `@extends('layouts/multi.blade.tpl')
@section('content')<p>Main content</p>@endsection
@section('header')<h1>Custom Header</h1>@endsection`
	if err := os.WriteFile(pagePath, []byte(pageContent), 0644); err != nil {
		t.Fatalf("write page: %v", err)
	}

	c := NewCompilerWithOptions(tmp, "blade", nil)
	compiled, err := c.Compile(pagePath)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// header overridden, title should remain default
	if !strings.Contains(compiled, "Custom Header") {
		t.Fatalf("expected custom header present, got: %s", compiled)
	}
	if !strings.Contains(compiled, "Default Title") {
		t.Fatalf("expected default title preserved, got: %s", compiled)
	}
}
