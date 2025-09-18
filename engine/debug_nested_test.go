package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDebugNestedCompile(t *testing.T) {
	tmp := t.TempDir()
	layoutsDir := filepath.Join(tmp, "layouts")
	pagesDir := filepath.Join(tmp, "pages")
	_ = os.MkdirAll(layoutsDir, 0755)
	_ = os.MkdirAll(pagesDir, 0755)

	layoutPath := filepath.Join(layoutsDir, "nested.blade.tpl")
	layoutContent := `{{block "outer" .}}{{block "inner" .}}Inner Default{{end}}{{end}}<div>{{template "content" .}}</div>`
	_ = os.WriteFile(layoutPath, []byte(layoutContent), 0644)

	pagePath := filepath.Join(pagesDir, "nested_home.blade.tpl")
	pageContent := `@extends('layouts/nested.blade.tpl')
@section('inner')Overridden Inner@endsection
@section('content')<p>Body</p>@endsection`
	_ = os.WriteFile(pagePath, []byte(pageContent), 0644)

	c := NewCompilerWithOptions(tmp, "blade", nil)

	data, _ := os.ReadFile(pagePath)
	content, layout, err := c.processExtends(string(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Log("AFTER extends:")
	t.Log(content)
	content2, err := c.processAllDirectives(content, pagePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("AFTER directives:")
	t.Log(content2)
	combined, err := c.combineWithLayout(content2, layout)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("COMBINED:")
	t.Log(combined)
}
