# Compiler, Capabilities, and Plugin Architecture

This document covers the compiler pipeline, compiler plugin architecture, capability registry, lowering policy, and content plugin architecture. It continues from the core model defined in [overview.md](overview.md). Schema sketches are in [schemas.md](schemas.md).

---

## 1. Compiler Architecture

### 1.1 Source IR

The source IR is the parsed representation of files under `.ai/`.

At this stage the compiler has:

- raw object fields
- raw markdown
- raw paths
- unresolved inheritance
- unresolved references
- unresolved external dependencies

### 1.2 Dependency Resolution

Before normalization, the compiler must resolve external dependencies declared in `manifest.yaml`.

This phase:

1. reads the dependency declarations and existing lock file
2. resolves versions against configured registries
3. fetches or validates cached packages
4. verifies integrity hashes
5. merges resolved external objects into the source IR
6. updates the lock file if new resolutions occurred

External packages contribute the same canonical object types as local `.ai/` content. After resolution, the normalizer treats them uniformly.

Dependency resolution must respect profile trust policies. For example, `enterprise-locked` may restrict which registries and publishers are allowed.

### 1.3 Normalized IR

The normalized IR is the first semantically meaningful graph.

It must resolve:

- object IDs
- inheritance chains
- scope selectors
- relative paths
- profile and target applicability
- duplicate detection
- default values

Renderers must consume this IR, not the raw source.

### 1.4 Build Graph

The build graph expands normalized objects into concrete build units:

```text
normalized object set
  x selected targets
  x selected profiles
  x selected scopes
  -> target/profile object instances
```

This is where the compiler decides which objects are active in which outputs.

### 1.5 Capability Resolution Graph

For each build unit, the compiler must resolve:

- required capabilities
- candidate providers
- whether the provider is native, MCP-backed, script-backed, or plugin-backed
- security and profile compatibility
- whether auto-install or manual installation is allowed

### 1.6 Lowered IR

The lowered IR is target-specific but still semantic.

Examples:

- a rule lowered into an instruction section
- a hook lowered into a command
- a plugin omitted because the target has no plugin surface
- an agent split into target-native agent files and shared skill references

The lowered IR must preserve provenance back to normalized objects.

### 1.7 Emission Plan

The emission plan is a concrete list of outputs:

- files to generate
- directories to create
- assets and scripts to copy or symlink
- plugin bundles to package
- install metadata to emit
- warnings and errors to report

Only after the emission plan is complete should the compiler write files.

### 1.8 Emission Layers

A single build unit typically emits artifacts across two distinct layers:

#### Instruction Layer

Target-native files that carry instructions, rules, agent definitions, skill content, and commands as model-facing text. These are the files the AI agent reads on startup.

Examples:

- `CLAUDE.md` and subtree `CLAUDE.md` files for Claude Code
- `AGENTS.md` and `SKILL.md` files for GitHub Copilot
- `.cursorrules` and `.cursor/rules/*.mdc` files for Cursor
- `AGENTS.md` for Codex

These files form a hierarchy matching the repository structure. The renderer decides the exact file names, placement, and syntax.

#### Extension Layer

Target-native configuration that references plugins, MCP servers, tools, and other runtime extensions. These are settings files that configure what capabilities the agent has access to.

Examples:

- `.claude/settings.json` with `mcpServers` entries
- `.cursor/mcp.json` with MCP server bindings
- `.vscode/mcp.json` with MCP server definitions
- `.vscode/settings.json` with extension configuration

#### Hybrid Emission

The default and most common emission mode is **hybrid**: a build unit emits both instruction-layer files and extension-layer configuration in a single pass.

This is not an optional mode — it is the natural consequence of a source tree that contains both authoring primitives (instructions, rules, skills, agents) and runtime delivery primitives (plugins, capabilities). The compiler must emit both layers because the instruction layer tells the agent *how to work* while the extension layer gives the agent *tools to work with*.

The manifest may control layer behavior:

```yaml
build:
  emission:
    instructionLayer: true
    extensionLayer: true
```

Setting either to `false` allows pure-instruction or pure-extension builds when needed, but hybrid is the default.

---

## 2. Compiler Plugin Architecture

Every pipeline stage and every target renderer is a **compiler plugin**. The compiler core is a thin orchestrator that loads plugins, orders them, and passes IR between stages. Concrete behavior lives entirely in plugins.

### 2.1 Why Compiler Plugins

Without pluggable stages the compiler is a monolith that must be forked to:

- add a new target renderer
- customize lowering for an organization's conventions
- inject a validation stage between normalization and build graph expansion
- replace the default dependency resolver with a corporate proxy
- add a new emission format (for example, a documentation site generator)

Compiler plugins make these changes composable without forking.

### 2.2 Stage Interface

Every pipeline stage implements a common port:

```go
// Stage is the core compiler plugin contract.
type Stage interface {
    // Descriptor returns metadata: name, phase, ordering priority.
    Descriptor() StageDescriptor

    // Execute runs the stage, receiving input IR and returning output IR.
    // The concrete IR types depend on the phase.
    Execute(ctx context.Context, input any) (any, error)
}
```

The `StageDescriptor` declares:

- **name**: unique stage identifier
- **phase**: which pipeline phase this stage belongs to (parse, resolve, normalize, plan, capability, lower, render, materialize, report)
- **order**: priority within the phase (stages in the same phase run in order)
- **before / after**: explicit ordering constraints relative to other stage names
- **target**: optional target filter (only runs for specific targets)

### 2.3 Pipeline Phases

The compiler defines these ordered phases. Each phase may contain one or more stage plugins.

| Phase | Default Stage | Input IR | Output IR |
|---|---|---|---|
| `parse` | YAML/Markdown parser | filesystem paths | SourceTree |
| `validate` | Schema validator | SourceTree | SourceTree (validated) |
| `resolve` | Dependency resolver | SourceTree | SourceTree (with externals) |
| `normalize` | Normalizer | SourceTree | SemanticGraph |
| `plan` | Build graph expander | SemanticGraph | BuildPlan |
| `capability` | Capability resolver | BuildPlan | CapabilityGraph |
| `lower` | Lowering engine | CapabilityGraph | LoweredGraph |
| `render` | Target renderer | LoweredGraph | EmissionPlan |
| `materialize` | File writer | EmissionPlan | MaterializationResult |
| `report` | Provenance generator | MaterializationResult | BuildReport |

### 2.4 Stage Hooks

In addition to full stage plugins, the pipeline supports lightweight hooks:

- **before-phase**: runs before all stages in a phase
- **after-phase**: runs after all stages in a phase
- **transform**: receives and may modify the IR between two stages

Hooks are useful for cross-cutting concerns like logging, metrics, or organizational policy checks without replacing a full stage.

### 2.5 Target Renderer as Compiler Plugin

Target renderers (claude, cursor, copilot, codex) are stage plugins registered for the `render` phase with a target filter. When the pipeline reaches the render phase, it dispatches each build unit to the renderer plugin matching the unit's target.

A renderer plugin implements:

```go
// Renderer is a specialized Stage for the render phase.
type Renderer interface {
    Stage

    // Target returns the target this renderer handles.
    Target() string

    // SupportedCapabilities returns the capability registry for this target.
    SupportedCapabilities() CapabilityRegistry
}
```

New targets are added by registering a new renderer plugin. No compiler core changes are needed.

### 2.6 Plugin Registration

Compiler plugins are registered through:

1. **Built-in**: compiled into the binary, registered at init time
2. **Go plugin**: loaded as shared objects via Go's `plugin` package (platform-dependent)
3. **External process**: executed as a subprocess communicating over stdin/stdout with a defined protocol (portable, language-agnostic)

The manifest can configure which plugins are active:

```yaml
compiler:
  plugins:
    - name: acme-custom-lowering
      source: ./plugins/acme-lowering.so
      phase: lower
      order: 50
    - name: docs-renderer
      source: external://docs-renderer
      phase: render
      target: docs
```

### 2.7 Plugin Discovery and Trust

External compiler plugins are executable code and carry security implications:

- built-in plugins are trusted implicitly
- Go plugins must match the compiler's Go version and module checksums
- external process plugins must be allow-listed in the manifest
- profiles may restrict which compiler plugins are permitted (for example, `enterprise-locked` may forbid external process plugins)

---

## 3. Capability Registry and Lowering Policy

The capability registry is structured and loss-aware.

### 3.1 Capability Support Levels

Each target capability surface should be classified as one of:

- `native`: first-class support exists
- `adapted`: same semantics, different syntax or file placement
- `lowered`: compiler can map the concept to another primitive
- `emulated`: only an approximation exists
- `skipped`: not available

`true` and `partial` are too vague for compiler decisions.

### 3.2 Capability Registry Shape

Illustrative example:

```yaml
targets:
  codex:
    instructions:
      layeredFiles: native
      scopedSections: adapted
    rules:
      scopedRules: lowered
    skills:
      bundles: native
      scriptPackaging: adapted
    agents:
      subagents: native
      toolPolicies: native
    hooks:
      lifecycle: skipped
      blockingValidation: skipped
    commands:
      explicitEntryPoints: lowered
    plugins:
      installablePackages: native
      capabilityProviders: native
    capabilities:
      nativeTools: native
      mcpBindings: native
```

The actual registry belongs under `targets/` and should be versioned separately from the source schema because target support changes over time.

### 3.3 Safe and Unsafe Lowering

Lowering is only acceptable when semantics remain acceptable for the object's preservation level.

Safe lowerings:

- rule -> instruction section
- subtree hierarchy -> flattened file sections with retained provenance
- skill resources -> packaged sidecar files

Conditionally safe lowerings:

- command -> documented script invocation
- plugin provider -> MCP-backed provider

Unsafe lowerings:

- blocking hook -> advisory command
- required capability -> plain text mention
- restricted tool policy -> unrestricted tool access
- required plugin-provided behavior -> omitted extension

If lowering is unsafe and preservation is `required`, the build must fail.

### 3.4 Provider Selection Order

A reasonable default provider selection order is:

1. native target capability
2. configured plugin
3. MCP-backed provider
4. script-backed provider
5. textual documentation fallback

This order must still respect:

- preservation level
- profile security policy
- trust policy
- explicit target overrides

---

## 4. Content Plugin Architecture

Plugins must be first-class because several target ecosystems expose extension packaging that is neither pure instruction text nor pure skill content.

### 4.1 Why Plugins Are Distinct

A plugin is not:

- a skill
- an agent
- a hook
- a target override

A plugin is a deployable runtime provider.

This distinction matters because a plugin can carry:

- executable runtime code
- tool schemas
- installation metadata
- MCP transport details
- permissions and trust requirements
- target-native manifests

None of that belongs inside skill content.

### 4.2 Relationship Between Concepts

- **Skill** consumes capabilities and describes workflow.
- **Agent** links skills, permissions, and delegation.
- **Capability** names a required contract.
- **Plugin** provides one or more capabilities.
- **Renderer** decides how the selected plugin is packaged for the target.

This preserves native power without contaminating the authoring layer with target-specific packaging.

### 4.3 Plugin Lifecycle

For each build unit the compiler should:

1. collect required capabilities
2. find provider candidates (local, external, and registry)
3. select zero or one provider per capability according to policy
4. resolve distribution mode for each selected plugin:
   - **inline**: copy or symlink plugin artifacts into the build output
   - **external**: emit only a target-native reference (for example, an MCP server entry in a settings file)
   - **registry**: resolve from registry, cache locally, then emit as inline or external depending on target
5. emit installation metadata or manifests
6. record whether install is automatic, opt-in, or manual

### 4.4 Plugin Selection Policy

Plugins should support selection modes:

- `auto-if-selected`
- `opt-in`
- `manual-only`
- `disabled`

Plugins should also declare an install strategy:

- `materialize`: copy full plugin artifacts into the build output
- `reference-only`: emit a configuration reference; do not materialize code
- `auto-detect`: let the renderer choose based on target capabilities

Profiles may restrict selection. For example, `enterprise-locked` may forbid `auto-if-selected` for networked plugins or require `reference-only` for externally-managed extensions.

### 4.5 Plugin Security Model

Plugin definitions should declare:

- trust level
- filesystem permissions
- network permissions
- process execution requirements
- secret requirements

Example:

```yaml
security:
  trust: review-required
  permissions:
    filesystem: read-repo
    network: outbound
    processExec: false
    secrets: []
```

This is necessary because plugins are executable delivery units, not just documentation.
