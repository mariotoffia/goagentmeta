package claude_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	generator "github.com/mariotoffia/goagentmeta/internal/adapter/marketplace/claude"
	domain "github.com/mariotoffia/goagentmeta/internal/domain/marketplace"
	portmp "github.com/mariotoffia/goagentmeta/internal/port/marketplace"
)

func TestGenerator_Format(t *testing.T) {
	memfs := filesystem.NewMemFS()
	g := generator.New(memfs)

	if g.Format() != portmp.FormatClaude {
		t.Errorf("expected format %s, got %s", portmp.FormatClaude, g.Format())
	}
}

func TestGenerator_BasicMarketplace(t *testing.T) {
	memfs := filesystem.NewMemFS()
	ctx := context.Background()

	g := generator.New(memfs)

	strict := true
	mp := domain.Marketplace{
		SchemaVersion: 1,
		Name:          "company-tools",
		Owner:         domain.Owner{Name: "DevTools Team", Email: "dev@example.com"},
		Metadata: domain.Metadata{
			Description: "Internal tools for the team",
			Version:     "1.0.0",
		},
		Plugins: []domain.PluginEntry{
			{
				Name: "code-formatter",
				Source: domain.Source{
					Type:     domain.SourceRelativePath,
					Location: "./plugins/formatter",
				},
				Description: "Automatic code formatting",
				Version:     "2.1.0",
				Author:      domain.Author{Name: "DevTools Team"},
				Category:    "development-workflows",
				Tags:        []string{"formatting", "style"},
				Keywords:    []string{"format", "lint"},
			},
			{
				Name: "deploy-tools",
				Source: domain.Source{
					Type:     domain.SourceGitHub,
					Location: "company/deploy-plugin",
					Ref:      "v2.0.0",
				},
				Description: "Deployment automation",
				Category:    "external-integrations",
				Strict:      &strict,
			},
			{
				Name: "npm-plugin",
				Source: domain.Source{
					Type:    domain.SourceNPM,
					Package: "@acme/claude-plugin",
					Version: "^2.0.0",
				},
				Description: "From npm",
			},
			{
				Name: "monorepo-tool",
				Source: domain.Source{
					Type:     domain.SourceGitSubdir,
					Location: "https://github.com/acme-corp/monorepo.git",
					Path:     "tools/claude-plugin",
					Ref:      "main",
					SHA:      "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
				},
			},
			{
				Name: "git-plugin",
				Source: domain.Source{
					Type:     domain.SourceGitURL,
					Location: "https://gitlab.com/team/plugin.git",
				},
			},
		},
	}

	output, err := g.Generate(ctx, portmp.GeneratorInput{
		Marketplace: mp,
		OutputDir:   "dist/marketplace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.CatalogPath != "dist/marketplace/.claude-plugin/marketplace.json" {
		t.Errorf("unexpected catalog path: %s", output.CatalogPath)
	}

	// Parse and validate the generated JSON.
	data, err := memfs.ReadFile(ctx, output.CatalogPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}

	var catalog map[string]any
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}

	// Validate top-level fields.
	if catalog["name"] != "company-tools" {
		t.Errorf("name = %v, want company-tools", catalog["name"])
	}

	owner, ok := catalog["owner"].(map[string]any)
	if !ok {
		t.Fatal("owner is not an object")
	}
	if owner["name"] != "DevTools Team" {
		t.Errorf("owner.name = %v, want DevTools Team", owner["name"])
	}
	if owner["email"] != "dev@example.com" {
		t.Errorf("owner.email = %v, want dev@example.com", owner["email"])
	}

	// Validate plugins.
	plugins, ok := catalog["plugins"].([]any)
	if !ok {
		t.Fatal("plugins is not an array")
	}
	if len(plugins) != 5 {
		t.Fatalf("expected 5 plugins, got %d", len(plugins))
	}

	// Plugins should be sorted by name.
	for i, expected := range []string{"code-formatter", "deploy-tools", "git-plugin", "monorepo-tool", "npm-plugin"} {
		p := plugins[i].(map[string]any)
		if p["name"] != expected {
			t.Errorf("plugin[%d] name = %v, want %s", i, p["name"], expected)
		}
	}

	// Validate relative path source.
	formatter := findPlugin(plugins, "code-formatter")
	if formatter["source"] != "./plugins/formatter" {
		t.Errorf("formatter source = %v, want ./plugins/formatter", formatter["source"])
	}
	if formatter["category"] != "development-workflows" {
		t.Errorf("formatter category = %v, want development-workflows", formatter["category"])
	}

	// Validate GitHub source.
	deploy := findPlugin(plugins, "deploy-tools")
	src, ok := deploy["source"].(map[string]any)
	if !ok {
		t.Fatal("deploy source is not an object")
	}
	if src["source"] != "github" {
		t.Errorf("deploy source.source = %v, want github", src["source"])
	}
	if src["repo"] != "company/deploy-plugin" {
		t.Errorf("deploy source.repo = %v, want company/deploy-plugin", src["repo"])
	}
	if src["ref"] != "v2.0.0" {
		t.Errorf("deploy source.ref = %v, want v2.0.0", src["ref"])
	}
	if deploy["strict"] != true {
		t.Errorf("deploy strict = %v, want true", deploy["strict"])
	}

	// Validate npm source.
	npmPlug := findPlugin(plugins, "npm-plugin")
	npmSrc, ok := npmPlug["source"].(map[string]any)
	if !ok {
		t.Fatal("npm source is not an object")
	}
	if npmSrc["source"] != "npm" {
		t.Errorf("npm source.source = %v, want npm", npmSrc["source"])
	}
	if npmSrc["package"] != "@acme/claude-plugin" {
		t.Errorf("npm source.package = %v, want @acme/claude-plugin", npmSrc["package"])
	}

	// Validate git-subdir source.
	mono := findPlugin(plugins, "monorepo-tool")
	monoSrc, ok := mono["source"].(map[string]any)
	if !ok {
		t.Fatal("monorepo source is not an object")
	}
	if monoSrc["source"] != "git-subdir" {
		t.Errorf("mono source.source = %v, want git-subdir", monoSrc["source"])
	}
	if monoSrc["path"] != "tools/claude-plugin" {
		t.Errorf("mono source.path = %v, want tools/claude-plugin", monoSrc["path"])
	}
	if monoSrc["sha"] != "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0" {
		t.Errorf("mono source.sha = %v", monoSrc["sha"])
	}

	// Validate git URL source.
	gitPlug := findPlugin(plugins, "git-plugin")
	gitSrc, ok := gitPlug["source"].(map[string]any)
	if !ok {
		t.Fatal("git source is not an object")
	}
	if gitSrc["source"] != "url" {
		t.Errorf("git source.source = %v, want url", gitSrc["source"])
	}
	if gitSrc["url"] != "https://gitlab.com/team/plugin.git" {
		t.Errorf("git source.url = %v", gitSrc["url"])
	}
}

func TestGenerator_EmptyMarketplace(t *testing.T) {
	memfs := filesystem.NewMemFS()
	ctx := context.Background()
	g := generator.New(memfs)

	output, err := g.Generate(ctx, portmp.GeneratorInput{
		Marketplace: domain.Marketplace{
			Name:  "empty",
			Owner: domain.Owner{Name: "Test"},
		},
		OutputDir: "dist",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := memfs.ReadFile(ctx, output.CatalogPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}

	var catalog map[string]any
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}

	plugins := catalog["plugins"].([]any)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestGenerator_ComponentsAndExtras(t *testing.T) {
	memfs := filesystem.NewMemFS()
	ctx := context.Background()
	g := generator.New(memfs)

	mp := domain.Marketplace{
		Name:  "extended",
		Owner: domain.Owner{Name: "Test"},
		Plugins: []domain.PluginEntry{
			{
				Name: "extended-plugin",
				Source: domain.Source{
					Type:     domain.SourceRelativePath,
					Location: "./extended",
				},
				Components: map[string]any{
					"skills": []string{"./skills/", "./extra-skills/"},
					"agents": "./agents/reviewer.md",
					"hooks":  map[string]any{"PostToolUse": []any{}},
				},
				Extra: map[string]any{
					"customField": "customValue",
				},
			},
		},
	}

	output, err := g.Generate(ctx, portmp.GeneratorInput{
		Marketplace: mp,
		OutputDir:   "dist",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := memfs.ReadFile(ctx, output.CatalogPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}

	var catalog map[string]any
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}

	plugins := catalog["plugins"].([]any)
	p := plugins[0].(map[string]any)

	// Components should be merged into the plugin entry.
	if p["agents"] != "agents/reviewer.md" {
		// Components are forwarded as-is.
		if p["agents"] != "./agents/reviewer.md" {
			t.Errorf("agents = %v, expected ./agents/reviewer.md", p["agents"])
		}
	}

	// Extra should be merged.
	if p["customField"] != "customValue" {
		t.Errorf("customField = %v, want customValue", p["customField"])
	}
}

// findPlugin finds a plugin by name in the parsed plugins array.
func findPlugin(plugins []any, name string) map[string]any {
	for _, p := range plugins {
		pm := p.(map[string]any)
		if pm["name"] == name {
			return pm
		}
	}
	return nil
}
