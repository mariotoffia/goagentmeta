# Target Rendering — Emission Map

This document describes how the GoAgentMeta compiler maps canonical `.ai/` objects
to native output files for each target ecosystem.

## Emission Overview

The compiler processes each `.ai/` object kind through a target-specific renderer.
Every renderer produces a different set of native files reflecting the conventions
of that ecosystem.

### Claude Code

| `.ai/` Kind | Output Path(s) | Notes |
|-------------|---------------|-------|
| **Instruction** (root-scoped) | `CLAUDE.md` | All root-scoped instructions merged into one file, sorted by ID |
| **Instruction** (subtree-scoped) | `{dir}/CLAUDE.md` | `scope.paths` with clean directory paths produce subtree files |
| **Rule** | `.claude/rules/{id}.md` | Glob patterns go into YAML frontmatter `applyTo.paths`, not in file path |
| **Skill** | `.claude/skills/{id}/SKILL.md` | Plus sidecar reference/asset files |
| **Agent** | `.claude/agents/{id}.md` | Agent prompt as markdown body |
| **Hook** | `.claude/settings.json` | Scope paths become `matcher` fields in hook entries |
| **Command** | `.claude/skills/{id}/SKILL.md` | Lowered to skill (Claude has no native command concept) |
| **Plugin** (inline) | `.claude-plugin/{id}/` | Inline plugin artifacts |
| **Plugin** (MCP) | `.mcp.json` | MCP server configuration entries |
| **Plugin** (external) | — | Install metadata only (no file emission) |
| **Provenance** | `provenance.json` | Build provenance and lowering records |

### GitHub Copilot

| `.ai/` Kind | Output Path(s) | Notes |
|-------------|---------------|-------|
| **Instruction** (root-scoped) | `AGENTS.md` + `.github/copilot-instructions.md` | Merged into both root files |
| **Instruction** (subtree-scoped) | `{dir}/AGENTS.md` | Subtree-scoped instructions |
| **Rule** | Lowered to instruction section in `AGENTS.md` | Rules don't have a native Copilot concept |
| **Skill** | `.github/skills/{id}/SKILL.md` | Skill as markdown |
| **Agent** | `.github/agents/{id}.agent.md` | Agent definition file |
| **Hook** | `.github/hooks/{id}.json` | Hook as JSON config |
| **Command** | `.github/prompts/{id}.prompt.md` | Commands become prompt files |
| **Plugin** (MCP) | `.vscode/mcp.json` | VS Code MCP configuration |
| **Provenance** | `provenance.json` | Build provenance |

### Cursor

| `.ai/` Kind | Output Path(s) | Notes |
|-------------|---------------|-------|
| **Instruction** (root-scoped) | `AGENTS.md` | Merged root file |
| **Instruction** (subtree-scoped) | `{dir}/AGENTS.md` | Subtree-scoped instructions |
| **Rule** | Lowered to instruction section in `AGENTS.md` | No native rule concept |
| **Skill** | `.cursor/rules/{id}.mdc` | Skills emitted as Cursor rule files (`.mdc` format) |
| **Agent** | `.cursor/rules/{id}.mdc` | Agents emitted as Cursor rule files |
| **Hook** | — | Not supported; omitted with warning |
| **Command** | — | Not supported; omitted |
| **Provenance** | `provenance.json` | Build provenance |

### OpenAI Codex

| `.ai/` Kind | Output Path(s) | Notes |
|-------------|---------------|-------|
| **Instruction** (root-scoped) | `AGENTS.md` | Merged root file |
| **Instruction** (subtree-scoped) | `{dir}/AGENTS.md` | Subtree-scoped instructions |
| **Rule** | Lowered to instruction section in `AGENTS.md` | No native rule concept |
| **Skill** | `.codex/skills/{id}/SKILL.md` | Skill as markdown |
| **Agent** | `.codex/agents/{id}.md` | Agent prompt file |
| **Hook** | `.codex/settings.json` | Hook events in settings (limited support) |
| **Command** | `.codex/skills/{id}/SKILL.md` | Lowered to skill |
| **Plugin** (MCP) | `.mcp.json` | MCP server configuration |
| **Provenance** | `provenance.json` | Build provenance |

## Instruction Scoping Rules

Instructions support two scoping modes that affect file placement:

### Root-scoped (no `scope.paths` or `scope.paths: ["/"]`)

All root-scoped instructions are **merged** into the target's root file (`CLAUDE.md`,
`AGENTS.md`, etc.), sorted alphabetically by ID, and separated by blank lines.

### Subtree-scoped (`scope.paths` with directory paths)

Instructions with directory-path scopes (e.g., `scope.paths: ["services/api"]`) are
emitted as separate files in that subtree. On targets that support hierarchical
instruction files (Claude Code, Copilot), this gives context-specific guidance.

> **Important**: `scope.paths` must contain **directory paths** (e.g., `internal/adapter`),
> not glob patterns (e.g., `**/*.go`). Glob patterns are for **rules**, where they
> appear in YAML frontmatter metadata — not in the filesystem path.

## Rule Scoping

Rules use glob patterns in `scope.paths` / `condition` to define where they apply.
Unlike instructions, these globs are **not** used for file placement. Instead:

- **Claude**: Globs go into `.claude/rules/{id}.md` frontmatter as `applyTo.paths`
- **Copilot/Cursor/Codex**: Rules are lowered to instruction sections in the root file

## Lowering Decisions

When a target doesn't natively support a kind, the compiler **lowers** it to a
supported representation:

| From Kind | To Kind | Targets Affected | Mechanism |
|-----------|---------|-----------------|-----------|
| **Rule** | Instruction section | copilot, cursor, codex | Rule body appended as section in root file |
| **Command** (script-backed) | Skill | claude, codex | Command description becomes skill content |
| **Command** (skill-backed) | Skill | claude, codex | Delegates directly to referenced skill |
| **Hook** | — (omitted) | cursor | Warning emitted; cursor has no hook support |
| **Hook** | Settings entry | codex | Limited hook support via settings.json |

Each lowering decision is recorded in `provenance.json` with the original kind,
target kind, reason, preservation level, and outcome (kept/lowered/skipped).

## Example: GoAgentMeta Dogfood Build

Building the project's own `.ai/` source tree produces:

```
.ai-build/
├── claude/local-dev/
│   ├── CLAUDE.md                              ← 4 instructions merged (238 lines)
│   ├── .claude/
│   │   ├── agents/
│   │   │   ├── architecture-reviewer.md       ← agent prompt
│   │   │   └── pipeline-debugger.md           ← agent prompt
│   │   ├── skills/
│   │   │   ├── build-check/SKILL.md           ← command → skill lowering
│   │   │   ├── canonical-authoring/SKILL.md
│   │   │   ├── compiler-stage-dev/SKILL.md
│   │   │   ├── domain-modeling/SKILL.md
│   │   │   ├── new-stage/SKILL.md             ← command → skill lowering
│   │   │   ├── target-renderer-dev/SKILL.md
│   │   │   └── tool-plugin-dev/SKILL.md
│   │   └── settings.json                      ← hook → settings
│   ├── internal/
│   │   ├── adapter/**/CLAUDE.md               ← adapter-isolation rule (scoped)
│   │   ├── adapter/stage/**/CLAUDE.md         ← pipeline-stage rule (scoped)
│   │   ├── domain/**/CLAUDE.md                ← domain-purity rule (scoped)
│   │   └── port/**/CLAUDE.md                  ← port-contracts rule (scoped)
│   └── provenance.json
│
├── copilot/local-dev/
│   ├── AGENTS.md                              ← 4 instructions merged
│   ├── .github/
│   │   ├── copilot-instructions.md            ← same 4 instructions
│   │   ├── agents/
│   │   │   ├── architecture-reviewer.agent.md
│   │   │   └── pipeline-debugger.agent.md
│   │   ├── hooks/pre-commit.json
│   │   ├── prompts/
│   │   │   ├── build-check.prompt.md          ← command → prompt
│   │   │   └── new-stage.prompt.md            ← command → prompt
│   │   └── skills/
│   │       ├── canonical-authoring/SKILL.md
│   │       ├── compiler-stage-dev/SKILL.md
│   │       ├── domain-modeling/SKILL.md
│   │       ├── target-renderer-dev/SKILL.md
│   │       └── tool-plugin-dev/SKILL.md
│   ├── internal/.../AGENTS.md                 ← rule scoped files
│   └── provenance.json
│
├── cursor/local-dev/
│   ├── AGENTS.md                              ← 4 instructions merged
│   ├── .cursor/rules/
│   │   ├── architecture-reviewer.mdc          ← agent → .mdc
│   │   ├── canonical-authoring.mdc            ← skill → .mdc
│   │   ├── compiler-stage-dev.mdc
│   │   ├── domain-modeling.mdc
│   │   ├── pipeline-debugger.mdc
│   │   ├── target-renderer-dev.mdc
│   │   └── tool-plugin-dev.mdc
│   ├── internal/.../AGENTS.md                 ← rule scoped files
│   └── provenance.json
│
├── codex/local-dev/
│   ├── AGENTS.md                              ← 4 instructions merged
│   ├── .codex/
│   │   ├── agents/
│   │   │   ├── architecture-reviewer.md
│   │   │   └── pipeline-debugger.md
│   │   ├── skills/
│   │   │   ├── build-check/SKILL.md
│   │   │   ├── canonical-authoring/SKILL.md
│   │   │   ├── compiler-stage-dev/SKILL.md
│   │   │   ├── domain-modeling/SKILL.md
│   │   │   ├── new-stage/SKILL.md
│   │   │   ├── target-renderer-dev/SKILL.md
│   │   │   └── tool-plugin-dev/SKILL.md
│   │   └── settings.json
│   └── provenance.json
│
├── report.json
└── report.md
```
