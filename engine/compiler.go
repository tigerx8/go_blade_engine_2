package engine

import (
	"encoding/json"
	"fmt"
	"html/template"
	fsys "io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"blade_engine/engine/expr"
)

type Compiler struct {
	templatesDir string
	funcMap      template.FuncMap
	cacheDir     string
	mode         string  // "blade" or "go"
	fs           fsys.FS // optional embedded FS, used when mode == "go"
	manifestPath string
	manifest     map[string]string // templatePath -> compiledFilename
	// skipCompiledExtensions lists file extensions (including leading dot) for which
	// the compiler should NOT write standalone compiled cache files.
	skipCompiledExtensions []string
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
	manifestPath := filepath.Join(cacheDir, "compiled_manifest.json")
	// initialize default skip list from env or fallback
	var skipList []string
	if v := os.Getenv("BLADE_SKIP_COMPILED_EXT"); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" && !strings.HasPrefix(p, ".") {
				p = "." + p
			}
			if p != "" {
				skipList = append(skipList, p)
			}
		}
	}
	if len(skipList) == 0 {
		skipList = []string{".gohtml", ".html"}
	}

	c := &Compiler{
		templatesDir: templatesDir,
		cacheDir:     cacheDir,
		mode:         mode,
		fs:           fs,
		manifestPath: manifestPath,
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
			// join: helper for tests that join []string with a separator
			"join": func(sep string, items []string) string {
				return strings.Join(items, sep)
			},
		},
		skipCompiledExtensions: skipList,
	}

	// ensure cache dir exists
	_ = os.MkdirAll(cacheDir, 0755)
	// ensure manifest file exists (create empty JSON if not)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		_ = os.WriteFile(manifestPath, []byte("{}"), 0644)
	}
	// load manifest
	if err := c.loadManifest(); err != nil {
		log.Printf("warning: could not load compiled manifest: %v", err)
	}

	return c
}

// manifest helpers
func (c *Compiler) loadManifest() error {
	c.manifest = make(map[string]string)
	data, err := os.ReadFile(c.manifestPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &c.manifest)
}

func (c *Compiler) saveManifest() error {
	data, err := json.MarshalIndent(c.manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.manifestPath, data, 0644); err != nil {
		return err
	}
	return nil
}

// registerCompiled records that templatePath should have compiled file compiledName
func (c *Compiler) registerCompiled(templatePath, compiledName string) error {
	if c.manifest == nil {
		c.manifest = make(map[string]string)
	}
	c.manifest[templatePath] = compiledName
	if err := c.saveManifest(); err != nil {
		log.Printf("warning: could not save compiled manifest: %v", err)
		return nil
	}
	return nil
}

// manifestFilenameForTemplate returns compiled filename stored in manifest or default derived name
func (c *Compiler) manifestFilenameForTemplate(templatePath string) string {
	if c.manifest == nil {
		return strings.ReplaceAll(templatePath, string(os.PathSeparator), "_") + ".compiled"
	}
	if name, ok := c.manifest[templatePath]; ok {
		return name
	}
	return strings.ReplaceAll(templatePath, string(os.PathSeparator), "_") + ".compiled"
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
	relPath = filepath.ToSlash(relPath)
	// Determine compiled filename using manifest (relative path used as key)
	compiledName := c.manifestFilenameForTemplate(relPath)
	cacheFile := filepath.Join(c.cacheDir, compiledName)

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
		if err := os.WriteFile(cacheFile, []byte(compiled), 0644); err == nil {
			// record mapping in manifest (use relative path key)
			_ = c.registerCompiled(relPath, compiledName)
		}
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
	// skip configured extensions
	for _, ext := range c.skipCompiledExtensions {
		if strings.HasSuffix(rel, ext) {
			return false
		}
	}
	return true
}

// SetSkipCompiledExtensions replaces the skip list used by the compiler. Extensions should include the leading dot.
func (c *Compiler) SetSkipCompiledExtensions(exts []string) {
	cleaned := make([]string, 0, len(exts))
	for _, e := range exts {
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		cleaned = append(cleaned, e)
	}
	c.skipCompiledExtensions = cleaned
}

// processExtends xử lý directive @extends
func (c *Compiler) processExtends(content string) (string, string, error) {
	re := regexp.MustCompile(`@extends\s*\(\s*'([^']+)'\s*\)`)
	matches := re.FindStringSubmatch(content)

	if len(matches) > 1 {
		layout := matches[1]
		// Remove extends directive from content
		content = re.ReplaceAllString(content, "")
		return strings.TrimSpace(content), layout, nil
	}

	return content, "", nil
}

// CompileString compiles Blade-style content into Go template syntax
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

	// Step 1: process extends
	content, layout, err := c.processExtends(content)
	if err != nil {
		return "", fmt.Errorf("error processing extends: %w", err)
	}

	// Step 2: process all directives
	content, err = c.processAllDirectives(content, templatePath)
	if err != nil {
		return "", fmt.Errorf("error processing directives: %w", err)
	}

	// Step 3: if layout provided, combine
	if layout != "" {
		content, err = c.combineWithLayout(content, layout)
		if err != nil {
			return "", fmt.Errorf("error combining with layout %s: %w", layout, err)
		}
	}

	// Step 4: validate template syntax
	if err := c.validateTemplateSyntax(content); err != nil {
		return "", fmt.Errorf("invalid template syntax: %w", err)
	}

	return content, nil
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

	// Ensure no leftover @section/@endsection markers (we matched pairs above).
	if strings.Contains(content, "@section") || strings.Contains(content, "@endsection") {
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

// processInclude handles @include directives
func (c *Compiler) processInclude(content, templatePath string) (string, error) {
	re := regexp.MustCompile(`@include(?:\s*\(\s*)?(?:'([^']+)'|"([^\"]+)"|([a-zA-Z0-9_\-/\.]+))(?:\s*\))?`)
	var includeErr error
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		sub := re.FindStringSubmatch(match)
		var componentName string
		if len(sub) > 1 && sub[1] != "" {
			componentName = sub[1]
		} else if len(sub) > 2 && sub[2] != "" {
			componentName = sub[2]
		} else if len(sub) > 3 {
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

// processIfStatements handles @if directives (translated from Blade to Go templates)
func (c *Compiler) processIfStatements(content, templatePath string) (string, error) {
	// Helper to normalize conditions: $var -> .var
	sanitize := func(cond string) string {
		reVar := regexp.MustCompile(`\$(\w+)`)
		return reVar.ReplaceAllString(cond, ".${1}")
	}

	// 1) Handle @if ... @else ... @endif blocks (multiline) first
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

	// 3) Handle @elseif (placed before @else to avoid conflicts)
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

	// 5) Handle any remaining single @if (open block)
	openIfRe := regexp.MustCompile(`@if\s*\((.*?)\)`)
	content = openIfRe.ReplaceAllStringFunc(content, func(m string) string {
		sub := openIfRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		return "{{if " + sanitize(sub[1]) + "}}"
	})

	// 6) Replace any leftover @endif (start of line, may have indentation)
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
		// Left side (item variable name) used to rewrite the body
		if strings.HasPrefix(item, "$") {
			// keep it for rewrite, but the range will not use this variable
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
		// Use direct range over the collection to avoid ':='
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
func (c *Compiler) processVariables1(content, templatePath string) (string, error) {
	// Use the expr parser/AST to robustly convert $-variables and normalize expressions.
	re := regexp.MustCompile(`{{-?\s*(.*?)\s*-?}}`)

	content = re.ReplaceAllStringFunc(content, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		inner := strings.TrimSpace(sub[1])

		// If expression starts with native Go template keyword, keep as-is
		if toks := regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*)`).FindStringSubmatch(inner); len(toks) > 1 {
			kw := toks[1]
			switch kw {
			case "if", "range", "else", "end", "define", "template", "with", "block":
				return match
			}
		}

		// Try to parse with our expression parser
		p := expr.NewParser(inner)
		ast, err := p.Parse()
		if err != nil {
			// fallback: simple $var -> .var normalization
			innerNorm := regexp.MustCompile(`\$(\w+)`).ReplaceAllString(inner, ".$1")
			return strings.Replace(match, sub[1], innerNorm, 1)
		}

		// Convert AST back to template expression
		s, err := expr.ToTemplate(ast)
		if err != nil {
			innerNorm := regexp.MustCompile(`\$(\w+)`).ReplaceAllString(inner, ".$1")
			return strings.Replace(match, sub[1], innerNorm, 1)
		}

		// If it's a simple dollar variable, wrap with escape
		if expr.IsSimpleDollarVariable(ast) {
			return "{{escape " + s + "}}"
		}

		// otherwise preserve the original delimiters/spacing
		return strings.Replace(match, sub[1], s, 1)
	})

	return content, nil
}
func (c *Compiler) processVariables(content string, templatePath string) (string, error) {
	// match {{ ... }} blocks (non-greedy)
	re := regexp.MustCompile(`{{\s*(.*?)\s*}}`)
	var containsCall func(expr.Expr) bool
	containsCall = func(e expr.Expr) bool {
		switch v := e.(type) {
		case *expr.CallExpr:
			return true
		case *expr.PipeExpr:
			return containsCall(v.Left) || containsCall(v.Right)
		case *expr.DotAccess:
			return containsCall(v.Base)
		case *expr.IndexAccess:
			return containsCall(v.Base) || containsCall(v.Key)
		default:
			return false
		}
	}

	replace := func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		raw := strings.TrimSpace(sub[1])

		// Try parse expression with AST parser.
		if p := expr.NewParser(raw); p != nil {
			if ast, err := p.Parse(); err == nil {
				// If AST contains a call (method/function), preserve original expression exactly.
				if containsCall(ast) {
					// return original unchanged block (preserve spacing inside)
					return fmt.Sprintf("{{ %s }}", raw)
				}
				// Otherwise convert simple $var -> .var style using a conservative replacement.
				// Use AST -> template serialization if available, otherwise fallback to simple replace.
				// Try to stringify AST if package provides ToTemplate (best effort).
				if s, ok := tryASTToTemplate(ast); ok {
					return fmt.Sprintf("{{ %s }}", s)
				}
			}
		}

		// Fallback: simple $var -> .var replacement (conservative)
		conv := regexp.MustCompile(`\$(\w+)`).ReplaceAllString(raw, `.${1}`)
		return fmt.Sprintf("{{ %s }}", conv)
	}

	return re.ReplaceAllStringFunc(content, replace), nil
}

// tryASTToTemplate attempts to render an expr.Expr back to a Go template fragment.
// If the expr package exposes a ToTemplate helper, use it; otherwise return ok=false
func tryASTToTemplate(e expr.Expr) (string, bool) {
	// best-effort: use interface method if implemented
	type toTemplater interface {
		ToTemplate() string
	}
	if tt, ok := e.(toTemplater); ok {
		return tt.ToTemplate(), true
	}
	// no helper available, bail out
	return "", false
}

// helper char checks
// NOTE: previous tokenizer helpers removed after migrating to expr parser.

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

	// Extract define blocks from content: {{define "name"}}...{{end}}
	// We must correctly match the corresponding {{end}} for the define, because the
	// body can contain other {{end}} tokens (from if/range), so a naive regex may
	// stop at the first {{end}}. We'll scan the content and balance block starts/ends.
	defines := make(map[string]string)
	contentNoDefines := content
	startRe := regexp.MustCompile(`{{\s*define\s*"([^"]+)"\s*}}`)
	for {
		loc := startRe.FindStringSubmatchIndex(contentNoDefines)
		if loc == nil {
			break
		}
		// loc gives [start, end, groupStart, groupEnd]
		defStart := loc[0]
		defNameStart := loc[2]
		defNameEnd := loc[3]
		defBodyStart := loc[1]
		name := contentNoDefines[defNameStart:defNameEnd]

		// Now find the matching {{end}} for this define by scanning forward and
		// balancing nested block starts (if, range, define) and ends.
		i := defBodyStart
		depth := 0
		for i < len(contentNoDefines) {
			// find next '{{'
			next := strings.Index(contentNoDefines[i:], "{{")
			if next == -1 {
				// unmatched
				break
			}
			i += next
			// find the end '}}'
			close := strings.Index(contentNoDefines[i:], "}}")
			if close == -1 {
				break
			}
			token := strings.TrimSpace(contentNoDefines[i+2 : i+close])
			if strings.HasPrefix(token, "define") || strings.HasPrefix(token, "if") || strings.HasPrefix(token, "range") || strings.HasPrefix(token, "with") || strings.HasPrefix(token, "block") {
				depth++
			} else if token == "end" {
				if depth == 0 {
					// body runs from defBodyStart to i (start of this '{{end}}')
					body := contentNoDefines[defBodyStart:i]
					defines[name] = body
					// remove the whole define block from contentNoDefines
					// blockEnd is i + len("{{end}}")
					blockEnd := i + close + 2
					contentNoDefines = contentNoDefines[:defStart] + contentNoDefines[blockEnd:]
					// continue scanning from start
					break
				}
				depth--
			}
			// advance past this '}}'
			i = i + close + 2
		}
		// loop again to find more defines
	}

	// Helper to replace a specific template placeholder with the defined body if present
	replaceTemplateWithBody := func(match string) string {
		// match looks like {{template "name" .}}
		r := regexp.MustCompile(`{{\s*template\s*"([^"]+)"\s*\.\s*}}`)
		sub := r.FindStringSubmatch(match)
		if len(sub) >= 2 {
			nm := sub[1]
			if body, ok := defines[nm]; ok {
				return body
			}
			// if no define body, leave as-is (so runtime template lookup may succeed)
			return match
		}
		return match
	}

	// Replace content placeholder first
	yieldRe := regexp.MustCompile(`{{\s*template\s*"content"\s*\.\s*}}`)
	if yieldRe.MatchString(compiledLayout) {
		compiledLayout = yieldRe.ReplaceAllStringFunc(compiledLayout, replaceTemplateWithBody)
	}

	// Replace any other {{template "name" .}} with defined body when available
	anyYieldRe := regexp.MustCompile(`{{\s*template\s*"([^"]+)"\s*\.\s*}}`)
	compiledLayout = anyYieldRe.ReplaceAllStringFunc(compiledLayout, func(m string) string {
		sub := anyYieldRe.FindStringSubmatch(m)
		if len(sub) >= 2 {
			nm := sub[1]
			if body, ok := defines[nm]; ok {
				return body
			}
		}
		return m
	})

	// If layout contains block placeholders (which provide default content),
	// and the page provided a section (define) with the same name, replace the
	// entire block (including its default content) with the page's section body.
	// This implements Blade-style section overriding where layout blocks are
	// replaced by page sections.
	blockRe := regexp.MustCompile(`(?s){{\s*block\s*"([^"]+)"\s*\.\s*}}(.*?){{\s*end\s*}}`)
	// Iteratively replace blocks to correctly handle nested blocks: replace until
	// no block tokens remain or replacements stop changing the layout.
	for blockRe.MatchString(compiledLayout) {
		before := compiledLayout
		compiledLayout = blockRe.ReplaceAllStringFunc(compiledLayout, func(m string) string {
			sub := blockRe.FindStringSubmatch(m)
			if len(sub) >= 3 {
				nm := sub[1]
				if body, ok := defines[nm]; ok {
					// Use the page-provided body instead of layout default
					return body
				}
				// otherwise keep the default content (sub[2])
				return sub[2]
			}
			return m
		})
		if compiledLayout == before {
			break
		}
	}

	// If layout had no placeholders replaced, prefer content bodies by appending
	if contentNoDefines != "" {
		compiledLayout = compiledLayout + contentNoDefines
	} else {
		compiledLayout = compiledLayout + content
	}

	// Prepend all define blocks as named templates so any {{template "name" .}} lookups
	// resolve even if we didn't inline them. This prevents 'no such template' errors.
	if len(defines) > 0 {
		var defs strings.Builder
		for nm, body := range defines {
			// don't prepend if compiledLayout already contains a define or block for this name
			// ({{block "name" ...}} also registers a template with that name and will
			// cause a duplicate-definition error if we prepend another {{define}}).
			if strings.Contains(compiledLayout, "{{define \""+nm+"\"}}") || strings.Contains(compiledLayout, "{{block \""+nm+"\"") {
				continue
			}
			defs.WriteString("{{define \"")
			defs.WriteString(nm)
			defs.WriteString("\"}}")
			defs.WriteString(body)
			defs.WriteString("{{end}}")
		}
		if defs.Len() > 0 {
			compiledLayout = defs.String() + compiledLayout
		}
	}

	return compiledLayout, nil
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

	// Validate balanced directives (count opening directive tokens vs closing {{end}})
	startIf := strings.Count(content, "{{if")
	startRange := strings.Count(content, "{{range")
	startDefine := strings.Count(content, "{{define")
	startWith := strings.Count(content, "{{with")
	startBlock := strings.Count(content, "{{block")
	totalStart := startIf + startRange + startDefine + startWith + startBlock
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
