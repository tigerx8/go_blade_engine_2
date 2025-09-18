package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher theo dõi thay đổi template files
type FileWatcher struct {
	watcher    *fsnotify.Watcher
	blade      *BladeEngine
	watchDir   string
	extensions []string
}

// NewFileWatcher tạo file watcher mới
func NewFileWatcher(blade *BladeEngine, watchDir string) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:    watcher,
		blade:      blade,
		watchDir:   watchDir,
		extensions: []string{".gohtml", ".blade.tpl"},
	}

	// Ensure engine-configured extension is included
	if blade.templateExtension != "" {
		found := false
		for _, e := range fw.extensions {
			if e == blade.templateExtension {
				found = true
				break
			}
		}
		if !found {
			fw.extensions = append(fw.extensions, blade.templateExtension)
		}
	}

	// Thêm các thư mục để watch
	if err := fw.addWatchRecursive(watchDir); err != nil {
		return nil, err
	}

	return fw, nil
}

// addWatchRecursive thêm đệ quy các thư mục để watch
func (fw *FileWatcher) addWatchRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return fw.watcher.Add(path)
		}

		return nil
	})
}

// Start bắt đầu watching
func (fw *FileWatcher) Start() {
	go func() {
		for {
			select {
			case event, ok := <-fw.watcher.Events:
				if !ok {
					return
				}

				if fw.isTemplateFile(event.Name) && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Remove == fsnotify.Remove) {
					fmt.Printf("Template changed: %s, clearing cache...\n", event.Name)

					// Xóa template khỏi cache (memory + compiled file)
					relPath, err := filepath.Rel(fw.watchDir, event.Name)
					if err == nil {
						fw.blade.ClearCacheFor(relPath)
						// Also remove compiled file cache on disk if exists
						compiledName := strings.ReplaceAll(relPath, string(os.PathSeparator), "_") + ".compiled"
						compiledPath := filepath.Join(filepath.Dir(fw.watchDir), "cache", compiledName)
						_ = os.Remove(compiledPath)
					} else {
						fw.blade.ClearCache()
					}
				}

			case err, ok := <-fw.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()
}

// isTemplateFile kiểm tra nếu file là template file
func (fw *FileWatcher) isTemplateFile(filename string) bool {
	ext := filepath.Ext(filename)
	for _, allowedExt := range fw.extensions {
		if ext == allowedExt {
			return true
		}
	}
	return false
}

// Stop dừng watching
func (fw *FileWatcher) Stop() {
	fw.watcher.Close()
}

// ClearCacheFor xóa template cụ thể khỏi cache
func (b *BladeEngine) ClearCacheFor(templateName string) {
	if b.cacheManager != nil {
		b.cacheManager.Remove(templateName)
	}
}
