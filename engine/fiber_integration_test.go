package engine

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Test that templates rendered via the Fiber adapter receive a SafeFiberCtx
// under the `_fiber` key and can call Header/Query/Param helpers.
func TestFiberAdapter_Injection(t *testing.T) {
	// Create a temporary templates directory
	tmp, err := os.MkdirTemp("", "blade_test_templates")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmp)

	// Create templates/pages dir
	pagesDir := filepath.Join(tmp, "pages")
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		t.Fatalf("mkdir pages: %v", err)
	}

	// Write a simple template that uses _fiber helpers
	tpl := `<!doctype html>
<html><body>
Host: {{ call ._fiber.Header "Host" }}
Query foo: {{ call ._fiber.Query "foo" }}
</body></html>`
	if err := os.WriteFile(filepath.Join(pagesDir, "home.blade.tpl"), []byte(tpl), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// Create engine
	cfg := BladeConfig{
		TemplatesDir:      tmp,
		TemplateExtension: ".blade.tpl",
		CacheEnabled:      false,
		Development:       true,
	}
	be := NewBladeEngineWithConfig(cfg)

	adapter := &FiberViewsAdapter{Engine: be}
	app := fiber.New(fiber.Config{Views: adapter})

	app.Get("/pages/home", func(c *fiber.Ctx) error {
		// Use adapter.RenderWithCtx to inject _fiber automatically
		return adapter.RenderWithCtx(c, "pages/home.blade.tpl", map[string]interface{}{"x": 1})
	})

	// Create request with query param using httptest and use Fiber's app.Test
	req := httptest.NewRequest("GET", "/pages/home?foo=bar", nil)
	req.Host = "example.local"
	resp, err := app.Test(req, 5*time.Second)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	if !strings.Contains(body, "example.local") {
		t.Fatalf("expected host in body; got: %s", body)
	}
	if !strings.Contains(body, "bar") {
		t.Fatalf("expected query value in body; got: %s", body)
	}
}
