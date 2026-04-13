---
id: coding-standards
kind: instruction
version: 1
description: Go coding conventions, style rules, and quality standards
preservation: required
scope:
  paths:
    - "**/*.go"
  fileTypes:
    - ".go"
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Go Coding Standards

## File Size Limits

- **Go files**: Maximum ~500 lines. Use `wc -l` to verify. Split files rather than reducing documentation.
- **Documentation files**: Maximum ~600 lines.

## Naming & Style

- Follow standard Go idioms and Effective Go guidelines.
- Use descriptive variable names — avoid single-letter names except loop indices.
- Package names: short, lowercase, no underscores.
- No package name stuttering (e.g., `model.ModelInstruction` is wrong; `model.Instruction` is right).

## Error Handling

Always check errors. Wrap with context:

```go
return fmt.Errorf("parse stage: %w", err)
```

For compiler/pipeline operations, use structured domain errors:

```go
return pipeline.NewCompilerError(pipeline.ErrLowering, "capability not supported", target)
```

## Functional Options Pattern

Use `WithXxx(value)` for configuration:

```go
func NewPipeline(opts ...Option) *Pipeline { ... }
func WithTargets(targets ...build.Target) Option { ... }
```

## Interface Verification

Verify interface satisfaction at compile time:

```go
var (
    _ port.Renderer = (*ClaudeRenderer)(nil)
    _ port.Stage    = (*ParseStage)(nil)
)
```

## Construction Patterns

- **Factory registration**: Pipeline stages and renderers register as plugins via factory functions.
- **Small interfaces**: Use multiple interfaces if objects only need partial implementation.
- **Dependency injection**: Wire in `adapter/cli/wire.go` — constructor injection, no global state.

## Binary Output

Manually compiled Go binaries **must** have `.out` postfix so `.gitignore` ignores them.

## Comments

Only comment code that needs clarification. Do not add obvious comments.
