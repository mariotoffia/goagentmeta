# Schema Sketches

This document provides illustrative schemas for the canonical object model defined in [overview.md](overview.md). Some types (manifest, hook, capability, plugin) are expressed as YAML; body-carrying types (skill, agent) use Markdown with YAML frontmatter. The exact schema can evolve, but the architecture should converge on shapes like the following.

---

## 1. Manifest

```yaml
schemaVersion: 1
project:
  name: goagentmeta
  monorepo: true

build:
  defaultTargets:
    - claude
    - cursor
    - copilot
    - codex
  defaultProfiles:
    - local-dev
  outputRoot: .ai-build
  syncMode: explicit

compilation:
  hierarchyMode: preserve-when-supported
  materialization: copy-or-symlink
  unsupportedMode: fail-on-required
  provenance: full

preservation:
  default: preferred

targets:
  claude:
    enabled: true
  cursor:
    enabled: true
  copilot:
    enabled: true
  codex:
    enabled: true

profiles:
  local-dev: profiles/local-dev.yaml
  ci: profiles/ci.yaml
  enterprise-locked: profiles/enterprise-locked.yaml

dependencies:
  "@acme/go-lambda-skill": "^1.3.0"
  "@community/github-mcp": "~2.1.0"

registries:
  - name: acme-internal
    url: https://registry.acme.com/ai-packages
    priority: 1
  - name: community
    url: https://registry.aicontrolplane.dev/v1
    priority: 2

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

---

## 2. Skill

### 2.1 Authoring Format (Markdown with YAML Frontmatter)

```markdown
---
id: go-aws-lambda
kind: skill
description: Build and validate Go Lambda services with AWS SDK v2
packageVersion: "1.3.0"
license: MIT
scope:
  paths:
    - "services/**"
preservation: preferred
requires:
  - terminal.exec
  - repo.search
resources:
  references:
    - references/aws-lambda-patterns.md
    - references/sdk-v2-migration.md
  assets:
    - assets/templates/lambda_main.go.tmpl
  scripts:
    - scripts/skills/go-aws-lambda/test.sh
activation:
  hints:
    - lambda
    - aws
    - go
allowedTools:
  - Read
  - Write
  - "Bash(go:*)"
  - "Bash(golangci-lint:*)"
compatibility: "Designed for AI coding agents using Go projects."
binaryDeps:
  - go
  - golangci-lint
installSteps:
  - kind: go
    package: golang.org/x/tools/cmd/goimports@latest
    bins: [goimports]
publishing:
  author: acme-team
  homepage: https://github.com/acme/go-lambda-skill
  emoji: "🚀"
---

Use Go 1.24+, context-first APIs, AWS SDK v2, and table-driven tests.
```

### 2.2 AgentSkills.io Frontmatter (import format)

The AgentSkills.io SKILL.md frontmatter uses a different layout. The parser
adapter maps it to the canonical model above:

```yaml
---
name: golang-benchmark
description: "Golang benchmarking, profiling, and performance measurement."
user-invocable: true
license: MIT
compatibility: Designed for Claude Code or similar AI coding agents.
metadata:
  author: samber
  version: "1.1.3"
  openclaw:
    emoji: "📊"
    homepage: https://github.com/samber/cc-skills-golang
    requires:
      bins:
        - go
        - benchstat
    install:
      - kind: go
        package: golang.org/x/perf/cmd/benchstat@latest
        bins: [benchstat]
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Bash(golangci-lint:*)
---
```

| AgentSkills.io field | Canonical model field |
|---|---|
| `name` | `ObjectMeta.ID` |
| `description` | `ObjectMeta.Description` |
| `user-invocable` | `Skill.UserInvocable` |
| `license` | `ObjectMeta.License` |
| `compatibility` | `Skill.Compatibility` |
| `metadata.author` | `Skill.Publishing.Author` |
| `metadata.version` | `ObjectMeta.PackageVersion` |
| `metadata.openclaw.emoji` | `Skill.Publishing.Emoji` |
| `metadata.openclaw.homepage` | `Skill.Publishing.Homepage` |
| `metadata.openclaw.requires.bins` | `Skill.BinaryDeps` |
| `metadata.openclaw.install[]` | `Skill.InstallSteps` |
| `allowed-tools` | `Skill.AllowedTools` |

---

## 3. Agent

```markdown
---
id: go-implementer
kind: agent
description: Go Implementer
preservation: preferred
skills:
  - go-aws-lambda
requires:
  - filesystem.read
  - filesystem.write
  - terminal.exec
  - repo.search
toolPolicy:
  filesystem.write: allow
  terminal.exec: allow
  network.http: deny
delegation:
  mayCall:
    - test-runner
handoffs:
  - label: Start Review
    agent: security-reviewer
    prompt: Review the implementation above for security issues.
    autoSend: false
---

Produce minimal, correct code changes with tests.
```

---

## 4. Hook

```yaml
id: post-edit-validate
kind: hook
scope:
  paths:
    - "services/**"
event: post-edit
preservation: preferred
action:
  type: script
  ref: scripts/hooks/post-edit-validate.sh
effect:
  class: validating
  enforcement: blocking
inputs:
  include:
    - changedFiles
    - workingDirectory
policy:
  timeoutSeconds: 60       # YAML int → Go time.Duration (HookPolicy.Timeout)
```

---

## 5. Capability

```yaml
id: repo.graph.query
kind: capability
contract:
  category: tool
  description: Query repository structure and relationships
security:
  network: none
  filesystem: read-repo
```

---

## 6. Plugin

### 6.1 Inline Plugin (code lives in repo)

```yaml
id: repo-graph
kind: plugin
preservation: optional
description: Provides repository graph queries as a runtime extension
  # display name can go in description (e.g. "Repo Graph")
distribution:
  mode: inline
provides:
  capabilities:
    - repo.graph.query
security:
  trust: review-required
  permissions:
    filesystem: read-repo
    network: none
artifacts:
  scripts:
    - scripts/plugins/repo-graph/server.sh
targets:
  codex:
    packaging:
      kind: native-plugin
      sourceDir: targets/codex/plugins/repo-graph
  cursor:
    packaging:
      kind: mcp-server
      entrypoint: scripts/plugins/repo-graph/server.sh
```

### 6.2 External Plugin (reference only)

```yaml
id: github-mcp
kind: plugin
preservation: preferred
description: GitHub API access through MCP
  # display name can go in description (e.g. "GitHub MCP")
distribution:
  mode: external
  ref: "@modelcontextprotocol/server-github"
provides:
  capabilities:
    - mcp.github
security:
  trust: verified-publisher
  permissions:
    network: outbound
    secrets:
      - GITHUB_TOKEN
targets:
  claude:
    install:
      kind: mcp-server-ref
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
  cursor:
    install:
      kind: mcp-server-ref
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
  copilot:
    install:
      kind: mcp-server-ref
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
```

### 6.3 Registry Plugin (resolved at compile time)

```yaml
id: repo-graph
kind: plugin
preservation: optional
description: Provides repository graph queries as a runtime extension
  # display name can go in description (e.g. "Repo Graph")
distribution:
  mode: registry
  source: "registry.example.com/plugins/repo-graph"
  version: "^1.2.0"
provides:
  capabilities:
    - repo.graph.query
security:
  trust: review-required
  permissions:
    filesystem: read-repo
    network: none
```

These examples are illustrative. The key architectural point is the separation of concerns, not the exact syntax.
