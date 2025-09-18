package engine

import (
	"io/fs"
	"os"
	"path/filepath"
)

// HybridFS implements fs.FS and tries to open files from disk first (relative to baseDir),
// then falls back to an embedded fs.FS if provided.
type HybridFS struct {
	baseDir  string
	embedded fs.FS
}

// NewHybridFS creates a HybridFS rooted at baseDir. If embedded is nil, it behaves like disk FS.
func NewHybridFS(baseDir string, embedded fs.FS) *HybridFS {
	return &HybridFS{baseDir: baseDir, embedded: embedded}
}

// Open tries disk first, then embedded.
func (h *HybridFS) Open(name string) (fs.File, error) {
	// Normalize name to OS path then try disk
	diskPath := filepath.Join(h.baseDir, filepath.FromSlash(name))
	if f, err := os.Open(diskPath); err == nil {
		return f, nil
	}

	// Fallback to embedded FS if available
	if h.embedded != nil {
		return h.embedded.Open(name)
	}

	// Final: try to open relative to baseDir using embedded semantics
	return nil, fs.ErrNotExist
}
