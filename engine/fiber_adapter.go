package engine

import (
	"io"

	"github.com/gofiber/fiber/v2"
)

// FiberViewsAdapter implements fiber.Views by delegating to BladeEngine
type FiberViewsAdapter struct {
	Engine *BladeEngine
}

// Render implements fiber.Views: Render(w io.Writer, name string, data interface{}, layout ...string) error
// Fiber passes optional layout/template names as variadic args; we ignore them here and delegate.
func (v *FiberViewsAdapter) Render(w io.Writer, name string, data interface{}, layout ...string) error {
	return v.Engine.Render(w, name, data)
}

// Load is an optional method in fiber.Views to warm templates; implement as no-op
func (v *FiberViewsAdapter) Load() error {
	// No-op: the engine manages its own preload if desired
	return nil
}

// WithFiberContext injects a SafeFiberCtx under the `_fiber` key and returns a
// data value suitable for passing to template execution. It accepts any data
// shape and returns a map[string]interface{} that includes original fields plus
// the `_fiber` entry.
func WithFiberContext(c *fiber.Ctx, data interface{}) map[string]interface{} {
	// Always return a map[string]interface{} with _fiber as a concrete *SafeFiberCtx
	if data == nil {
		return map[string]interface{}{"_fiber": NewSafeFiberCtx(c)}
	}
	if m, ok := data.(map[string]interface{}); ok {
		nm := make(map[string]interface{}, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm["_fiber"] = NewSafeFiberCtx(c)
		return nm
	}
	return map[string]interface{}{"_fiber": NewSafeFiberCtx(c), "_data": data}
}

// RenderWithCtx is a convenience that injects a SafeFiberCtx and writes the
// rendered template directly to the Fiber response writer. Use this in Fiber
// handlers to avoid manually mutating template data.
func (v *FiberViewsAdapter) RenderWithCtx(c *fiber.Ctx, name string, data interface{}, layout ...string) error {
	// Inject SafeFiberCtx
	enriched := WithFiberContext(c, data)
	// Stream to the response body writer for lower memory usage
	w := c.Context().Response.BodyWriter()
	c.Set("Content-Type", "text/html; charset=utf-8")
	// or c.Type("html")
	return v.Engine.Render(w, name, enriched)
}
