# Packaging Architecture

This document describes the packaging subsystem that transforms compiled
emission plans into distributable artifacts for various target platforms.

## Overview

The packaging system follows the hexagonal architecture pattern:

```
┌─────────────────────────────────────────────────────┐
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

## Extensibility Summary

| Concept | Pattern | How to Extend |
|---------|---------|---------------|
| Packager format | Semi-open `Format` string enum | Add constant + implement interface |
| Marketplace format | Semi-open `TargetFormat` string enum | Add constant + implement interface |
| Plugin source type | Semi-open `SourceType` string enum | Add constant |
| Component kind | Semi-open `ComponentKind` string enum | Add constant |
| Plugin entry fields | `Components map[string]any` + `Extra map[string]any` | Add keys at runtime |
| Manifest fields | `Settings map[string]any` + `Extra map[string]any` | Add keys at runtime |
