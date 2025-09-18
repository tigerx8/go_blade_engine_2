package engine

import (
	"fmt"
	fsys "io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// DebugTemplate debug template compilation
func (b *BladeEngine) DebugTemplate(templateName string) {
	fmt.Printf("=== DEBUG TEMPLATE: %s ===\n", templateName)

	templatePath := filepath.Join(b.templatesDir, templateName)
	if !(b.mode == "go" && b.fs != nil) {
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			log.Printf("Template not found: %s", templatePath)
			return
		}
	}

	err := b.compiler.DebugCompile(templatePath)
	if err != nil {
		log.Printf("Debug error: %v", err)
	}
}

// ValidateAllTemplates validate tất cả templates
func (b *BladeEngine) ValidateAllTemplates() error {
	var errors []string
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
			if filepath.Ext(path) != b.templateExtension {
				return nil
			}
			fmt.Println("ValidateAllTemplates " + path)
			// For validation in go+embed mode, read via compiler which knows FS
			joined := filepath.Join(b.templatesDir, path)
			_, err := b.compiler.Compile(joined)
			if err != nil {
				errors = append(errors, fmt.Sprintf("error compiling %s: %v", path, err))
			}
			return nil
		})
	} else {
		err = filepath.Walk(b.templatesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				errors = append(errors, fmt.Sprintf("error accessing %s: %v", path, err))
				return nil
			}

			if !info.IsDir() && (filepath.Ext(path) == b.templateExtension) {
				relPath, err := filepath.Rel(b.templatesDir, path)
				fmt.Println("ValidateAllTemplates " + relPath)
				if err != nil {
					errors = append(errors, fmt.Sprintf("error getting relative path for %s: %v", path, err))
					return nil
				}

				_, err = b.compiler.Compile(path)
				if err != nil {
					errors = append(errors, fmt.Sprintf("error compiling %s: %v", relPath, err))
				}
			}

			return nil
		})
	}

	if err != nil {
		errors = append(errors, fmt.Sprintf("error walking templates directory: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %v", strings.Join(errors, "\n"))
	}

	return nil
}
