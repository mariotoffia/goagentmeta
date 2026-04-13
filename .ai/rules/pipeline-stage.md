---
id: pipeline-stage-compliance
kind: rule
version: 1
description: Pipeline stages must follow the Stage interface and descriptor patterns
preservation: required
scope:
  paths:
    - "internal/adapter/stage/**"
conditions:
  - type: path-pattern
    value: "internal/adapter/stage/**/*.go"
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Pipeline Stage Rule

When editing pipeline stage adapters in `internal/adapter/stage/`:

## Must

- Implement `port/stage.Stage` interface:
  ```go
  type Stage interface {
      Descriptor() pipeline.StageDescriptor
      Execute(ctx context.Context, input any) (any, error)
  }
  ```
- Return a `StageDescriptor` with correct `Phase`, unique `Name`, and optional `Before`/`After` ordering
- Accept the correct IR type as input (type-assert from `any`) and return the correct output IR
- Use `pipeline.NewCompilerError(...)` for domain-level failures
- Register in `adapter/cli/wire.go` via `compiler.WithStage(...)`

## IR Type Contract

| Phase | Input Type | Output Type |
|-------|-----------|-------------|
| parse | `string` (root path) | `*pipeline.SourceTree` |
| validate | `*pipeline.SourceTree` | `*pipeline.SourceTree` |
| resolve | `*pipeline.SourceTree` | `*pipeline.SourceTree` |
| normalize | `*pipeline.SourceTree` | `*pipeline.SemanticGraph` |
| plan | `*pipeline.SemanticGraph` | `*pipeline.BuildPlan` |
| capability | `*pipeline.BuildPlan` | `*pipeline.CapabilityGraph` |
| lower | `*pipeline.CapabilityGraph` | `*pipeline.LoweredGraph` |
| render | `*pipeline.LoweredGraph` | `*pipeline.EmissionPlan` |
| materialize | `*pipeline.EmissionPlan` | `*pipeline.MaterializationResult` |
| report | `*pipeline.MaterializationResult` | `*pipeline.BuildReport` |

## Must Not

- Skip type assertions without error handling
- Mutate the input IR — create new output IR instead
- Access filesystem or network directly — use injected port interfaces
