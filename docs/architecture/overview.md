# AI Agent Metadata Control Plane Architecture

## 1. Purpose

This specification defines a compiler-based control plane for AI-agent metadata and behavior.

The architecture uses one canonical source tree and emits native artifacts for one or more target ecosystems. It preserves target-native power where possible, applies explicit lowering where necessary, and reports all loss of fidelity through provenance and build diagnostics.

The architecture treats platform differences as compiler concerns, not as a shared runtime abstraction. The canonical model describes intent, dependencies, preservation requirements, and delivery policy. Renderers emit native artifacts for each target.

Plugins are first-class in this specification. A skill is model-facing workflow content. A plugin is a runtime-facing integration package that can provide tools, MCP servers, commands, resources, hooks, or other target-native extension points.

---

## 2. Goals

This control plane exists to satisfy the following goals:

1. author once in a canonical source tree
2. compile that source to one or more target platforms
3. preserve the strongest native features of each target instead of flattening to the least common denominator
4. minimize target-specific deviations, but allow a small and explicit override surface
5. treat the source tree as compiler input and the generated target artifacts as compiler output
6. make all loss of fidelity visible through reports, provenance, and policy

Examples of targets include:

- Claude Code
- Cursor
- GitHub Copilot
- Codex
- adjacent ecosystems added later through new renderers

---

## 3. Non-Goals

This architecture does **not** try to do the following:

- define a shared runtime that every vendor executes identically
- guarantee identical behavior across targets
- erase security, permission, or filesystem differences between targets
- inline every target-specific feature into the canonical model
- silently downgrade required behavior

If a target cannot safely preserve required intent, the compiler must fail or require an explicit override.

---

## 4. Implementation Architecture

The system is implemented in Go (1.25+) following Domain-Driven Design, Hexagonal Architecture, and Clean Architecture.

### 4.1 Guiding Principles

- **Domain model at the center**: canonical objects (instructions, rules, skills, agents, hooks, commands, capabilities, plugins) are domain entities with no infrastructure dependencies.
- **Ports and adapters**: the domain defines port interfaces for registry access, filesystem I/O, target rendering, and plugin resolution. Adapters implement those ports for concrete infrastructure.
- **Dependency rule**: dependencies point inward. Domain knows nothing about renderers, registries, or CLI. Application services orchestrate domain logic through ports. Adapters live at the outer ring.
- **Bounded contexts**: the compiler pipeline naturally decomposes into bounded contexts — parsing, dependency resolution, normalization, capability resolution, lowering, rendering, materialization, and reporting. Each context owns its own aggregates and value objects.
- **Compiler plugins**: every pipeline stage and every target renderer is a plugin. The compiler core defines stage interfaces; concrete implementations register as plugins. Third parties can extend or replace any stage without forking.

### 4.2 Package Layout

```text
cmd/                        # CLI entry points
internal/
  domain/                   # Core domain model (entities, value objects, domain services)
    model/                  #   Canonical objects: instruction, rule, skill, agent, etc.
    capability/             #   Capability contracts and resolution logic
    plugin/                 #   Plugin domain (distribution, selection, security)
    build/                  #   Build coordinates (target, profile, scope, build unit)
    pipeline/               #   Pipeline stage interfaces and compiler plugin contracts
  application/              # Application services (use cases, orchestration)
    compiler/               #   Compiler pipeline orchestration and plugin registry
    dependency/             #   Dependency resolution use cases
    packaging/              #   Platform-native packaging use cases
  port/                     # Port interfaces (driven and driving)
    registry/               #   Registry access port
    renderer/               #   Target renderer port
    filesystem/             #   Filesystem I/O port
    reporter/               #   Reporting and provenance port
    stage/                  #   Pipeline stage port (compiler plugin SPI)
  adapter/                  # Infrastructure adapters
    cli/                    #   CLI adapter (cobra/flags)
    filesystem/             #   Filesystem adapter
    plugin/                 #   Plugin adapter
    registry/               #   Registry client adapters (HTTP, git, filesystem)
    renderer/               #   Target renderer backends (claude, cursor, copilot, codex)
    reporter/               #   Reporter adapter
    stage/                  #   Built-in pipeline stage implementations
      capability/           #     Capability resolution stage
      lowering/             #     Lowering stage
      materializer/         #     File/symlink materializer stage
      normalizer/           #     Normalization stage
      parser/               #     YAML/Markdown parser stage
      planner/              #     Build plan stage
      reporter/             #     Report generation stage
      resolver/             #     Dependency resolution stage
      validator/            #     Validation stage
    tool/                   #   Tool plugin adapters
pkg/                        # Public API types shared with external tools
  sdk/                      # Compiler plugin SDK for third-party stage/renderer authors
```

### 4.3 Domain Boundaries

| Bounded Context | Aggregate Root | Key Value Objects |
|---|---|---|
| Parsing | SourceTree | RawObject, SchemaVersion |
| Dependency | DependencyGraph | PackageRef, VersionConstraint, LockEntry |
| Normalization | SemanticGraph | NormalizedObject, Scope, InheritanceChain |
| Build Planning | BuildPlan | BuildUnit, BuildCoordinate |
| Capability | CapabilityGraph | Capability, Provider, ProviderCandidate |
| Lowering | LoweredGraph | LoweringDecision, PreservationLevel |
| Rendering | EmissionPlan | EmittedFile, EmissionLayer |
| Reporting | BuildReport | Provenance, Diagnostic, LoweringRecord |
| Pipeline | StageRegistry | StageDescriptor, StageHook, StageOrder |

Interfaces between bounded contexts are small and explicit. Each context exposes a port that the next stage in the pipeline consumes. Every stage is backed by a compiler plugin that implements the stage port interface.

---

## 5. Core Principle

The system must be built as a multi-stage compiler.

The `.ai/` tree is the source language.

The compiler performs:

1. parsing and validation
2. dependency resolution against registries and lock file
3. normalization into a semantic graph
4. expansion into a target/profile build graph
5. capability resolution
6. lowering of unsupported concepts
7. rendering into target-native files and packages
8. materialization of scripts, assets, resources, and plugins
9. reporting and provenance generation

Textually:

```text
.ai/ source tree
  -> parser + schema validator
  -> dependency resolver (registries + lock file)
  -> normalized semantic graph
  -> target x profile build graph
  -> capability resolver + lowering engine
  -> target renderer backends
  -> asset/script/plugin materializer
  -> .ai-build/ outputs + reports + optional repo sync
```

Renderers must not interpret raw source files directly. They must consume normalized IR so that:

- renderer behavior stays deterministic
- lowering decisions are centralized
- provenance is uniform
- new targets do not re-implement core semantics

---

## 6. Build Coordinates

The compiler operates on explicit build coordinates.

### 6.1 Target

A **target** is the vendor or ecosystem being emitted for, for example:

- `claude`
- `cursor`
- `copilot`
- `codex`

Targets own syntax, file placement, packaging rules, and capability support.

### 6.2 Profile

A **profile** is the runtime environment or policy shape, for example:

- `local-dev`
- `ci`
- `enterprise-locked`
- `oss-public`

Profiles control security-sensitive behavior such as:

- whether hooks are enabled
- whether scripts may be emitted
- whether plugins may auto-install
- whether networked capabilities are allowed
- whether secrets or MCP bindings may be referenced

### 6.3 Scope

A **scope** identifies where canonical objects apply.

Supported scope styles should include:

- repository root
- subtree path
- glob
- file type or language
- labels or bounded contexts

### 6.4 Build Unit

A **build unit** is:

```text
(target, profile, selected scopes)
```

The compiler may emit many build units in one run. This supports “compile to one or more environments” without duplicating source content.

---

## 7. Canonical Object Model

The object model must distinguish between authoring primitives and runtime delivery primitives.

### 7.1 Authoring Primitives

These are the main source-language concepts.

#### Instructions

Always-on guidance and context.

Use for:

- architecture principles
- coding standards
- testing expectations
- review policies
- domain vocabulary
- workflow guidance

Instructions are unconditional within their scope.

#### Rules

Scoped or conditional policy.

Use for:

- language-specific rules
- generated-code handling
- security restrictions for sensitive paths
- path- or file-type-specific conventions

Rules are semantically distinct from instructions even if some targets require lowering them into instructions.

#### Skills

Reusable, model-facing workflow bundles.

Use for:

- build a Go Lambda
- review IAM policy changes
- run Terraform validation
- scaffold a new bounded context

Skills may contain:

- markdown guidance
- examples
- templates
- prompt fragments
- assets
- references (supplemental knowledge documents)
- references to scripts or resources
- capability requirements

Skills are portable because they describe how work should be done, not how the runtime itself is extended. Skills follow the [AgentSkills.io](https://agentskills.io/) open standard, which is natively supported by Claude Code, GitHub Copilot, and Codex CLI. Cursor does not support skills and must receive lowered content.

#### Agents

Specialized delegates or orchestration wrappers.

Use for:

- Go implementer
- infrastructure reviewer
- security reviewer
- documentation refiner

Agents define:

- role or system prompt
- tool and permission policy
- allowed delegations
- linked skills
- required capabilities
- optional hooks

An agent is not a tool provider. It is a policy and orchestration surface around tools, skills, and delegation.

Agents may optionally define **handoffs**: guided sequential workflow transitions that suggest the user move to another agent with a pre-filled prompt (e.g., Plan → Implement → Review). Handoffs are currently supported natively by Copilot and are emitted only where the target supports them.

#### Hooks

Deterministic lifecycle automation triggered by defined events.

Use for:

- post-edit validation
- pre-run setup
- prompt transforms
- post-generation linting

Hooks are highly non-portable and must therefore carry explicit effect semantics.

#### Commands

Explicit user-invoked entry points.

Use for:

- `/review-iam`
- `/build-lambda`
- `/refactor-ddd-boundary`

Commands are often a fallback target surface when a platform lacks lifecycle hooks.

In Copilot, commands map to **prompt files** (`.prompt.md`), which are lightweight slash commands with optional frontmatter specifying agent, model, and tools. In Claude Code, commands have been merged into skills. In Cursor, commands have no native equivalent.

### 7.2 Runtime Delivery Primitives

These are not model-facing authoring concepts. They exist to satisfy or package runtime needs.

#### Capabilities

A **capability** is an abstract contract required by authoring primitives.

Examples:

- `filesystem.read`
- `terminal.exec`
- `repo.search`
- `repo.graph.query`
- `aws.validate.iam`
- `mcp.github`

Skills, agents, commands, and hooks may require capabilities. Targets do not consume capabilities directly; they consume concrete providers selected by the compiler.

#### Plugins

A **plugin** is a deployable extension package that provides one or more capabilities or native extension surfaces.

Examples:

- a target-native plugin manifest and runtime bundle
- an MCP-backed extension package
- a packaged code-search provider
- a repo-graph service exposed as tools

Plugins may provide:

- capabilities
- native tool manifests
- MCP server bindings
- commands
- resources
- assets
- target-native manifests
- optional hooks or event bindings where the target supports them

Plugins are runtime-facing packaging units. They must be modeled separately from skills.

##### Distribution Modes

A plugin may be distributed through one of three modes:

- **inline**: plugin code and artifacts live in `.ai/plugins/` in the repository. The compiler copies or symlinks them into the build output.
- **external**: the plugin is installed and managed outside the repository. The compiler emits only a reference (for example, an entry in `.claude/settings.json` or a Cursor MCP config block). No plugin code is materialized.
- **registry**: the plugin is declared as a dependency, resolved from a marketplace or package registry at compile time, cached locally, and either materialized or referenced depending on the target.

This distinction is critical because many targets (Claude Code, Cursor, Copilot) only need a configuration reference to an externally-installed MCP server or tool, not a full copy of its source.

#### References

Supplemental knowledge documents that provide in-depth material on a topic. References are demand-loaded: the AI reads them only when deeper knowledge is needed during a workflow, rather than being injected into context unconditionally.

Use for:

- detailed pattern catalogs or cheatsheets
- deep-dive documentation on a domain concept
- tool or CLI reference manuals
- architectural decision rationale
- extended examples too large for the main skill content

References are currently used within skills but the concept is applicable to any object that carries content (agents, instructions). A skill's `references/` directory contains markdown files that the skill's main content links to.

References are distinct from assets: an asset is a static file consumed by tooling or emitted into output (template, diagram, prompt partial). A reference is a knowledge document consumed by the AI model at read time.

#### Assets

Static files used by other objects, for example:

- templates
- examples
- diagrams
- prompt partials
- sample outputs

#### Scripts

Executable artifacts used by hooks, commands, skills, or plugins.

Examples:

- validation scripts
- resource generators
- packaging helpers
- local servers used by MCP or plugin runtimes

### 7.3 Required Separation

The following distinctions are required for the architecture to remain coherent:

- **Instruction**: always-on policy text
- **Rule**: conditional or scoped policy
- **Skill**: reusable workflow content
- **Agent**: orchestration wrapper around prompts, permissions, skills, and delegation
- **Capability**: abstract contract that some object needs
- **Plugin**: deployable provider or extension that satisfies capabilities

This separation prevents the common failure mode where “skill”, “tool”, “plugin”, and “agent” collapse into one ambiguous concept.

---

## 8. Common Metadata Contract

Every canonical object should share a small common envelope:

```yaml
id: unique-id
kind: instruction | rule | skill | agent | hook | command | capability | plugin
version: 1
scope:
  paths: ["/"]
appliesTo:
  targets: ["*"]
  profiles: ["*"]
extends: []
preservation: required | preferred | optional
labels: []
owner: platform-team
targetOverrides: {}
```

### 8.1 Preservation Semantics

Preservation must be explicit.

- `required`: unsupported or unsafe lowering fails the build
- `preferred`: lower when safe, otherwise warn and skip
- `optional`: may skip with reporting

This is stronger than a single global `unsupportedMode`. Different objects have different importance.

### 8.2 Override Semantics

`targetOverrides` must be delta-based. They may adjust:

- syntax
- file placement
- packaging hints
- enablement
- optional target-native features

They must not silently redefine canonical meaning unless the override is marked as a semantic replacement with an explicit reason.

---

## 9. Canonical Repository Layout

Recommended source layout:

```text
.ai/
  manifest.yaml
  instructions/
  rules/
  skills/
  agents/
  hooks/
  commands/
  capabilities/
  plugins/
  references/
  assets/
  scripts/
  profiles/
  targets/
  schema/
```

Responsibilities:

- `manifest.yaml`: build defaults, selected targets and profiles, compiler policy, materialization policy
- `instructions/`: always-on content by scope
- `rules/`: conditional or scoped policy
- `skills/`: reusable workflow bundles
- `agents/`: specialized orchestration definitions
- `hooks/`: lifecycle automation definitions
- `commands/`: explicit user-invoked workflows
- `capabilities/`: abstract contracts consumed by skills, agents, hooks, and commands
- `plugins/`: deployable integration packages that provide capabilities or native extension surfaces
- `references/`: supplemental knowledge documents consumed by the AI on demand (may also live inside skill or agent directories)
- `assets/`: static reusable files
- `scripts/`: executable artifacts
- `profiles/`: environment-specific policy and enablement data
- `targets/`: renderer configuration, target-specific capability registry, override fragments, packaging glue
- `schema/`: versioned validation schemas

This layout keeps authoring concerns separate from delivery concerns.

---

## 10. Architecture Summary

This architecture is defined by the following properties:

- the `.ai/` tree is a source language, not a runtime
- the compiler emits a build matrix of `target x profile`
- authoring primitives and delivery primitives stay separate
- skills remain model-facing workflow bundles
- plugins become first-class runtime packaging units with three distribution modes (inline, external, registry)
- each build unit emits both an instruction layer (CLAUDE.md, AGENTS.md, `.cursor/rules/*.mdc`, `copilot-instructions.md`) and an extension layer (MCP config, settings) in hybrid mode by default
- capabilities bridge canonical intent to concrete runtime providers
- lowering is explicit, loss-aware, and governed by preservation levels
- target overrides exist, but remain narrow and auditable
- reports and provenance are mandatory
- external dependencies are resolved from registries with versioning, integrity, and trust controls
- compiled output can be wrapped as platform-native packages (VS Code extensions, npm packages) for distribution and updates through each platform's marketplace

These properties satisfy the stated goals:

1. single source of truth
2. access to target-native bells and whistles where supported
3. one or many target outputs from the same source
4. strong abstraction with a small override surface
5. a compiler-shaped architecture rather than hand-maintained copies

---

## Related Documents

- [schemas.md](schemas.md) — illustrative YAML schemas for all canonical object types
- [compiler.md](compiler.md) — compiler pipeline, compiler plugin architecture, capability registry, content plugin architecture
- [build-output.md](build-output.md) — hierarchy, hooks, output layout, reporting, validation, target strategy, implementation, testing
- [marketplace.md](marketplace.md) — package distribution, registry, dependency resolution, trust policies, external plugin references
- [target-grounding.md](target-grounding.md) — verified mapping of canonical concepts to actual platform documentation for Claude Code, Cursor, Copilot, and Codex
- [editor-tooling.md](editor-tooling.md) — VS Code extensions, language server, build integration, registry UI, preview, extension plugin architecture

