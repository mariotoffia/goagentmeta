---
id: adapter-isolation
kind: rule
version: 1
description: Adapters must implement port interfaces and stay in the outermost ring
preservation: required
scope:
  paths:
    - "internal/adapter/**"
conditions:
  - type: path-pattern
    value: "internal/adapter/**/*.go"
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Adapter Isolation Rule

When editing files in `internal/adapter/`:

## Must

- Implement one or more port interfaces from `internal/port/`
- Verify interface satisfaction at compile time:
  ```go
  var _ port.Renderer = (*ClaudeRenderer)(nil)
  ```
- Keep infrastructure concerns (I/O, HTTP, filesystem, YAML parsing) here
- Use constructor injection for dependencies (receive port interfaces, not concrete types)

## Must Not

- Import other adapter packages (adapters don't depend on each other)
- Expose infrastructure types in public API — return domain types through port interfaces
- Contain business logic — delegate to application services or domain types

## Adapter Categories

| Package | Implements | Purpose |
|---------|-----------|---------|
| `stage/*` | `port/stage.Stage` | Pipeline stage plugins |
| `renderer/*` | `port/renderer.Renderer` | Target-specific renderers |
| `filesystem/` | `port/filesystem.Reader`, `Writer`, `Materializer` | OS and in-memory filesystem |
| `registry/` | `port/registry.PackageResolver`, `Fetcher`, etc. | Registry clients |
| `reporter/` | `port/reporter.Reporter`, `DiagnosticSink`, etc. | Diagnostics and provenance |
| `cli/` | (driving adapter) | Cobra commands, wiring |
| `tool/` | `domain/tool.Plugin` | Built-in tool plugins |
| `plugin/` | (external plugin protocol) | Subprocess plugin loading |
