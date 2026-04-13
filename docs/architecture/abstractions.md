# Complete Abstractions Reference

This document inventories every abstraction (type, interface, constant set) in the goagentmeta codebase, organized by bounded context. Each entry shows the Go package, type name, and purpose. Use this as the canonical map when writing new stages, renderers, or tool plugins.

> **Sync policy:** This file must stay in sync with code. When adding a new type, add an entry here. When removing one, remove the entry.

---

## 1. Domain: Canonical Object Model

**Package:** `internal/domain/model`

### Object Metadata

| Type | Kind | Purpose |
|---|---|---|
| `Kind` | string enum | Object kind discriminator: `instruction`, `rule`, `skill`, `agent`, `hook`, `command`, `capability`, `plugin` |
| `Preservation` | string enum | Preservation level: `required`, `preferred`, `optional` |
| `Scope` | struct | Addressable domain: `Paths`, `FileTypes`, `Labels` |
| `AppliesTo` | struct | Target/profile filtering: `Targets`, `Profiles` |
| `TargetOverride` | struct | Per-target delta overrides: `Enabled`, `Syntax`, `Placement`, `Extra` |
| `ObjectMeta` | struct | Common envelope shared by all objects: `ID`, `Kind`, `Version`, `Description`, `PackageVersion`, `License`, `Scope`, `AppliesTo`, `Extends`, `Preservation`, `Labels`, `Owner`, `TargetOverrides` |

### Authoring Primitives

| Type | Fields (beyond ObjectMeta) | Purpose |
|---|---|---|
| `Instruction` | `Content` | Always-on guidance text, scoped by path/fileType |
| `Rule` | `Content`, `Conditions []RuleCondition` | Conditional guidance with activation predicates |
| `RuleCondition` | `Type`, `Value` | Condition predicate (language, path-pattern, generated, file-extension, label) |
| `Skill` | `Content`, `Requires`, `Resources`, `ActivationHints`, `UserInvocable`, `DisableModelInvocation`, `Tools`, `DisallowedTools`, `Compatibility`, `BinaryDeps`, `InstallSteps`, `Publishing` | Reusable workflow bundle with tool permissions |
| `SkillResources` | `References`, `Assets`, `Scripts` | Supporting files for a skill |
| `InstallStep` | `Kind`, `Package`, `Bins` | Binary dependency install step |
| `SkillPublishing` | `Author`, `Homepage`, `Emoji` | Marketplace publishing metadata |
| `Agent` | `RolePrompt`, `Skills`, `Requires`, `Tools`, `DisallowedTools`, `Delegation`, `Handoffs`, `Hooks`, `Model` | Specialized orchestration persona |
| `AgentDelegation` | `MayCall` | Agents this agent may delegate to |
| `Handoff` | `Label`, `Agent`, `Prompt`, `AutoSend` | Cross-agent handoff definition |
| `Hook` | `Event`, `Action`, `Effect`, `Inputs`, `Policy` | Lifecycle automation trigger |
| `HookAction` | `Type`, `Ref` | What a hook executes (script, command, http, prompt, agent) |
| `EffectClass` | string enum | Hook effect: `observing`, `validating`, `transforming`, `setup`, `reporting` |
| `EnforcementMode` | string enum | Hook enforcement: `blocking`, `advisory`, `best-effort` |
| `HookEffect` | `Class`, `Enforcement` | Hook's behavioral classification |
| `HookInputs` | `Include` | File patterns the hook receives |
| `HookPolicy` | `Timeout`, `MaxRetries`, `FailurePolicy` | Execution constraints |
| `Command` | `Description`, `Action` | User-invoked workflow |
| `CommandAction` | `Type`, `Ref` | Command action binding (skill, prompt, script, agent) |

### Runtime Delivery Primitives

| Type | Fields | Purpose |
|---|---|---|
| `Reference` | `Path`, `Description` | Demand-loaded knowledge document |
| `Asset` | `Path`, `Description` | Static file for tooling |
| `Script` | `Path`, `Description`, `Interpreter` | Executable artifact for hooks/commands |

---

## 2. Domain: Build Coordinates

**Package:** `internal/domain/build`

| Type | Kind | Purpose |
|---|---|---|
| `Target` | string enum | Compilation target: `claude`, `cursor`, `copilot`, `codex` |
| `Profile` | string enum | Build profile: `local-dev`, `ci`, `enterprise-locked`, `oss-public` |
| `BuildScope` | struct | Subset of source tree: `Paths`, `FileTypes`, `Labels` |
| `BuildUnit` | struct | A `(Target, Profile, Scopes)` triple — one compilation unit |
| `BuildCoordinate` | struct | A `BuildUnit` + `OutputDir` — fully qualified build output location |

**Functions:** `AllTargets() []Target`

---

## 3. Domain: Pipeline IR

**Package:** `internal/domain/pipeline`

### Phases

| Type | Kind | Purpose |
|---|---|---|
| `Phase` | int enum | Pipeline phase ordinal (0–9): Parse, Validate, Resolve, Normalize, Plan, Capability, Lower, Render, Materialize, Report |
| `PhaseCount` | const | Total number of phases (10) |

### Intermediate Representations

Each IR type flows between exactly two pipeline phases:

| IR Type | Phase Transition | Purpose |
|---|---|---|
| `SourceTree` | Parse → Validate | Raw parsed objects from `.ai/` directory |
| `RawObject` | (within SourceTree) | Single parsed object: `Meta`, `SourcePath`, `RawContent`, `RawFields` |
| `SemanticGraph` | Validate → Resolve → Normalize | Validated + inherited objects with scope index |
| `NormalizedObject` | (within SemanticGraph) | Resolved object: `Meta`, `SourcePath`, `Content`, `ResolvedFields` |
| `BuildPlan` | Plan → Capability | List of `BuildPlanUnit` (active objects per build coordinate) |
| `BuildPlanUnit` | (within BuildPlan) | One coordinate's active object set |
| `CapabilityGraph` | Capability → Lower | Resolved capabilities per build unit |
| `UnitCapabilities` | (within CapabilityGraph) | Resolved/unsatisfied capabilities + provider candidates |
| `LoweredGraph` | Lower → Render | Target-adapted objects with lowering decisions |
| `LoweredUnit` | (within LoweredGraph) | Per-coordinate lowered objects |
| `LoweredObject` | (within LoweredUnit) | Single lowered object with decision metadata |
| `LoweringDecision` | (within LoweredObject) | Action/reason/preservation/safety classification |
| `EmissionPlan` | Render → Materialize | Files, assets, scripts, plugins to emit per unit |
| `UnitEmission` | (within EmissionPlan) | Per-coordinate emission: files, dirs, assets, scripts, plugin bundles, install metadata |
| `EmissionLayer` | string enum | File layer: `instruction` (model-facing) or `extension` (runtime config) |
| `EmittedFile` | (within UnitEmission) | File to write: path, content, layer, source objects |
| `EmittedAsset` | (within UnitEmission) | Asset to copy: source→dest paths |
| `EmittedScript` | (within UnitEmission) | Script to copy: source→dest paths |
| `EmittedPlugin` | (within UnitEmission) | Plugin bundle to emit |
| `InstallEntry` | (within UnitEmission) | Plugin install metadata |
| `MaterializationResult` | Materialize → Report | Written files, created dirs, symlinks, errors |
| `MaterializationError` | (within MaterializationResult) | Path + error string |
| `BuildReport` | Report → (output) | Final report: timestamp, duration, per-unit reports, diagnostics |
| `UnitReport` | (within BuildReport) | Per-coordinate: emitted files, lowering records, providers, skipped, warnings |
| `LoweringRecord` | (within UnitReport) | Single lowering decision record for provenance |

### Stage Infrastructure

| Type | Kind | Purpose |
|---|---|---|
| `StageDescriptor` | struct | Stage metadata: `Name`, `Phase`, `Order`, `Before`, `After`, `TargetFilter` |
| `HookPoint` | string enum | Hook attachment point: `before-phase`, `after-phase`, `transform` |
| `StageHook` | struct | Named hook bound to a phase and point |
| `StageHookFunc` | func | Hook handler signature: `func(ctx, ir any) (any, error)` |
| `Diagnostic` | struct | Compiler diagnostic: `Severity`, `Code`, `Message`, `SourcePath`, `ObjectID`, `Phase` |

### Error Types

| Type | Kind | Purpose |
|---|---|---|
| `ErrorCode` | string enum | 11 error codes: PARSE, VALIDATION, RESOLUTION, NORMALIZATION, PLANNING, CAPABILITY, LOWERING, RENDERING, MATERIALIZATION, REPORTING, PIPELINE |
| `CompilerError` | struct | Structured compiler error: `Code`, `Message`, `Context`, `Wrapped` |

---

## 4. Domain: Capability

**Package:** `internal/domain/capability`

| Type | Kind | Purpose |
|---|---|---|
| `Capability` | struct | Abstract capability contract: `ID`, `Contract`, `Security` |
| `Contract` | struct | Capability contract: `Category`, `Description` |
| `CapabilitySecurity` | struct | Required access: `Network`, `Filesystem` |
| `SupportLevel` | string enum | Target support: `native`, `adapted`, `lowered`, `emulated`, `skipped` |
| `CapabilityRegistry` | struct | Per-target capability surface map: `Target`, `Surfaces map[string]SupportLevel` |
| `Provider` | struct | Capability provider: `ID`, `Type`, `Capabilities` |
| `ProviderCandidate` | struct | Candidate provider for resolution: `Provider`, `Priority`, `Compatible`, `Reason` |

---

## 5. Domain: Plugin

**Package:** `internal/domain/plugin`

| Type | Kind | Purpose |
|---|---|---|
| `Plugin` | struct | Deployable extension package: `ObjectMeta`, `Distribution`, `Provides`, `Security`, `Selection`, `Install`, `Artifacts` |
| `Distribution` | struct | How plugin is distributed: `Mode`, `Ref`, `Version` |
| `DistributionMode` | string enum | Distribution: `inline`, `external`, `registry` |
| `SelectionMode` | string enum | Selection policy: `auto-if-selected`, `opt-in`, `manual-only`, `disabled` |
| `InstallStrategy` | string enum | Install behavior: `materialize`, `reference-only`, `auto-detect` |
| `PluginSecurity` | struct | Trust + permissions |
| `PluginPermissions` | struct | `Filesystem`, `Network`, `ProcessExec`, `Secrets` |
| `PluginArtifacts` | struct | Bundled files: `Scripts`, `Configs`, `Manifests` |

---

## 6. Domain: Tool

**Package:** `internal/domain/tool`

| Type | Kind | Purpose |
|---|---|---|
| `Plugin` | interface | Tool plugin contract: `Keyword()`, `Description()`, `HasSyntax()`, `SyntaxHelp()`, `Targets()`, `RelatedCapabilities()`, `Validate(expr)` |
| `Expression` | struct | Parsed tool expression: `Raw`, `Keyword`, `Args` |
| `Registry` | struct | Thread-safe tool plugin registry with capability cross-referencing |
| `UnknownToolError` | struct | Error for unrecognized tool keyword |
| `ValidationError` | struct | Error for invalid tool expression syntax |
| `TargetUnavailableError` | struct | Error when tool is not available for a specific target |

**Functions:** `ParseExpression(raw) Expression`, `NewRegistry() *Registry`

---

## 7. Ports (Interfaces)

### Stage Port

**Package:** `internal/port/stage`

| Type | Kind | Purpose |
|---|---|---|
| `Stage` | interface | Core pipeline stage: `Descriptor() StageDescriptor`, `Execute(ctx, input) (any, error)` |
| `StageHookHandler` | interface | Stage hook provider: `Hook() StageHook` |
| `StageFactory` | func type | Stage constructor: `func() (Stage, error)` |
| `StageValidator` | interface | Optional stage self-validation: `Validate() error` |

### Renderer Port

**Package:** `internal/port/renderer`

| Type | Kind | Purpose |
|---|---|---|
| `Renderer` | interface | Target renderer: extends `Stage` with `Target() build.Target`, `SupportedCapabilities() CapabilityRegistry` |

### Filesystem Port

**Package:** `internal/port/filesystem`

| Type | Kind | Purpose |
|---|---|---|
| `Reader` | interface | Read operations: `ReadFile`, `ReadDir`, `Stat`, `Glob` |
| `Writer` | interface | Write operations: `WriteFile`, `MkdirAll`, `Symlink`, `Remove` |
| `Materializer` | interface | Emit an `EmissionPlan` to the filesystem |

### Registry Port

**Package:** `internal/port/registry`

| Type | Kind | Purpose |
|---|---|---|
| `PackageResolver` | interface | Resolve package name + constraint → `ResolvedPackage` |
| `PackageFetcher` | interface | Fetch resolved package → `PackageContents` |
| `PackageSearcher` | interface | Search registry by query → `[]PackageMetadata` |
| `IntegrityVerifier` | interface | Verify package integrity hash |
| `VersionConstraint` | struct | Raw version constraint string |
| `ResolvedPackage` | struct | Resolved package: `Name`, `Version`, `Registry`, `IntegrityHash`, `Publisher` |
| `PackageContents` | struct | Fetched package: `Package`, `RootDir` |
| `PackageMetadata` | struct | Package search result metadata |

### Reporter Port

**Package:** `internal/port/reporter`

| Type | Kind | Purpose |
|---|---|---|
| `Reporter` | interface | Lowering/skip/failure event reporting |
| `DiagnosticSink` | interface | Compiler diagnostic collector: `Emit`, `Diagnostics` |
| `ProvenanceRecorder` | interface | Source→output provenance tracking: `Record` |
| `BuildReportWriter` | interface | Serialize `BuildReport` to output format |

---

## 8. Application Services

### Compiler Pipeline

**Package:** `internal/application/compiler`

| Type | Kind | Purpose |
|---|---|---|
| `Pipeline` | struct | Main compiler orchestrator: `Execute(ctx, rootPaths) (*BuildReport, error)` |
| `CompilerContext` | struct | Context carried through the pipeline: `Config`, `Report` |
| `PipelineConfig` | struct | All pipeline dependencies: registry, reporters, filesystem, targets, profile, failFast |
| `StageRegistry` | struct | Stage + hook registration with topological sorting (Kahn's algorithm) |
| `Option` | func type | Functional option for `NewPipeline()` |

**Options:** `WithRegistry`, `WithStage`, `WithHook`, `WithReporter`, `WithDiagnosticSink`, `WithProvenanceRecorder`, `WithReportWriter`, `WithFSReader`, `WithFSWriter`, `WithMaterializer`, `WithFailFast`, `WithProfile`, `WithTargets`

### Dependency Resolution

**Package:** `internal/application/dependency`

| Type | Kind | Purpose |
|---|---|---|
| `DependencyResolver` | struct | Resolves manifest dependencies against registries |
| `Cache` | interface | Package contents cache: `Get`, `Put` |
| `VersionSatisfier` | func type | Version constraint check function |
| `Manifest` | struct | Parsed `manifest.yaml`: `Dependencies`, `Registries` |
| `ManifestDependency` | struct | Declared dependency: `Name`, `Version`, `Registry` |
| `RegistryConfig` | struct | Registry connection: `Name`, `Type`, `URL`, `Auth` |
| `LockFile` | struct | Reproducible resolution: `SchemaVersion`, `Dependencies []LockEntry` |
| `LockEntry` | struct | Locked dependency: `Name`, `Version`, `Registry`, `Digest`, `ResolvedAt` |
| `ResolutionError` | struct | Dependency resolution failure |

---

## 9. Adapters

### Filesystem Adapters

**Package:** `internal/adapter/filesystem`

| Type | Purpose |
|---|---|
| `OSReader` | OS-backed `filesystem.Reader` |
| `OSWriter` | OS-backed `filesystem.Writer` |
| `OSMaterializer` | OS-backed `filesystem.Materializer` |
| `MemFS` | In-memory `Reader` + `Writer` + `Materializer` (testing) |

### Registry Adapters

**Package:** `internal/adapter/registry`

| Type | Purpose |
|---|---|
| `LocalRegistry` | Local filesystem `PackageResolver` + `PackageFetcher` |
| `HTTPRegistry` | HTTP REST API `PackageResolver` + `PackageFetcher` + `PackageSearcher` |
| `GitRegistry` | Git-based `PackageResolver` + `PackageFetcher` |
| `SHA256Verifier` | SHA256-based `IntegrityVerifier` |
| `DiskCache` | Local disk `dependency.Cache` |
| `Version` | Parsed semver: `Major`, `Minor`, `Patch` |

### Reporter Adapters

**Package:** `internal/adapter/reporter`

| Type | Purpose |
|---|---|
| `Reporter` | Thread-safe `reporter.Reporter` implementation |
| `DiagnosticSink` | Thread-safe, sorted `reporter.DiagnosticSink` |
| `ProvenanceRecorder` | Thread-safe `reporter.ProvenanceRecorder` |
| `JSONReportWriter` | `BuildReportWriter` → JSON format |
| `MarkdownReportWriter` | `BuildReportWriter` → Markdown format |
| `SupportMatrixWriter` | Generates capability support matrix |

### Renderer Adapters

**Package:** `internal/adapter/renderer/{claude,copilot,codex}`

| Renderer | Target | Output |
|---|---|---|
| `claude.Renderer` | `TargetClaude` | CLAUDE.md, .mdc rules, SKILL.md, claude_code_config.json |
| `copilot.Renderer` | `TargetCopilot` | copilot-instructions.md, .prompt.md, SKILL.md, .agents.md |
| `codex.Renderer` | `TargetCodex` | AGENTS.md, agents.yaml |

> **Note:** A Cursor renderer (`TargetCursor`) is defined in the domain but **not yet implemented** as an adapter. Cursor support is currently planned via lowering to existing formats.

### Tool Plugin Adapters

**Package:** `internal/adapter/tool`

| Type | Purpose |
|---|---|
| `NewDefaultRegistry()` | Creates a `tool.Registry` pre-loaded with all built-in plugins and capability IDs |
| `AllBuiltins()` | Returns all 27 built-in tool plugins |

Built-in tool plugins (27):

| Category | Plugins |
|---|---|
| Filesystem | `Read`, `Write`, `Edit`, `MultiEdit` |
| Search | `Glob`, `Grep`, `Search` |
| Terminal | `Bash` (with `Bash(cmd:glob)` syntax), `Terminal` |
| Web | `WebFetch`, `WebSearch` |
| Agent | `Agent`, `AskUser`, `AskUserQuestion` |
| Task | `Task`, `TodoRead`, `TodoWrite` |
| Version Control | `GitCommit`, `GitDiff`, `GitLog`, `GitStatus` |
| Code Intelligence | `LSP`, `Symbols`, `References`, `Definition` |
| Notebook | `Notebook`, `REPL` |
| MCP | `mcp__*__*` (with `mcp__server__tool` syntax) |

### Stage Adapters

**Package:** `internal/adapter/stage/{subpackage}`

| Subpackage | Stage Name | Phase | Purpose |
|---|---|---|---|
| `parser` | `parse` | PhaseParse | Reads `.md` frontmatter + `.yaml` files → `SourceTree` |
| `validator` | `validate` | PhaseValidate | Structural + semantic validation with tool expression checks |
| `resolver` | `resolve` | PhaseResolve | Dependency + reference resolution |
| `normalizer` | `normalize` | PhaseNormalize | Inheritance flattening + field merging |
| `planner` | `plan` | PhasePlan | Build plan generation from active objects |
| `capability` | `capability` | PhaseCapability | Capability resolution + provider selection |
| `lowering` | `lower` | PhaseLower | Target-specific IR lowering with safe/unsafe classification |
| `materializer` | `materialize` | PhaseMaterialize | Write emission plan to filesystem |
| `reporter` | `report` | PhaseReport | Build report generation |

### CLI Adapter

**Package:** `internal/adapter/cli`

| Command | Purpose |
|---|---|
| `build` | Full pipeline execution |
| `validate` | Parse + validate only |
| `init` | Scaffold a new `.ai/` directory |
| `targets` | List supported targets |
| `version` | Print version |

---

## 10. Public SDK

**Package:** `pkg/sdk`

Re-exports for third-party plugin authors:

| Category | Exports |
|---|---|
| Phase constants | `PhaseParse` through `PhaseReport` (10 phases) |
| IR types | `SourceTree`, `SemanticGraph`, `BuildPlan`, `CapabilityGraph`, `LoweredGraph`, `EmissionPlan`, `MaterializationResult`, `BuildReport` |
| Supporting types | `Phase`, `StageDescriptor`, `Target`, `Diagnostic`, `CompilerError` |
| Factory functions | `NewDescriptor(name, phase, order)`, `NewDescriptorWithTarget(name, phase, order, target)` |

---

## Abstractions Not Yet Implemented

The following are designed in documentation but have no production code:

| Abstraction | Documented In | Status |
|---|---|---|
| Cursor renderer | target-grounding.md | Domain `TargetCursor` exists; no adapter renderer |
| VS Code extension / LSP server | editor-tooling.md | Entirely future specification |
| Platform-native packaging (VS Code .vsix, npm, OCI) | marketplace.md | Specification only |
| Go plugin (`.so`) loader | ARCHITECTURE.md, compiler.md | Registration modes described; no loader implemented |
| External process stage adapter | ARCHITECTURE.md, compiler.md | Protocol described; no adapter implemented |
| `application/packaging` package | (historical reference) | Never existed; replaced by dependency resolution |

---

## See Also

- [ARCHITECTURE.md](../../ARCHITECTURE.md) — High-level architecture and design principles
- [docs/architecture/overview.md](overview.md) — Master architecture specification
- [docs/architecture/compiler.md](compiler.md) — Pipeline and plugin architecture details
- [docs/language/README.md](../language/README.md) — Language reference and syntax docs
