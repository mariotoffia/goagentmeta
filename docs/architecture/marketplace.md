# Marketplace and Registry Architecture

This document specifies the package distribution, registry, and marketplace layer that extends the core compiler architecture defined in [overview.md](overview.md).

---

## 1. Purpose

The core architecture treats all canonical objects as repository-local. This works for single-team repositories but breaks down when:

- teams want to share skills, plugins, or agents across repositories
- the community publishes reusable components
- organizations want a curated internal catalog
- externally-installed tools (MCP servers, VS Code extensions) must be referenced without copying source into the repo

The marketplace layer provides discovery, versioning, dependency resolution, trust, and caching for shared components.

---

## 2. Concepts

### 2.1 Package

A **package** is a versioned, publishable unit containing one or more canonical objects.

A package may contain:

- skills
- plugins
- agents
- instructions
- rules
- capabilities
- hooks
- commands
- assets
- scripts

A package has:

- a unique name (scoped by publisher, for example `@acme/go-lambda-skill`)
- a version following semver
- declared dependencies on other packages
- a manifest listing its contents and metadata
- integrity hashes for verification

### 2.2 Registry

A **registry** is a discovery and resolution endpoint that indexes packages.

Registry types:

- **public**: community-operated, open to all publishers (comparable to npm, crates.io)
- **organizational**: private, scoped to an enterprise or team
- **git-based**: packages resolved directly from git repositories by tag or commit

The compiler must support multiple registries in priority order. Organizational registries typically take precedence over public ones.

### 2.3 Publisher

A **publisher** is an identity that signs and releases packages. Trust policies bind to publishers, not individual packages.

### 2.4 Dependency Declaration

Dependencies are declared in `manifest.yaml` under a `dependencies:` block. Each entry names a package and a version constraint.

### 2.5 Lock File

The lock file (`manifest.lock.yaml`) pins exact resolved versions and integrity hashes. It must be committed to version control so that builds are reproducible.

---

## 3. Package Manifest

Every publishable package contains a `package.yaml` at its root:

```yaml
name: "@acme/go-lambda-skill"
version: "1.3.0"
description: Go Lambda development skill with AWS SDK v2 best practices
publisher: acme
license: Apache-2.0

contents:
  skills:
    - skills/go-aws-lambda.yaml
  assets:
    - assets/templates/lambda_main.go.tmpl
  scripts:
    - scripts/test.sh

requires:
  capabilities:
    - terminal.exec
    - repo.search

dependencies:
  "@acme/go-base": "^2.0.0"

targets:
  supported:
    - claude
    - cursor
    - copilot
    - codex

keywords:
  - go
  - lambda
  - aws
```

---

## 4. Dependency Declaration in Manifest

The repository `manifest.yaml` gains a `dependencies:` section:

```yaml
schemaVersion: 1
project:
  name: my-service
  monorepo: false

dependencies:
  "@acme/go-lambda-skill": "^1.3.0"
  "@community/github-mcp": "~2.1.0"
  "@acme/security-rules": ">=1.0.0 <3.0.0"

registries:
  - name: acme-internal
    url: https://registry.acme.com/ai-packages
    priority: 1
  - name: community
    url: https://registry.aicontrolplane.dev/v1
    priority: 2
```

### 4.1 Version Constraints

Version constraints follow semver range syntax:

- `^1.3.0` — compatible with 1.3.0 (>=1.3.0, <2.0.0)
- `~2.1.0` — approximately 2.1.0 (>=2.1.0, <2.2.0)
- `>=1.0.0 <3.0.0` — explicit range
- `1.3.0` — exact pin

### 4.2 Registry Priority

When multiple registries contain the same package name, the compiler resolves from the highest-priority (lowest number) registry first. This allows organizations to shadow community packages with internal forks.

---

## 5. Lock File

The lock file provides reproducible builds:

```yaml
schemaVersion: 1
resolved:
  "@acme/go-lambda-skill":
    version: "1.3.2"
    registry: acme-internal
    integrity: "sha256:a1b2c3d4..."
    dependencies:
      "@acme/go-base": "2.1.0"
  "@acme/go-base":
    version: "2.1.0"
    registry: acme-internal
    integrity: "sha256:e5f6a7b8..."
  "@community/github-mcp":
    version: "2.1.3"
    registry: community
    integrity: "sha256:c9d0e1f2..."
  "@acme/security-rules":
    version: "2.4.1"
    registry: acme-internal
    integrity: "sha256:f3a4b5c6..."
```

The lock file must be updated only by explicit compiler commands (`ai-build update-deps`), never silently during a normal build.

---

## 6. Compiler Integration

Dependency resolution inserts between parsing and normalization (see overview.md Section 10.2).

### 6.1 Resolution Algorithm

1. read `manifest.yaml` dependencies and `manifest.lock.yaml`
2. if lock file exists and all constraints satisfied, use locked versions
3. if lock file is missing or stale, resolve from registries in priority order
4. fetch or validate cached packages
5. verify integrity hashes
6. merge resolved package contents into the source IR as if they were local `.ai/` objects, namespaced by package name
7. update lock file if any resolution changed

### 6.2 Namespace Isolation

External package objects are namespaced to prevent collisions:

- a skill `go-aws-lambda` from package `@acme/go-lambda-skill` becomes `@acme/go-lambda-skill/go-aws-lambda` in the semantic graph
- local objects may reference external objects by their fully-qualified ID
- shorthand references are allowed when unambiguous

### 6.3 Caching

Resolved packages should be cached locally (for example, under `.ai-cache/`). The cache is not committed to version control. Cache invalidation is hash-based.

---

## 7. Trust and Security

### 7.1 Publisher Trust

Trust is configured per profile:

```yaml
# profiles/enterprise-locked.yaml
trust:
  publishers:
    allowed:
      - acme
      - verified-community
    denied:
      - "*"
  registries:
    allowed:
      - acme-internal
    denied:
      - community
```

### 7.2 Permission Escalation

If an external package declares permissions that exceed the consuming profile's security policy, the compiler must:

- fail the build if preservation is `required`
- warn and skip the package if preservation is `preferred`
- silently skip if preservation is `optional`

### 7.3 Integrity Verification

Every resolved package must match its declared integrity hash. Hash mismatch fails the build unconditionally — this is not subject to preservation level.

### 7.4 Supply Chain Provenance

Build reports (overview.md Section 16) must include for each external dependency:

- package name and resolved version
- source registry
- publisher identity
- integrity hash
- which local objects consume it

---

## 8. External Plugin References

Many target ecosystems (Claude Code, Cursor, Copilot) support MCP servers and tools as externally-installed processes referenced by configuration. The compiler must handle plugins where no code is materialized.

### 8.1 Reference-Only Emission

When a plugin has `distribution.mode: external`, the renderer emits only a target-native configuration entry.

Examples:

**Claude Code** — entry in `.claude/settings.json`:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_TOKEN}"
      }
    }
  }
}
```

**Cursor** — entry in `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"]
    }
  }
}
```

**Copilot** — entry in `.vscode/mcp.json`:

```json
{
  "servers": {
    "github": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"]
    }
  }
}
```

The plugin YAML declares the logical intent. The renderer decides the exact file and format.

### 8.2 Active Plugin Selection

Targets like Claude Code only load MCP servers that are configured in the active settings file for the current repository. The compiler should therefore:

1. only emit references for plugins that are actually required by the selected capabilities in the build unit
2. avoid emitting unused plugin references that would bloat the settings file
3. allow profile-level overrides to force-include or force-exclude specific plugins

This keeps the emitted configuration minimal and directly aligned with the repository's declared needs.

---

## 9. Marketplace Discovery

Discovery is out of scope for the compiler itself but the architecture should define the contract that a marketplace service must satisfy.

### 9.1 Search API Contract

A registry should support:

- search by keyword, capability provided, target supported, publisher
- list versions for a package
- fetch package metadata without downloading contents
- fetch package contents by version

### 9.2 Package Metadata Response

```yaml
name: "@community/github-mcp"
version: "2.1.3"
publisher: community-tools
description: GitHub API access through MCP
license: MIT
targets:
  supported: [claude, cursor, copilot, codex]
provides:
  capabilities: [mcp.github]
distribution:
  preferredMode: external
downloads: 12450
verified: true
```

### 9.3 CLI Integration

The compiler CLI should support:

- `ai-build search <query>` — search registries
- `ai-build add <package>[@version]` — add a dependency
- `ai-build remove <package>` — remove a dependency
- `ai-build update-deps` — re-resolve and update lock file
- `ai-build audit` — check dependencies for trust and security policy violations

---

## 10. Platform-Native Packaging and Distribution

The compiler output is a set of target-specific files. Some use cases require distributing those files (or a subset of them) through each platform's native marketplace or extension mechanism so that end users receive updates through familiar platform tooling.

### 10.1 Why Platform-Native Packaging Matters

Consider an organization that maintains a set of Go development skills, security rules, and MCP plugin references. Without platform-native packaging:

- every consuming repository must declare the dependency in its own `manifest.yaml`
- every repository must run the compiler to resolve and render
- updates require re-running the compiler in each repository

With platform-native packaging, the compiled output can be wrapped as:

- a **VS Code extension** (for Copilot and Cursor users) that installs via the VS Code Marketplace and auto-updates
- an **npm package** that provides MCP servers installable via `npx`
- a **Homebrew formula** or **apt package** for CLI tools used as plugin backends
- a **Claude Code project template** or shareable configuration bundle
- an **OCI artifact** for container-based CI environments

Users install once and receive updates through the platform's own mechanism.

### 10.2 Packaging as a Compiler Post-Step

Platform-native packaging is a post-compilation step. The compiler emits target artifacts first; a separate packaging stage wraps them.

```text
.ai/ source tree
  -> compiler pipeline
  -> .ai-build/ target artifacts
  -> platform packager (optional)
  -> distributable packages (VS Code .vsix, npm tarball, OCI image, etc.)
```

The manifest controls packaging:

```yaml
build:
  packaging:
    vscode-extension:
      enabled: true
      publisher: acme
      targets: [copilot, cursor]
      includeSkills: true
      includeMcpConfig: true
    npm:
      enabled: true
      scope: "@acme"
      targets: [claude, cursor, copilot]
      includeOnly: extensionLayer
    oci:
      enabled: false
```

### 10.3 VS Code Extension Packaging

A VS Code extension can carry:

- `copilot-instructions.md` and `AGENTS.md` files contributed to the workspace
- `.vscode/mcp.json` entries merged into workspace configuration
- skill markdown files surfaced through Copilot's chat participants
- bundled MCP server code

The extension uses VS Code's contribution points to inject files and configuration. Users install it from the VS Code Marketplace and receive updates automatically.

### 10.4 npm Package Distribution

An npm package can carry:

- MCP server entry points (`bin` field)
- skill and instruction markdown shipped as package data
- a post-install script that emits target configuration

Users reference it via `npx` in their MCP config, or install it globally.

### 10.5 Update Lifecycle

When the source `.ai/` tree changes:

1. the compiler re-emits target artifacts
2. the platform packager re-wraps them
3. new version is published to the platform marketplace
4. end users receive the update through the platform's native update flow

This means consuming repositories do not need to re-run the compiler for shared content. The platform's own update mechanism handles delivery.

### 10.6 Relationship to Registry Packages

Platform-native packages and registry packages serve different audiences:

| Concern | Registry package | Platform-native package |
|---|---|---|
| Consumer | `.ai/` compiler in another repo | End user via platform UI |
| Resolution | Compiler dependency resolver | Platform package manager (VS Code, npm, brew) |
| Update | `ai-build update-deps` + re-compile | Platform auto-update |
| Contents | Canonical `.ai/` objects (pre-compilation) | Compiled target artifacts (post-compilation) |
| Trust | Registry publisher trust + integrity hash | Platform marketplace verification |

Both may be published from the same source tree. A registry package distributes canonical objects for other compilers to consume. A platform-native package distributes compiled output for end users to consume directly.

---

## 11. Architecture Summary

The marketplace layer adds four capabilities to the core architecture:

1. **External supply chain**: packages can be published, discovered, resolved, and consumed across repositories
2. **Reference-only distribution**: plugins that exist outside the repo are emitted as target-native configuration references, not materialized code
3. **Trust and reproducibility**: lock files, integrity hashes, publisher trust policies, and profile-level security controls govern what enters the build
4. **Platform-native packaging**: compiled output can be wrapped and distributed through each target platform's native marketplace, giving end users updates through familiar tooling

These capabilities close the gap between the repository-local compiler model and the reality that skills, plugins, and agents are shared artifacts with an external lifecycle.
