---
id: canonical-authoring
kind: skill
version: 1
description: How to author .ai/ canonical source files for all entity types
preservation: preferred
requires:
  - filesystem.read
  - filesystem.write
resources:
  references:
    - references/language-reference.md
activation:
  hints:
    - author .ai
    - canonical file
    - write instruction
    - write rule
    - write skill
    - write agent
    - create hook
    - create command
tools:
  - Read
  - Write
  - Edit
  - Glob
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Authoring Canonical Source Files

The `.ai/` directory is the source tree for the GoAgentMeta compiler. Each entity type has a specific format and location.

## File Format Rules

| Kind | Format | Directory | Body? |
|------|--------|-----------|-------|
| instruction | `.md` (YAML frontmatter) | `.ai/instructions/` | ✅ Content |
| rule | `.md` (YAML frontmatter) | `.ai/rules/` | ✅ Content |
| skill | `.md` (YAML frontmatter) | `.ai/skills/` | ✅ Content |
| agent | `.md` (YAML frontmatter) | `.ai/agents/` | ✅ RolePrompt |
| hook | `.yaml` | `.ai/hooks/` | ❌ |
| command | `.yaml` | `.ai/commands/` | ❌ |
| capability | `.yaml` | `.ai/capabilities/` | ❌ |
| plugin | `.yaml` | `.ai/plugins/` | ❌ |

## Common Envelope (ObjectMeta)

Every entity has these frontmatter fields:

```yaml
---
id: my-entity          # Required, unique identifier
kind: instruction      # Required, entity type
version: 1             # Schema version (default: 1)
description: ...       # Human-readable description
preservation: preferred  # required | preferred | optional
scope:
  paths: ["src/**"]       # Glob patterns for addressable domain
  fileTypes: [".go"]      # File extension filter
  labels: ["security"]    # Semantic tags
appliesTo:
  targets: ["claude", "copilot"]  # Which targets (* = all)
  profiles: ["local-dev"]          # Which profiles (* = all)
extends:
  - base-instruction       # Inheritance from another object
---
```

## Authoring Instructions

Always-on guidance, injected unconditionally within scope:

```markdown
---
id: go-conventions
kind: instruction
description: Go coding conventions
---

Your instruction content in **markdown** here.
```

## Authoring Rules

Conditional policy with activation conditions:

```markdown
---
id: no-reflect-in-domain
kind: rule
conditions:
  - type: path-pattern
    value: "internal/domain/**"
  - type: language
    value: go
---

Do not use the `reflect` package in domain code.
```

## Authoring Skills

Reusable workflow bundles with tool access:

```markdown
---
id: review-architecture
kind: skill
requires: [filesystem.read, terminal.exec]
activation:
  hints: [architecture, review, hexagonal]
tools: [Read, Glob, Grep, Bash(go:*)]
---

Workflow steps in **markdown**.
```

## Authoring Agents

Specialized delegates with role prompt as body:

```markdown
---
id: code-reviewer
kind: agent
skills: [review-architecture]
tools:
  - Read
disallowedTools:
  - Bash
delegation:
  mayCall: [test-runner]
---

You are a code reviewer specializing in Go hexagonal architecture...
```

## Preservation Levels

Choose based on how critical the content is:
- **`required`** — Build fails if target cannot represent it
- **`preferred`** — Warn and skip if not supported (default)
- **`optional`** — Silently skip with provenance record
