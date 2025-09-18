package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSectionOverridesBlockAndDefinePrepend(t *testing.T) {
	tmp := t.TempDir()

	// create layout with a block and template placeholder
	layoutsDir := filepath.Join(tmp, "layouts")
	pagesDir := filepath.Join(tmp, "pages")
	_ = os.MkdirAll(layoutsDir, 0755)
	_ = os.MkdirAll(pagesDir, 0755)

	layoutPath := filepath.Join(layoutsDir, "base.blade.tpl")
	layoutContent := `<!doctype html>
<html>
<head>
  <title>{{block "title" .}}Default Title{{end}}</title>
</head>
<body>
  {{template "content" .}}
</body>
</html>`
	if err := os.WriteFile(layoutPath, []byte(layoutContent), 0644); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	// create page that extends layout and defines sections
	pagePath := filepath.Join(pagesDir, "home.blade.tpl")
	pageContent := `@extends('layouts/base.blade.tpl')
@section('title')My Page Title@endsection
@section('content')<p>Hello world</p>@endsection`
	if err := os.WriteFile(pagePath, []byte(pageContent), 0644); err != nil {
		t.Fatalf("write page: %v", err)
	}

	c := NewCompilerWithOptions(tmp, "blade", nil)
	compiled, err := c.Compile(pagePath)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// The compiled should contain the define for title and content
	if !strings.Contains(compiled, `{{define "title"}}My Page Title{{end}}`) {
		t.Fatalf("expected define title present, got: %s", compiled)
	}
	if !strings.Contains(compiled, `My Page Title`) {
		t.Fatalf("expected title visible in compiled layout, got: %s", compiled)
	}

	// Ensure the define appears before the <title> usage so runtime template lookup will succeed
	idxDefine := strings.Index(compiled, `{{define "title"}}`)
	idxTitle := strings.Index(compiled, "<title>")
	if idxDefine == -1 || idxTitle == -1 || idxDefine > idxTitle {
		t.Fatalf("expected defines to be prepended before title usage; idxDefine=%d idxTitle=%d", idxDefine, idxTitle)
	}
}
