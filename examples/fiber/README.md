# Fiber Example for blade_engine

This folder contains a minimal example showing how to use `blade_engine` with the Fiber web framework.

Files:

- `main.go` - minimal Fiber server that registers `engine.FiberViewsAdapter` and renders a Blade template.
- `templates/pages/home.blade.tpl` - example Blade template demonstrating access to the injected `_fiber` safe helper.

Run:

```bash
cd examples/fiber
go run .
```

Then open `http://localhost:4004`.

Notes:

- The template uses `{{ call ._fiber.Header "Host" }}` and other safe accessors; `_fiber` is a `SafeFiberCtx` that exposes only a small, safe subset of the Fiber context to templates.
- This example runs with `Development: true` in the engine config for easier iteration.
