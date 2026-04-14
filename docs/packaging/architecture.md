# Packaging Architecture

This document describes the packaging subsystem that transforms compiled
emission plans into distributable artifacts for various target platforms.

## Quick Start

Package your `.ai/` source tree into a distributable Claude Code plugin:

```bash
# Build & package as a Claude Code plugin
goagentmeta package --format plugin --name my-plugin --version 1.0.0 \
  --author "Your Name" --license MIT --description "My skills & agents"

# Generate a marketplace catalog referencing plugins
goagentmeta package --format marketplace --name my-plugin \
  --marketplace-name my-tools --marketplace-owner "Your Team" \
  --source ./plugins/my-plugin --category development-workflows

# Dry-run to see what would be produced without writing files
goagentmeta package --format plugin --name my-plugin --dry-run
```

See the [CLI Reference](#cli-reference) section below for all flags and options.

## Overview

The packaging system follows the hexagonal architecture pattern:

```
┌─────────────────────────────────────────────────────┐
│                    CLI Layer                          │
│                                                       │
│   adapter/cli/package_cmd.go                         │
│   └─ goagentmeta package  (build + package in one)   │
│                                                       │
├───────────────────────────────────────────────────────┤
│                  Application Layer                    │
│                                                       │
│   PackagingService                                   │
│   ├─ Package()            ← legacy if/else dispatch  │
│   └─ PackageWithEmission() ← registry-based dispatch │
│                                                       │
├───────────────────────────────────────────────────────┤
│                    Port Layer                          │
│                                                       │
│   port/packager/                                     │
│   ├─ Packager interface   (Format, Targets, Package) │
│   ├─ PackagerRegistry     (Register, ByFormat, All)  │
│   └─ DefaultRegistry      (thread-safe impl)        │
│                                                       │
│   port/marketplace/                                  │
│   ├─ Generator interface  (Format, Generate)         │
│   ├─ GeneratorRegistry    (Register, ByFormat, All)  │
│   └─ DefaultRegistry      (thread-safe impl)        │
│                                                       │
├───────────────────────────────────────────────────────┤
│                   Adapter Layer                        │
│                                                       │
│   adapter/packager/                                  │
│   ├─ vscode/   → .vsix archives                     │
│   ├─ npm/      → .tgz tarballs                      │
│   └─ claude/   → Claude Code plugin directories      │
│                                                       │
│   adapter/marketplace/                               │
│   └─ claude/   → marketplace.json catalogs           │
└───────────────────────────────────────────────────────┘
```

## Packager Port

The `Packager` interface (`internal/port/packager/packager.go`) is the core
extensibility point:

```go
type Packager interface {
    Format() Format
    Targets() []build.Target
    Package(ctx context.Context, input PackagerInput) (*PackagerOutput, error)
}
```

### Format (semi-open enum)

```go
type Format string

const (
    FormatVSIX         Format = "vsix"
    FormatNPM          Format = "npm"
    FormatOCI          Format = "oci"
    FormatClaudePlugin Format = "claude-plugin"
    FormatMarketplace  Format = "marketplace"
)
```

New formats are added by:
1. Defining a new `Format` constant (optional — custom string values work)
2. Implementing the `Packager` interface
3. Registering with a `PackagerRegistry`

### PackagerInput

Packagers receive both the structured `EmissionPlan` (with plugin bundles,
install metadata, per-unit coordinates) and the `MaterializationResult` (flat
file paths). This allows packagers to use either structured data or filesystem
access as needed.

## Marketplace Generator Port

The `Generator` interface (`internal/port/marketplace/marketplace.go`)
generates target-native marketplace catalogs:

```go
type Generator interface {
    Format() TargetFormat
    Generate(ctx context.Context, input GeneratorInput) (*GeneratorOutput, error)
}
```

## Built-in Packagers

### VS Code Extension (`.vsix`)

Produces a ZIP archive with:
- `[Content_Types].xml`
- `extension.vsixmanifest`
- `extension/package.json`
- Content files under `extension/`

### npm Package (`.tgz`)

Produces a gzipped tarball with:
- `package/package.json`
- Content files under `package/`

### Claude Code Plugin

Produces a directory structure matching Claude Code's plugin format:

```
<name>/
├── .claude-plugin/
│   └── plugin.json          ← generated manifest
├── skills/
│   └── <id>/SKILL.md        ← from .claude/skills/
├── agents/
│   └── <id>.md              ← from .claude/agents/
├── hooks/
│   └── hooks.json           ← extracted from .claude/settings.json
├── .mcp.json                ← from .mcp.json
└── bin/                     ← from emitted scripts
```

## Claude Code Marketplace Generator

Produces a `.claude-plugin/marketplace.json` catalog compatible with Claude
Code's `/plugin marketplace add` command. Supports all source types:

| Source Type | Format |
|-------------|--------|
| Relative path | `"./plugins/formatter"` |
| GitHub | `{"source": "github", "repo": "owner/repo"}` |
| Git URL | `{"source": "url", "url": "https://..."}` |
| Git subdirectory | `{"source": "git-subdir", "url": "...", "path": "..."}` |
| npm | `{"source": "npm", "package": "@scope/name"}` |

## Writing a Custom Packager

1. Create a package under `internal/adapter/packager/<name>/`
2. Implement `packager.Packager`:

```go
package myformat

type Packager struct { /* dependencies */ }

func (p *Packager) Format() packager.Format { return packager.Format("my-format") }
func (p *Packager) Targets() []build.Target { return nil } // target-agnostic
func (p *Packager) Package(ctx context.Context, input packager.PackagerInput) (*packager.PackagerOutput, error) {
    cfg := input.Config.(*MyConfig)
    // ... produce artifacts ...
    return &packager.PackagerOutput{Artifacts: []packager.PackagedArtifact{...}}, nil
}

var _ packager.Packager = (*Packager)(nil) // compile-time check
```

3. Register with the `PackagerRegistry`:

```go
reg := packager.NewRegistry()
reg.MustRegister(myformat.New(fsReader, fsWriter))
```

## Configuration

Marketplace and plugin packaging are configured in the manifest:

```yaml
packaging:
  claude-plugin:
    enabled: true
    name: "my-plugin"
    version: "1.0.0"
    author: { name: "Team" }
    keywords: ["go", "testing"]

  marketplace:
    enabled: true
    format: claude
    name: "company-tools"
    owner: { name: "DevTools Team" }
    plugins:
      - name: "my-plugin"
        source: "./plugins/my-plugin"
        category: "development"
        tags: ["go", "testing"]
```

## CLI Reference

The `goagentmeta package` command compiles `.ai/` source files and packages the
output into distributable formats. It runs the full compiler pipeline first, then
invokes the appropriate packager or marketplace generator.

### Synopsis

```bash
goagentmeta package [paths...] [flags]
```

### Packaging Formats

| Format | Flag | Description |
|--------|------|-------------|
| `plugin` | `--format plugin` (default) | Produces a distributable Claude Code plugin directory |
| `marketplace` | `--format marketplace` | Generates a `marketplace.json` catalog |

### Common Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | *(required)* | Plugin name (kebab-case) |
| `--version` | `0.0.1` | Plugin version (semver) |
| `--author` | | Plugin author name |
| `--description` | | Plugin description |
| `--license` | | SPDX license identifier (e.g., `MIT`, `Apache-2.0`) |
| `--keyword` | | Discovery keywords (repeatable) |
| `--category` | | Plugin category (e.g., `code-intelligence`, `development-workflows`) |
| `--tag` | | Searchability tags (repeatable) |
| `--output-dir` | `.ai-build/dist` | Output directory for packaged artifacts |
| `-t, --target` | `claude` | Build targets (`claude`, `cursor`, `copilot`, `codex`, `all`) |
| `-p, --profile` | `local-dev` | Build profile |
| `--dry-run` | `false` | Print what would be produced without writing files |

### Marketplace-Specific Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--marketplace-name` | *(required)* | Marketplace catalog name |
| `--marketplace-owner` | *(required)* | Marketplace owner name |
| `--source` | `./<name>` | Plugin source (relative path, `github:owner/repo`, `npm:@scope/name`, or URL) |

### Source Formats

The `--source` flag supports multiple formats that map to Claude Code's source types:

| Prefix | Source Type | Example |
|--------|------------|---------|
| *(none)* | Relative path | `./plugins/my-plugin` |
| `github:` | GitHub repository | `github:acme/my-plugin` |
| `npm:` | npm package | `npm:@acme/my-plugin` |
| `https://` or `git@` | Git URL | `https://github.com/acme/my-plugin.git` |

### Examples

```bash
# Build & package as a Claude Code plugin with full metadata
goagentmeta package --format plugin --name my-plugin \
  --version 1.0.0 --author "DevTools Team" --license MIT \
  --description "Reusable Go development skills" \
  --keyword go --keyword testing --keyword aws \
  --category code-intelligence

# Build for Claude only with custom output directory
goagentmeta package -t claude --format plugin --name my-plugin \
  --version 1.0.0 --output-dir dist/plugins

# Generate a marketplace catalog with a relative source
goagentmeta package --format marketplace --name my-plugin \
  --marketplace-name company-tools --marketplace-owner "DevTools Team" \
  --source ./plugins/my-plugin --category development-workflows \
  --tag go --tag ai

# Generate a marketplace catalog with a GitHub source
goagentmeta package --format marketplace --name my-plugin \
  --marketplace-name company-tools --marketplace-owner "DevTools Team" \
  --source github:acme/my-plugin

# Dry-run to preview output
goagentmeta package --format plugin --name my-plugin --dry-run
```

## End-to-End Workflow

### 1. Package as a Claude Code Plugin

```bash
goagentmeta package --format plugin --name goagentmeta-skills \
  --version 1.0.0 --author "Mario Toffia" --license MIT
```

This produces:

```
.ai-build/dist/goagentmeta-skills/
├── .claude-plugin/
│   └── plugin.json          ← generated manifest
├── skills/
│   └── <id>/SKILL.md        ← from .claude/skills/
├── agents/
│   └── <id>.md              ← from .claude/agents/
├── hooks/
│   └── hooks.json           ← extracted from settings.json
├── .mcp.json                ← MCP server config (if any)
└── bin/                     ← emitted scripts (if any)
```

### 2. Test the Plugin Locally

```bash
claude --plugin-dir .ai-build/dist/goagentmeta-skills
```

### 3. Generate a Marketplace Catalog

```bash
goagentmeta package --format marketplace --name goagentmeta-skills \
  --marketplace-name my-marketplace --marketplace-owner "DevTools Team" \
  --source ./plugins/goagentmeta-skills --category development-workflows
```

This produces `.ai-build/dist/.claude-plugin/marketplace.json`:

```json
{
  "name": "my-marketplace",
  "owner": { "name": "DevTools Team" },
  "plugins": [
    {
      "name": "goagentmeta-skills",
      "source": "./plugins/goagentmeta-skills",
      "category": "development-workflows",
      "version": "1.0.0"
    }
  ]
}
```

### 4. Distribute via Git Repository

Push the marketplace output to a Git repository, then users install with:

```
/plugin marketplace add owner/repo
/plugin install goagentmeta-skills@my-marketplace
```

### 5. Submit to Official Claude Marketplace

For public distribution, submit the plugin at
[claude.ai/settings/plugins/submit](https://claude.ai/settings/plugins/submit).
Users can then discover and install it through Claude Code's built-in plugin UI
(`/plugin` → Discover tab).

## Pipeline Integration

The `package` command integrates compilation and packaging in a single invocation:

```
.ai/ source tree
  → compiler pipeline (parse → validate → resolve → normalize → plan →
    capability → lower → render → materialize → report)
  → packager (Claude plugin directory) or generator (marketplace.json)
  → distributable artifacts under .ai-build/dist/
```

The command:
1. Constructs and runs the full compiler pipeline (`wirePipeline`)
2. Collects materialized files from the `BuildReport`
3. Invokes the appropriate packager adapter or marketplace generator
4. Writes distributable artifacts to `--output-dir`

## Extensibility Summary

| Concept | Pattern | How to Extend |
|---------|---------|---------------|
| Packager format | Semi-open `Format` string enum | Add constant + implement interface |
| Marketplace format | Semi-open `TargetFormat` string enum | Add constant + implement interface |
| Plugin source type | Semi-open `SourceType` string enum | Add constant |
| Component kind | Semi-open `ComponentKind` string enum | Add constant |
| Plugin entry fields | `Components map[string]any` + `Extra map[string]any` | Add keys at runtime |
| Manifest fields | `Settings map[string]any` + `Extra map[string]any` | Add keys at runtime |
