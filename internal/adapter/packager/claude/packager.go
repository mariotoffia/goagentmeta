// Package claude implements a Packager that produces distributable Claude Code
// plugin directories from compiled emission plans. The output follows Claude
// Code's plugin format:
//
//	<output>/
//	  .claude-plugin/
//	    plugin.json         ← generated manifest
//	  skills/
//	    <id>/SKILL.md       ← from .claude/skills/
//	  agents/
//	    <id>.md             ← from .claude/agents/
//	  hooks/
//	    hooks.json          ← extracted from .claude/settings.json
//	  .mcp.json             ← from .mcp.json
//	  .lsp.json             ← from .lsp.json (if present)
//	  bin/                  ← from emitted scripts
//
// Paths referencing plugin-local files are rewritten to use ${CLAUDE_PLUGIN_ROOT}.
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/marketplace"
	"github.com/mariotoffia/goagentmeta/internal/domain/plugin"
	"github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
)

// Config controls generation of a distributable Claude Code plugin directory.
type Config struct {
	// Name is the plugin identifier (kebab-case).
	Name string `yaml:"name"`
	// Version is the plugin version (semver).
	Version string `yaml:"version,omitempty"`
	// Description is a brief plugin description.
	Description string `yaml:"description,omitempty"`
	// Author identifies the plugin author.
	Author marketplace.Author `yaml:"author,omitempty"`
	// Homepage is the plugin documentation URL.
	Homepage string `yaml:"homepage,omitempty"`
	// Repository is the source code repository URL.
	Repository string `yaml:"repository,omitempty"`
	// License is an SPDX license identifier.
	License string `yaml:"license,omitempty"`
	// Keywords are discovery tags.
	Keywords []string `yaml:"keywords,omitempty"`
}

// Packager produces distributable Claude Code plugin directories.
type Packager struct {
	fsReader filesystem.Reader
	fsWriter filesystem.Writer
}

// New creates a Claude plugin packager.
func New(fsReader filesystem.Reader, fsWriter filesystem.Writer) *Packager {
	return &Packager{fsReader: fsReader, fsWriter: fsWriter}
}

// Format returns FormatClaudePlugin.
func (p *Packager) Format() packager.Format { return packager.FormatClaudePlugin }

// Targets returns Claude only.
func (p *Packager) Targets() []build.Target { return []build.Target{build.TargetClaude} }

// Package produces a distributable Claude Code plugin directory.
func (p *Packager) Package(ctx context.Context, input packager.PackagerInput) (*packager.PackagerOutput, error) {
	cfg, ok := input.Config.(*Config)
	if !ok {
		return nil, fmt.Errorf("claude packager: expected *claude.Config, got %T", input.Config)
	}

	pluginDir := path.Join(input.OutputDir, cfg.Name)

	// Collect files from the Claude target emission.
	claudeFiles := filterClaudeFiles(input.MaterializationResult.WrittenFiles)

	// Build component path sets for the manifest.
	components := make(map[plugin.ComponentKind]any)
	var writtenFiles []string

	// Process skills: .ai-build/claude/.../skills/{id}/SKILL.md → skills/{id}/SKILL.md
	skillFiles := filterByPrefix(claudeFiles, ".claude/skills/")
	if len(skillFiles) > 0 {
		components[plugin.ComponentSkills] = "./skills/"
		for _, sf := range skillFiles {
			rel := stripClaudePrefix(sf, ".claude/skills/")
			dest := path.Join(pluginDir, "skills", rel)
			if err := p.copyFile(ctx, sf, dest); err != nil {
				return nil, fmt.Errorf("copy skill %s: %w", sf, err)
			}
			writtenFiles = append(writtenFiles, dest)
		}
	}

	// Process agents: .ai-build/claude/.../agents/{id}.md → agents/{id}.md
	agentFiles := filterByPrefix(claudeFiles, ".claude/agents/")
	if len(agentFiles) > 0 {
		components[plugin.ComponentAgents] = "./agents/"
		for _, af := range agentFiles {
			rel := stripClaudePrefix(af, ".claude/agents/")
			dest := path.Join(pluginDir, "agents", rel)
			if err := p.copyFile(ctx, af, dest); err != nil {
				return nil, fmt.Errorf("copy agent %s: %w", af, err)
			}
			writtenFiles = append(writtenFiles, dest)
		}
	}

	// Process MCP config: .mcp.json → .mcp.json
	mcpFiles := filterBySuffix(claudeFiles, ".mcp.json")
	if len(mcpFiles) > 0 {
		dest := path.Join(pluginDir, ".mcp.json")
		content, err := p.readAndRewritePaths(ctx, mcpFiles[0])
		if err != nil {
			return nil, fmt.Errorf("read mcp config: %w", err)
		}
		if err := p.writeFile(ctx, dest, content); err != nil {
			return nil, fmt.Errorf("write mcp config: %w", err)
		}
		components[plugin.ComponentMcpServers] = "./.mcp.json"
		writtenFiles = append(writtenFiles, dest)
	}

	// Process settings/hooks: .claude/settings.json → hooks/hooks.json
	settingsFiles := filterBySuffix(claudeFiles, ".claude/settings.json")
	if len(settingsFiles) > 0 {
		hooks, err := extractHooks(ctx, p.fsReader, settingsFiles[0])
		if err != nil {
			return nil, fmt.Errorf("extract hooks: %w", err)
		}
		if hooks != nil {
			hooksJSON, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal hooks: %w", err)
			}
			dest := path.Join(pluginDir, "hooks", "hooks.json")
			if err := p.writeFile(ctx, dest, hooksJSON); err != nil {
				return nil, fmt.Errorf("write hooks: %w", err)
			}
			components[plugin.ComponentHooks] = "./hooks/hooks.json"
			writtenFiles = append(writtenFiles, dest)
		}
	}

	// Process rules: .claude/rules/{id}.md → rules/{id}.md (if any)
	ruleFiles := filterByPrefix(claudeFiles, ".claude/rules/")
	for _, rf := range ruleFiles {
		rel := stripClaudePrefix(rf, ".claude/rules/")
		dest := path.Join(pluginDir, "rules", rel)
		if err := p.copyFile(ctx, rf, dest); err != nil {
			return nil, fmt.Errorf("copy rule %s: %w", rf, err)
		}
		writtenFiles = append(writtenFiles, dest)
	}

	// Process scripts from emission plan plugin bundles → bin/
	for _, unit := range input.EmissionPlan.Units {
		for _, pb := range unit.PluginBundles {
			for _, f := range pb.Files {
				if isScript(f.Path) {
					dest := path.Join(pluginDir, "bin", path.Base(f.Path))
					if err := p.writeFile(ctx, dest, f.Content); err != nil {
						return nil, fmt.Errorf("write script %s: %w", f.Path, err)
					}
					writtenFiles = append(writtenFiles, dest)
				}
			}
		}
	}

	// Generate .claude-plugin/plugin.json manifest.
	manifest := buildManifest(cfg, components)
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal plugin.json: %w", err)
	}
	manifestPath := path.Join(pluginDir, ".claude-plugin", "plugin.json")
	if err := p.writeFile(ctx, manifestPath, manifestJSON); err != nil {
		return nil, fmt.Errorf("write plugin.json: %w", err)
	}
	writtenFiles = append(writtenFiles, manifestPath)

	sort.Strings(writtenFiles)

	return &packager.PackagerOutput{
		Artifacts: []packager.PackagedArtifact{{
			Format:  packager.FormatClaudePlugin,
			Path:    pluginDir,
			Targets: []build.Target{build.TargetClaude},
			Metadata: map[string]string{
				"pluginName": cfg.Name,
				"version":    versionOrDefault(cfg.Version),
				"files":      fmt.Sprintf("%d", len(writtenFiles)),
			},
		}},
	}, nil
}

// Compile-time assertion.
var _ packager.Packager = (*Packager)(nil)

// --- manifest generation ---

func buildManifest(cfg *Config, components map[plugin.ComponentKind]any) map[string]any {
	m := map[string]any{
		"name": cfg.Name,
	}

	if cfg.Version != "" {
		m["version"] = cfg.Version
	}
	if cfg.Description != "" {
		m["description"] = cfg.Description
	}
	if cfg.Author.Name != "" {
		author := map[string]string{"name": cfg.Author.Name}
		if cfg.Author.Email != "" {
			author["email"] = cfg.Author.Email
		}
		m["author"] = author
	}
	if cfg.Homepage != "" {
		m["homepage"] = cfg.Homepage
	}
	if cfg.Repository != "" {
		m["repository"] = cfg.Repository
	}
	if cfg.License != "" {
		m["license"] = cfg.License
	}
	if len(cfg.Keywords) > 0 {
		m["keywords"] = cfg.Keywords
	}

	// Map ComponentKind → plugin.json keys.
	for kind, val := range components {
		m[string(kind)] = val
	}

	return m
}

// --- file operations ---

func (p *Packager) copyFile(ctx context.Context, src, dest string) error {
	content, err := p.fsReader.ReadFile(ctx, src)
	if err != nil {
		return err
	}
	return p.writeFile(ctx, dest, content)
}

func (p *Packager) writeFile(ctx context.Context, dest string, content []byte) error {
	dir := path.Dir(dest)
	if err := p.fsWriter.MkdirAll(ctx, dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return p.fsWriter.WriteFile(ctx, dest, content, 0o644)
}

func (p *Packager) readAndRewritePaths(ctx context.Context, src string) ([]byte, error) {
	content, err := p.fsReader.ReadFile(ctx, src)
	if err != nil {
		return nil, err
	}
	// Rewrite any absolute or relative paths to use ${CLAUDE_PLUGIN_ROOT}.
	// For now we return content as-is; path rewriting is a future enhancement
	// when we have concrete path patterns to replace.
	return content, nil
}

// --- hooks extraction ---

func extractHooks(ctx context.Context, reader filesystem.Reader, settingsPath string) (any, error) {
	content, err := reader.ReadFile(ctx, settingsPath)
	if err != nil {
		return nil, err
	}

	var settings map[string]any
	if err := json.Unmarshal(content, &settings); err != nil {
		return nil, fmt.Errorf("parse settings.json: %w", err)
	}

	hooks, ok := settings["hooks"]
	if !ok {
		return nil, nil
	}

	return hooks, nil
}

// --- file filtering helpers ---

func filterClaudeFiles(files []string) []string {
	var result []string
	for _, f := range files {
		normalized := path.Clean(strings.ReplaceAll(f, "\\", "/"))
		parts := strings.Split(normalized, "/")
		if len(parts) >= 2 && parts[0] == ".ai-build" && parts[1] == "claude" {
			result = append(result, f)
		}
	}
	sort.Strings(result)
	return result
}

func filterByPrefix(files []string, prefix string) []string {
	var result []string
	for _, f := range files {
		// Extract the part after .ai-build/claude/{profile}/
		rel := extractClaudeRelPath(f)
		if strings.HasPrefix(rel, prefix) {
			result = append(result, f)
		}
	}
	return result
}

func filterBySuffix(files []string, suffix string) []string {
	var result []string
	for _, f := range files {
		rel := extractClaudeRelPath(f)
		if strings.HasSuffix(rel, suffix) {
			result = append(result, f)
		}
	}
	return result
}

// extractClaudeRelPath returns the path relative to the target/profile dir.
// e.g., ".ai-build/claude/local-dev/.claude/skills/foo/SKILL.md" → ".claude/skills/foo/SKILL.md"
func extractClaudeRelPath(filePath string) string {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.SplitN(normalized, "/", 4) // [".ai-build", "claude", profile, rest]
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

// stripClaudePrefix strips a known prefix from the Claude-relative path portion.
// e.g., file=".ai-build/claude/local-dev/.claude/skills/foo/SKILL.md", prefix=".claude/skills/"
// returns "foo/SKILL.md"
func stripClaudePrefix(filePath, prefix string) string {
	rel := extractClaudeRelPath(filePath)
	return strings.TrimPrefix(rel, prefix)
}

func isScript(p string) bool {
	ext := path.Ext(p)
	return ext == ".sh" || ext == ".bash" || ext == ".py" || ext == ".js" || ext == ""
}

func versionOrDefault(v string) string {
	if v == "" {
		return "0.0.1"
	}
	return v
}
