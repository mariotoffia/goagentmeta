---
id: domain-modeling
kind: skill
version: 1
description: How to model domain types following GoAgentMeta patterns
preservation: preferred
requires:
  - filesystem.read
  - filesystem.write
  - terminal.exec
activation:
  hints:
    - domain type
    - domain model
    - value object
    - entity
    - IR type
    - canonical object
tools:
  - Read
  - Write
  - Edit
  - Bash(go:*)
  - Grep
  - Glob
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Domain Modeling Guide

Follow these patterns when adding or modifying types in `internal/domain/`.

## Canonical Objects (`model/`)

All authoring primitives embed `ObjectMeta`:

```go
type ObjectMeta struct {
    ID              string
    Kind            Kind
    Version         int
    Description     string
    Preservation    Preservation
    Scope           Scope
    AppliesTo       AppliesTo
    Extends         []string
    TargetOverrides map[string]TargetOverride
    Labels          []string
}
```

When adding a new canonical object kind:

1. Add the `Kind` constant in `model/object.go`
2. Create the struct embedding `ObjectMeta`
3. Add it to the `SourceTree.Objects` (or appropriate IR collection)
4. Update the parser to recognize the new kind
5. Update the validator with structural checks
6. Update each renderer to handle (or skip) the new kind

## Value Objects

Value objects are immutable and compared by value:

```go
type Scope struct {
    Paths     []string
    FileTypes []string
    Labels    []string
}
```

- No pointer receivers on value objects
- No setters — create new instances instead
- Use zero values as meaningful defaults

## Pipeline IR Types (`pipeline/`)

Each phase has typed IR. When adding new IR:

1. Define the type in `pipeline/ir_*.go`
2. Ensure it carries provenance (where it came from)
3. Document the phase that produces and consumes it
4. Add it to the pipeline phase chain

## Build Coordinates (`build/`)

`Target`, `Profile`, `BuildUnit` are value objects:

```go
type Target string
const (
    TargetClaude  Target = "claude"
    TargetCursor  Target = "cursor"
    TargetCopilot Target = "copilot"
    TargetCodex   Target = "codex"
)
```

## Testing Domain Types

- Test invariant enforcement in constructors
- Test equality/comparison for value objects
- Test serialization round-trips if applicable
- Keep tests in `<package>_test` package
