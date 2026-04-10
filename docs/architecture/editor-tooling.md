# Editor Tooling Architecture

This document specifies the VS Code extension layer that provides IDE support for `.ai/` source trees. It builds on the compiler defined in [compiler.md](compiler.md) and the object model defined in [overview.md](overview.md).

---

## 1. Purpose

The compiler operates as a CLI tool. Developers also need real-time feedback, navigation, and automation inside their editor. The editor tooling layer bridges the compiler into VS Code through a set of extensions that share the compiler's domain model and plugin architecture.

---

## 2. Extension Architecture

The tooling ships as four logical extensions. They may be bundled into a single VS Code extension for simplicity or published separately with the language server as a required dependency.

All extensions share a single Go binary that runs as an LSP (Language Server Protocol) server. The binary embeds the compiler domain model so that editor diagnostics and compiler output stay consistent.

```text
goagentmeta binary
  ├── LSP server (language features)
  ├── compiler pipeline (build integration)
  ├── registry client (package management)
  └── preview engine (live output rendering)
```

---

## 3. Language Server Extension

The core editing experience for `.ai/` source files.

### 3.1 Syntax Highlighting

Custom TextMate grammar for `.ai/` YAML files with semantic token support:

- object kind keywords (`kind: skill`, `kind: plugin`)
- capability references
- scope glob patterns
- cross-references to other object IDs
- preservation levels
- inline markdown content blocks

### 3.2 Schema Validation

Real-time diagnostics matching the compiler's structural and semantic validation:

- required fields missing
- unknown fields
- type mismatches
- duplicate object IDs
- missing referenced objects (skills, capabilities, plugins)
- circular agent delegation
- circular inheritance chains
- invalid scope patterns
- invalid target override structure
- ambiguous provider selection
- conflicting rules at equal priority

Validation runs on every keystroke (debounced) using the same schema and semantic rules as the compiler.

### 3.3 Code Completion

Context-aware completions for:

- object IDs when referencing skills, agents, capabilities, plugins
- capability names from the capability registry
- scope path patterns (informed by the workspace file structure)
- profile and target names from the manifest
- registry package names from configured registries
- YAML field names from the object schema
- preservation level values
- distribution mode values
- effect class and enforcement mode values for hooks

### 3.4 Navigation

- **Go-to-definition**: from capability reference to capability file, skill reference to skill file, plugin provider to plugin definition, agent delegation to delegated agent
- **Find all references**: where a capability, skill, or plugin ID is consumed
- **Workspace symbol search**: jump to any canonical object by name or ID
- **Breadcrumb**: scope path hierarchy for the current object

### 3.5 Hover Information

On hover, display:

- capability descriptions and security declarations
- plugin distribution mode, provided capabilities, and trust level
- preservation level with explanation of compiler behavior
- target support status from the capability registry
- resolved package version and registry for registry dependencies
- lowering decision for the hovered object per target

### 3.6 Quick Fixes

Code actions for common problems:

- add missing required fields with sensible defaults
- fix scope pattern syntax errors
- generate a capability stub from a `requires:` reference
- fix preservation level mismatches (object requires `required` but provides `optional`)
- add a missing target override when a warning indicates lowering
- add a missing plugin to `manifest.yaml` dependencies
- generate a plugin skeleton from a capability reference
- extract inline content to a referenced asset file

### 3.7 CodeLens

Inline annotations above objects:

- which targets support the object natively vs. lowered vs. skipped
- current provider candidates for required capabilities
- number of consumers for a capability or plugin
- lowering decisions per target

### 3.8 Rename

Safe rename of object IDs across all cross-references within the `.ai/` source tree.

### 3.9 Formatting

YAML formatting for `.ai/` files following the project's schema conventions:

- consistent field ordering
- canonical indentation
- normalized scope patterns

---

## 4. Compiler Integration Extension

Bridges the compiler CLI into the VS Code workflow.

### 4.1 Build Tasks

Task provider registered for VS Code's task system:

- `ai-build compile` — full compilation to `.ai-build/`
- `ai-build lint` — validation-only pass
- `ai-build sync` — materialize build output into the repository
- `ai-build update-deps` — resolve and update dependencies

Tasks are configured through `tasks.json` or run via the command palette.

### 4.2 Problem Matcher

Maps compiler diagnostics into VS Code's Problems panel:

- structural validation errors
- semantic validation warnings
- lowering decisions with preservation violations
- trust and security policy violations
- dependency resolution failures

Each diagnostic links to the source `.ai/` file and line.

### 4.3 Build Output Explorer

Tree view in the sidebar showing:

- `.ai-build/` directory structure
- per-target, per-profile breakdown
- emitted file list with icons for instruction layer vs. extension layer
- provenance links back to source objects

### 4.4 Target Diff View

Side-by-side comparison of rendered output across targets:

- select two targets (e.g., Claude vs. Copilot) and an object
- see how the same canonical object renders differently
- highlights lowering decisions and omitted content

### 4.5 Provenance Navigator

Click any generated file in `.ai-build/` to see:

- source objects that contributed to it
- lowering chain applied
- preservation level decisions
- compiler plugin that rendered it

### 4.6 Watch Mode

Re-compile on `.ai/` file save:

- incremental rebuild where possible
- instant diagnostic updates in the Problems panel
- updated build output explorer

---

## 5. Registry and Marketplace Extension

Package management inside the editor.

### 5.1 Package Browser

Search and browse configured registries:

- search by keyword, capability provided, target supported, publisher
- view package metadata (description, version, capabilities, security)
- one-click install (adds to `manifest.yaml` dependencies and runs `update-deps`)

### 5.2 Dependency Panel

Tree view of current dependencies:

- resolved version and source registry
- provided capabilities
- trust and security summary
- update available indicator

### 5.3 Version Notifications

- badge on the dependency panel when updates are available
- command to view changelog and diff between locked and latest version
- bulk update command with lock file regeneration

### 5.4 Publish Command

Publish a package to a registry from the editor:

- validate package manifest
- run trust and security checks
- prompt for version bump
- publish to the configured registry

---

## 6. Preview Extension

Visualization of what the compiler will produce.

### 6.1 Live Preview

Split-pane preview of rendered target output:

- select a target and profile
- rendered `CLAUDE.md`, `AGENTS.md`, `.cursorrules`, `copilot-instructions.md` updates as you edit source
- highlights sections contributed by the current file

### 6.2 Capability Matrix

Visual grid display:

- rows: canonical objects (skills, agents, hooks, commands, plugins)
- columns: targets (claude, cursor, copilot, codex)
- cells: support level (native, adapted, lowered, emulated, skipped) with color coding

### 6.3 Lowering Preview

Per-object, per-target lowering visualization:

- original canonical form on the left
- lowered target form on the right
- annotations explaining each lowering decision
- preservation level warning if lowering is unsafe

### 6.4 Emission Plan Preview

Full file tree preview of what `ai-build compile` would produce:

- instruction layer files
- extension layer files
- materialized assets and scripts
- plugin bundles or references
- without actually writing files

---

## 7. Extension Plugin Architecture

The language server extension is itself pluggable, mirroring the compiler plugin model.

### 7.1 Contribution Points

The core extension exposes VS Code contribution points that third-party extensions can consume:

| Contribution Point | Purpose |
|---|---|
| `goagentmeta.validators` | Register custom validation rules |
| `goagentmeta.completionProviders` | Add custom completions |
| `goagentmeta.quickFixProviders` | Add custom quick fixes |
| `goagentmeta.codeLensProviders` | Add custom CodeLens annotations |
| `goagentmeta.schemaExtensions` | Validate custom YAML fields |
| `goagentmeta.previewRenderers` | Add custom preview visualizations |

### 7.2 Custom Validators

Organizations register domain-specific validation rules:

- "all skills must reference a test script"
- "plugins with network permissions require security review approval"
- "agents in the `services/` subtree must link the `go-aws-lambda` skill"

Custom validators implement a simple interface and are loaded by the language server:

```typescript
interface CustomValidator {
  id: string;
  validate(document: AiDocument): Diagnostic[];
}
```

### 7.3 Custom Completion Providers

Add completions for organization-specific concepts:

- internal capability names
- custom scope conventions
- organization-specific profile names
- internal registry package names

### 7.4 Custom Quick Fixes

Fix patterns specific to an organization's conventions:

- auto-add required labels
- enforce naming conventions
- apply security policy templates

### 7.5 Custom Schema Extensions

Validate `targetOverrides` or custom YAML fields added by compiler plugins:

- compiler plugins may extend the canonical schema with custom fields
- the corresponding VS Code extension plugin adds validation and completion for those fields

---

## 8. LSP Protocol Details

The Go binary communicates with VS Code through the Language Server Protocol.

### 8.1 Supported LSP Features

| LSP Feature | Used For |
|---|---|
| `textDocument/completion` | Code completion |
| `textDocument/hover` | Hover information |
| `textDocument/definition` | Go-to-definition |
| `textDocument/references` | Find all references |
| `textDocument/rename` | Safe rename |
| `textDocument/codeAction` | Quick fixes |
| `textDocument/codeLens` | CodeLens annotations |
| `textDocument/formatting` | YAML formatting |
| `textDocument/publishDiagnostics` | Real-time validation |
| `workspace/symbol` | Workspace symbol search |

### 8.2 Custom LSP Extensions

Beyond standard LSP, the server exposes custom requests:

- `goagentmeta/compile` — trigger compilation
- `goagentmeta/preview` — request rendered preview for a target
- `goagentmeta/capabilityMatrix` — request the capability support matrix
- `goagentmeta/emissionPlan` — request the emission plan without writing files
- `goagentmeta/provenance` — request provenance chain for a generated file
- `goagentmeta/registrySearch` — search configured registries

### 8.3 Binary Distribution

The Go binary is platform-specific. The VS Code extension bundles prebuilt binaries for:

- darwin-arm64
- darwin-amd64
- linux-amd64
- linux-arm64
- windows-amd64

The extension downloads the correct binary on first activation if not bundled.

---

## 9. Relationship to Compiler Plugins

Editor tooling and compiler plugins are complementary:

| Concern | Compiler Plugin | Editor Extension Plugin |
|---|---|---|
| Runs when | `ai-build compile` | On every keystroke / save |
| Input | Full `.ai/` source tree as IR | Single file or partial tree |
| Output | Build artifacts | Diagnostics, completions, previews |
| Registration | `Stage` interface in Go | VS Code contribution points |
| Language | Go (or external process) | TypeScript (VS Code API) or Go (LSP) |
| Trust model | Allow-listed in manifest | Installed as VS Code extension |

A compiler plugin that adds a custom pipeline stage should be paired with an editor extension plugin that adds validation, completion, and quick fixes for the same custom fields. The `pkg/sdk/` package provides shared types for both.
