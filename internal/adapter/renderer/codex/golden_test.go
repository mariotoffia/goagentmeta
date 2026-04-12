package codex_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/codex"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func goldenDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "golden")
}

func goldenPath(dir, filename string) string {
	return filepath.Join(goldenDir(), dir, filename)
}

func pathToGoldenFilename(p string) string {
	return strings.ReplaceAll(p, "/", "__")
}

func updateOrCompareGolden(t *testing.T, dir, filename string, actual []byte) {
	t.Helper()

	fp := goldenPath(dir, filename)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(fp, actual, 0o644); err != nil {
			t.Fatalf("write golden file %s: %v", fp, err)
		}
		t.Logf("updated golden file: %s", fp)
		return
	}

	expected, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with -update to generate)", fp, err)
	}

	if !bytes.Equal(expected, actual) {
		t.Errorf("golden mismatch for %s\n--- expected (golden) ---\n%s\n--- actual ---\n%s",
			fp, string(expected), string(actual))
	}
}

// ---------------------------------------------------------------------------
// Golden tests
// ---------------------------------------------------------------------------

func TestGolden_MinimalInstruction(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"arch-inst": keptInstruction(
			"arch-inst",
			"Always use Go modules.\n\nPrefer table-driven tests.",
			nil,
		),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		gf := pathToGoldenFilename(f.Path)
		updateOrCompareGolden(t, "minimal-instruction", gf, f.Content)
	}
}

func TestGolden_FullProject(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"arch-instruction": keptInstruction("arch-instruction", "Use hexagonal architecture.", nil),
		"api-instruction":  keptInstruction("api-instruction", "Follow REST conventions.", []string{"services/api"}),
		"go-rule": keptRule("go-rule", "Use gofmt and golangci-lint.", []string{"**/*.go"}, []model.RuleCondition{
			{Type: "language", Value: "go"},
		}),
		"iam-skill": keptSkill("iam-skill", "Review IAM policies thoroughly.", map[string]any{
			"description":     "IAM review skill",
			"activationHints": []any{"IAM", "security"},
			"allowedTools":    []any{"Read"},
		}),
		"review-agent": keptAgent("review-agent", "You specialize in code review.", map[string]any{
			"description": "Review agent",
			"skills":      []any{"iam-skill"},
			"delegation":  map[string]any{"mayCall": []any{"deploy-agent"}},
		}),
		"start-hook":    keptHook("start-hook", "SessionStart", "command", "echo started", nil),
		"github-plugin": keptPlugin("github-plugin", map[string]any{
			"mcpServers": map[string]any{
				"github": map[string]any{
					"transport": "stdio",
					"command":   "npx",
					"args":      []any{"-y", "@modelcontextprotocol/server-github"},
				},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		gf := pathToGoldenFilename(f.Path)
		updateOrCompareGolden(t, "full-project", gf, f.Content)
	}
}

func TestGolden_HooksAndMCP(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"session-hook":  keptHook("session-hook", "SessionStart", "command", "echo starting", nil),
		"pre-tool-hook": keptHook("pre-tool-hook", "PreToolUse", "command", "validate-tool", []string{"**/*.go"}),
		"stop-hook":     keptHook("stop-hook", "Stop", "command", "cleanup", nil),
		"github-mcp": keptPlugin("github-mcp", map[string]any{
			"mcpServers": map[string]any{
				"github": map[string]any{
					"transport": "stdio",
					"command":   "npx",
					"args":      []any{"-y", "@modelcontextprotocol/server-github"},
					"env": map[string]any{
						"GITHUB_TOKEN": "${GITHUB_TOKEN}",
					},
				},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		gf := pathToGoldenFilename(f.Path)
		updateOrCompareGolden(t, "hooks-and-mcp", gf, f.Content)
	}
}
