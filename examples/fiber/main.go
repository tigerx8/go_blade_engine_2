package main

import (
	"blade_engine/engine"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

type User struct {
	Name    string
	IsAdmin bool
}

func main() {
	cfg := engine.BladeConfig{
		TemplatesDir:      "./templates",
		TemplateExtension: ".blade.tpl",
		CacheEnabled:      false,
		Development:       true,
	}

	be := engine.NewBladeEngineWithConfig(cfg)

	app := fiber.New(fiber.Config{
		Views: &engine.FiberViewsAdapter{Engine: be},
	})

	app.Get("/", func(c *fiber.Ctx) error {
		start := time.Now()
		data := map[string]interface{}{
			"user": User{Name: "Alice", IsAdmin: true},
		}
		// Render via fiber's Render which delegates to our adapter
		if err := c.Render("pages/home.blade.tpl", engine.WithFiberContext(c, data)); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		log.Printf("Rendered in %v", time.Since(start))
		return nil
	})

	fmt.Println("Example server running on http://localhost:4004")
	log.Fatal(app.Listen(":4004"))
}
