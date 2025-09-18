package engine

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkBladeEngine benchmark hiệu năng
func BenchmarkBladeEngine(b *testing.B) {
	blade := NewBladeEngine("./templates")

	// Warm up cache
	blade.PreloadTemplates()

	data := map[string]interface{}{
		"user":  map[string]interface{}{"Name": "Test", "IsAdmin": true},
		"items": []map[string]interface{}{{"HTMLContent": "test"}},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := blade.RenderString("pages/home.blade.tpl", data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestCachePerformance test hiệu năng cache
func TestCachePerformance() {
	blade := NewBladeEngine("./templates")

	// Test không cache
	blade.EnableCache(false)
	start := time.Now()
	for i := 0; i < 1000; i++ {
		blade.RenderString("pages/home.blade.tpl", nil)
	}
	withoutCache := time.Since(start)

	// Test với cache
	blade.EnableCache(true)
	blade.ClearCache()
	// Preload first time
	blade.RenderString("pages/home.blade.tpl", nil)

	start = time.Now()
	for i := 0; i < 1000; i++ {
		blade.RenderString("pages/home.blade.tpl", nil)
	}
	withCache := time.Since(start)

	fmt.Printf("Without cache: %v\n", withoutCache)
	fmt.Printf("With cache: %v\n", withCache)
	fmt.Printf("Performance improvement: %.1fx faster\n",
		float64(withoutCache.Microseconds())/float64(withCache.Microseconds()))
}
