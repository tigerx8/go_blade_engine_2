# Blade Engine for Go

Blade Engine is a Laravel-inspired template engine for Go, supporting friendly syntax, directives, sections, layouts, and easy integration with Fiber.

## 1. Installation

    go get github.com/gofiber/fiber/v2
    go get github.com/your-module/blade_engine/engine
    mkdir -p templates/layouts templates/pages

    or

    mkdir -p views/layouts views/pages

## 2. Initialize Blade Engine

You can initialize Blade Engine with helper functions (escape, raw, ...) and cache configuration:

    import (
        "blade_engine/engine"
        "html/template"
    )

    funcs := template.FuncMap{
        "escape": func(s string) string { return template.HTMLEscapeString(s) },
        "raw": func(s string) template.HTML { return template.HTML(s) },
    }

    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        TemplatesDir: "./templates",
        CacheEnabled: true,
        DiskCacheOnly: true,
        AutoModeExtensions: []string{".gohtml"},
        FuncMap: funcs,
    })

## 3. Integrate with Fiber

Blade Engine supports Fiber through the FiberViewsAdapter. You just need to pass it to Fiber when initializing the app:

    import (
        "blade_engine/engine"
        "blade_engine/engine/fiber_adapter"
        "github.com/gofiber/fiber/v2"
    )

    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        TemplatesDir: "./templates",
        CacheEnabled: true,
        DiskCacheOnly: true,
    })

    adapter := &engine.FiberViewsAdapter{Engine: blade}

    app := fiber.New(fiber.Config{
        Views: adapter,
    })

## 4. Render template trong Fiber handler

    app.Get("/", func(c *fiber.Ctx) error {
        data := map[string]interface{}{
            "user": map[string]interface{}{
                "Name": "Alice",
                "IsAdmin": true,
            },
        }
        return adapter.RenderWithCtx(c, "pages/home.blade.tpl", data)
    })

## 5. Using SafeFiberCtx in Templates

Blade Engine provides `SafeFiberCtx`, a struct that allows you to safely access certain information from the Fiber context within your templates. You can inject it into your data using the key "\_fiber":

    <p>Request Host: {{ ._fiber.Header "Host" }}</p>
    <p>Query foo: {{ ._fiber.Query "foo" }}</p>
    <p>Param id: {{ ._fiber.Param "id" }}</p>
    <p>Local user: {{ ._fiber.Local "user" }}</p>

The available methods on `SafeFiberCtx` are:

    Header(key string)
    Query(key string)
    Param(key string)
    Local(key string)

## 6. Project Structure

.
├── main.go
├── templates/
│ ├── layouts/
│ │ └── app.blade.tpl
│ └── pages/
│ └── home.blade.tpl

## 7. Example Templates

    @extends('layouts/app.blade.tpl')
    @section('title')
    Welcome to Our Website
    @endsection
    @section('content')

    <h1>Hello, {{ $user.Name }}!</h1>
    @if ($user.IsAdmin)
        <p>You have administrator privileges</p>
    @else
        <p>Welcome to our website!</p>
    @endif
    <p>Request Host: {{ ._fiber.Header "Host" }}</p>
    @endsection

## 8. Expose compiler stats endpoint

Add the following route to your Fiber app to expose Blade compiler stats:

    app.Get("/compiler-stats", func(c *fiber.Ctx) error {
        return c.SendString(blade.CompilerStatsHandler()(nil))
    })

## 9. Run the Example

    cd examples/fiber
    go run .

    Access http://localhost:4004

Notes:

To use helpers like escape, raw, you need to inject them into FuncMap when initializing the engine.
To access request information in templates, always use RenderWithCtx or inject SafeFiberCtx into data with the key "\_fiber".

## 10. Switching Between Blade and Go Mode, Changing Template/Cache Directory

Switch Mode (Blade or Go)
You can control which engine is used for rendering by file extension or by setting the Mode in BladeConfig:

    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        TemplatesDir: "./templates",         // Template directory
        CacheEnabled: true,
        DiskCacheOnly: true,
        Mode: "blade",                       // Use Blade syntax for all templates
        AutoModeExtensions: []string{".gohtml"}, // Use Go template engine for .gohtml files
    })
    # or
    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        TemplatesDir: "./templates",         // Template directory
        CacheEnabled: true,
        DiskCacheOnly: true,
        Mode: "go",                          // Use Go template engine for all templates
        AutoModeExtensions: []string{".blade.tpl"}, // Use Blade syntax for .blade.tpl files
    })

- Mode: "blade" — All templates use Blade syntax unless extension matches AutoModeExtensions.
- Mode: "go" — All templates use Go's html/template syntax.

Change Template Directory
Just set TemplatesDir in your config:

    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        TemplatesDir: "./views", // Use ./views instead of ./templates
        ...
    })

Change Template Directory
Just set TemplatesDir in your config:

    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        TemplatesDir: "./views", // Use ./views instead of ./templates
        ...
    })

Change Cache Directory
You can set CacheDir in your config to change where compiled templates are cached:

    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        CacheDir: "./my_cache", // Use ./my_cache instead of default ./cache
        ...
    })

Or set it programmatically (if supported in your version):

    os.Setenv("BLADE_CACHE_DIR", "/path/to/cache")
    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{ ... })

Example: Full Config

    blade := engine.NewBladeEngineWithConfig(engine.BladeConfig{
        TemplatesDir: "./views",
        CacheEnabled: true,
        DiskCacheOnly: true,
        Mode: "go", // Use Go template engine for all templates
        AutoModeExtensions: []string{".blade.tpl"}, // Use Blade engine for .blade.tpl files
        FuncMap: funcs,
    })

Tip:

Use Mode and AutoModeExtensions together for flexible engine switching.
Change TemplatesDir and BLADE_CACHE_DIR to organize your project as you like.

## 11. Fiber Example for blade_engine

This folder contains a minimal example showing how to use `blade_engine` with the Fiber web framework.

Files:

- `main.go` - minimal Fiber server that registers `engine.FiberViewsAdapter` and renders a Blade template.
- `templates/pages/home.blade.tpl` - example Blade template demonstrating access to the injected `_fiber` safe helper.
  Run:

```bash
cd examples/fiber
go run .

Access http://localhost:4004
```
