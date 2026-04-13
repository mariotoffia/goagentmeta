---
id: project-context
kind: instruction
version: 1
description: What GoAgentMeta is, its purpose, and key domain concepts
preservation: preferred
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Project Context

**GoAgentMeta** is a compiler-based control plane for AI-agent metadata. It compiles a canonical source tree (`.ai/`) into native artifacts for multiple target ecosystems — Claude Code, Cursor, GitHub Copilot, and Codex.

Platform differences are **compiler concerns**, not runtime abstractions.

## Technology

- **Language**: Go 1.25+
- **Dependencies**: Minimal (cobra + yaml.v3)
- **Architecture**: Clean + Hexagonal + DDD
- **Pipeline**: 10-phase multi-stage compiler with pluggable stages

## Ubiquitous Language

### Authoring Primitives (what authors write)

| Concept | Definition |
|---------|------------|
| **Instruction** | Always-on guidance (architecture, standards) |
| **Rule** | Scoped/conditional policy (language rules, security) |
| **Skill** | Reusable workflow bundle (model-facing content) |
| **Agent** | Specialized delegate with role, tools, permissions |
| **Hook** | Lifecycle automation triggered by events |
| **Command** | User-invoked entry point (`/review-iam`) |

### Runtime Delivery Primitives

| Concept | Definition |
|---------|------------|
| **Capability** | Abstract contract (`filesystem.read`, `mcp.github`) |
| **Plugin** | Deployable extension providing capabilities |
| **Reference** | Supplemental knowledge document (demand-loaded) |
| **Asset** | Static file (template, diagram, prompt partial) |
| **Script** | Executable artifact for hooks/commands |

### Build Coordinates

| Concept | Definition |
|---------|------------|
| **Target** | Vendor ecosystem: `claude`, `cursor`, `copilot`, `codex` |
| **Profile** | Runtime policy: `local-dev`, `ci`, `enterprise-locked` |
| **BuildUnit** | `(target, profile, scopes)` — fundamental compilation unit |

### Preservation Semantics

Every canonical object carries a preservation level:
- **`required`** — Unsupported lowering fails the build
- **`preferred`** — Lower when safe, warn and skip otherwise
- **`optional`** — May skip with reporting

### IR Flow

`SourceTree` → `SemanticGraph` → `BuildPlan` → `CapabilityGraph` → `LoweredGraph` → `EmissionPlan` → `MaterializationResult` → `BuildReport`
