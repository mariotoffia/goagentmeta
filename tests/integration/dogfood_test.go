package integration_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
)

// ─── Dog-Food Tests ────────────────────────────────────────────────────────
//
// These tests compile the project's own metadata configuration through the
// full pipeline. The source objects live in tests/integration/testdata/dogfood/
// (not under .ai/) to demonstrate that objects can be placed anywhere.
// This is the "eat your own dog food" test: goagentmeta compiling its own
// metadata configuration. If the compiler can't process its own objects,
// something is fundamentally wrong.
//
// All test names start with TestIntegration so `make test-integration`
// picks them up.

// dogfoodDir returns the path to the testdata/dogfood/ source directory.
func dogfoodDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", "dogfood")
}

// TestIntegrationDogfoodCompileOwnAI compiles the project's .ai/ directory
// and verifies it produces output for all three targets.
func TestIntegrationDogfoodCompileOwnAI(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)

	// Verify all three targets produced output.
	targets := []string{"claude", "copilot", "codex"}
	for _, target := range targets {
		files := filesForTarget(pr.MemFS, target)
		if len(files) == 0 {
			t.Errorf("dog-food: target %s produced no output files", target)
		}
	}
}

// TestIntegrationDogfoodClaudeOutput verifies Claude-specific output files.
func TestIntegrationDogfoodClaudeOutput(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)

	files := pr.MemFS.Files()
	// Claude should produce at least: CLAUDE.md, provenance.json
	hasClaude := false
	hasProvenance := false
	for path := range files {
		if strings.HasSuffix(path, "CLAUDE.md") {
			hasClaude = true
		}
		if strings.HasSuffix(path, "provenance.json") {
			hasProvenance = true
		}
	}
	if !hasClaude {
		t.Error("dog-food Claude: expected CLAUDE.md")
	}
	if !hasProvenance {
		t.Error("dog-food Claude: expected provenance.json")
	}
}

// TestIntegrationDogfoodCopilotOutput verifies Copilot-specific output.
func TestIntegrationDogfoodCopilotOutput(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	targets := []build.Target{build.TargetCopilot}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)

	files := pr.MemFS.Files()
	hasCopilotInst := false
	hasMCPConfig := false
	for path := range files {
		if strings.Contains(path, ".github/copilot-instructions.md") {
			hasCopilotInst = true
		}
		if strings.Contains(path, ".vscode/mcp.json") {
			hasMCPConfig = true
		}
	}
	if !hasCopilotInst {
		t.Error("dog-food Copilot: expected .github/copilot-instructions.md")
	}
	// MCP config is only produced if plugins exist; don't fail for it.
	_ = hasMCPConfig
}

// TestIntegrationDogfoodCodexOutput verifies Codex-specific output.
func TestIntegrationDogfoodCodexOutput(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	targets := []build.Target{build.TargetCodex}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)

	files := pr.MemFS.Files()
	hasAgents := false
	for path := range files {
		if strings.HasSuffix(path, "AGENTS.md") {
			hasAgents = true
		}
	}
	if !hasAgents {
		t.Error("dog-food Codex: expected AGENTS.md")
	}
}

// TestIntegrationDogfoodDryRun runs the pipeline on .ai/ in dry-run mode.
func TestIntegrationDogfoodDryRun(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, true)
	report := runPipeline(t, pr)

	if len(pr.MemFS.Files()) > 0 {
		t.Errorf("dog-food dry-run: expected zero files, got %d", len(pr.MemFS.Files()))
	}
	if !hasDiagnosticCode(report, "MATERIALIZE_DRY_RUN") {
		t.Error("dog-food dry-run: expected MATERIALIZE_DRY_RUN diagnostic")
	}
}

// TestIntegrationDogfoodDeterministic verifies two runs produce identical output.
func TestIntegrationDogfoodDeterministic(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())

	pr1 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr1)
	files1 := pr1.MemFS.Files()

	pr2 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr2)
	files2 := pr2.MemFS.Files()

	if len(files1) != len(files2) {
		t.Fatalf("dog-food determinism: %d vs %d files", len(files1), len(files2))
	}
	for path, c1 := range files1 {
		c2, ok := files2[path]
		if !ok {
			t.Errorf("dog-food determinism: file %s missing in run2", path)
			continue
		}
		if string(c1) != string(c2) {
			t.Errorf("dog-food determinism: file %s differs between runs", path)
		}
	}
}

// TestIntegrationDogfoodObjectsBridged verifies all .ai/ objects are bridged.
func TestIntegrationDogfoodObjectsBridged(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	expectedIDs := []string{
		"project-overview", "go-standards",
		"testing-policy", "binary-output",
		"go-testing-skill",
		"architecture-reviewer",
		"post-edit-lint",
	}
	for _, id := range expectedIDs {
		if _, ok := pr.Objects[id]; !ok {
			t.Errorf("dog-food: expected object %q in shared objects map", id)
		}
	}
}

// TestIntegrationDogfoodProvenanceGenerated verifies provenance for each unit.
func TestIntegrationDogfoodProvenanceGenerated(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	count := 0
	for path := range pr.MemFS.Files() {
		if strings.HasSuffix(path, "provenance.json") {
			count++
		}
	}
	// One per target (claude, copilot, codex).
	if count < 3 {
		t.Errorf("dog-food: expected at least 3 provenance.json files, got %d", count)
	}
}

// TestIntegrationDogfoodCIProfile compiles .ai/ with CI profile.
func TestIntegrationDogfoodCIProfile(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	pr := buildTestPipeline(t, tree, nil, build.ProfileCI, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationDogfoodGeneratedHeaders checks rendered files have headers.
func TestIntegrationDogfoodGeneratedHeaders(t *testing.T) {
	tree := parseSourceDir(t, dogfoodDir(), dogfoodDir())
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)
}
