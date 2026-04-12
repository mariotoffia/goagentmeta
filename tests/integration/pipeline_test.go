package integration_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ─── Golden Output Integration Tests ───────────────────────────────────────
//
// These tests exercise the full pipeline using real stages (validate →
// normalize → plan → capability → lower → render → materialize → report)
// against fixture .ai/ directories and compare materialized output to
// golden files.
//
// Run with -update to regenerate golden files:
//   go test ./tests/integration/ -run Integration -update -count=1
//
// The Makefile target `make test-integration` uses `-run Integration` to
// match these tests.

// TestIntegrationExampleInstruction runs the full pipeline on a minimal
// single-instruction fixture and compares output for each target.
func TestIntegrationExampleInstruction(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
	compareGoldenOutput(t, fixture, pr.MemFS)
}

// TestIntegrationScopedRule runs the full pipeline on a fixture with an
// instruction and a scoped rule that has conditions.
func TestIntegrationScopedRule(t *testing.T) {
	fixture := "example-02-rule"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
	compareGoldenOutput(t, fixture, pr.MemFS)
}

// TestIntegrationFullProject runs the full pipeline on a complex fixture
// containing instructions, rules, skills, agents, hooks, and plugins.
func TestIntegrationFullProject(t *testing.T) {
	fixture := "full-project"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
	compareGoldenOutput(t, fixture, pr.MemFS)
}

// TestIntegrationSingleTargetClaude runs the pipeline targeting only Claude.
func TestIntegrationSingleTargetClaude(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	files := pr.MemFS.Files()

	// Only Claude output should be present.
	for path := range files {
		if !hasPrefix(path, ".ai-build/claude/") {
			t.Errorf("unexpected non-claude file: %s", path)
		}
	}
	if len(files) == 0 {
		t.Error("expected at least one file written for claude target")
	}
}

// TestIntegrationSingleTargetCopilot runs the pipeline targeting only Copilot.
func TestIntegrationSingleTargetCopilot(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	targets := []build.Target{build.TargetCopilot}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	files := pr.MemFS.Files()

	for path := range files {
		if !hasPrefix(path, ".ai-build/copilot/") {
			t.Errorf("unexpected non-copilot file: %s", path)
		}
	}
	if len(files) == 0 {
		t.Error("expected at least one file written for copilot target")
	}
}

// TestIntegrationSingleTargetCodex runs the pipeline targeting only Codex.
func TestIntegrationSingleTargetCodex(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	targets := []build.Target{build.TargetCodex}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	files := pr.MemFS.Files()

	for path := range files {
		if !hasPrefix(path, ".ai-build/codex/") {
			t.Errorf("unexpected non-codex file: %s", path)
		}
	}
	if len(files) == 0 {
		t.Error("expected at least one file written for codex target")
	}
}

// ─── Behavioral Integration Tests ──────────────────────────────────────────

// TestIntegrationDeterministicOutput verifies that running the pipeline
// twice on the same input produces byte-identical output.
func TestIntegrationDeterministicOutput(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))

	// Run 1.
	pr1 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr1)
	files1 := pr1.MemFS.Files()

	// Run 2.
	pr2 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr2)
	files2 := pr2.MemFS.Files()

	// Compare file sets.
	if len(files1) != len(files2) {
		t.Fatalf("determinism: run1 produced %d files, run2 produced %d", len(files1), len(files2))
	}
	for path, content1 := range files1 {
		content2, ok := files2[path]
		if !ok {
			t.Errorf("determinism: file %s exists in run1 but not run2", path)
			continue
		}
		if !bytes.Equal(content1, content2) {
			t.Errorf("determinism: file %s differs between runs\n  run1: %s\n  run2: %s",
				path, string(content1), string(content2))
		}
	}
	for path := range files2 {
		if _, ok := files1[path]; !ok {
			t.Errorf("determinism: file %s exists in run2 but not run1", path)
		}
	}
}

// TestIntegrationDryRun verifies that dry-run mode writes zero files.
func TestIntegrationDryRun(t *testing.T) {
	fixture := "full-project"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, true)
	report := runPipeline(t, pr)

	files := pr.MemFS.Files()
	if len(files) > 0 {
		t.Errorf("dry-run should produce zero files, got %d:", len(files))
		for path := range files {
			t.Logf("  %s", path)
		}
	}

	// Report should still exist and contain diagnostics about what WOULD be written.
	if report == nil {
		t.Fatal("expected BuildReport even in dry-run mode")
	}

	// Check for dry-run diagnostic.
	hasDryRunDiag := false
	for _, d := range report.Diagnostics {
		if d.Code == "MATERIALIZE_DRY_RUN" {
			hasDryRunDiag = true
			break
		}
	}
	if !hasDryRunDiag {
		t.Error("expected MATERIALIZE_DRY_RUN diagnostic in report")
	}
}

// TestIntegrationContextCancellation verifies that cancelling the context
// mid-pipeline produces a clean error without panics.
func TestIntegrationContextCancellation(t *testing.T) {
	fixture := "full-project"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give the context time to expire.
	time.Sleep(1 * time.Millisecond)

	_, err := pr.Pipeline.Execute(ctx, []string{"."})
	if err == nil {
		t.Fatal("expected error from cancelled context pipeline execution")
	}
	if ctx.Err() == nil {
		t.Error("context should be done")
	}
}

// TestIntegrationBuildReportPopulated verifies the BuildReport has expected
// fields populated after a successful run.
func TestIntegrationBuildReportPopulated(t *testing.T) {
	fixture := "full-project"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	if report.Timestamp.IsZero() {
		t.Error("report.Timestamp should be set")
	}
	if report.Duration <= 0 {
		t.Error("report.Duration should be positive")
	}
	if len(report.Diagnostics) == 0 {
		t.Error("expected at least some diagnostics (info-level render messages)")
	}
}

// TestIntegrationObjectsBridged verifies the normalize hook correctly
// populates the shared objects map used by downstream stages.
func TestIntegrationObjectsBridged(t *testing.T) {
	fixture := "full-project"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	// The objects map should be populated with all fixture objects.
	expectedIDs := []string{"go-standards", "go-testing", "github-mcp",
		"implementer", "post-edit-lint", "project-overview", "secrets-policy"}
	for _, id := range expectedIDs {
		if _, ok := pr.Objects[id]; !ok {
			t.Errorf("expected object %q in shared objects map", id)
		}
	}
}

// TestIntegrationMultiTargetOutput verifies that all targets produce output
// when running with all targets enabled.
func TestIntegrationMultiTargetOutput(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	files := pr.MemFS.Files()

	targets := map[string]bool{"claude": false, "copilot": false, "codex": false}
	for path := range files {
		for target := range targets {
			if hasPrefix(path, ".ai-build/"+target+"/") {
				targets[target] = true
			}
		}
	}
	for target, found := range targets {
		if !found {
			t.Errorf("expected output files for target %s", target)
		}
	}
}

// TestIntegrationProvenanceGenerated verifies provenance.json is generated
// for each build unit.
func TestIntegrationProvenanceGenerated(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	files := pr.MemFS.Files()
	provenanceCount := 0
	for path := range files {
		if filepath.Base(path) == "provenance.json" {
			provenanceCount++
		}
	}
	if provenanceCount == 0 {
		t.Error("expected at least one provenance.json file")
	}
}

// TestIntegrationCIProfile verifies the CI profile produces deterministic output.
func TestIntegrationCIProfile(t *testing.T) {
	fixture := "example-01-instruction"
	tree := parseFixtureDir(t, filepath.Join(fixturesDir(), fixture))

	// Run with CI profile.
	pr1 := buildTestPipeline(t, tree, nil, build.ProfileCI, false)
	runPipeline(t, pr1)
	files1 := pr1.MemFS.Files()

	pr2 := buildTestPipeline(t, tree, nil, build.ProfileCI, false)
	runPipeline(t, pr2)
	files2 := pr2.MemFS.Files()

	if len(files1) != len(files2) {
		t.Fatalf("CI profile not deterministic: %d vs %d files", len(files1), len(files2))
	}
	for path, content1 := range files1 {
		content2, ok := files2[path]
		if !ok {
			t.Errorf("CI: file %s in run1 but not run2", path)
			continue
		}
		if !bytes.Equal(content1, content2) {
			t.Errorf("CI: file %s differs between runs", path)
		}
	}
}

// ─── Test Helpers ──────────────────────────────────────────────────────────

func assertNoErrors(t *testing.T, report *pipeline.BuildReport) {
	t.Helper()
	for _, d := range report.Diagnostics {
		if d.Severity == "error" {
			t.Errorf("unexpected error diagnostic: [%s] %s (phase: %s)",
				d.Code, d.Message, d.Phase)
		}
	}
}

func assertFilesWritten(t *testing.T, memfs interface{ Files() map[string][]byte }) {
	t.Helper()
	files := memfs.Files()
	if len(files) == 0 {
		t.Fatal("expected at least one file to be materialized")
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
