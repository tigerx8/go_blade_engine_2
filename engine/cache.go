package engine

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheItem đại diện cho một item trong cache
type CacheItem struct {
	Template   *template.Template
	CreatedAt  time.Time
	LastUsedAt time.Time
	Size       int // Kích thước ước tính của template
}

// CacheManager quản lý cache template
type CacheManager struct {
	items           map[string]*CacheItem
	mutex           sync.RWMutex  // Bảo vệ truy cập đồng thời
	maxSize         int           // Kích thước cache tối đa (bytes)
	currentSize     int           // Kích thước cache hiện tại
	ttl             time.Duration // Time-to-live cho cache items
	cleanupInterval time.Duration // Khoảng thời gian dọn dẹp
	stopCleanup     chan bool     // Kênh để dừng routine dọn dẹp
	cacheDir        string        // Thư mục lưu file cache
}

// NewCacheManager tạo cache manager mới
func NewCacheManager(maxSizeMB int, ttlMinutes, cleanupMinutes int) *CacheManager {
	maxSize := maxSizeMB * 1024 * 1024 // Convert MB to bytes

	cacheDir := "./cache"
	os.MkdirAll(cacheDir, 0755)
	cm := &CacheManager{
		items:           make(map[string]*CacheItem),
		maxSize:         maxSize,
		ttl:             time.Duration(ttlMinutes) * time.Minute,
		cleanupInterval: time.Duration(cleanupMinutes) * time.Minute,
		stopCleanup:     make(chan bool),
		cacheDir:        cacheDir,
	}

	// Bắt đầu routine dọn dẹp cache định kỳ
	go cm.startCleanupRoutine()

	return cm
}

// Get lấy template từ cache
func (cm *CacheManager) Get(key string) (*template.Template, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if item, exists := cm.items[key]; exists {
		item.LastUsedAt = time.Now()
		return item.Template, true
	}

	return nil, false
}

// Set thêm template vào cache
func (cm *CacheManager) Set(key string, tmpl *template.Template, size int) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Kiểm tra nếu cache đã đầy
	if cm.currentSize+size > cm.maxSize {
		cm.evictOldItems()
	}

	// Nếu vẫn còn đầy sau khi evict, return error
	if cm.currentSize+size > cm.maxSize {
		return fmt.Errorf("cache is full, cannot add item")
	}

	cm.items[key] = &CacheItem{
		Template:   tmpl,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		Size:       size,
	}

	cm.currentSize += size
	// Lưu file cache compiled
	cacheFile := filepath.Join(cm.cacheDir, strings.ReplaceAll(key, string(os.PathSeparator), "_")+".compiled")
	compiledStr := ""
	if tmpl != nil {
		var buf bytes.Buffer
		_ = tmpl.Execute(&buf, map[string]interface{}{})
		compiledStr = buf.String()
	}
	_ = os.WriteFile(cacheFile, []byte(compiledStr), 0644)
	return nil
}

// Remove xóa template khỏi cache
func (cm *CacheManager) Remove(key string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if item, exists := cm.items[key]; exists {
		cm.currentSize -= item.Size
		delete(cm.items, key)
		// Xóa file cache
		cacheFile := filepath.Join(cm.cacheDir, strings.ReplaceAll(key, string(os.PathSeparator), "_")+".compiled")
		_ = os.Remove(cacheFile)
	}
}

// Clear xóa toàn bộ cache
func (cm *CacheManager) Clear() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.items = make(map[string]*CacheItem)
	cm.currentSize = 0
	// Xóa toàn bộ file cache
	entries, _ := os.ReadDir(cm.cacheDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			_ = os.Remove(filepath.Join(cm.cacheDir, entry.Name()))
		}
	}
}

// GetFileCache đọc nội dung file cache compiled
func (cm *CacheManager) GetFileCache(key string) (string, bool) {
	cacheFile := filepath.Join(cm.cacheDir, strings.ReplaceAll(key, string(os.PathSeparator), "_")+".compiled")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// evictOldItems xóa các item cũ để giải phóng bộ nhớ
func (cm *CacheManager) evictOldItems() {
	// Ưu tiên xóa items ít được sử dụng nhất và cũ nhất
	var keysToRemove []string

	for key, item := range cm.items {
		// Kiểm tra TTL
		if time.Since(item.CreatedAt) > cm.ttl {
			keysToRemove = append(keysToRemove, key)
			continue
		}

		// Kiểm tra nếu không được sử dụng trong 2 lần TTL
		if time.Since(item.LastUsedAt) > cm.ttl*2 {
			keysToRemove = append(keysToRemove, key)
		}
	}

	// Xóa các items đã chọn
	for _, key := range keysToRemove {
		cm.currentSize -= cm.items[key].Size
		delete(cm.items, key)
	}
}

// startCleanupRoutine chạy routine dọn dẹp cache định kỳ
func (cm *CacheManager) startCleanupRoutine() {
	ticker := time.NewTicker(cm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.cleanupExpiredItems()
		case <-cm.stopCleanup:
			return
		}
	}
}

// cleanupExpiredItems dọn dẹp các items đã hết hạn
func (cm *CacheManager) cleanupExpiredItems() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	now := time.Now()
	for key, item := range cm.items {
		if now.Sub(item.CreatedAt) > cm.ttl {
			cm.currentSize -= item.Size
			delete(cm.items, key)
		}
	}
}

// Stop dừng cleanup routine
func (cm *CacheManager) Stop() {
	close(cm.stopCleanup)
}

// Stats trả về thống kê cache
func (cm *CacheManager) Stats() map[string]interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return map[string]interface{}{
		"total_items":     len(cm.items),
		"current_size_mb": cm.currentSize / (1024 * 1024),
		"max_size_mb":     cm.maxSize / (1024 * 1024),
		"memory_usage":    fmt.Sprintf("%.1f%%", float64(cm.currentSize)/float64(cm.maxSize)*100),
	}
}

// GetKeys trả về danh sách keys trong cache
func (cm *CacheManager) GetKeys() []string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	keys := make([]string, 0, len(cm.items))
	for key := range cm.items {
		keys = append(keys, key)
	}
	return keys
}
