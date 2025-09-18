package main

import (
	"blade_engine/engine"
	"embed"
	"fmt"
	fsys "io/fs"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ... adapter moved to engine/fiber_adapter.go

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
		Development:       false,   // Set true để development mode (auto reload)
		CacheMaxSizeMB:    50,      // 50MB cache
		CacheTTLMinutes:   30,      // 30 minutes
		TemplateExtension: ".tpl",  // dùng ".tpl" nếu là Blade
		Mode:              "blade", // "blade" hoặc "go". Set to "go" to use EmbeddedFS with native Go templates.
		EmbeddedFS:        subFS,   // Provided for go mode; ignored in blade mode
	}

	blade := engine.NewBladeEngineWithConfig(config)

	// Thiết lập Fiber với adapter
	app := fiber.New(fiber.Config{
		Views: &engine.FiberViewsAdapter{Engine: blade},
	})

	app.Get("/", func(c *fiber.Ctx) error {
		start := time.Now()

		data := map[string]interface{}{
			"user": User{Name: "John Doe", IsAdmin: true},
			"items": []Product{
				{Name: "Item 1", Price: "$10", HTMLContent: "<strong>Item 1</strong>"},
				{Name: "Item 2", Price: "$20", HTMLContent: "<strong>Item 2</strong>"},
				{Name: "Item 3", Price: "$30", HTMLContent: "<strong>Item 3</strong>"},
			},
			"isSticky": true,
		}

		duration := time.Since(start)
		log.Printf("Rendered template in %v", duration)
		return c.Render("pages/home.blade.tpl", engine.WithFiberContext(c, data))
	})

	app.Get("/about", func(c *fiber.Ctx) error {
		data := map[string]interface{}{
			"title":       "About Us",
			"isSticky":    false,
			"headerClass": "bg-info",
		}
		return c.Render("pages/about.blade.tpl", engine.WithFiberContext(c, data))
	})

	app.Get("/contact", func(c *fiber.Ctx) error {
		data := map[string]interface{}{
			"title":       "Contact Us",
			"isSticky":    false,
			"headerClass": "bg-success",
		}
		return c.Render("pages/contact.blade.tpl", engine.WithFiberContext(c, data))
	})

	app.Get("/gohtml", func(c *fiber.Ctx) error {

		data := map[string]interface{}{
			"user": User{Name: "John Doe", IsAdmin: true},
			"items": []Item{
				{HTMLContent: "<strong>Item 1</strong>"},
				{HTMLContent: "<em>Item 2</em>"},
			},
		}

		return c.Render("pages/home.gohtml", engine.WithFiberContext(c, data))
	})

	// Endpoint để xem cache stats
	app.Get("/cache-stats", func(c *fiber.Ctx) error {
		stats := blade.CacheStats()
		var b strings.Builder
		b.WriteString("Cache Statistics:\n")
		for k, v := range stats {
			b.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
		return c.Type("text").SendString(b.String())
	})

	// Endpoint để clear cache
	app.Get("/clear-cache", func(c *fiber.Ctx) error {
		blade.ClearCache()
		return c.Type("text").SendString("Cache cleared successfully")
	})

	// Expose compiler stats endpoint
	app.Get("/compiler-stats", func(c *fiber.Ctx) error {
		out := blade.CompilerStatsHandler()(nil)
		return c.Type("text").SendString(out)
	})

	fmt.Println("Server running on http://localhost:5004")
	log.Fatal(app.Listen(":5004"))
}
