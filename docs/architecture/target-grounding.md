# Target Platform Grounding

This document maps the canonical object model from [overview.md](overview.md) to what each target platform actually supports, based on current platform documentation. It serves as the authoritative reference for renderer implementations and the capability registry.

---

## 1. Platform Overview

| Platform | Instruction File | Rules Mechanism | Skills | Agents | Hooks | Plugins | MCP Config | Prompt Files |
|---|---|---|---|---|---|---|---|---|
| **Claude Code** | `CLAUDE.md` (hierarchical) | `.claude/rules/*.md` (`paths:` frontmatter) | `.claude/skills/` (SKILL.md) | `.claude/agents/*.md` | 24+ events, 4 types | `.claude-plugin/` (marketplace) | `.mcp.json` (`mcpServers`) | — (skills serve this role) |
| **Cursor** | `AGENTS.md` (root + subdirs) | `.cursor/rules/*.md\|.mdc` (globs, alwaysApply, description) | — | — (single built-in Agent) | — | — | `.cursor/mcp.json` (`mcpServers`) | — |
| **Copilot (VS Code)** | `.github/copilot-instructions.md` + `AGENTS.md` | `.github/instructions/*.instructions.md` (`applyTo:` globs) | `.github/skills/` (SKILL.md) | `.github/agents/*.agent.md` | 8 events, command type | agent plugins (marketplace) | `.vscode/mcp.json` (`servers`) | `.github/prompts/*.prompt.md` |
| **Codex CLI** | `AGENTS.md` (root + subdirs) | Rules | `.codex/skills/` (SKILL.md) | Custom agents | Hooks (supported) | Plugins (marketplace) | MCP (supported) | — |

---

## 2. Instructions

### 2.1 Claude Code

- **Primary file**: `CLAUDE.md` at repository root, then per-directory `CLAUDE.md` files
- **Personal**: `CLAUDE.local.md` (gitignored)
- **User-level**: `~/.claude/CLAUDE.md`
- **Managed/Enterprise**: `/Library/Application Support/ClaudeCode/CLAUDE.md` (macOS)
- **Behavior**: hierarchical, directory-walking. All `CLAUDE.md` files from root to CWD are concatenated
- **Also reads**: `AGENTS.md` (for compatibility)

### 2.2 Cursor

- **Primary file**: `AGENTS.md` at project root and subdirectories
- **User Rules**: global in Cursor Settings → Rules (applied to all projects)
- **Team Rules**: dashboard-managed, enforced organization-wide (Team/Enterprise plans)
- **Precedence**: Team Rules → Project Rules → User Rules
- **Legacy**: `.cursorrules` is deprecated; replaced by `.cursor/rules/`

### 2.3 GitHub Copilot (VS Code)

- **Primary file**: `.github/copilot-instructions.md` (always-on, repository-wide)
- **Path-scoped**: `.github/instructions/*.instructions.md` (frontmatter: `applyTo: glob`, `excludeAgent: "code-review"|"cloud-agent"`)
- **Agent instructions**: `AGENTS.md` anywhere in tree (nearest takes precedence)
- **Also reads**: `CLAUDE.md`, `GEMINI.md` at repository root
- **Priority**: Personal > Repository > Organization
- **Parent repo discovery**: `chat.useCustomizationsInParentRepositories` for monorepos

### 2.4 Codex CLI

- **Primary file**: `AGENTS.md` at project root and subdirectories
- **Rules**: supported as a separate concept

### 2.5 Renderer Implications

| Canonical Concept | Claude Code | Cursor | Copilot | Codex |
|---|---|---|---|---|
| Root instructions | → `CLAUDE.md` | → `AGENTS.md` | → `.github/copilot-instructions.md` | → `AGENTS.md` |
| Subtree instructions | → subtree `CLAUDE.md` | → subtree `AGENTS.md` | → subtree `AGENTS.md` | → subtree `AGENTS.md` |
| User-level instructions | → `~/.claude/CLAUDE.md` | → User Rules setting | — | — |

---

## 3. Rules

### 3.1 Scoping Mechanisms

| Platform | File Format | Scope Mechanism | Application Modes |
|---|---|---|---|
| **Claude Code** | `.claude/rules/*.md` | `paths:` frontmatter (glob list) | All matching paths |
| **Cursor** | `.cursor/rules/*.md\|.mdc` | `globs:` frontmatter | Always Apply, Apply Intelligently (description-based), Apply to Specific Files (globs), Apply Manually (@mention) |
| **Copilot** | `.github/instructions/*.instructions.md` | `applyTo:` frontmatter (glob) | Auto (when files match) |
| **Codex** | Rules | — | — |

### 3.2 Renderer Implications

- Rules with `preservation: required` targeting Cursor MUST be lowered to `.cursor/rules/*.mdc` with appropriate `globs:` and `alwaysApply:` frontmatter
- Claude Code rules use `paths:` (array), Cursor uses `globs:` (string or array)
- Copilot path-scoped instructions use `applyTo:` (comma-separated globs)
- Cursor's "Apply Intelligently" mode has no direct equivalent in Claude Code or Copilot — the renderer must choose between `alwaysApply: true` (over-apply) or converting to path-based scoping (under-apply)

---

## 4. Skills

### 4.1 Format

All platforms that support skills follow the [AgentSkills.io](https://agentskills.io/) open standard:

| Platform | Skill Location | Standard |
|---|---|---|
| **Claude Code** | `.claude/skills/<name>/SKILL.md` | AgentSkills.io |
| **Copilot** | `.github/skills/<name>/SKILL.md` or `.claude/skills/` or `.agents/skills/` | AgentSkills.io |
| **Codex** | `.codex/skills/<name>/SKILL.md` | AgentSkills.io |
| **Cursor** | — (not supported) | — |

### 4.2 Common Frontmatter

```yaml
name: skill-name              # required, must match directory name
description: what it does      # required
argument-hint: hint text       # optional
user-invocable: true           # optional (slash command visibility)
disable-model-invocation: false # optional (auto-load control)
```

Claude Code adds: `allowed-tools`, `model`, `effort`, `context` (fork), `agent`, `hooks`, `paths`, `shell`.

### 4.3 Supporting Files

Skills may contain supporting files (templates, scripts, examples, references) alongside `SKILL.md`. These are loaded on-demand when the skill instructions reference them via Markdown links.

### 4.4 Renderer Implications

- **Cursor**: skills must be lowered. Options: (a) inline skill content into a `.cursor/rules/*.mdc` file, (b) emit as a plain referenced markdown file, or (c) skip with reporting
- **Loading model**: all platforms use 3-level loading (discovery → instructions → resource access). This is an architectural constraint from the standard, not a target-specific feature

---

## 5. Agents

### 5.1 Format Comparison

| Platform | File Format | Key Properties |
|---|---|---|
| **Claude Code** | `.claude/agents/*.md` | name, description, tools, disallowedTools, model, permissionMode, maxTurns, skills, mcpServers, hooks, memory, background, effort, isolation |
| **Copilot** | `.github/agents/*.agent.md` | description, name, tools, agents (subagent list), model, user-invocable, disable-model-invocation, target, mcp-servers, handoffs, hooks |
| **Codex** | Custom agents | Subagents, custom toolsets |
| **Cursor** | — (not supported) | — |

### 5.2 Cross-Platform Agent Features

| Feature | Claude Code | Copilot | Codex | Cursor |
|---|---|---|---|---|
| Subagent delegation | `mayCall` list | `agents` list | Subagents | — |
| Tool restriction | `tools`, `disallowedTools` | `tools` list | Yes | — |
| Model selection | `model` | `model` (single or priority list) | Yes | — |
| Scoped hooks | In frontmatter | `hooks` in frontmatter (preview) | — | — |
| Handoffs | — | `handoffs` (label, agent, prompt, send, model) | — | — |
| Skills preloading | `skills` | — | — | — |
| Scoped MCP | `mcpServers` | `mcp-servers` | — | — |

### 5.3 Handoffs (Copilot-Specific)

Copilot agents support **handoffs**: guided sequential workflows where one agent suggests transitioning to another with a pre-filled prompt. This enables multi-step workflows (e.g., Plan → Implement → Review).

The canonical model should represent handoffs as an optional agent property that renderers emit only where supported (currently Copilot only).

### 5.4 Renderer Implications

- **Cursor**: agents cannot be emitted. Agent definitions must be lowered to rules or skipped
- **Copilot reads Claude format**: `.claude/agents/*.md` files are auto-detected alongside `.github/agents/*.agent.md`. Claude tool names are mapped to Copilot tools
- **Agent file extension**: Claude uses `.md`, Copilot uses `.agent.md` (but also detects `.md` in `.github/agents/`)

---

## 6. Hooks

### 6.1 Lifecycle Event Comparison

| Event | Claude Code | Copilot | Codex | Cursor |
|---|---|---|---|---|
| SessionStart | ✓ | ✓ | ✓ | — |
| UserPromptSubmit | ✓ | ✓ | ✓ | — |
| PreToolUse | ✓ | ✓ | ✓ | — |
| PostToolUse | ✓ | ✓ | ✓ | — |
| Stop | ✓ | ✓ | ✓ | — |
| SubagentStart | ✓ | ✓ | — | — |
| SubagentStop | ✓ | ✓ | — | — |
| PreCompact | ✓ | ✓ | — | — |
| SessionEnd | ✓ | — | — | — |
| PermissionRequest | ✓ | — | — | — |
| CwdChanged | ✓ | — | — | — |
| FileChanged | ✓ | — | — | — |
| TaskCreated | ✓ | — | — | — |
| TaskCompleted | ✓ | — | — | — |

### 6.2 Hook Configuration Formats

| Platform | Config Location | Format |
|---|---|---|
| **Claude Code** | `.claude/settings.json` | `hooks.EventName[].{type, command, matcher}` |
| **Copilot** | `.github/hooks/*.json` | `hooks.EventName[].{type, command}` |
| **Copilot (compat)** | `.claude/settings.json` | Reads Claude format, maps tool names |
| **Codex** | Hooks config | Similar to Copilot CLI format |
| **Cursor** | — | Not supported |

### 6.3 Hook Types

- Claude Code: `command`, `http`, `prompt`, `agent`
- Copilot: `command` only
- Codex: `command` (bash/powershell variants)

### 6.4 Renderer Implications

- **Cursor**: hooks must be omitted entirely. If `preservation: required`, the build must fail
- **Copilot reads Claude hook format**: renderers may emit Claude-format hooks and Copilot will consume them. However, matcher values are ignored by Copilot (hooks run on all matching events)
- **Hook type lowering**: `http`, `prompt`, and `agent` hook types (Claude-only) must be lowered to `command` or skipped for other targets

---

## 7. Plugins

### 7.1 Format Convergence

Claude Code and Copilot share the same plugin format:

```text
my-plugin/
  plugin.json              # Manifest
  skills/                  # Bundled skills
  agents/                  # Bundled agents  
  hooks/hooks.json         # Bundled hooks
  .mcp.json                # Bundled MCP servers
  scripts/                 # Hook scripts
```

- Copilot explicitly references Claude Code's plugin marketplace (`anthropics/claude-code`)
- Both use `${CLAUDE_PLUGIN_ROOT}` for path references within plugin scripts
- Copilot auto-detects plugin format (Claude vs Copilot layout)

### 7.2 Marketplace Sources

| Platform | Default Marketplaces | Custom Marketplaces |
|---|---|---|
| **Claude Code** | Claude plugin marketplace system | Git repos, npm, local paths |
| **Copilot** | `github/copilot-plugins`, `github/awesome-copilot`, `anthropics/claude-code` | Via `chat.plugins.marketplaces` setting |
| **Codex** | Plugin marketplace | — |
| **Cursor** | — (not supported) | — |

### 7.3 Renderer Implications

- **Cursor**: plugins cannot be emitted. Plugin capabilities must be lowered to MCP server references if the plugin provides MCP, or skipped
- **Format compatibility**: renderers for Claude Code, Copilot, and Codex can share plugin packaging logic

---

## 8. MCP Configuration

### 8.1 Config Differences

| Platform | File Location | Top-Level Key | Transports |
|---|---|---|---|
| **Claude Code** | `.mcp.json` (project), `~/.claude.json` (user) | `mcpServers` | stdio, http, sse, ws |
| **Cursor** | `.cursor/mcp.json` (project), `~/.cursor/mcp.json` (global) | `mcpServers` | stdio, SSE, Streamable HTTP |
| **Copilot** | `.vscode/mcp.json` (workspace), user profile | `servers` | stdio, http |
| **Codex** | MCP config | — | — |

### 8.2 Renderer Implications

- **Critical**: Copilot uses `"servers"` as the top-level key, while Claude Code and Cursor use `"mcpServers"`. Renderers must emit the correct key for each target
- **Variable interpolation**: Cursor supports `${env:NAME}`, `${workspaceFolder}`, etc. Copilot supports similar VS Code variable syntax. Claude Code uses environment variables directly
- **Sandbox support**: Copilot supports MCP sandbox restrictions (macOS/Linux). This is not available on other platforms

---

## 9. Prompt Files (Copilot-Specific)

Copilot supports `.prompt.md` files in `.github/prompts/` as lightweight slash commands. These map to the canonical "command" concept.

```yaml
# Frontmatter
description: what this prompt does
name: slash-command-name
agent: ask | agent | plan | custom-agent-name
model: model-name
tools: [tool-list]
```

### 9.1 Renderer Implications

- Canonical "commands" should emit as `.prompt.md` files for Copilot
- For Claude Code, commands are lowered into skills (Claude merged commands into skills)
- For Cursor, commands have no native equivalent — lower to rules or skip
- For Codex, map to skill invocation

---

## 10. Organization and Team Configuration

Enterprise and team-level configuration is a cross-cutting concern:

| Platform | Mechanism | Scope |
|---|---|---|
| **Claude Code** | Managed `CLAUDE.md`, managed settings, managed MCP | Enterprise-wide |
| **Cursor** | Team Rules (dashboard), enforced/optional | Team/Enterprise |
| **Copilot** | Organization-level agents, enterprise MCP policies | GitHub org |
| **Codex** | — | — |

The canonical model handles this through **profiles** (e.g., `enterprise-locked`). Renderers must translate profile constraints into the target's native organizational configuration mechanism.

---

## 11. Notation for Capability Registry

Based on the above grounding, the capability registry should reflect these verified support levels:

```yaml
targets:
  claude:
    instructions:
      layeredFiles: native           # CLAUDE.md hierarchy
      scopedSections: native         # subtree CLAUDE.md
    rules:
      scopedRules: native            # .claude/rules/*.md with paths:
    skills:
      bundles: native                # .claude/skills/ SKILL.md
      supportingFiles: native        # on-demand file loading
    agents:
      subagents: native              # .claude/agents/*.md
      toolPolicies: native           # tools, disallowedTools
      handoffs: skipped              # not supported
    hooks:
      lifecycle: native              # 24+ events, 4 types
      blockingValidation: native     # PreToolUse deny
    commands:
      explicitEntryPoints: lowered   # merged into skills
    plugins:
      installablePackages: native    # .claude-plugin marketplace
      capabilityProviders: native
    mcp:
      serverBindings: native         # .mcp.json mcpServers key

  cursor:
    instructions:
      layeredFiles: adapted          # AGENTS.md (not native instruction format)
      scopedSections: native         # subdirectory AGENTS.md
    rules:
      scopedRules: native            # .cursor/rules/*.mdc with globs
    skills:
      bundles: skipped               # not supported
      supportingFiles: skipped
    agents:
      subagents: skipped             # single built-in agent
      toolPolicies: skipped
      handoffs: skipped
    hooks:
      lifecycle: skipped             # not supported
      blockingValidation: skipped
    commands:
      explicitEntryPoints: skipped   # no native equivalent
    plugins:
      installablePackages: skipped   # not supported
      capabilityProviders: skipped
    mcp:
      serverBindings: native         # .cursor/mcp.json mcpServers key

  copilot:
    instructions:
      layeredFiles: native           # copilot-instructions.md + AGENTS.md
      scopedSections: native         # .instructions.md with applyTo
    rules:
      scopedRules: native            # .instructions.md with applyTo globs
    skills:
      bundles: native                # .github/skills/ SKILL.md
      supportingFiles: native        # on-demand via Markdown links
    agents:
      subagents: native              # .github/agents/*.agent.md
      toolPolicies: native           # tools list
      handoffs: native               # handoffs in agent frontmatter
    hooks:
      lifecycle: native              # 8 events, command type
      blockingValidation: native     # PreToolUse permissionDecision
    commands:
      explicitEntryPoints: native    # .prompt.md files
    plugins:
      installablePackages: native    # agent plugins (marketplace)
      capabilityProviders: native
    mcp:
      serverBindings: native         # .vscode/mcp.json servers key

  codex:
    instructions:
      layeredFiles: native           # AGENTS.md hierarchy
      scopedSections: native         # subdirectory AGENTS.md
    rules:
      scopedRules: native            # rules supported
    skills:
      bundles: native                # .codex/skills/ SKILL.md
      supportingFiles: native
    agents:
      subagents: native              # custom agents
      toolPolicies: native
      handoffs: skipped
    hooks:
      lifecycle: native              # hooks supported
      blockingValidation: native
    commands:
      explicitEntryPoints: lowered   # via skills
    plugins:
      installablePackages: native    # plugins with marketplace
      capabilityProviders: native
    mcp:
      serverBindings: native         # MCP supported
```

---

## Related Documents

- [overview.md](overview.md) — canonical object model and architecture
- [compiler.md](compiler.md) — compiler pipeline, lowering policy, capability registry
- [build-output.md](build-output.md) — output layout, validation, target strategy
- [marketplace.md](marketplace.md) — package distribution and plugin references
- [schemas.md](schemas.md) — illustrative YAML schemas
