---
id: compiler-stage-dev
kind: skill
version: 1
description: Step-by-step guide to creating a new compiler pipeline stage plugin
preservation: preferred
requires:
  - filesystem.read
  - filesystem.write
  - terminal.exec
resources:
  references:
    - references/compiler-pipeline.md
activation:
  hints:
    - new stage
    - pipeline stage
    - compiler plugin
    - stage implementation
allowedTools:
  - Read
  - Write
  - Edit
  - Bash(go:*)
  - Bash(make:*)
  - Glob
  - Grep
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Creating a New Pipeline Stage

Follow this workflow to implement a new compiler pipeline stage plugin.

## Step 1: Identify the Phase

Determine which pipeline phase your stage belongs to. Stages execute within exactly one phase:

`parse` → `validate` → `resolve` → `normalize` → `plan` → `capability` → `lower` → `render` → `materialize` → `report`

## Step 2: Create the Package

Create a new package under `internal/adapter/stage/<your-stage>/`:

```
internal/adapter/stage/<your-stage>/
├── <your_stage>.go       # Stage implementation
├── <your_stage>_test.go  # Unit tests
└── fixtures/             # Test fixtures (if needed)
```

## Step 3: Implement the Stage Interface

```go
package yourstage

import (
    "context"
    "github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
    portstage "github.com/mariotoffia/goagentmeta/internal/port/stage"
)

var _ portstage.Stage = (*YourStage)(nil)

type YourStage struct {
    // injected dependencies (port interfaces only)
}

func New(/* port interfaces */) *YourStage {
    return &YourStage{...}
}

func (s *YourStage) Descriptor() pipeline.StageDescriptor {
    return pipeline.StageDescriptor{
        Name:  "your-stage",
        Phase: pipeline.PhaseValidate, // your target phase
        Order: 10,                      // execution order within phase
    }
}

func (s *YourStage) Execute(ctx context.Context, input any) (any, error) {
    // Type-assert the input IR for your phase
    sourceTree, ok := input.(*pipeline.SourceTree)
    if !ok {
        return nil, pipeline.NewCompilerError(
            pipeline.ErrInternal, "unexpected input type", nil,
        )
    }

    // Process and return the output IR
    return sourceTree, nil
}
```

## Step 4: Register the Stage

Add registration in `internal/adapter/cli/wire.go`:

```go
compiler.WithStage(yourstage.New(/* dependencies */))
```

## Step 5: Write Tests

Create table-driven tests covering:
- Valid input processing
- Invalid/missing input handling
- Edge cases specific to your stage
- Benchmark tests for hot paths

## Step 6: Verify

```bash
make test && make lint && make vet
```
