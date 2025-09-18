package engine

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	fsys "io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BladeEngine với cache system
type BladeEngine struct {
	templatesDir      string
	templateExtension string
	cacheManager      *CacheManager
	compiler          *Compiler
	enableCache       bool
	development       bool    // Chế độ development (tắt cache)
	mode              string  // "blade" or "go"
	fs                fsys.FS // optional embedded FS for go mode
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
}

// NewBladeEngineWithConfig tạo Blade Engine với cấu hình
func NewBladeEngineWithConfig(config BladeConfig) *BladeEngine {
	var cacheManager *CacheManager

	if config.CacheEnabled {
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
	compiler := NewCompilerWithOptions(config.TemplatesDir, m, fsForCompiler)

	return &BladeEngine{
		templatesDir:      config.TemplatesDir,
		templateExtension: config.TemplateExtension,
		cacheManager:      cacheManager,
		compiler:          compiler,
		enableCache:       config.CacheEnabled,
		development:       config.Development,
		mode:              m,
		fs:                config.EmbeddedFS,
	}
}

// NewBladeEngine tạo Blade Engine với cấu hình mặc định
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
	// Trong chế độ development, không sử dụng cache
	if b.development {
		return b.renderWithoutCache(w, templateName, data)
	}

	// Sử dụng cache nếu được enable
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
	tmpl, err := b.compiler.ParseTemplate(templatePath)
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

	// Compile template
	tmpl, err := b.compiler.ParseTemplate(templatePath)
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

// SetDevelopmentMode đặt chế độ development
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

// GetCachedTemplates trả về danh sách templates đã cache
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
		// Nếu không execute được, trả về file size
		if fileInfo, err := os.Stat(templatePath); err == nil {
			return int(fileInfo.Size()), nil
		}
		return 0, err
	}

	return buf.Len(), nil
}
