---
id: target-renderer-dev
kind: skill
version: 1
description: How to create a new target renderer for a platform ecosystem
preservation: preferred
requires:
  - filesystem.read
  - filesystem.write
  - terminal.exec
resources:
  references:
    - references/target-grounding.md
activation:
  hints:
    - new renderer
    - target renderer
    - render target
    - new platform
    - emit files
tools:
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

# Creating a New Target Renderer

Follow this workflow to implement a renderer for a new target ecosystem.

## Step 1: Define the Target

Add the target constant in `internal/domain/build/coordinates.go` if it doesn't exist.

## Step 2: Create the Renderer Package

```
internal/adapter/renderer/<target>/
├── renderer.go            # Main renderer (implements Renderer interface)
├── instruction_layer.go   # Instruction emission
├── skills_layer.go        # Skills emission
├── agents_layer.go        # Agent emission
├── hooks_layer.go         # Hook emission
├── mcp_layer.go           # MCP configuration emission
├── plugin_layer.go        # Plugin emission
├── provenance.go          # Provenance metadata emission
├── renderer_test.go       # Unit tests
├── golden_test.go         # Golden file tests
└── golden/                # Expected output files
```

## Step 3: Implement the Renderer Interface

```go
package newtarget

import (
    "github.com/mariotoffia/goagentmeta/internal/domain/build"
    "github.com/mariotoffia/goagentmeta/internal/domain/capability"
    "github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
    portrenderer "github.com/mariotoffia/goagentmeta/internal/port/renderer"
)

var _ portrenderer.Renderer = (*Renderer)(nil)

type Renderer struct{}

func New() *Renderer { return &Renderer{} }

func (r *Renderer) Target() build.Target {
    return build.Target("newtarget")
}

func (r *Renderer) SupportedCapabilities() capability.CapabilityRegistry {
    // Return capabilities this target supports natively
}

func (r *Renderer) Descriptor() pipeline.StageDescriptor {
    return pipeline.NewDescriptorWithTarget("newtarget-renderer", pipeline.PhaseRender, "newtarget")
}

func (r *Renderer) Execute(ctx context.Context, input any) (any, error) {
    lowered, ok := input.(*pipeline.LoweredGraph)
    if !ok { return nil, pipeline.NewCompilerError(...) }

    // Emit files into EmissionPlan
    plan := &pipeline.EmissionPlan{}
    // ... emit instruction layer, skills layer, etc.
    return plan, nil
}
```

## Step 4: Create Target Capability Registry

Add `internal/adapter/stage/capability/target_registry/<target>.yaml` defining which capabilities are native, adapted, or emulated.

## Step 5: Register and Test

1. Register in `wire.go` via `compiler.WithStage(newtarget.New())`
2. Create golden tests comparing expected output
3. Run `make check`
