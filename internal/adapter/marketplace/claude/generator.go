// Package claude implements a marketplace Generator that produces Claude
// Code-compatible marketplace.json catalogs. The generated catalog follows
// the schema expected by Claude Code's /plugin marketplace add command.
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"

	domain "github.com/mariotoffia/goagentmeta/internal/domain/marketplace"
	"github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	portmp "github.com/mariotoffia/goagentmeta/internal/port/marketplace"
)

// Generator produces Claude Code-compatible marketplace.json catalogs.
type Generator struct {
	fsWriter filesystem.Writer
}

// New creates a Claude marketplace generator.
func New(fsWriter filesystem.Writer) *Generator {
	return &Generator{fsWriter: fsWriter}
}

// Format returns FormatClaude.
func (g *Generator) Format() portmp.TargetFormat { return portmp.FormatClaude }

// Generate creates the .claude-plugin/marketplace.json catalog.
func (g *Generator) Generate(ctx context.Context, input portmp.GeneratorInput) (*portmp.GeneratorOutput, error) {
	catalog := buildCatalog(input.Marketplace)

	catalogJSON, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal marketplace.json: %w", err)
	}

	catalogDir := path.Join(input.OutputDir, ".claude-plugin")
	catalogPath := path.Join(catalogDir, "marketplace.json")

	if err := g.fsWriter.MkdirAll(ctx, catalogDir, 0o755); err != nil {
		return nil, fmt.Errorf("create catalog dir: %w", err)
	}
	if err := g.fsWriter.WriteFile(ctx, catalogPath, catalogJSON, 0o644); err != nil {
		return nil, fmt.Errorf("write marketplace.json: %w", err)
	}

	return &portmp.GeneratorOutput{
		CatalogPath: catalogPath,
		Files:       []string{catalogPath},
		Metadata: map[string]string{
			"format":      "claude",
			"pluginCount": fmt.Sprintf("%d", len(input.Marketplace.Plugins)),
		},
	}, nil
}

// Compile-time assertion.
var _ portmp.Generator = (*Generator)(nil)

// --- catalog construction ---

// buildCatalog transforms the domain Marketplace into Claude Code's
// marketplace.json schema.
func buildCatalog(mp domain.Marketplace) map[string]any {
	catalog := map[string]any{
		"name": mp.Name,
		"owner": map[string]any{
			"name": mp.Owner.Name,
		},
	}

	if mp.Owner.Email != "" {
		catalog["owner"] = map[string]any{
			"name":  mp.Owner.Name,
			"email": mp.Owner.Email,
		}
	}

	if mp.Metadata.Description != "" || mp.Metadata.Version != "" || mp.Metadata.PluginRoot != "" {
		meta := map[string]any{}
		if mp.Metadata.Description != "" {
			meta["description"] = mp.Metadata.Description
		}
		if mp.Metadata.Version != "" {
			meta["version"] = mp.Metadata.Version
		}
		if mp.Metadata.PluginRoot != "" {
			meta["pluginRoot"] = mp.Metadata.PluginRoot
		}
		catalog["metadata"] = meta
	}

	plugins := make([]map[string]any, 0, len(mp.Plugins))
	for _, p := range mp.Plugins {
		plugins = append(plugins, buildPluginEntry(p))
	}

	// Deterministic ordering by plugin name.
	sort.Slice(plugins, func(i, j int) bool {
		ni, _ := plugins[i]["name"].(string)
		nj, _ := plugins[j]["name"].(string)
		return ni < nj
	})

	catalog["plugins"] = plugins

	return catalog
}

// buildPluginEntry transforms a domain PluginEntry into the Claude
// marketplace.json plugin entry format.
func buildPluginEntry(p domain.PluginEntry) map[string]any {
	entry := map[string]any{
		"name":   p.Name,
		"source": buildSource(p.Source),
	}

	if p.Description != "" {
		entry["description"] = p.Description
	}
	if p.Version != "" {
		entry["version"] = p.Version
	}
	if p.Author.Name != "" {
		author := map[string]string{"name": p.Author.Name}
		if p.Author.Email != "" {
			author["email"] = p.Author.Email
		}
		entry["author"] = author
	}
	if p.Homepage != "" {
		entry["homepage"] = p.Homepage
	}
	if p.Repository != "" {
		entry["repository"] = p.Repository
	}
	if p.License != "" {
		entry["license"] = p.License
	}
	if len(p.Keywords) > 0 {
		entry["keywords"] = p.Keywords
	}
	if p.Category != "" {
		entry["category"] = p.Category
	}
	if len(p.Tags) > 0 {
		entry["tags"] = p.Tags
	}
	if p.Strict != nil {
		entry["strict"] = *p.Strict
	}

	// Merge open component declarations.
	for k, v := range p.Components {
		entry[k] = v
	}

	// Merge extra fields.
	for k, v := range p.Extra {
		entry[k] = v
	}

	return entry
}

// buildSource transforms a domain Source into the Claude marketplace.json
// source format.
func buildSource(s domain.Source) any {
	switch s.Type {
	case domain.SourceRelativePath:
		return s.Location

	case domain.SourceGitHub:
		src := map[string]any{
			"source": "github",
			"repo":   s.Location,
		}
		if s.Ref != "" {
			src["ref"] = s.Ref
		}
		if s.SHA != "" {
			src["sha"] = s.SHA
		}
		return src

	case domain.SourceGitURL:
		src := map[string]any{
			"source": "url",
			"url":    s.Location,
		}
		if s.Ref != "" {
			src["ref"] = s.Ref
		}
		if s.SHA != "" {
			src["sha"] = s.SHA
		}
		return src

	case domain.SourceGitSubdir:
		src := map[string]any{
			"source": "git-subdir",
			"url":    s.Location,
			"path":   s.Path,
		}
		if s.Ref != "" {
			src["ref"] = s.Ref
		}
		if s.SHA != "" {
			src["sha"] = s.SHA
		}
		return src

	case domain.SourceNPM:
		src := map[string]any{
			"source":  "npm",
			"package": s.Package,
		}
		if s.Version != "" {
			src["version"] = s.Version
		}
		if s.Registry != "" {
			src["registry"] = s.Registry
		}
		return src

	default:
		// Unknown source type — pass through as string for forward compatibility.
		return s.Location
	}
}
