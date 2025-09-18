package engine

import "github.com/gofiber/fiber/v2"

// FiberAccessor defines the minimal methods templates are allowed to call.
type FiberAccessor interface {
    Header(string) string
    Param(string) string
    Local(string) interface{}
    Query(string) string
}

// SafeFiberCtx wraps *fiber.Ctx and exposes only safe accessors for templates.
type SafeFiberCtx struct{
    C *fiber.Ctx
}

// NewSafeFiberCtx creates a SafeFiberCtx
func NewSafeFiberCtx(c *fiber.Ctx) *SafeFiberCtx {
    return &SafeFiberCtx{C: c}
}

func (s *SafeFiberCtx) Header(k string) string {
    if s == nil || s.C == nil { return "" }
    return s.C.Get(k)
}

func (s *SafeFiberCtx) Param(name string) string {
    if s == nil || s.C == nil { return "" }
    return s.C.Params(name)
}

func (s *SafeFiberCtx) Local(key string) interface{} {
    if s == nil || s.C == nil { return nil }
    return s.C.Locals(key)
}

func (s *SafeFiberCtx) Query(key string) string {
    if s == nil || s.C == nil { return "" }
    return s.C.Query(key)
}
