package claude_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	claude "github.com/mariotoffia/goagentmeta/internal/adapter/packager/claude"
	"github.com/mariotoffia/goagentmeta/internal/domain/marketplace"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
)

func TestClaudePackager_Format(t *testing.T) {
	memfs := filesystem.NewMemFS()
	p := claude.New(memfs, memfs)

	if p.Format() != packager.FormatClaudePlugin {
		t.Errorf("expected format %s, got %s", packager.FormatClaudePlugin, p.Format())
	}
}

func TestClaudePackager_BasicPlugin(t *testing.T) {
	memfs := filesystem.NewMemFS()
	ctx := context.Background()

	// Seed Claude-target files.
	seedFile(t, memfs, ".ai-build/claude/local-dev/.claude/skills/code-review/SKILL.md",
		"---\nname: code-review\ndescription: Reviews code\n---\nReview code for issues.")
	seedFile(t, memfs, ".ai-build/claude/local-dev/.claude/agents/security.md",
		"---\nname: security\n---\nSecurity agent.")
	seedFile(t, memfs, ".ai-build/claude/local-dev/.mcp.json",
		`{"mcpServers":{"github":{"command":"npx","args":["-y","@mcp/github"]}}}`)
	seedFile(t, memfs, ".ai-build/claude/local-dev/.claude/settings.json",
		`{"hooks":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"lint.sh"}]}]}}`)

	result := pipeline.MaterializationResult{
		WrittenFiles: []string{
			".ai-build/claude/local-dev/.claude/skills/code-review/SKILL.md",
			".ai-build/claude/local-dev/.claude/agents/security.md",
			".ai-build/claude/local-dev/.mcp.json",
			".ai-build/claude/local-dev/.claude/settings.json",
		},
	}

	p := claude.New(memfs, memfs)
	output, err := p.Package(ctx, packager.PackagerInput{
		EmissionPlan:          pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}},
		MaterializationResult: result,
		Config: &claude.Config{
			Name:        "my-plugin",
			Version:     "1.0.0",
			Description: "A test plugin",
			Author:      marketplace.Author{Name: "Test Author"},
			License:     "MIT",
			Keywords:    []string{"testing", "code-review"},
		},
		OutputDir: ".ai-build/dist",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(output.Artifacts))
	}

	art := output.Artifacts[0]
	if art.Format != packager.FormatClaudePlugin {
		t.Errorf("expected format %s, got %s", packager.FormatClaudePlugin, art.Format)
	}
	if art.Metadata["pluginName"] != "my-plugin" {
		t.Errorf("expected pluginName my-plugin, got %s", art.Metadata["pluginName"])
	}

	// Verify plugin.json manifest was created.
	manifestData, err := memfs.ReadFile(ctx, ".ai-build/dist/my-plugin/.claude-plugin/plugin.json")
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("parse plugin.json: %v", err)
	}
	if manifest["name"] != "my-plugin" {
		t.Errorf("manifest name = %v, want my-plugin", manifest["name"])
	}
	if manifest["version"] != "1.0.0" {
		t.Errorf("manifest version = %v, want 1.0.0", manifest["version"])
	}
	if manifest["license"] != "MIT" {
		t.Errorf("manifest license = %v, want MIT", manifest["license"])
	}

	// Verify skills were copied.
	skillData, err := memfs.ReadFile(ctx, ".ai-build/dist/my-plugin/skills/code-review/SKILL.md")
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(skillData), "code-review") {
		t.Error("skill content missing expected text")
	}

	// Verify agents were copied.
	agentData, err := memfs.ReadFile(ctx, ".ai-build/dist/my-plugin/agents/security.md")
	if err != nil {
		t.Fatalf("read agent: %v", err)
	}
	if !strings.Contains(string(agentData), "Security agent") {
		t.Error("agent content missing expected text")
	}

	// Verify MCP config was copied.
	mcpData, err := memfs.ReadFile(ctx, ".ai-build/dist/my-plugin/.mcp.json")
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	if !strings.Contains(string(mcpData), "mcpServers") {
		t.Error("MCP config missing expected content")
	}

	// Verify hooks were extracted.
	hooksData, err := memfs.ReadFile(ctx, ".ai-build/dist/my-plugin/hooks/hooks.json")
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}
	var hooks map[string]any
	if err := json.Unmarshal(hooksData, &hooks); err != nil {
		t.Fatalf("parse hooks.json: %v", err)
	}
	if _, ok := hooks["hooks"]; !ok {
		t.Error("hooks.json missing 'hooks' key")
	}
}

func TestClaudePackager_MinimalConfig(t *testing.T) {
	memfs := filesystem.NewMemFS()
	ctx := context.Background()

	result := pipeline.MaterializationResult{WrittenFiles: []string{}}
	p := claude.New(memfs, memfs)

	output, err := p.Package(ctx, packager.PackagerInput{
		EmissionPlan:          pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}},
		MaterializationResult: result,
		Config:                &claude.Config{Name: "minimal"},
		OutputDir:             ".ai-build/dist",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(output.Artifacts))
	}

	// Should still produce a plugin.json.
	manifestData, err := memfs.ReadFile(ctx, ".ai-build/dist/minimal/.claude-plugin/plugin.json")
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("parse plugin.json: %v", err)
	}
	if manifest["name"] != "minimal" {
		t.Errorf("manifest name = %v, want minimal", manifest["name"])
	}
}

func TestClaudePackager_WrongConfigType(t *testing.T) {
	memfs := filesystem.NewMemFS()
	p := claude.New(memfs, memfs)

	_, err := p.Package(context.Background(), packager.PackagerInput{
		Config:    "not-a-config",
		OutputDir: "out",
	})
	if err == nil {
		t.Fatal("expected error for wrong config type")
	}
}

func seedFile(t *testing.T, memfs *filesystem.MemFS, path, content string) {
	t.Helper()
	if err := memfs.WriteFile(context.Background(), path, []byte(content), 0o644); err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
}
