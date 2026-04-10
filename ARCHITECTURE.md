# Architecture

GoAgentMeta is a compiler-based control plane for AI-agent metadata. It compiles a canonical source tree (`.ai/`) into native artifacts for multiple target ecosystems — Claude Code, Cursor, GitHub Copilot, and Codex — treating platform differences as compiler concerns rather than runtime abstractions.

For detailed specifications, see [`docs/architecture/`](docs/architecture/).

---

## Design Principles

- **Compiler, not runtime** — the `.ai/` tree is a source language; generated artifacts are compiler output
- **Plugin-first** — every pipeline stage and target renderer is a compiler plugin; third parties can extend or replace any stage
- **Hexagonal architecture** — domain types at center, port interfaces define contracts, adapters implement infrastructure
- **Loss-aware** — lowering is explicit, governed by preservation levels, and reported through provenance

---

## Hexagonal Layers

```
┌──────────────────────────────────────────────────────────────┐
│                        Adapters                              │
│  ┌──────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐   │
│  │  Parser  │ │  Registry  │ │  Renderer  │ │    CLI     │   │
│  │ (YAML/MD)│ │  (HTTP/Git)│ │(per target)│ │  (Cobra)   │   │
│  └─────┬────┘ └─────┬──────┘ └─────┬──────┘ └─────┬──────┘   │
│        │              │              │              │        │
│  ┌─────┴──────────────┴──────────────┴──────────────┴─────┐  │
│  │                    Port Interfaces                     │  │
│  │  stage · renderer · registry · filesystem · reporter   │  │
│  └─────┬──────────────┬──────────────┬──────────────┬─────┘  │
│        │              │              │              │        │
│  ┌─────┴──────────────┴──────────────┴──────────────┴─────┐  │
│  │                 Application Services                   │  │
│  │  Pipeline orchestrator · Dependency resolver · Packager│  │
│  └─────┬──────────────────────────────────────────────────┘  │
│        │                                                     │
│  ┌─────┴──────────────────────────────────────────────────┐  │
│  │                    Domain Model                        │  │
│  │  model · capability · plugin · build · pipeline        │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

Dependencies point **inward**. Domain knows nothing about renderers, registries, or CLI.

---

## Pipeline Phases

The compiler executes a fixed sequence of phases. Each phase contains one or more stage plugins.

| Phase | Purpose | Input IR | Output IR |
|---|---|---|---|
| **parse** | Read `.ai/` files into raw objects | filesystem paths | `SourceTree` |
| **validate** | Validate against schemas | `SourceTree` | `SourceTree` (validated) |
| **resolve** | Resolve external dependencies | `SourceTree` | `SourceTree` (with externals) |
| **normalize** | Build semantic graph with resolved inheritance | `SourceTree` | `SemanticGraph` |
| **plan** | Expand into target×profile build units | `SemanticGraph` | `BuildPlan` |
| **capability** | Resolve capabilities and select providers | `BuildPlan` | `CapabilityGraph` |
| **lower** | Lower unsupported concepts per target | `CapabilityGraph` | `LoweredGraph` |
| **render** | Emit target-native files | `LoweredGraph` | `EmissionPlan` |
| **materialize** | Write files to disk | `EmissionPlan` | `MaterializationResult` |
| **report** | Generate provenance and build report | `MaterializationResult` | `BuildReport` |

---

## Port Interfaces

Ports are small Go interfaces in `internal/port/` that define contracts between the domain and infrastructure layers.

| Port | Package | Purpose |
|---|---|---|
| **Stage** | `port/stage` | Core compiler plugin SPI — every pipeline stage implements `Stage` |
| **Renderer** | `port/renderer` | Specialized stage for the render phase, one per target |
| **Registry** | `port/registry` | Package discovery, resolution, fetching, integrity verification |
| **Filesystem** | `port/filesystem` | Read/write filesystem access, materialization |
| **Reporter** | `port/reporter` | Diagnostics, provenance recording, build report serialization |

---

## Domain Model

### Canonical Objects (`internal/domain/model/`)

Authoring primitives — the source language concepts authors write:

| Type | Purpose |
|---|---|
| **Instruction** | Always-on guidance (architecture, standards, policies) |
| **Rule** | Scoped or conditional policy |
| **Skill** | Reusable workflow bundle (AgentSkills.io standard) |
| **Agent** | Specialized delegate with role, tools, delegation, handoffs |
| **Hook** | Lifecycle automation with effect class and enforcement mode |
| **Command** | Explicit user-invoked entry point |

Runtime delivery primitives — exist to package and satisfy runtime needs:

| Type | Purpose |
|---|---|
| **Capability** | Abstract contract (e.g., `filesystem.read`, `mcp.github`) |
| **Plugin** | Deployable extension (inline, external, or registry-distributed) |
| **Reference** | Supplemental knowledge document, demand-loaded |
| **Asset** | Static file (template, diagram, prompt partial) |
| **Script** | Executable artifact for hooks, commands, or plugins |

### Build Coordinates (`internal/domain/build/`)

| Concept | Description |
|---|---|
| **Target** | Vendor ecosystem: `claude`, `cursor`, `copilot`, `codex` |
| **Profile** | Runtime policy: `local-dev`, `ci`, `enterprise-locked`, `oss-public` |
| **BuildUnit** | `(target, profile, scopes)` — fundamental compilation unit |

### Pipeline IR (`internal/domain/pipeline/`)

Intermediate representations that flow between phases:

`SourceTree` → `SemanticGraph` → `BuildPlan` → `CapabilityGraph` → `LoweredGraph` → `EmissionPlan` → `MaterializationResult` → `BuildReport`

---

## Plugin Architecture

Every stage is a plugin implementing the `Stage` interface:

```go
type Stage interface {
    Descriptor() StageDescriptor
    Execute(ctx context.Context, input any) (any, error)
}
```

Renderers extend `Stage` with target-specific methods:

```go
type Renderer interface {
    Stage
    Target() build.Target
    SupportedCapabilities() capability.CapabilityRegistry
}
```

Stages register via:
- **Built-in** — compiled into the binary
- **Go plugin** — loaded as shared objects
- **External process** — subprocess over stdin/stdout protocol

---

## Package Layout

```
cmd/                              CLI entry points
internal/
  domain/
    model/                        Canonical objects (instruction, rule, skill, agent, etc.)
    capability/                   Capability contracts and resolution logic
    plugin/                       Plugin domain (distribution, selection, security)
    build/                        Build coordinates (target, profile, scope, build unit)
    pipeline/                     Pipeline phases, stage descriptors, IR types, errors
  application/
    compiler/                     Pipeline orchestrator, stage registry, options
  port/
    stage/                        Stage interface (core compiler plugin SPI)
    renderer/                     Renderer interface (target-specific stage)
    registry/                     Package resolver, fetcher, searcher, verifier
    filesystem/                   Reader, writer, materializer
    reporter/                     Reporter, diagnostic sink, provenance recorder
  adapter/                        Infrastructure implementations (future)
pkg/
  sdk/                            Public SDK for third-party plugin authors
```

---

## Preservation Semantics

Every canonical object carries a preservation level controlling lowering behavior:

| Level | Behavior |
|---|---|
| `required` | Unsupported lowering → build failure |
| `preferred` | Lower when safe, warn and skip otherwise |
| `optional` | May skip with reporting |

---

## Build Output

The compiler emits two layers per build unit:

- **Instruction layer** — model-facing text files (`CLAUDE.md`, `AGENTS.md`, `.cursor/rules/*.mdc`)
- **Extension layer** — runtime configuration (MCP config, settings files)

Output is written to `.ai-build/{target}/{profile}/` with full provenance.

---

## Detailed Documentation

| Document | Content |
|---|---|
| [overview.md](docs/architecture/overview.md) | System design, domain model, bounded contexts |
| [compiler.md](docs/architecture/compiler.md) | Pipeline stages, plugin architecture, capability registry |
| [build-output.md](docs/architecture/build-output.md) | Output structure, validation, testing strategy |
| [target-grounding.md](docs/architecture/target-grounding.md) | Target capability mapping (Claude, Cursor, Copilot, Codex) |
| [schemas.md](docs/architecture/schemas.md) | YAML schema sketches for canonical objects |
| [marketplace.md](docs/architecture/marketplace.md) | Package distribution, registry, trust policies |
| [editor-tooling.md](docs/architecture/editor-tooling.md) | VS Code extension, LSP, preview |
