package engine

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	fsys "io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// BladeEngine với cache system
type BladeEngine struct {
	templatesDir       string
	templateExtension  string
	cacheManager       *CacheManager
	compiler           *Compiler
	goCompiler         *Compiler
	autoModeExtensions []string
	// instrumentation for which compiler was used recently
	mu               sync.Mutex
	lastUsedCompiler string
	usageCounts      map[string]int
	enableCache      bool
	development      bool    // Development mode (disables cache)
	mode             string  // "blade" or "go"
	fs               fsys.FS // optional embedded FS for go mode
}

// BladeConfig cấu hình cho Blade Engine
type BladeConfig struct {
	TemplatesDir      string
	TemplateExtension string
	CacheEnabled      bool
	Development       bool
	CacheMaxSizeMB    int
	CacheTTLMinutes   int
	Mode              string  // "blade" or "go"
	EmbeddedFS        fsys.FS // optional: use embed.FS when Mode == "go"
	// SkipCompiledExtensions optionally configures which file extensions should not have compiled cache files written.
	// Example: []string{".gohtml", ".html"}
	SkipCompiledExtensions []string
	// AutoModeExtensions configures which extensions should switch to the native Go compiler.
	// Default: [".gohtml", ".html"]
	AutoModeExtensions []string
	// DiskCacheOnly when true disables the in-memory template cache and relies only on
	// compiler-produced compiled files on disk. This is useful when you want to avoid
	// storing parsed *template.Template objects in process memory.
	DiskCacheOnly bool
}

// NewBladeEngineWithConfig tạo Blade Engine với cấu hình
func NewBladeEngineWithConfig(config BladeConfig) *BladeEngine {
	var cacheManager *CacheManager

	// If DiskCacheOnly is requested, avoid creating the in-memory CacheManager.
	// Compiled files on disk are still managed by the Compiler (it creates cacheDir).
	if config.CacheEnabled && !config.DiskCacheOnly {
		cacheManager = NewCacheManager(config.CacheMaxSizeMB, config.CacheTTLMinutes, 5)
	}

	m := strings.ToLower(strings.TrimSpace(config.Mode))
	if m == "" {
		m = "blade"
	}
	// Use HybridFS: in dev we prefer disk, otherwise fallback to bundled embedded FS
	var fsForCompiler fsys.FS
	hybrid := NewHybridFS(config.TemplatesDir, config.EmbeddedFS)
	fsForCompiler = hybrid
	// When in blade mode we keep compiler file IO on disk (compiler uses os.ReadFile)
	compiler := NewCompilerWithOptions(config.TemplatesDir, "blade", fsForCompiler)
	// Also create a native Go compiler that can parse .gohtml/.html templates using the embedded FS when needed
	goCompiler := NewCompilerWithOptions(config.TemplatesDir, "go", config.EmbeddedFS)

	autoExts := config.AutoModeExtensions
	if len(autoExts) == 0 {
		autoExts = []string{".gohtml", ".html"}
	}

	be := &BladeEngine{
		templatesDir:       config.TemplatesDir,
		templateExtension:  config.TemplateExtension,
		cacheManager:       cacheManager,
		compiler:           compiler,
		goCompiler:         goCompiler,
		autoModeExtensions: autoExts,
		usageCounts:        make(map[string]int),
		enableCache:        config.CacheEnabled && !config.DiskCacheOnly,
		development:        config.Development,
		mode:               m,
		fs:                 config.EmbeddedFS,
	}
	// Validate all templates trước khi start
	if err := be.ValidateAllTemplates(); err != nil {
		log.Printf("Template validation errors: %v", err)
		// Có thể continue hoặc exit tùy requirement
	}

	// Preload với error logging
	if err := be.PreloadTemplates(); err != nil {
		log.Printf("Preload warnings: %v", err)
	}

	// Trong development mode, start file watcher
	if config.Development {
		watcher, err := NewFileWatcher(be, config.TemplatesDir)
		if err != nil {
			log.Printf("Could not start file watcher: %v", err)
		} else {
			watcher.Start()
			defer watcher.Stop()
		}
	}
	// If using native Go mode and cache is enabled, remove any legacy compiled cache files for configured extensions
	if be.mode == "go" && be.cacheManager != nil && be.enableCache {
		// perform cleanup for all configured skip extensions
		var exts []string
		if len(config.SkipCompiledExtensions) > 0 {
			exts = config.SkipCompiledExtensions
		} else {
			exts = []string{".gohtml", ".html"}
		}
		for _, ext := range exts {
			_ = be.cacheManager.CleanupCompiledForExtension(ext)
		}
		// propagate skip list to compiler as well
		be.compiler.SetSkipCompiledExtensions(exts)
	} else {
		// Even if not in go mode, allow config to set skip list on compiler
		if len(config.SkipCompiledExtensions) > 0 {
			be.compiler.SetSkipCompiledExtensions(config.SkipCompiledExtensions)
		}
	}

	// // Always remove legacy compiled files for blade templates (e.g., .blade.tpl)
	// if be.cacheManager != nil {
	// 	_ = be.cacheManager.CleanupCompiledForExtension(".blade.tpl")
	// 	// Also remove compiled files that look like they came from layouts/ or components/
	// 	_ = be.cacheManager.CleanupLegacyCompiledForDirs([]string{"layouts", "components"})
	// }

	return be

}

// NewBladeEngine creates a Blade Engine with default configuration
func NewBladeEngine(templatesDir string) *BladeEngine {
	return NewBladeEngineWithConfig(BladeConfig{
		TemplatesDir:    templatesDir,
		CacheEnabled:    true,
		Development:     false,
		CacheMaxSizeMB:  100, // 100MB cache
		CacheTTLMinutes: 60,  // 60 minutes TTL
	})
}

// Render render template với data
func (b *BladeEngine) Render(w io.Writer, templateName string, data interface{}) error {
	// In development mode, do not use cache
	if b.development {
		return b.renderWithoutCache(w, templateName, data)
	}

	// Use cache if enabled
	if b.enableCache && b.cacheManager != nil {
		return b.renderWithCache(w, templateName, data)
	}

	return b.renderWithoutCache(w, templateName, data)
}

// renderWithCache render template sử dụng cache
func (b *BladeEngine) renderWithCache(w io.Writer, templateName string, data interface{}) error {
	// Kiểm tra cache
	if tmpl, found := b.cacheManager.Get(templateName); found {
		return tmpl.Execute(w, data)
	}

	// Template không có trong cache, compile và cache
	tmpl, size, err := b.compileAndCacheTemplate(templateName)
	if err != nil {
		return err
	}

	// Thêm vào cache
	if err := b.cacheManager.Set(templateName, tmpl, size); err != nil {
		fmt.Printf("Warning: Could not cache template %s: %v\n", templateName, err)
	}

	return tmpl.Execute(w, data)
}

// renderWithoutCache render template không sử dụng cache
func (b *BladeEngine) renderWithoutCache(w io.Writer, templateName string, data interface{}) error {
	templatePath := filepath.Join(b.templatesDir, templateName)
	fmt.Println("templatePath", templatePath)
	comp := b.chooseCompilerFor(templateName)
	tmpl, err := comp.ParseTemplate(templatePath)
	if err != nil {
		return err
	}

	return tmpl.Execute(w, data)
}

// compileAndCacheTemplate compile template và tính toán kích thước
func (b *BladeEngine) compileAndCacheTemplate(templateName string) (*template.Template, int, error) {
	templatePath := filepath.Join(b.templatesDir, templateName)

	// Kiểm tra template tồn tại (disk); when using embedded FS in go mode, skip disk check
	if !(b.mode == "go" && b.fs != nil) {
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("template not found: %s", templateName)
		}
	}

	// Choose compiler based on extension
	comp := b.chooseCompilerFor(templateName)

	// Compile template
	tmpl, err := comp.ParseTemplate(templatePath)
	if err != nil {
		return nil, 0, fmt.Errorf("error compiling template %s: %w", templateName, err)
	}

	// Ước tính kích thước template
	size, err := b.estimateTemplateSize(templatePath, tmpl)
	if err != nil {
		// Fallback: sử dụng file size
		if !(b.mode == "go" && b.fs != nil) {
			if fileInfo, err := os.Stat(templatePath); err == nil {
				size = int(fileInfo.Size())
			} else {
				size = 1024 // Default size 1KB
			}
		} else {
			size = 1024 // Default estimate when using embedded FS
		}
	}

	return tmpl, size, nil
}

// RenderString render template thành string
func (b *BladeEngine) RenderString(templateName string, data interface{}) (string, error) {
	var buf bytes.Buffer
	err := b.Render(&buf, templateName, data)
	return buf.String(), err
}

// ClearCache xóa toàn bộ cache
func (b *BladeEngine) ClearCache() {
	if b.cacheManager != nil {
		b.cacheManager.Clear()
	}
}

// CacheStats trả về thống kê cache
func (b *BladeEngine) CacheStats() map[string]interface{} {
	if b.cacheManager != nil {
		return b.cacheManager.Stats()
	}
	return map[string]interface{}{
		"cache_enabled": false,
	}
}

// PreloadTemplates preload các templates vào cache với error handling tốt hơn
func (b *BladeEngine) PreloadTemplates() error {
	if !b.enableCache || b.cacheManager == nil {
		return nil
	}

	var errors []string

	// Clear existing cache before preloading
	b.cacheManager.Clear()

	var err error
	if b.mode == "go" && b.fs != nil {
		// Walk embedded FS
		err = fsys.WalkDir(b.fs, ".", func(path string, d fsys.DirEntry, walkErr error) error {
			if walkErr != nil {
				errors = append(errors, fmt.Sprintf("error accessing %s: %v", path, walkErr))
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if b.templateExtension != "" && filepath.Ext(path) != b.templateExtension {
				return nil
			}
			relPath := filepath.ToSlash(path) // already relative to FS root
			fmt.Println("PreloadTemplates (embed):", relPath)
			tmpl, size, err := b.compileAndCacheTemplate(relPath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("error compiling %s: %v", relPath, err))
				return nil
			}
			if err := b.cacheManager.Set(relPath, tmpl, size); err != nil {
				errors = append(errors, fmt.Sprintf("error caching %s: %v", relPath, err))
			}
			return nil
		})
	} else {
		// Duyệt qua tất cả các file trong templatesDir trên disk
		err = filepath.Walk(b.templatesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				errors = append(errors, fmt.Sprintf("error accessing %s: %v", path, err))
				return nil
			}

			if !info.IsDir() && (b.templateExtension == "" || filepath.Ext(path) == b.templateExtension) {
				relPath, err := filepath.Rel(b.templatesDir, path)
				fmt.Println("PreloadTemplates:", relPath)

				if err != nil {
					errors = append(errors, fmt.Sprintf("error getting relative path for %s: %v", path, err))
					return nil
				}

				// Preload template vào cache
				tmpl, size, err := b.compileAndCacheTemplate(relPath)
				if err != nil {
					errors = append(errors, fmt.Sprintf("error compiling %s: %v", relPath, err))
					return nil
				}

				if err := b.cacheManager.Set(relPath, tmpl, size); err != nil {
					errors = append(errors, fmt.Sprintf("error caching %s: %v", relPath, err))
				}
			}

			return nil
		})
	}

	if err != nil {
		errors = append(errors, fmt.Sprintf("error walking templates directory: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("preload completed with errors: %v", strings.Join(errors, "; "))
	}

	return nil
}

// EnableCache bật/tắt cache
func (b *BladeEngine) EnableCache(enabled bool) {
	b.enableCache = enabled
}

// SetDevelopmentMode sets development mode
func (b *BladeEngine) SetDevelopmentMode(development bool) {
	b.development = development
	// Recreate compiler to use HybridFS (disk-first, fallback to embedded)
	hybrid := NewHybridFS(b.templatesDir, b.fs)
	b.compiler = NewCompilerWithOptions(b.templatesDir, b.mode, hybrid)
}

// SetMode allows switching between "blade" and "go"
func (b *BladeEngine) SetMode(mode string) {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m != "blade" && m != "go" {
		m = "blade"
	}
	b.mode = m
	// recreate compiler with the new mode
	hybrid := NewHybridFS(b.templatesDir, b.fs)
	b.compiler = NewCompilerWithOptions(b.templatesDir, b.mode, hybrid)
}

// IsBladeMode returns true if engine is using Blade syntax
func (b *BladeEngine) IsBladeMode() bool {
	return b.mode == "blade"
}

// GetCachedTemplates returns the list of cached templates
func (b *BladeEngine) GetCachedTemplates() []string {
	if b.cacheManager != nil {
		return b.cacheManager.GetKeys()
	}
	return []string{}
}

// WarmupCache làm nóng cache bằng cách preload các templates thường dùng
func (b *BladeEngine) WarmupCache(templateNames []string) {
	if !b.enableCache || b.cacheManager == nil {
		return
	}

	for _, name := range templateNames {
		if _, found := b.cacheManager.Get(name); !found {
			tmpl, size, err := b.compileAndCacheTemplate(name)
			if err == nil {
				b.cacheManager.Set(name, tmpl, size)
			}
		}
	}
}

// estimateTemplateSize ước tính kích thước template
func (b *BladeEngine) estimateTemplateSize(templatePath string, tmpl *template.Template) (int, error) {
	var buf bytes.Buffer

	// Thử execute template với dữ liệu rỗng
	err := tmpl.Execute(&buf, map[string]interface{}{})
	if err != nil {
		// If execution fails, return the file size
		if fileInfo, err := os.Stat(templatePath); err == nil {
			return int(fileInfo.Size()), nil
		}
		return 0, err
	}

	return buf.Len(), nil
}

// chooseCompilerFor selects the appropriate compiler based on template filename extension
// and records which compiler was used for debugging/metrics.
func (b *BladeEngine) chooseCompilerFor(templateName string) *Compiler {
	// default
	comp := b.compiler
	chosen := "blade"
	for _, ext := range b.autoModeExtensions {
		if strings.HasSuffix(templateName, ext) {
			if b.goCompiler != nil {
				comp = b.goCompiler
				chosen = "go"
			}
			break
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastUsedCompiler = chosen
	b.usageCounts[chosen] = b.usageCounts[chosen] + 1
	// log selection for runtime debugging
	log.Printf("BladeEngine: chose %s compiler for %s", chosen, templateName)
	return comp
}

// LastUsedCompiler returns the name of the last compiler used ("blade" or "go") and a copy of usage counts.
func (b *BladeEngine) LastUsedCompiler() (string, map[string]int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	copyCounts := make(map[string]int, len(b.usageCounts))
	for k, v := range b.usageCounts {
		copyCounts[k] = v
	}
	return b.lastUsedCompiler, copyCounts
}

// CompilerStatsHandler returns an http.HandlerFunc that writes last-used compiler and usage counts
func (b *BladeEngine) CompilerStatsHandler() func(w io.Writer) string {
	// Note: return a helper that returns a string for simplicity; caller may wrap it for HTTP
	return func(w io.Writer) string {
		last, counts := b.LastUsedCompiler()
		var sb strings.Builder
		sb.WriteString("last_used:")
		sb.WriteString(last)
		sb.WriteString("\n")
		for k, v := range counts {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(fmt.Sprintf("%d", v))
			sb.WriteString("\n")
		}
		out := sb.String()
		if w != nil {
			_, _ = w.Write([]byte(out))
		}
		return out
	}
}

// CompilerStatsHTTPHandler returns an http.HandlerFunc that writes the compiler stats
func (b *BladeEngine) CompilerStatsHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Use the helper to get the output string and also write it
		helper := b.CompilerStatsHandler()
		out := helper(nil)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(out))
	}
}
