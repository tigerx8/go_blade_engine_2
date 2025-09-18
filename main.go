package main

import (
	"blade_engine/engine"
	"embed"
	"fmt"
	fsys "io/fs"
	"log"
	"net/http"
	"time"
)

// Embed templates at build time
//
//go:embed templates/**
var embeddedTemplates embed.FS

type User struct {
	Name    string
	Email   string
	IsAdmin bool
}

type Product struct {
	Name        string
	Price       string
	HTMLContent string
}
type Item struct {
	HTMLContent string
}

func main() {
	// Root the FS at templates/ so paths like "pages/home.html" work
	var subFS fsys.FS
	if f, err := fsys.Sub(embeddedTemplates, "templates"); err == nil {
		subFS = f
	}

	// Cấu hình với cache
	config := engine.BladeConfig{
		TemplatesDir:      "./templates",
		CacheEnabled:      true,
		Development:       true,    // Set true để development mode (auto reload)
		CacheMaxSizeMB:    50,      // 50MB cache
		CacheTTLMinutes:   30,      // 30 minutes
		TemplateExtension: ".tpl",  // dùng ".tpl" nếu là Blade
		Mode:              "blade", // "blade" hoặc "go". Set to "go" to use EmbeddedFS with native Go templates.
		EmbeddedFS:        subFS,   // Provided for go mode; ignored in blade mode
	}

	blade := engine.NewBladeEngineWithConfig(config)

	// Validate all templates trước khi start
	if err := blade.ValidateAllTemplates(); err != nil {
		log.Printf("Template validation errors: %v", err)
		// Có thể continue hoặc exit tùy requirement
	}

	// Preload với error logging
	if err := blade.PreloadTemplates(); err != nil {
		log.Printf("Preload warnings: %v", err)
	}

	// // Debug template cụ thể nếu cần
	// blade.DebugTemplate("pages/home.gohtml")

	// Trong development mode, start file watcher
	if config.Development {
		watcher, err := engine.NewFileWatcher(blade, "./templates")
		if err != nil {
			log.Printf("Could not start file watcher: %v", err)
		} else {
			watcher.Start()
			defer watcher.Stop()
		}
	}

	http.HandleFunc("/blade", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		data := map[string]interface{}{
			"user": User{Name: "John Doe", IsAdmin: true},
			"items": []Product{
				{Name: "Item 1", Price: "$10", HTMLContent: "<strong>Item 1</strong>"},
				{Name: "Item 2", Price: "$20", HTMLContent: "<strong>Item 2</strong>"},
				{Name: "Item 3", Price: "$30", HTMLContent: "<strong>Item 3</strong>"},
			},
		}

		err := blade.Render(w, "pages/home.blade.tpl", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Log performance
		duration := time.Since(start)
		log.Printf("Rendered template in %v", duration)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		data := map[string]interface{}{
			"user": User{Name: "John Doe", IsAdmin: true},
			"items": []Item{
				{HTMLContent: "<strong>Item 1</strong>"},
				{HTMLContent: "<em>Item 2</em>"},
			},
		}

		err := blade.Render(w, "pages/home.gohtml", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Log performance
		duration := time.Since(start)
		log.Printf("Rendered template in %v", duration)
	})

	// Endpoint để xem cache stats
	http.HandleFunc("/cache-stats", func(w http.ResponseWriter, r *http.Request) {
		stats := blade.CacheStats()
		fmt.Fprintf(w, "Cache Statistics:\n")
		for k, v := range stats {
			fmt.Fprintf(w, "%s: %v\n", k, v)
		}
	})

	// Endpoint để clear cache
	http.HandleFunc("/clear-cache", func(w http.ResponseWriter, r *http.Request) {
		blade.ClearCache()
		fmt.Fprintf(w, "Cache cleared successfully")
	})

	fmt.Println("Server running on http://localhost:5004")
	log.Fatal(http.ListenAndServe(":5004", nil))
}
