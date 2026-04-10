# AI Agent Metadata Control Plane Architecture

## 1. Purpose

This architecture defines a **single source of truth** for AI-agent metadata and behavior, then compiles that source into target-specific artifacts for:

- Cursor
- Claude
- GitHub Copilot+
- Codex
- Adjacent ecosystems when useful, such as Windsurf, Cline, OpenCode, Gemini CLI, Roo, Goose, AMP, and Kiro

The goal is not to force all platforms into one runtime abstraction. The goal is to create a **canonical authoring model** and a **compiler pipeline** that renders native artifacts per target, while preserving hierarchy, scripts, assets, hooks, skills, and agent specialization wherever supported.

---

## 2. Core Architectural Principle

The system must be built as a **compiler pipeline**, not as a shared runtime format.

The canonical model describes platform-agnostic intent. A target renderer then transforms that intent into native files and folder structures.

This approach is required because the target environments do not expose the same primitives:

- some support hierarchical instruction files
- some support native rules
- some support skills with scripts/resources
- some support hooks
- some support subagents or custom agents
- some support only a subset of the above

Therefore, the architecture must:

1. keep one canonical source model
2. compile to native target artifacts
3. preserve hierarchy where supported
4. degrade gracefully where not supported
5. package scripts/assets/resources once and attach them when a target supports them
6. emit build reports for skipped and lowered features

---

## 3. Conceptual Model

The canonical control plane separates the following concepts.

### 3.1 Instructions

Always-on guidance for a repository, directory subtree, or bounded context.

Examples:
- architecture principles
- coding standards
- testing expectations
- review policy
- domain language
- repo workflow

These typically compile to files such as:
- `AGENTS.md`
- `CLAUDE.md`
- target-specific instruction surfaces
- generated instruction sections inside other native files

### 3.2 Rules

Scoped or conditional guidance.

Examples:
- Go rules under `services/**/*.go`
- CDK/TypeScript rules under `infra/**/*.ts`
- policy for generated code
- security rules for IAM-related files

Rules are distinct from instructions. However, when a target does not expose rules as a first-class concept, rules may be lowered into instructions.

### 3.3 Skills

Reusable, on-demand capability bundles.

Examples:
- build a Go AWS Lambda
- validate Terraform/CDK
- review IAM policies
- generate Rego tests

A skill may include:
- instructions
- assets
- templates
- scripts
- resources
- activation hints

Skills are the strongest cross-platform common denominator and should carry most reusable workflows.

### 3.4 Agents

Specialized delegates or subagents.

Examples:
- Go implementer
- AWS architecture reviewer
- security reviewer
- test runner
- documentation refiner

An agent typically contains:
- system prompt / role prompt
- tool permissions
- allowed delegations
- linked skills
- optional agent-specific hooks

Agents are orchestration wrappers around prompts, tools, and linked skills. Their metadata is not portable enough to be treated as a fully universal schema at the emitted layer.

### 3.5 Hooks

Deterministic lifecycle automation executed outside the model’s discretion.

Examples:
- post-edit validation
- pre-run environment setup
- pre-commit checks
- async reporting
- prompt-time transformations

Hooks are the least portable primitive and must be modeled separately, then emitted only when the target supports them.

### 3.6 Commands

Explicit user-invoked workflows, where a target supports that concept.

Examples:
- `/review-iam`
- `/build-lambda`
- `/refactor-ddd-boundary`

Commands can be used as a fallback emission path when hooks are unsupported.

### 3.7 Tools and MCP Bindings

External capabilities made available to an agent runtime.

Examples:
- filesystem tools
- terminal tools
- MCP tool definitions
- code search tools
- repo graph tools
- deployment validators

These are target-specific and should be treated as renderer concerns driven by canonical capability intent.

---

## 4. Canonical Repository Layout

Recommended source-of-truth layout:

```text
.ai/
  manifest.yaml
  instructions/
  rules/
  skills/
  agents/
  hooks/
  commands/
  assets/
  scripts/
  targets/
  schema/
```

### 4.1 Responsibilities

- `manifest.yaml` contains global metadata, compilation defaults, target selection, inheritance rules, and build policy.
- `instructions/` contains always-on policy and guidance by scope.
- `rules/` contains scoped or conditional policy.
- `skills/` contains reusable capability bundles, usually with markdown, scripts, and resources.
- `agents/` contains canonical agent definitions.
- `hooks/` contains global, skill-bound, or agent-bound lifecycle automation.
- `commands/` contains explicit user-triggered workflows.
- `assets/` contains images, templates, prompt fragments, examples, diagrams, and other static resources.
- `scripts/` contains shell, Python, Go, or other scripts referenced by hooks, skills, agents, or commands.
- `targets/` contains target-specific overrides and renderer configuration.
- `schema/` contains versioned JSON Schema or equivalent validation definitions.

---

## 5. Canonical Primitive Schemas

## 5.1 Manifest

The manifest defines build policy and target selection.

Example:

```yaml
schemaVersion: 1
project:
  name: goagentmeta
  monorepo: true

targets:
  cursor:
    enabled: true
  claude:
    enabled: true
  copilot:
    enabled: true
  codex:
    enabled: true

compilation:
  hierarchyMode: preserve-when-supported
  unsupportedMode: skip-with-report
  assetPackaging: symlink-or-copy
  scriptPackaging: preserve-relative-paths

inheritance:
  instructions: nearest-wins-with-parent-merge
  rules: additive
  hooks: merge-by-scope
  skills: global-plus-local
  agents: namespaced
```

## 5.2 Instruction

```yaml
id: repo-core
kind: instruction
scope:
  type: path
  path: /
inheritance:
  extends: []
content:
  markdown: |
    Coding standards, architecture, testing, review policy...
attachments:
  assets: [assets/architecture/clean-arch.png]
  scripts: [scripts/validate.sh]
capabilities:
  mayReferenceSkills: true
  mayReferenceAgents: true
```

## 5.3 Rule

```yaml
id: go-backend-rule
kind: rule
match:
  paths:
    - "services/**/*.go"
  when:
    fileType: go
priority: 80
content:
  markdown: |
    Use Go 1.24+, table-driven tests, context first, AWS SDK v2...
attachments:
  scripts: [scripts/go-lint.sh]
```

## 5.4 Skill

```yaml
id: go-aws-lambda
kind: skill
metadata:
  name: go-aws-lambda
  description: Build and validate Go Lambda services with AWS SDK v2
content:
  markdown: |
    Steps, conventions, validation, examples...
resources:
  files:
    - assets/templates/lambda_main.go.tmpl
scripts:
  - path: scripts/skills/go-aws-lambda/test.sh
  - path: scripts/skills/go-aws-lambda/package.sh
activation:
  hints:
    - lambda
    - aws sdk v2
    - go
```

## 5.5 Agent

```yaml
id: go-implementer
kind: agent
metadata:
  name: Go Implementer
  description: Implements Go backend changes with tests
systemPrompt: |
  Focused on minimal, correct code changes.
tools:
  allow:
    - read
    - edit
    - search
    - terminal
delegation:
  mayCall:
    - aws-reviewer
    - test-runner
attachments:
  skills:
    - go-aws-lambda
  scripts:
    - scripts/agents/go-implementer/prep.sh
```

## 5.6 Hook

```yaml
id: post-edit-validate
kind: hook
event: post-edit
scope:
  paths:
    - "services/**"
action:
  run: scripts/hooks/post-edit-validate.sh
inputs:
  include:
    - changedFiles
    - workingDirectory
policy:
  onFailure: block
  timeoutSeconds: 60
```

---

## 6. Hierarchy and Monorepo Support

Hierarchy must be a first-class architectural concern.

The architecture supports three logical layers.

### 6.1 Global Layer

Applies to the whole repository or workspace.

Examples:
- root instructions
- global security rules
- shared skills
- default hooks

### 6.2 Subtree Layer

Applies to a bounded context, package, service, module, or directory subtree.

Examples:
- `services/energy-optimizer/`
- `packages/rec-model/`
- `infra/cdk/`

These may define:
- local instructions
- local rules
- subtree-specific skills
- subtree-specific agent overrides
- subtree-specific hooks

### 6.3 Target Override Layer

Used only when a given target requires special handling.

Examples:

```text
.ai/
  instructions/
    root.md
    services/
      root.md
      energy-optimizer.md
  targets/
    claude/
      overrides/
    cursor/
      overrides/
```

### 6.4 Merge Order

Recommended merge order:

1. global canonical content
2. subtree canonical content
3. target-specific override
4. renderer-added target glue

This ensures:
- stable inheritance
- path-local specialization
- optional per-target tuning
- deterministic generated output

---

## 7. Capability-Aware Compiler Pipeline

The architecture is centered on a capability-aware compiler.

## 7.1 Pipeline Phases

### Phase 1: Load

Read all canonical YAML/Markdown content and validate structure.

### Phase 2: Normalize

Resolve:
- inheritance
- relative asset/script paths
- scope expansion
- target applicability
- name collisions
- default values

### Phase 3: Capability Resolution

Maintain a target capability registry such as:

```yaml
capabilities:
  claude:
    instructions: true
    rules: partial
    skills: true
    agents: true
    hooks: true
    hierarchy: true
  cursor:
    instructions: partial
    rules: true
    skills: true
    agents: true
    hooks: true
    hierarchy: partial
  copilot:
    instructions: true
    rules: partial
    skills: true
    agents: true
    hooks: limited_or_none
    hierarchy: partial
  codex:
    instructions: true
    rules: partial
    skills: true
    agents: true
    hooks: limited_or_none
    hierarchy: true
```

Capability levels should be classified as:
- native
- lowered
- skipped

### Phase 4: Lowering

Transform unsupported primitives into nearest supported equivalents.

Examples:
- rule -> instruction section
- hook -> command or skill script reference
- unsupported hierarchy -> flatten to nearest supported root
- agent-specific resources -> linked skill resources

### Phase 5: Render

Emit target-native files and folder structures.

### Phase 6: Materialize Assets

Copy or symlink scripts, templates, examples, and other resources into expected locations.

### Phase 7: Report

Generate a build report showing:
- emitted files
- skipped features
- degraded features
- collisions and conflicts
- provenance from source to output

---

## 8. Target Rendering Strategy

The renderers are separate backends driven by the same canonical model.

## 8.1 Claude Renderer

Use:
- `CLAUDE.md` for instructions
- native hooks
- native subagents
- native skills

Renderer behavior:
- preserve distinction between instructions, skills, hooks, and agents
- lower rules into instructions when no closer native representation exists
- package assets/scripts/resources alongside emitted skills where appropriate

## 8.2 Cursor Renderer

Use:
- native rules where possible
- native hooks
- native subagents
- native skills
- target instruction surfaces where needed

Renderer behavior:
- preserve rules as rules whenever supported
- map scoped guidance to Cursor-native modular concepts rather than flatten too early
- attach scripts/resources to skills and hooks where possible

## 8.3 GitHub Copilot Renderer

Use:
- custom agents such as `.agent.md`
- agent tool declarations
- Agent Skills as the reusable workflow layer
- generated instruction surfaces for repo guidance

Renderer behavior:
- keep reusable workflow logic in skills
- render agents as thin wrappers around prompt + tools + linked skills
- lower unsupported hooks into commands or documented scripts when necessary

## 8.4 Codex Renderer

Use:
- layered `AGENTS.md`
- Agent Skills
- subagents/custom agents

Renderer behavior:
- preserve hierarchical instruction layering strongly
- keep skills native
- lower rules into `AGENTS.md` sections when no native rule form exists
- skip or downgrade unsupported hook semantics with clear reporting

---

## 9. Scripts, Assets, and Resources

Scripts, assets, and resources must be canonical and reusable.

## 9.1 Assets

Canonical location:

```text
.ai/assets/
```

Examples:
- diagrams
- templates
- prompt fragments
- markdown partials
- code samples
- example inputs/outputs

## 9.2 Scripts

Canonical location:

```text
.ai/scripts/
```

Examples:
- shell validation scripts
- Go helpers
- Python transforms
- packaging scripts
- policy validation scripts

## 9.3 Packaging Policy

Each renderer decides whether to:
- reference the asset/script
- copy it
- symlink it
- inline it
- skip it with a warning

### Recommended policy

- **Skills** should package scripts/resources alongside `SKILL.md`-style outputs where supported.
- **Hooks** should point directly to scripts for targets that support native hooks.
- **Instructions and rules** should usually reference scripts textually, unless the target offers native automation.

---

## 10. Hook Model

Hooks must be modeled at three scopes.

### 10.1 Global Hooks

Apply repository-wide.

Examples:
- post-edit validation
- prompt sanitization
- environment checks

### 10.2 Agent Hooks

Attached to a specific agent.

Examples:
- pre-run repository prep
- post-edit language-specific validation
- agent-specific telemetry

### 10.3 Skill Hooks

Attached to a specific skill.

Examples:
- setup scripts
- resource generation
- post-skill verification

Recommended layout:

```text
.ai/hooks/
  global/
  agents/
    go-implementer/
  skills/
    go-aws-lambda/
```

If a target does not support a given hook class, the compiler should:
- skip it
- lower it to commands or script references where possible
- record the downgrade in the build report

---

## 11. Build Output Layout

Generated files should not be emitted directly into the repository root during compilation. Use a build tree first.

Recommended build layout:

```text
.ai-build/
  claude/
  cursor/
  copilot/
  codex/
  report.md
```

Then either:
- sync generated files into the working repository
- expose them through symlinks
- commit selected generated files

Examples of final repo-visible outputs may include:

```text
CLAUDE.md
AGENTS.md
.github/
  copilot/
.claude/
  agents/
  skills/
.cursor/
.codex/
```

Each generated file should carry a header such as:

```md
<!-- generated by ai-control-plane; source: .ai/... ; do not edit -->
```

---

## 12. Validation and Linting

The architecture requires strong validation.

### 12.1 Structural Validation

Validate all source artifacts against versioned schemas.

Suggested schemas:

```text
.ai/schema/
  instruction.schema.json
  rule.schema.json
  skill.schema.json
  agent.schema.json
  hook.schema.json
  target-capabilities.schema.json
```

### 12.2 Semantic Validation

Check for:
- duplicate IDs
- duplicate agent names
- circular agent delegation
- missing referenced skills
- missing scripts/assets/resources
- invalid path scopes
- unsupported hook events
- conflicting rule priorities
- conflicting target overrides

### 12.3 Security Validation

Classify executable artifacts by trust level.

Example:

```yaml
security:
  level: trusted | review-required | disabled-by-default
```

This supports safer profile-based rendering.

---

## 13. Environment Profiles

Not every runtime environment should receive the same emitted result.

Recommended profiles:
- `local-dev`
- `ci`
- `enterprise-locked`
- `oss-public`

Profiles can control:
- hook enablement
- script emission
- MCP bindings
- agent availability
- asset exposure
- permission-sensitive defaults

This is necessary because script and tool access vary across environments.

---

## 14. Target Overrides and Escape Hatches

A pure abstraction is insufficient in practice. The architecture must allow target-specific escape hatches.

Example:

```yaml
targetOverrides:
  claude: ...
  cursor: ...
  copilot: ...
  codex: ...
```

Use this only when:
- the canonical abstraction cannot express a target-native behavior
- a platform requires unusual syntax or file placement
- a specific target feature should be enabled or suppressed

The canonical layer remains authoritative. Overrides are exceptions, not the primary authoring model.

---

## 15. Conflict Resolution and Determinism

Generated outputs must be deterministic.

Recommended rules:
- stable file ordering
- stable merge ordering
- nearest-scope precedence for instructions
- additive merge for rules unless overridden by priority
- namespaced agents
- explicit errors for ambiguous conflicts
- explicit warnings for downgraded features

Determinism is critical for:
- CI validation
- reproducible generation
- version-control clarity
- code review

---

## 16. What Belongs Where

The canonical authoring guidance should be:

- **Instructions** = always-on policy and context
- **Rules** = scoped or conditional policy
- **Skills** = reusable workflows with scripts/resources/assets
- **Agents** = orchestration wrappers around prompts, tools, delegations, and linked skills
- **Hooks** = deterministic automation
- **Commands** = explicit user-triggered workflows or fallback automation surfaces

This separation is the most robust abstraction across Claude, Cursor, GitHub Copilot, and Codex.

---

## 17. Recommended Runtime Architecture

Build a project such as:

```text
ai-control-plane
```

It should contain:

1. a canonical DSL in YAML + Markdown
2. a schema validator
3. a normalization engine
4. a target capability registry
5. a lowering engine for unsupported concepts
6. target-specific renderers
7. an asset/script materializer
8. a reporting engine
9. CI lint and validation steps
10. environment/profile support

This gives:
- true single source of truth
- native target rendering
- graceful degradation
- multi-target support in the same repo
- hierarchical rendering where supported
- repeatable generation and reviewability

---

## 18. Additional Considerations

The following should also be included in the architecture.

### 18.1 Provenance Tracking

Every emitted artifact should record:
- source objects used
- merge order
- overrides applied
- lowering decisions

### 18.2 Documentation Generation

The compiler should generate:
- support matrix
- rendered file map
- skip/degradation report
- source-to-output provenance map

### 18.3 Future Target Expansion

The model should be extensible to additional targets such as:
- Windsurf
- Cline
- OpenCode
- Gemini CLI
- Roo
- Goose
- AMP
- Kiro

This is another reason the control plane must be capability-driven, not hardcoded around one vendor.

### 18.4 Testing Strategy

The compiler should include:
- unit tests for normalization and lowering
- golden-file tests for rendered target outputs
- integration tests for build trees
- validation tests for schema evolution

---

## 19. Summary

The correct architecture is a **canonical metadata control plane plus capability-aware renderers**.

It should:
- define instructions, rules, skills, agents, hooks, commands, assets, and scripts separately
- preserve hierarchy where a target supports it
- lower unsupported constructs into nearest equivalents
- package scripts/assets/resources once and attach them per target
- support multiple targets in the same repository
- emit deterministic outputs and explicit reports

This avoids false standardization while still providing one authoritative source model for all supported environments.
