---
id: port-contracts
kind: rule
version: 1
description: Port interfaces must be small, contract-only, with no implementation
preservation: required
scope:
  paths:
    - "internal/port/**"
conditions:
  - type: path-pattern
    value: "internal/port/**/*.go"
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Port Contracts Rule

When editing files in `internal/port/`:

## Must

- Define **small Go interfaces** — one method or a small cohesive group
- Use domain types (`internal/domain/`) in method signatures
- Keep interfaces in their own subpackage (`stage/`, `renderer/`, `registry/`, `filesystem/`, `reporter/`)
- Document the contract purpose and expected behavior

## Must Not

- Include any implementation code
- Import adapter packages (`internal/adapter/`)
- Import external dependencies
- Define concrete types (structs with methods) — only interfaces and supporting value types for signatures

## Interface Design

```go
// Good — small, focused interface
type Reader interface {
    ReadFile(ctx context.Context, path string) ([]byte, error)
    ListDir(ctx context.Context, dir string) ([]string, error)
}

// Bad — too broad, mixes concerns
type FileSystem interface {
    Read(...) ...
    Write(...) ...
    Delete(...) ...
    Chmod(...) ...
    Watch(...) ...
}
```

Split into `Reader`, `Writer`, `Materializer` instead.
