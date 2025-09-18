package engine

import (
	"fmt"
	"html/template"
	fsys "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Compiler struct {
	templatesDir string
	funcMap      template.FuncMap
	cacheDir     string
	mode         string  // "blade" or "go"
	fs           fsys.FS // optional embedded FS, used when mode == "go"
}

func NewCompiler(templatesDir string) *Compiler {
	return NewCompilerWithMode(templatesDir, "blade")
}

// NewCompilerWithMode allows selecting compilation mode: "blade" or "go"
func NewCompilerWithMode(templatesDir, mode string) *Compiler {
	return NewCompilerWithOptions(templatesDir, mode, nil)
}

// NewCompilerWithOptions allows providing an embedded FS for go mode
func NewCompilerWithOptions(templatesDir, mode string, fs fsys.FS) *Compiler {
	cacheDir := filepath.Join(filepath.Dir(templatesDir), "cache")
	return &Compiler{
		templatesDir: templatesDir,
		cacheDir:     cacheDir,
		mode:         mode,
		fs:           fs,
		funcMap: template.FuncMap{
			"escape": func(s string) template.HTML {
				return template.HTML(template.HTMLEscapeString(s))
			},
			"raw": func(s string) template.HTML {
				return template.HTML(s)
			},
			"isset": func(data interface{}, key string) bool {
				if m, ok := data.(map[string]interface{}); ok {
					_, exists := m[key]
					return exists
				}
				return false
			},
		},
	}
}

// Compile biên dịch template từ Blade syntax sang Go template syntax
func (c *Compiler) Compile(templatePath string) (string, error) {
	// If in native Go mode, skip compiled file cache entirely: read and validate template then return
	if c.mode == "go" {
		var content []byte
		var err error
		if c.fs != nil {
			rel, _ := filepath.Rel(c.templatesDir, templatePath)
			content, err = fsys.ReadFile(c.fs, filepath.ToSlash(rel))
		} else {
			content, err = os.ReadFile(templatePath)
		}
		if err != nil {
			return "", fmt.Errorf("error reading template %s: %w", templatePath, err)
		}
		// CompileString in go mode will validate and return the content unchanged
		compiled, err := c.CompileString(string(content), templatePath)
		if err != nil {
			return "", err
		}
		return compiled, nil
	}
	// For blade mode, we may or may not write compiled cache files.
	// Layouts and components are typically included into pages and don't need standalone compiled cache files.
	relPath, _ := filepath.Rel(c.templatesDir, templatePath)
	cacheFile := filepath.Join(c.cacheDir, strings.ReplaceAll(relPath, string(os.PathSeparator), "_")+".compiled")

	// If we should read from cache for this template, and cache is fresh, return it
	if c.shouldWriteCompiled(templatePath) {
		cacheInfo, cacheErr := os.Stat(cacheFile)
		tplInfo, tplErr := os.Stat(templatePath)
		if cacheErr == nil && tplErr == nil && cacheInfo.ModTime().After(tplInfo.ModTime()) {
			compiled, err := os.ReadFile(cacheFile)
			if err == nil {
				return string(compiled), nil
			}
		}
	}

	// Compile by reading from disk (blade mode)
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("error reading template %s: %w", templatePath, err)
	}
	compiled, err := c.CompileString(string(content), templatePath)
	if err != nil {
		return "", err
	}

	// Write compiled cache only for templates that need it
	if c.shouldWriteCompiled(templatePath) {
		_ = os.WriteFile(cacheFile, []byte(compiled), 0644)
	}

	return compiled, nil
}

// shouldWriteCompiled returns false for templates that shouldn't generate standalone compiled cache files
func (c *Compiler) shouldWriteCompiled(templatePath string) bool {
	rel, err := filepath.Rel(c.templatesDir, templatePath)
	if err != nil {
		return true
	}
	rel = filepath.ToSlash(rel)
	// skip layouts and components, they are included into other templates
	if strings.HasPrefix(rel, "layouts/") || strings.HasPrefix(rel, "components/") {
		return false
	}
	return true
}

// CompileString biên dịch từ string content
func (c *Compiler) CompileString(content, templatePath string) (string, error) {
	var err error

	// In native Go template mode, return content unchanged (no Blade transforms)
	if c.mode == "go" {
		// Still validate template syntax for Go templates
		if err := c.validateTemplateSyntax(content); err != nil {
			return "", fmt.Errorf("invalid template syntax: %w", err)
		}
		return content, nil
	}

	// Bước 1: Phân tích và xử lý extends
	content, layout, err := c.processExtends(content)
	if err != nil {
		return "", fmt.Errorf("error processing extends: %w", err)
	}

	// Bước 2: Xử lý tất cả các directive
	content, err = c.processAllDirectives(content, templatePath)
	if err != nil {
		return "", fmt.Errorf("error processing directives: %w", err)
	}

	// Bước 3: Nếu có layout, kết hợp với layout
	if layout != "" {
		content, err = c.combineWithLayout(content, layout)
		if err != nil {
			return "", fmt.Errorf("error combining with layout %s: %w", layout, err)
		}
	}

	// Bước 4: Validate template syntax
	if err := c.validateTemplateSyntax(content); err != nil {
		return "", fmt.Errorf("invalid template syntax: %w", err)
	}

	return content, nil
}

// processExtends xử lý directive @extends
func (c *Compiler) processExtends(content string) (string, string, error) {
	re := regexp.MustCompile(`@extends\s*\(\s*'([^']+)'\s*\)`)
	matches := re.FindStringSubmatch(content)

	if len(matches) > 1 {
		layout := matches[1]
		// Remove extends directive từ content
		content = re.ReplaceAllString(content, "")
		return strings.TrimSpace(content), layout, nil
	}

	return content, "", nil
}

// processAllDirectives xử lý tất cả các directive Blade
func (c *Compiler) processAllDirectives(content, templatePath string) (string, error) {
	directives := []func(string, string) (string, error){
		c.processSections,
		c.processYield,
		c.processInclude,
		c.processIfStatements,
		c.processElse,
		c.processEndif,
		c.processForeach,
		c.processEndForeach,
		c.processVariables,
		c.processRawVariables,
		c.processComments,
		c.processPhp,
		c.processUnless,
	}

	var err error
	for _, directive := range directives {
		content, err = directive(content, templatePath)
		if err != nil {
			return "", err
		}
	}

	return content, nil
}

// processSections xử lý @section và @endsection
func (c *Compiler) processSections(content, templatePath string) (string, error) {
	// Xử lý @section('name') ... @endsection trên nhiều dòng (dùng (?s) cho dotall)
	re := regexp.MustCompile(`(?s)@section\s*\(\s*['"]([^'"]+)['"]\s*\)\s*(.*?)\s*@endsection`)
	content = re.ReplaceAllString(content, "{{define \"$1\"}}$2{{end}}")

	// Xử lý @section('name') trên single line
	re = regexp.MustCompile(`@section\s*\(\s*['"]([^'"]+)['"]\s*\)\s*`)
	content = re.ReplaceAllString(content, "{{define \"$1\"}}")

	// Đảm bảo tất cả sections đều có end
	if strings.Count(content, "{{define") != strings.Count(content, "{{end}}") {
		return "", fmt.Errorf("unclosed section in template %s", templatePath)
	}

	return content, nil
}

// processYield xử lý @yield
func (c *Compiler) processYield(content, templatePath string) (string, error) {
	re := regexp.MustCompile(`@yield(?:\s*\(\s*)?(?:'([^']+)'|"([^"]+)"|([a-zA-Z0-9_\-/\.]+))(?:\s*\))?`)
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		sub := re.FindStringSubmatch(match)
		var name string
		if sub[1] != "" {
			name = sub[1]
		} else if sub[2] != "" {
			name = sub[2]
		} else {
			name = sub[3]
		}
		return "{{template \"" + name + "\" .}}"
	})
	return content, nil
}

// processInclude xử lý @include
func (c *Compiler) processInclude(content, templatePath string) (string, error) {
	re := regexp.MustCompile(`@include(?:\s*\(\s*)?(?:'([^']+)'|"([^"]+)"|([a-zA-Z0-9_\-/\.]+))(?:\s*\))?`)
	var includeErr error
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		sub := re.FindStringSubmatch(match)
		var componentName string
		if sub[1] != "" {
			componentName = sub[1]
		} else if sub[2] != "" {
			componentName = sub[2]
		} else {
			componentName = sub[3]
		}
		componentPath := filepath.Join(c.templatesDir, componentName)
		if _, err := os.Stat(componentPath); os.IsNotExist(err) {
			includeErr = fmt.Errorf("included template not found: %s", componentName)
			return match
		}
		componentContent, err := c.Compile(componentPath)
		if err != nil {
			includeErr = fmt.Errorf("error compiling included template %s: %w", componentName, err)
			return match
		}
		return componentContent
	})
	if includeErr != nil {
		return "", includeErr
	}
	return content, nil
}

// processIfStatements xử lý @if
func (c *Compiler) processIfStatements(content, templatePath string) (string, error) {
	// Helper chuẩn hoá điều kiện: $var -> .var
	sanitize := func(cond string) string {
		reVar := regexp.MustCompile(`\$(\w+)`)
		return reVar.ReplaceAllString(cond, ".$1")
	}

	// 1) Xử lý khối @if ... @else ... @endif (đa dòng) trước
	ifElseRe := regexp.MustCompile(`(?s)@if\s*\((.*?)\)\s*(.*?)\s*@else\s*(.*?)\s*@endif`)
	content = ifElseRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := ifElseRe.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		cond := sanitize(sub[1])
		return "{{if " + cond + "}}" + sub[2] + "{{else}}" + sub[3] + "{{end}}"
	})

	// 2) Xử lý khối @if ... @endif không có else
	ifRe := regexp.MustCompile(`(?s)@if\s*\((.*?)\)\s*(.*?)\s*@endif`)
	content = ifRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := ifRe.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		cond := sanitize(sub[1])
		return "{{if " + cond + "}}" + sub[2] + "{{end}}"
	})

	// 3) Xử lý @elseif (đặt trước @else để tránh xung đột)
	elseifRe := regexp.MustCompile(`(?m)^\s*@elseif\s*\((.*?)\)\s*$`)
	content = elseifRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := elseifRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		return "{{else if " + sanitize(sub[1]) + "}}"
	})

	// 4) Xử lý @else trên nhiều dòng
	elseRe := regexp.MustCompile(`(?m)^\s*@else\s*$`)
	content = elseRe.ReplaceAllString(content, "{{else}}")

	// 5) Xử lý @if đơn lẻ còn sót (mở block)
	openIfRe := regexp.MustCompile(`@if\s*\((.*?)\)`)
	content = openIfRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := openIfRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		return "{{if " + sanitize(sub[1]) + "}}"
	})

	// 6) Thay thế @endif còn sót (đầu dòng, có thể có thụt lề)
	endIfRe := regexp.MustCompile(`(?m)^[ \t]*@endif[ \t]*$`)
	content = endIfRe.ReplaceAllString(content, "{{end}}")

	return content, nil
}

func (c *Compiler) processElse(content, templatePath string) (string, error) {
	// Xử lý @elseif trên nhiều dòng và có thể có thụt lề
	elseifRe := regexp.MustCompile(`(?m)^\s*@elseif\s*\((.*?)\)`)
	content = elseifRe.ReplaceAllString(content, "{{else if $1}}")

	// Xử lý @else trên nhiều dòng và có thể có thụt lề
	elseRe := regexp.MustCompile(`(?m)^\s*@else\s*`)
	content = elseRe.ReplaceAllString(content, "{{else}}")

	return content, nil
}

func (c *Compiler) processEndif(content, templatePath string) (string, error) {
	content = strings.ReplaceAll(content, "@endif", "{{end}}")
	return content, nil
}

// processForeach xử lý @foreach
func (c *Compiler) processForeach(content, templatePath string) (string, error) {
	normalize := func(coll, item string) (string, string) {
		coll = strings.TrimSpace(coll)
		item = strings.TrimSpace(item)
		// Chuẩn hoá collection: luôn dùng dot context
		if strings.HasPrefix(coll, "$") {
			coll = "." + strings.TrimPrefix(coll, "$")
		} else if !strings.HasPrefix(coll, ".") {
			coll = "." + coll
		}
		// Bên trái (tên biến item) để phục vụ rewrite body
		if strings.HasPrefix(item, "$") {
			// giữ lại để rewrite, nhưng range sẽ không dùng biến này
		} else {
			item = "$" + item
		}
		return coll, item
	}

	// Xử lý block @foreach ... @endforeach trên nhiều dòng, kể cả thụt lề
	reBlock := regexp.MustCompile(`(?s)@foreach\s*\((.*?)\s+as\s+(.*?)\)\s*(.*?)\n[ \t]*@endforeach`)
	content = reBlock.ReplaceAllStringFunc(content, func(m string) string {
		sub := reBlock.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		coll, item := normalize(sub[1], sub[2])
		body := sub[3]
		// Chuyển tham chiếu $item.xxx -> .xxx trong body
		itemName := strings.TrimPrefix(item, "$")
		reVar := regexp.MustCompile(`{{\s*\$` + regexp.QuoteMeta(itemName) + `\.`)
		body = reVar.ReplaceAllString(body, "{{ .")
		reRaw := regexp.MustCompile(`{!!\s*\$` + regexp.QuoteMeta(itemName) + `\.`)
		body = reRaw.ReplaceAllString(body, "{!! .")
		// Dùng range collection trực tiếp để tránh ':='
		return "{{range " + coll + "}}" + body + "{{end}}"
	})

	// Xử lý @foreach trên single line (mở block)
	reOpen := regexp.MustCompile(`@foreach\s*\((.*?)\s+as\s+(.*?)\)`)
	content = reOpen.ReplaceAllStringFunc(content, func(m string) string {
		sub := reOpen.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		coll, _ := normalize(sub[1], sub[2])
		return "{{range " + coll + "}}"
	})

	// Xử lý @endforeach còn sót lại (có thể có thụt lề)
	reEnd := regexp.MustCompile(`(?m)^[ \t]*@endforeach[ \t]*$`)
	content = reEnd.ReplaceAllString(content, "{{end}}")

	return content, nil
}

func (c *Compiler) processEndForeach(content, templatePath string) (string, error) {
	content = strings.ReplaceAll(content, "@endforeach", "{{end}}")
	return content, nil
}

// processUnless xử lý @unless
func (c *Compiler) processUnless(content, templatePath string) (string, error) {
	// Xử lý block @unless ... @endunless trên nhiều dòng
	re := regexp.MustCompile(`(?s)@unless\s*\((.*?)\)\s*(.*?)\s*@endunless`)
	content = re.ReplaceAllString(content, "{{if not $1}}$2{{end}}")

	// Xử lý @unless trên single line
	re = regexp.MustCompile(`@unless\s*\((.*?)\)`)
	content = re.ReplaceAllString(content, "{{if not $1}}")
	content = strings.ReplaceAll(content, "@endunless", "{{end}}")
	return content, nil
}

// processVariables xử lý {{ }}
func (c *Compiler) processVariables(content, templatePath string) (string, error) {
	// Xử lý các biến thông thường {{ $var }} và chuyển $var -> .var
	re := regexp.MustCompile(`{{\s*(.*?)\s*}}`)
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		// Không xử lý các directive đã được convert
		if strings.Contains(match, "{{if") || strings.Contains(match, "{{range") ||
			strings.Contains(match, "{{else") || strings.Contains(match, "{{end") ||
			strings.Contains(match, "{{define") || strings.Contains(match, "{{template") {
			return match
		}

		// Xử lý các biến bình thường
		inner := re.FindStringSubmatch(match)[1]
		// Thay $name -> .name
		inner = regexp.MustCompile(`\$(\w+)`).ReplaceAllString(inner, ".$1")
		return "{{escape " + inner + "}}"
	})

	return content, nil
}

// processRawVariables xử lý {!! !!}
func (c *Compiler) processRawVariables(content, templatePath string) (string, error) {
	re := regexp.MustCompile(`{!!\s*(.*?)\s*!!}`)
	content = re.ReplaceAllStringFunc(content, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		expr := regexp.MustCompile(`\$(\w+)`).ReplaceAllString(sub[1], ".$1")
		return "{{raw " + expr + "}}"
	})
	return content, nil
}

// processComments xử lý {{-- --}}
func (c *Compiler) processComments(content, templatePath string) (string, error) {
	re := regexp.MustCompile(`{{--(.*?)--}}`)
	content = re.ReplaceAllString(content, "")
	return content, nil
}

// processPhp xử lý @php @endphp
func (c *Compiler) processPhp(content, templatePath string) (string, error) {
	re := regexp.MustCompile(`@php(.*?)@endphp`)
	content = re.ReplaceAllString(content, "")
	return content, nil
}

// combineWithLayout kết hợp content với layout
func (c *Compiler) combineWithLayout(content, layoutName string) (string, error) {
	layoutPath := filepath.Join(c.templatesDir, layoutName)

	// Kiểm tra layout tồn tại
	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		return "", fmt.Errorf("layout not found: %s", layoutName)
	}

	layoutContent, err := os.ReadFile(layoutPath)
	if err != nil {
		return "", fmt.Errorf("error reading layout %s: %w", layoutName, err)
	}

	compiledLayout, err := c.CompileString(string(layoutContent), layoutPath)
	if err != nil {
		return "", fmt.Errorf("error compiling layout %s: %w", layoutName, err)
	}

	// Tìm và thay thế @yield('content') trong layout
	yieldRe := regexp.MustCompile(`{{\s*template\s*"content"\s*\.\s*}}`)
	if yieldRe.MatchString(compiledLayout) {
		return yieldRe.ReplaceAllString(compiledLayout, content), nil
	}

	// Fallback: tìm bất kỳ yield nào
	anyYieldRe := regexp.MustCompile(`{{\s*template\s*"[^"]+"\s*\.\s*}}`)
	if anyYieldRe.MatchString(compiledLayout) {
		return anyYieldRe.ReplaceAllString(compiledLayout, content), nil
	}

	// Nếu không tìm thấy yield, append content
	return compiledLayout + content, nil
}

// validateTemplateSyntax validate cú pháp template
func (c *Compiler) validateTemplateSyntax(content string) error {
	// Tạo template test để validate syntax
	testTmpl := template.New("validation").Funcs(c.funcMap)
	_, err := testTmpl.Parse(content)
	if err != nil {
		return fmt.Errorf("template syntax error: %w", err)
	}

	// Validate balanced brackets
	if strings.Count(content, "{{") != strings.Count(content, "}}") {
		return fmt.Errorf("unbalanced template brackets")
	}

	// Validate balanced directives (tính tổng số mở và tổng số đóng)
	startIf := strings.Count(content, "{{if")
	startRange := strings.Count(content, "{{range")
	startDefine := strings.Count(content, "{{define")
	totalStart := startIf + startRange + startDefine
	totalEnd := strings.Count(content, "{{end}}")
	if totalStart != totalEnd {
		return fmt.Errorf("unbalanced directives: starts=%d (if=%d, range=%d, define=%d) ends=%d", totalStart, startIf, startRange, startDefine, totalEnd)
	}

	return nil
}

// ParseTemplate phân tích cú pháp template và trả về Go template
func (c *Compiler) ParseTemplate(templatePath string) (*template.Template, error) {
	// In go mode with embedded FS, parse the full template set using ParseFS so cross-file templates are available
	if c.mode == "go" && c.fs != nil {
		rootName := filepath.Base(templatePath)
		tmpl := template.New(rootName).Funcs(c.funcMap)
		// parse common folders, support both .html and .gohtml
		if _, err := tmpl.ParseFS(c.fs, "components/*.*html", "layouts/*.*html", "pages/*.*html"); err == nil {
			return tmpl, nil
		}
		// fallback to reading just the requested file if pattern parsing fails
		rel, _ := filepath.Rel(c.templatesDir, templatePath)
		if data, err := fsys.ReadFile(c.fs, filepath.ToSlash(rel)); err == nil {
			return tmpl.Parse(string(data))
		}
		// continue to fallback below
	}

	compiledContent, err := c.Compile(templatePath)
	if err != nil {
		return nil, err
	}

	tmpl := template.New(filepath.Base(templatePath)).Funcs(c.funcMap)
	return tmpl.Parse(compiledContent)
}

// DebugCompile hiển thị quá trình biên dịch từng bước
func (c *Compiler) DebugCompile(templatePath string) error {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	original := string(content)

	fmt.Println("=== ORIGINAL TEMPLATE ===")
	fmt.Println(original)

	fmt.Println("\n=== AFTER PROCESSING EXTENDS ===")
	content1, layout, err := c.processExtends(original)
	if err != nil {
		return err
	}
	fmt.Printf("Layout: %s\n", layout)
	fmt.Println(content1)

	fmt.Println("\n=== AFTER PROCESSING SECTIONS ===")
	content2, err := c.processSections(content1, templatePath)
	if err != nil {
		return err
	}
	fmt.Println(content2)

	fmt.Println("\n=== FINAL COMPILED TEMPLATE ===")
	final, err := c.Compile(templatePath)
	if err != nil {
		return err
	}
	fmt.Println(final)

	return nil
}
