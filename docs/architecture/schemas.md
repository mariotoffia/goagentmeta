# Schema Sketches

This document provides illustrative YAML schemas for the canonical object model defined in [overview.md](overview.md). The exact schema can evolve, but the architecture should converge on shapes like the following.

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

```yaml
id: go-aws-lambda
kind: skill
scope:
  paths:
    - "services/**"
preservation: preferred
metadata:
  name: go-aws-lambda
  description: Build and validate Go Lambda services with AWS SDK v2
content:
  markdown: |
    Use Go 1.24+, context-first APIs, AWS SDK v2, and table-driven tests.
requires:
  capabilities:
    - terminal.exec
    - repo.search
resources:
  assets:
    - assets/templates/lambda_main.go.tmpl
  scripts:
    - scripts/skills/go-aws-lambda/test.sh
activation:
  hints:
    - lambda
    - aws
    - go
```

---

## 3. Agent

```yaml
id: go-implementer
kind: agent
preservation: preferred
metadata:
  name: Go Implementer
rolePrompt: |
  Produce minimal, correct code changes with tests.
links:
  skills:
    - go-aws-lambda
requires:
  capabilities:
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
  timeoutSeconds: 60
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
metadata:
  name: Repo Graph
  description: Provides repository graph queries as a runtime extension
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
metadata:
  name: GitHub MCP
  description: GitHub API access through MCP
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
metadata:
  name: Repo Graph
  description: Provides repository graph queries as a runtime extension
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

These examples are illustrative. The key architectural point is the separation of concerns, not the exact YAML spelling.
