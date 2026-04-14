# goagentmeta

**Write once, deploy to every AI coding agent.**

`goagentmeta` is a Go compiler that transforms a single `.ai/` source tree into
native configurations for Claude Code, Cursor, GitHub Copilot, and Codex CLI.
Each target gets its own idiomatic layout — `CLAUDE.md` + `.claude/skills/` for
Claude, `AGENTS.md` + `.github/copilot-instructions.md` for Copilot, etc. — all
generated from the same canonical source files.

**It even supports to do hierarchical *AGENT.md* or *CLAUDE.md* based on glob expressions**

## Why

Every AI coding assistant uses a different configuration format. Maintaining
separate `CLAUDE.md`, `AGENTS.md`, `.cursorrules`, and Copilot instructions
leads to drift, duplication, and inconsistency. `goagentmeta` treats these
platform differences as a compiler concern:

```
.ai/ source tree  →  goagentmeta build  →  target-native configs
```

- **Instructions, rules, skills, agents, hooks, commands** — authored once
- **Capabilities and plugins** — abstract contracts resolved per-target
- **Lowering engine** — adapts concepts that don't map 1:1 across targets
- **Provenance** — every generated file traces back to its source

## Quick Start

```bash
# Install
go install github.com/mariotoffia/goagentmeta/cmd/goagentmeta@latest

# Build for all targets
goagentmeta build

# Build for a specific target
goagentmeta build -t claude
goagentmeta build -t copilot
```

## The `.ai/` Source Tree

```
.ai/
├── manifest.yaml            # Project config, targets, profiles
├── instructions/            # Always-on guidance (→ CLAUDE.md / copilot-instructions.md)
├── rules/                   # Scoped policies (→ path-scoped CLAUDE.md / AGENTS.md)
├── skills/                  # Reusable workflows (→ .claude/skills/ / .github/skills/)
├── agents/                  # Specialized personas (→ .claude/agents/ / .github/agents/)
├── hooks/                   # Lifecycle automation
├── commands/                # User-invoked workflows (→ skills in Claude, prompts in Copilot)
├── capabilities/            # Abstract contracts (filesystem.read, mcp.github, etc.)
├── plugins/                 # Runtime integrations (MCP servers, tools)
├── references/              # On-demand knowledge docs
├── assets/                  # Templates, examples, diagrams
├── scripts/                 # Executable artifacts for hooks/commands
├── profiles/                # Environment policies (local-dev, ci, enterprise)
└── targets/                 # Per-target rendering & capability config
```

## Build Output

Running `goagentmeta build` produces target-native configurations:

```
.ai-build/
├── claude/local-dev/
│   ├── CLAUDE.md                          # Merged instructions
│   ├── .claude/
│   │   ├── settings.json                  # MCP servers, hooks, permissions
│   │   ├── skills/<id>/SKILL.md           # Skills
│   │   └── agents/<id>.md                 # Agent personas
│   ├── internal/domain/**/CLAUDE.md       # Path-scoped rules
│   └── provenance.json
│
├── copilot/local-dev/
│   ├── AGENTS.md                          # Root agent config
│   ├── .github/
│   │   ├── copilot-instructions.md        # Merged instructions
│   │   ├── skills/<id>/SKILL.md           # Skills
│   │   ├── agents/<id>.agent.md           # Agent personas
│   │   └── prompts/<id>.prompt.md         # Commands → prompt files
│   ├── internal/domain/**/AGENTS.md       # Path-scoped rules
│   └── provenance.json
│
├── cursor/local-dev/...
└── codex/local-dev/...
```

## Example: Simple Instruction + Skill

### 1. Create the source files

```markdown
<!-- .ai/instructions/project.md -->
---
id: project-overview
kind: instruction
---

# My Go Service

A REST API built with Go 1.22. Uses hexagonal architecture.
Run `make test` before committing.
```

```markdown
<!-- .ai/skills/code-review.md -->
---
id: code-review
kind: skill
description: Review Go code for correctness and style
requires:
  - filesystem.read
tools:
  - Read
  - Grep
  - Glob
---

## Code Review Skill

Review changed Go files for:
1. Missing error handling
2. Race conditions in concurrent code
3. Exported functions without doc comments
```

### 2. Build for Claude and Copilot

```bash
goagentmeta build -t claude -t copilot
```

Claude gets `CLAUDE.md` + `.claude/skills/code-review/SKILL.md`.
Copilot gets `copilot-instructions.md` + `.github/skills/code-review/SKILL.md` + `AGENTS.md`.

## Packaging as a Claude Code Plugin

Package your compiled output into a distributable
[Claude Code plugin](https://docs.anthropic.com/en/docs/claude-code/plugins):

```bash
# Build & package as a plugin
goagentmeta package --format plugin --name my-skills \
  --version 1.0.0 --author "Your Team" --license MIT

# Test locally
claude --plugin-dir .ai-build/dist/my-skills
```

This produces a ready-to-distribute plugin directory:

```
.ai-build/dist/my-skills/
├── .claude-plugin/plugin.json       # Plugin manifest
├── skills/<id>/SKILL.md
├── agents/<id>.md
└── hooks/hooks.json
```

### Publishing to a Marketplace

Create a marketplace catalog and push it to a Git repository:

```bash
# Generate marketplace.json referencing a GitHub-hosted plugin
goagentmeta package --format marketplace --name my-skills \
  --marketplace-name my-team-tools --marketplace-owner "My Team" \
  --source github:my-org/my-skills-plugin \
  --category development-workflows

# Push to a Git repo
cd .ai-build/dist && git init && git add . && git commit -m "marketplace"
git remote add origin git@github.com:my-org/my-team-marketplace.git
git push -u origin main
```

Users install plugins from your marketplace in Claude Code:

```
/plugin marketplace add my-org/my-team-marketplace
/plugin install my-skills@my-team-tools
```

Or submit to the [official Claude plugin marketplace](https://claude.ai/settings/plugins/submit)
for public discovery via the `/plugin` → Discover tab.

## Key Concepts

| Concept | Purpose |
|---------|---------|
| **Instruction** | Always-on guidance (architecture, standards, conventions) |
| **Rule** | Scoped/conditional policy (language-specific, path-scoped) |
| **Skill** | Reusable workflow bundle (build, review, scaffold) |
| **Agent** | Specialized persona with tool policies and delegation |
| **Hook** | Deterministic lifecycle automation (post-edit, pre-commit) |
| **Command** | User-invoked entry point (`/review-iam`, `/build-lambda`) |
| **Capability** | Abstract contract (`filesystem.read`, `mcp.github`) |
| **Plugin** | Runtime integration providing capabilities (MCP servers, tools) |

## Architecture

The compiler runs a 10-phase pipeline:

```
Parse → Validate → Resolve → Normalize → Plan → Capability → Lower → Render → Materialize → Report
```

- **Hexagonal architecture**: Domain → Ports → Adapters → Application
- **Lowering engine**: adapts concepts across targets (e.g., commands → skills in Claude, commands → prompt files in Copilot)
- **Provenance**: every output file traces back to its canonical source
- **Extensible**: new targets are added by implementing a renderer — no core changes needed

See [docs/architecture/](docs/architecture/) for the full specification.

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture Overview](docs/architecture/overview.md) | Full compiler architecture spec |
| [Language Reference](docs/language/README.md) | `.ai/` source language and all object types |
| [Language Examples](docs/language/examples/) | 10 worked examples from beginner to advanced |
| [Packaging Architecture](docs/packaging/architecture.md) | Plugin packaging and marketplace CLI reference |
| [Marketplace & Registry](docs/architecture/marketplace.md) | Distribution, dependency resolution, trust |
| [Target Grounding](docs/architecture/target-grounding.md) | How concepts map to each platform |

## License

[MIT](LICENSE)
