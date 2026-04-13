package packaging_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	claudepkg "github.com/mariotoffia/goagentmeta/internal/adapter/packager/claude"
	npmpkg "github.com/mariotoffia/goagentmeta/internal/adapter/packager/npm"
	vscodepkg "github.com/mariotoffia/goagentmeta/internal/adapter/packager/vscode"
	"github.com/mariotoffia/goagentmeta/internal/application/packaging"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/marketplace"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
)

func TestPackageWithEmission_Registry(t *testing.T) {
	memfs := filesystem.NewMemFS()
	ctx := context.Background()

	// Seed files.
	seedFile(t, memfs, ".ai-build/copilot/local-dev/instructions.md", "# Copilot")
	seedFile(t, memfs, ".ai-build/claude/local-dev/.claude/skills/test/SKILL.md", "---\nname: test\n---\nTest skill.")

	result := pipeline.MaterializationResult{
		WrittenFiles: []string{
			".ai-build/copilot/local-dev/instructions.md",
			".ai-build/claude/local-dev/.claude/skills/test/SKILL.md",
		},
	}
	emission := pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}}

	// Create registry with all packagers.
	reg := packager.NewRegistry()
	reg.MustRegister(vscodepkg.New(memfs, memfs))
	reg.MustRegister(npmpkg.New(memfs, memfs))
	reg.MustRegister(claudepkg.New(memfs, memfs))

	svc := packaging.NewPackagingService(memfs, memfs, packaging.WithRegistry(reg))

	configs := map[packager.Format]any{
		packager.FormatVSIX: &vscodepkg.Config{
			Publisher: "test-pub",
			Targets:   []build.Target{build.TargetCopilot},
			Version:   "1.0.0",
		},
		packager.FormatClaudePlugin: &claudepkg.Config{
			Name:    "test-plugin",
			Version: "1.0.0",
			Author:  marketplace.Author{Name: "Test"},
		},
	}

	pkgResult, err := svc.PackageWithEmission(ctx, emission, result, configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pkgResult.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(pkgResult.Artifacts))
	}

	types := map[string]bool{}
	for _, a := range pkgResult.Artifacts {
		types[a.Type] = true
	}
	if !types["vsix"] {
		t.Error("missing vsix artifact")
	}
	if !types["claude-plugin"] {
		t.Error("missing claude-plugin artifact")
	}
}

func TestPackageWithEmission_NoRegistry(t *testing.T) {
	memfs := filesystem.NewMemFS()
	svc := packaging.NewPackagingService(memfs, memfs)

	_, err := svc.PackageWithEmission(
		context.Background(),
		pipeline.EmissionPlan{},
		pipeline.MaterializationResult{},
		map[packager.Format]any{},
	)
	if err == nil {
		t.Fatal("expected error when no registry configured")
	}
}

func TestPackageWithEmission_UnknownFormatSkipped(t *testing.T) {
	memfs := filesystem.NewMemFS()

	reg := packager.NewRegistry()
	svc := packaging.NewPackagingService(memfs, memfs, packaging.WithRegistry(reg))

	configs := map[packager.Format]any{
		packager.Format("unknown-format"): nil,
	}

	pkgResult, err := svc.PackageWithEmission(
		context.Background(),
		pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}},
		pipeline.MaterializationResult{},
		configs,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgResult.Artifacts) != 0 {
		t.Errorf("expected 0 artifacts for unknown format, got %d", len(pkgResult.Artifacts))
	}
}

func TestPackageWithEmission_ClaudePluginManifest(t *testing.T) {
	memfs := filesystem.NewMemFS()
	ctx := context.Background()

	seedFile(t, memfs, ".ai-build/claude/local-dev/.claude/skills/hello/SKILL.md",
		"---\nname: hello\n---\nHello world skill.")

	result := pipeline.MaterializationResult{
		WrittenFiles: []string{
			".ai-build/claude/local-dev/.claude/skills/hello/SKILL.md",
		},
	}

	reg := packager.NewRegistry()
	reg.MustRegister(claudepkg.New(memfs, memfs))

	svc := packaging.NewPackagingService(memfs, memfs, packaging.WithRegistry(reg))
	pkgResult, err := svc.PackageWithEmission(ctx,
		pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}},
		result,
		map[packager.Format]any{
			packager.FormatClaudePlugin: &claudepkg.Config{
				Name:        "hello-plugin",
				Version:     "2.0.0",
				Description: "A hello plugin",
				Keywords:    []string{"hello", "greeting"},
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pkgResult.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(pkgResult.Artifacts))
	}

	// Verify generated manifest.
	manifestData, err := memfs.ReadFile(ctx, ".ai-build/dist/hello-plugin/.claude-plugin/plugin.json")
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("parse plugin.json: %v", err)
	}
	if manifest["name"] != "hello-plugin" {
		t.Errorf("name = %v, want hello-plugin", manifest["name"])
	}
	if manifest["description"] != "A hello plugin" {
		t.Errorf("description = %v, want A hello plugin", manifest["description"])
	}
	if manifest["skills"] != "./skills/" {
		t.Errorf("skills = %v, want ./skills/", manifest["skills"])
	}
}

func seedFile(t *testing.T, memfs *filesystem.MemFS, path, content string) {
	t.Helper()
	if err := memfs.WriteFile(context.Background(), path, []byte(content), 0o644); err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
}
