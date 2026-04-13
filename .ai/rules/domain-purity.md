---
id: domain-purity
kind: rule
version: 1
description: Domain layer must remain pure — no infrastructure imports
preservation: required
scope:
  paths:
    - "internal/domain/**"
conditions:
  - type: path-pattern
    value: "internal/domain/**/*.go"
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Domain Purity Rule

When editing files in `internal/domain/`:

## Must

- Define **pure value types and entities** only
- Use only Go standard library types (`fmt`, `strings`, `errors`, `context`, `sort`, `sync`)
- Keep types immutable where possible (value objects)
- Use constructor functions that validate invariants

## Must Not

- Import any package from `internal/adapter/`
- Import any package from `internal/port/` (domain does not know about ports)
- Import any package from `internal/application/`
- Import external dependencies (no `yaml.v3`, no `cobra`, no HTTP clients)
- Depend on filesystem, network, or any I/O
- Use global mutable state

## Rationale

The domain layer is the innermost ring of the hexagonal architecture. It defines the language of the compiler — canonical objects, IR types, build coordinates, capability contracts. Everything else depends on it; it depends on nothing.
