# GoAgentMeta - Agent Guidelines

A compiler-based control plane for AI-agent metadata. Author once in a canonical source tree, compile to native artifacts for multiple target ecosystems (Claude Code, Cursor, GitHub Copilot, Codex).

**Purpose**: Unified compiler that treats platform differences as compiler concerns, not runtime abstractions. The canonical model describes intent, dependencies, preservation requirements, and delivery policy. Renderers emit native artifacts for each target.

### Key Characteristics

- **Compiler architecture**: Multi-stage pipeline — parse → validate → resolve → normalize → plan → capability → lower → render → materialize → report
- **Go 1.25+**: Implementation language with zero-dependency core and adapter modules
- **Plugin-first**: Every pipeline stage and target renderer is a compiler plugin
- **Hexagonal architecture**: Domain types at center, ports define contracts, adapters implement infrastructure

## Architecture & Design Principles

- **Clean Architecture** — Dependency rules, layer separation, business logic isolation
- **Hexagonal Architecture** — Ports & Adapters for target renderers, registries, filesystem I/O
- **Domain-Driven Design** — Bounded contexts per pipeline stage, aggregates, ubiquitous language
- **Compiler-as-Control-Plane** — Canonical IR stages with explicit lowering and provenance
- **Plugin-First** — Every pipeline stage and target renderer registers as a plugin; third parties can extend or replace any stage

## Technology Stack

| Category | Technology |
|----------|------------|
| Language | Go 1.25+ |
| Cloud SDK | AWS SDK for Go v2 |
| Architecture | Clean + Hexagonal + DDD |
| Pipeline | Multi-stage compiler with pluggable stages |
| Skills Standard | AgentSkills.io open standard |
| Targets | Claude Code, Cursor, GitHub Copilot, Codex |

## Project Structure

```
goagentmeta/
├── cmd/                           # CLI entry points
├── internal/
│   ├── domain/                    # Core domain model (innermost ring)
│   │   ├── model/                 #   Canonical objects: instruction, rule, skill, agent, etc.
│   │   ├── capability/            #   Capability contracts and resolution logic
│   │   ├── plugin/                #   Plugin domain (distribution, selection, security)
│   │   ├── build/                 #   Build coordinates (target, profile, scope, build unit)
│   │   ├── tool/                  #   Tool plugin domain (expression parsing, registry, validation)
│   │   └── pipeline/              #   Pipeline IR types, phase definitions, compiler errors
│   ├── application/               # Application services (use cases, orchestration)
│   │   ├── compiler/              #   Compiler pipeline orchestration and plugin registry
│   │   ├── dependency/            #   Dependency resolution use cases
│   │   └── (reserved for future packaging use cases)
│   ├── port/                      # Port interfaces (driven and driving)
│   │   ├── registry/              #   Registry access port
│   │   ├── renderer/              #   Target renderer port
│   │   ├── filesystem/            #   Filesystem I/O port
│   │   ├── reporter/              #   Reporting and provenance port
│   │   └── stage/                 #   Pipeline stage port (compiler plugin SPI)
│   └── adapter/                   # Infrastructure adapters (outermost ring)
│       ├── stage/                 #   Built-in pipeline stage implementations
│       │   ├── parser/            #     YAML/Markdown frontmatter parser
│       │   ├── validator/         #     Structural + semantic validation
│       │   ├── resolver/          #     Dependency resolution
│       │   ├── normalizer/        #     Inheritance + merge normalization
│       │   ├── planner/           #     Build plan generation
│       │   ├── capability/        #     Capability resolution
│       │   ├── lowering/          #     IR lowering (safe/unsafe)
│       │   ├── materializer/      #     File/symlink materializer
│       │   └── reporter/          #     Build report generation
│       ├── tool/                  #   Built-in tool plugin registry
│       ├── registry/              #   Registry client adapters (HTTP, git, filesystem)
│       ├── renderer/              #   Target renderer backends (claude, copilot, codex)
│       ├── filesystem/            #   OS filesystem I/O
│       ├── reporter/              #   Diagnostic sink, provenance, formatters
│       └── cli/                   #   CLI adapter (cobra/flags)
├── pkg/sdk/                       # Public compiler plugin SDK for third-party authors
├── .ai/                           # Canonical source tree (compiler input)
│   ├── manifest.yaml              #   Build defaults, targets, profiles, compiler policy
│   ├── instructions/              #   Always-on content by scope
│   ├── rules/                     #   Conditional or scoped policy
│   ├── skills/                    #   Reusable workflow bundles
│   ├── agents/                    #   Specialized orchestration definitions
│   ├── hooks/                     #   Lifecycle automation definitions
│   ├── commands/                  #   User-invoked workflows
│   ├── capabilities/              #   Abstract capability contracts
│   ├── plugins/                   #   Deployable extension packages
│   ├── references/                #   Supplemental knowledge documents
│   ├── assets/                    #   Static files (templates, examples, diagrams)
│   ├── scripts/                   #   Executable artifacts for hooks/commands
│   ├── profiles/                  #   Profile definitions (local-dev, ci, enterprise-locked)
│   ├── targets/                   #   Target-specific overrides
│   └── schema/                    #   Schema definitions
├── .agents/skills/                # Shared skill library (AgentSkills.io format)
├── docs/architecture/             # Architecture specification documents
└── tests/integration/             # End-to-end integration tests
```

## Build Commands

```bash
make install          # Install system dependencies (Go, golangci-lint)
make dep              # Install/tidy project dependencies
make build            # Build all packages
make test             # Run unit tests (race detection, -short, skips Docker tests)
make test-integration # Run integration tests
make lint             # Run golangci-lint
make vet              # Run go vet
make fmt              # Format all Go files (gofmt)
make generate         # Generate code from specs/schemas
make clean            # Clean build artifacts
make check            # build + lint + vet + unit tests
make check-all        # build + lint + vet + all tests
```

## Development Standards

- **Go files**: Maximum ~500 lines (use `wc -l` to verify). Split files instead of reducing code documentation
- **Documentation files**: Maximum ~600 lines
- **Testing**: Always create unit and integration tests
- **Benchmarks**: Always create benchmarks — both simple and complex simulations (reuse unit/integration tests)
- **Verification**: Run `make test` successfully before stating work is complete
- **Code quality**: Run `make lint` and `make vet`
- **Binary output**: Manually compiled Go binaries must have `.out` postfix so `.gitignore` ignores them
- **Compilation**: MUST use `.out` postfix for any manually compiled Go binary

## Coding Conventions

### Go Style

- Follow standard Go idioms and effective Go guidelines
- Use descriptive variable names; avoid single-letter names except loop indices
- Error handling: always check errors; wrap with context using `fmt.Errorf("context: %w", err)`
- Use structured errors for compiler and pipeline operations

### Package Organization

- **Domain types in `internal/domain/`**: Pure value types and entities — no infrastructure dependencies
- **Port interfaces in `internal/port/`**: Contracts for registry, renderer, filesystem, reporter, stage
- **Application services in `internal/application/`**: Use-case orchestration through ports
- **Adapters in `internal/adapter/`**: Infrastructure implementations (outermost ring)
- **Dependency rule**: Dependencies point inward. Domain knows nothing about renderers, registries, or CLI

### Configuration & Construction

- **Functional options**: Use `WithXxx(value)` pattern for configuration
- **Factory pattern**: Pipeline stages and renderers register as plugins via factory registration
- **Small interfaces**: Use multiple interfaces if objects only need partial implementation

### Interface Verification

Verify interface satisfaction at compile time:

```go
var (
    _ port.Renderer = (*ClaudeRenderer)(nil)
    _ port.Stage    = (*ParseStage)(nil)
)
```

### Error Handling

Use structured domain errors for compiler operations:

```go
// Wrap with context
return fmt.Errorf("parse stage: %w", err)

// Domain-level errors for pipeline failures
return pipeline.NewCompilerError(pipeline.ErrLowering, "capability not supported", target)
```

## Testing Standards

- Use **go-test-expert** skill when creating tests
- Use **go-benchmark-testing** and **golang-benchmark** skills for benchmarks
- Table-driven tests for pipeline stages and renderers
- Unit tests in `<package>_test` package for exported functionality
- Integration tests for end-to-end compiler pipeline flows
- Benchmark tests for compiler hot paths (parsing, normalization, rendering)
- Do not simplify tests just because they fail — check both code and test
- Always verify with `make test` before completion

## Important Architecture Documents

| Document | Purpose |
|----------|---------|
| `docs/architecture/overview.md` | System design, hexagonal layers, domain model, bounded contexts, package layout |
| `docs/architecture/compiler.md` | Pipeline stages, stage interfaces, compiler plugin contracts |
| `docs/architecture/build-output.md` | Output structure, hybrid emission layers, provenance, deterministic reports |
| `docs/architecture/target-grounding.md` | Target capability mapping (Claude, Cursor, Copilot, Codex) |
| `docs/architecture/schemas.md` | Schema definitions for canonical objects |
| `docs/architecture/marketplace.md` | Registry/marketplace design for plugin distribution |
| `docs/architecture/editor-tooling.md` | Editor integration and tooling support |

## Skills & Agents

### Available Skills

| Skill | Trigger |
|-------|---------|
| `clean-ddd-hexagonal` | DDD, Clean Architecture, Hexagonal, ports and adapters, entities, value objects, domain events |
| `go-architecture-review` | Package structure, dependency direction, module boundaries, project layout |
| `go-benchmark-testing` | Benchmark test creation, performance testing |
| `go-performance-review` | Allocations, string handling, sync.Pool, profiling, pprof |
| `go-senior-developer` | Architecture, API-first design, advanced concurrency, performance tuning |
| `go-test-expert` | Testing patterns, table-driven tests, httptest, benchmarking, fuzzing |
| `golang-architect` | Module design, dependency management, project structure |
| `golang-benchmark` | Benchmarking, profiling, benchstat, CPU/memory profiles |
| `golang-enterprise-patterns` | Clean architecture, hexagonal, DDD, production-ready structure |
| `lsp-setup` | Language Server Protocol setup and configuration |
| `vscode-extension-expert` | VS Code extension development, WebView, activation events |

### Available Agents

| Agent | Purpose |
|-------|---------|
| `Explore` | Fast read-only codebase exploration and Q&A. Safe to call in parallel. Specify thoroughness: quick, medium, or thorough |

## Domain Concepts (Ubiquitous Language)

### Authoring Primitives

| Concept | Definition |
|---------|------------|
| **Instruction** | Always-on guidance and context (architecture principles, coding standards, review policies) |
| **Rule** | Scoped or conditional policy (language-specific rules, security restrictions) |
| **Skill** | Reusable, model-facing workflow bundle (build a Lambda, review IAM, scaffold bounded context) |
| **Agent** | Specialized delegate — role/prompt + tool policy + permissions + linked skills |
| **Hook** | Deterministic lifecycle automation triggered by defined events |
| **Command** | Explicit user-invoked entry point (`/review-iam`, `/build-lambda`) |

### Runtime Delivery Primitives

| Concept | Definition |
|---------|------------|
| **Capability** | Abstract contract required by authoring primitives (`filesystem.read`, `terminal.exec`, `mcp.github`) |
| **Plugin** | Deployable extension package providing capabilities, tool manifests, MCP bindings |
| **Reference** | Supplemental knowledge document — demand-loaded by the AI when deeper knowledge is needed |
| **Asset** | Static file consumed by tooling or emitted into output (template, diagram, prompt partial) |
| **Script** | Executable artifact used by hooks, commands, skills, or plugins |

### Build Coordinates

| Concept | Definition |
|---------|------------|
| **Target** | Vendor/ecosystem being emitted for: `claude`, `cursor`, `copilot`, `codex` |
| **Profile** | Runtime environment or policy shape: `local-dev`, `ci`, `enterprise-locked`, `oss-public` |
| **Scope** | Where canonical objects apply: repo root, subtree path, glob, file type, labels |
| **Build Unit** | `(target, profile, selected scopes)` — the compiler may emit many per run |

### Preservation Semantics

Every canonical object carries a preservation level:

- **`required`**: Unsupported or unsafe lowering fails the build
- **`preferred`**: Lower when safe, otherwise warn and skip
- **`optional`**: May skip with reporting

### Bounded Contexts

| Context | Aggregate Root | Key Value Objects |
|---------|---------------|-------------------|
| Parsing | SourceTree | RawObject, SchemaVersion |
| Dependency | DependencyGraph | PackageRef, VersionConstraint, LockEntry |
| Normalization | SemanticGraph | NormalizedObject, Scope, InheritanceChain |
| Build Planning | BuildPlan | BuildUnit, BuildCoordinate |
| Capability | CapabilityGraph | Capability, Provider, ProviderCandidate |
| Lowering | LoweredGraph | LoweringDecision, PreservationLevel |
| Rendering | EmissionPlan | EmittedFile, EmissionLayer |
| Reporting | BuildReport | Provenance, Diagnostic, LoweringRecord |
| Pipeline | StageRegistry | StageDescriptor, StageHook, StageOrder |
