# Build Output, Validation, and Operations

This document covers hierarchy, hooks, output layout, reporting, validation, target strategy, implementation modules, and testing. It continues from the compiler pipeline defined in [compiler.md](compiler.md).

---

## 1. Hierarchy, Monorepo Support, and Overrides

Hierarchy must be a first-class feature because instructions, rules, skills, and hooks often need subtree-local specialization.

### 1.1 Logical Layers

Recommended layers:

1. global repository layer
2. subtree or bounded-context layer
3. object-local specialization
4. profile override
5. target override
6. renderer-added glue

### 1.2 Merge Order

Deterministic merge order should be:

1. global canonical content
2. subtree canonical content
3. explicit object inheritance
4. profile-specific adjustments
5. target-specific override
6. renderer-generated boilerplate

This ensures that source intent remains authoritative and renderer glue is always last.

### 1.3 Monorepo Expectations

The architecture should support:

- repository-wide defaults
- subtree-specific instructions and rules
- local skills for bounded contexts
- subtree-local agents and hooks
- namespaced IDs to avoid collisions

The compiler should resolve nearest-scope precedence while still producing provenance that shows every contributing source object.

### 1.4 Escape Hatch Guardrails

Target overrides are necessary but dangerous.

Allowed use cases:

- target-native file placement
- target-native syntax
- enabling or suppressing optional features
- plugin packaging details
- current vendor quirks

Bad use cases:

- redefining core workflow intent only for one target
- duplicating full canonical content in override files
- using overrides as the primary authoring path

Overrides should stay small and auditable.

---

## 2. Hook Model

Hooks need stronger semantics than "event plus script".

### 2.1 Hook Dimensions

Hooks should declare:

- event
- scope
- action
- effect class
- enforcement mode
- timeout
- retry policy
- failure policy

### 2.2 Effect Classes

Useful effect classes include:

- `observing`
- `validating`
- `transforming`
- `setup`
- `reporting`

### 2.3 Enforcement Modes

Useful enforcement modes include:

- `blocking`
- `advisory`
- `best-effort`

This matters because not all lowerings preserve enforcement.

### 2.4 Lowering Rules for Hooks

- `blocking` hooks must not silently lower to advisory commands when preservation is `required`
- `advisory` hooks may lower to commands or documented scripts
- `setup` hooks may lower to explicit bootstrap commands
- `reporting` hooks may lower to background or manual workflows

If the target has no compatible hook model, the compiler should either fail or downgrade according to the hook's preservation level.

---

## 3. Output Layout and Materialization

Compilation should first produce a build tree, not write directly into the repository root.

Recommended layout:

```text
.ai-build/
  manifest.lock.json
  report.json
  report.md
  support-matrix.md
  provenance/
  claude/
    local-dev/
  cursor/
    local-dev/
  copilot/
    local-dev/
  codex/
    local-dev/
```

### 3.2 Build Unit Contents

### 3.1 Hybrid Build Unit Contents

Each build unit emits two layers (see [compiler.md](compiler.md) Section 2.8).

Example for `claude/local-dev/`:

```text
claude/local-dev/
  # Instruction layer — model-facing text
  CLAUDE.md                          # root instructions + rules
  services/
    CLAUDE.md                        # subtree instructions
  skills/
    go-aws-lambda.md                 # rendered skill
  agents/
    go-implementer.md                # rendered agent

  # Extension layer — runtime configuration
  settings.json                      # mcpServers, permissions

  # Supporting artifacts
  assets/
  scripts/
  provenance.json
```

Example for `copilot/local-dev/`:

```text
copilot/local-dev/
  # Instruction layer
  .github/
    copilot-instructions.md          # root instructions
    instructions/
      go-services.instructions.md    # path-scoped rules (applyTo: frontmatter)
    agents/
      go-implementer.agent.md        # custom agent definition
    skills/
      go-aws-lambda/
        SKILL.md                     # rendered skill (AgentSkills.io format)
    prompts/
      build-lambda.prompt.md         # command → prompt file
  AGENTS.md                          # agent instructions (also read by Copilot)

  # Extension layer
  .vscode/
    mcp.json                         # MCP server references (top-level key: "servers")

  # Supporting artifacts
  assets/
  scripts/
  provenance.json
```

Example for `cursor/local-dev/`:

```text
cursor/local-dev/
  # Instruction layer
  AGENTS.md                          # root instructions (Cursor reads AGENTS.md)
  .cursor/
    rules/
      go-services.mdc                # scoped rules (globs + alwaysApply frontmatter)
      architecture.md                # always-on rules

  # Extension layer
  .cursor/
    mcp.json                         # MCP server references (top-level key: "mcpServers")

  # Supporting artifacts
  assets/
  scripts/
  provenance.json
```

Note: Cursor does not support skills, custom agents, hooks, or plugins natively. Canonical skills and agents are lowered into rules or skipped for Cursor, with lowering decisions reported in provenance.

The instruction layer and extension layer are emitted together by default (hybrid mode). The manifest can disable either layer independently for specialized builds.

### 3.2 Build Unit Contents

- rendered files
- materialized assets
- materialized scripts
- plugin bundles or install manifests
- per-unit provenance and diagnostics

Optional sync modes:

- `build-only`
- `copy`
- `symlink`
- `adopt-selected`

Every generated file should carry a header similar to:

```md
<!-- generated by ai-control-plane; source: .ai/... ; do not edit -->
```

---

## 4. Reporting and Provenance

The reporting model is part of the architecture, not an afterthought.

For every build run, the compiler should report:

- selected targets and profiles
- emitted files
- selected plugins
- capability providers chosen
- lowerings performed
- skipped objects
- failed objects
- warnings
- source-to-output provenance

Suggested machine-readable output:

```yaml
buildUnit: codex/local-dev
object: hook.post-edit-validate
status: lowered
from:
  kind: hook
  id: post-edit-validate
to:
  kind: command
  path: .ai-build/codex/local-dev/commands/validate.md
reason: target lacks lifecycle hooks
preservation: preferred
```

Without this data the user cannot reason about whether "author once" actually held.

---

## 5. Validation and Safety

The compiler should perform four classes of validation.

### 5.1 Structural Validation

Validate source objects against versioned schemas.

Suggested schemas:

```text
.ai/schema/
  manifest.schema.json
  instruction.schema.json
  rule.schema.json
  skill.schema.json
  agent.schema.json
  hook.schema.json
  command.schema.json
  capability.schema.json
  plugin.schema.json
  profile.schema.json
  target-capabilities.schema.json
```

### 5.2 Semantic Validation

Check for:

- duplicate IDs
- duplicate names where target namespaces would collide
- missing referenced objects
- circular agent delegation
- circular inheritance
- invalid scopes
- invalid target overrides
- ambiguous provider selection
- conflicting rules at equal priority
- missing scripts or assets

### 5.3 Security Validation

Validate:

- hook execution policy
- script trust level
- plugin trust and permission declarations
- profile restrictions
- disallowed secret references

### 5.4 Determinism Validation

Guarantee:

- stable traversal order
- stable merge order
- stable output file ordering
- stable provider selection when priorities tie
- stable report ordering

Determinism is necessary for CI and code review.

---

## 6. Target Strategy

The canonical schema should remain stable while target behavior evolves behind renderer backends and target capability registries.

### 6.1 Renderer Responsibilities

Each renderer owns:

- current vendor file and folder conventions
- syntax mapping
- native capability mapping
- plugin packaging rules
- target-specific install manifests
- renderer-added glue text

### 6.2 Canonical Layer Responsibilities

The canonical layer owns:

- meaning of instructions, rules, skills, agents, hooks, commands, capabilities, and plugins
- inheritance and scoping semantics
- preservation policy
- dependency relationships
- profile gating
- reporting requirements

### 6.3 Avoid Hardcoding Vendor Trivia Into the Canonical Schema

The canonical model should avoid fields whose only meaning is "this exact vendor file name as of today". That detail belongs in renderers or target overrides.

This keeps the source language durable as vendor ecosystems change.

---

## 7. Recommended Implementation Modules

A practical implementation should separate code into modules such as:

1. compiler plugin registry and pipeline orchestrator
2. parser (compiler plugin, `parse` phase)
3. schema validator (compiler plugin, `validate` phase)
4. dependency resolver (compiler plugin, `resolve` phase)
5. normalizer (compiler plugin, `normalize` phase)
6. semantic graph builder
7. build graph expander (compiler plugin, `plan` phase)
8. capability resolver (compiler plugin, `capability` phase)
9. lowering engine (compiler plugin, `lower` phase)
10. target renderer backends (compiler plugins, `render` phase — one per target)
11. materializer (compiler plugin, `materialize` phase)
12. report and provenance generator (compiler plugin, `report` phase)
13. compiler plugin SDK (`pkg/sdk/`)
14. linter and validation CLI

Each numbered module except the orchestrator and SDK is a compiler plugin implementing the `Stage` interface. The orchestrator loads, orders, and dispatches plugins through the pipeline.

This structure matches the compiler model and reduces backend duplication.

---

## 8. Testing Strategy

The compiler should be tested like a language toolchain.

Recommended tests:

- unit tests for parsing and normalization
- dependency resolution and lock file tests
- semantic validation tests
- provider resolution tests
- lowering tests
- golden-file tests for target outputs
- fixture-based monorepo hierarchy tests
- profile-gating tests
- plugin packaging and reference emission tests
- deterministic output tests

Golden tests are especially important because the product is emitted artifacts.
