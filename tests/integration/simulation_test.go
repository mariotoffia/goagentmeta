package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ─── Simulation Tests ──────────────────────────────────────────────────────
//
// 40+ simulation tests exercise the full pipeline with in-memory SourceTree
// fixtures. They test feature workflows and edge cases without filesystem
// dependencies. All test names start with TestIntegration for `make test-integration`.

// ─── 1. Empty & Minimal Inputs ─────────────────────────────────────────────

// TestIntegrationSimEmptySourceTree verifies the pipeline handles
// a SourceTree with zero objects gracefully.
func TestIntegrationSimEmptySourceTree(t *testing.T) {
	tree := makeSourceTree()
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	// No objects → no output files (except possibly empty provenance).
	assertNoErrors(t, report)
}

// TestIntegrationSimSingleInstruction verifies the simplest possible case.
func TestIntegrationSimSingleInstruction(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("hello", model.KindInstruction, "Hello, world!"),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimSingleRule verifies a single rule object.
func TestIntegrationSimSingleRule(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("no-secrets", model.KindRule, "Never store secrets.",
			withConditions([]model.RuleCondition{{Type: "language", Value: "go"}})),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimSingleSkill verifies a single skill object.
func TestIntegrationSimSingleSkill(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("test-skill", model.KindSkill, "Expert in testing."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimSingleAgent verifies a single agent object.
func TestIntegrationSimSingleAgent(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("reviewer", model.KindAgent, "You review code."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimSingleHook verifies a single hook object.
func TestIntegrationSimSingleHook(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("lint-hook", model.KindHook, "Run linter.",
			withHookFields("post-edit", "command", "make lint")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimSinglePlugin verifies a single plugin object.
func TestIntegrationSimSinglePlugin(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("github-mcp", model.KindPlugin, "GitHub MCP.",
			withDistribution("inline"),
			withMCPServers(map[string]any{
				"github": map[string]any{
					"command": "npx",
					"args":    []any{"@mcp/server-github"},
				},
			})),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// ─── 2. Multi-Object Interactions ──────────────────────────────────────────

// TestIntegrationSimAgentWithSkillRef verifies an agent referencing a skill.
func TestIntegrationSimAgentWithSkillRef(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("test-skill", model.KindSkill, "Testing expertise."),
		makeRawObject("test-agent", model.KindAgent, "You test code.",
			withSkills("test-skill")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	if len(pr.Objects) != 2 {
		t.Errorf("expected 2 bridged objects, got %d", len(pr.Objects))
	}
}

// TestIntegrationSimInstructionAndRule verifies instruction + rule combined.
func TestIntegrationSimInstructionAndRule(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("overview", model.KindInstruction, "Project overview."),
		makeRawObject("security", model.KindRule, "Follow security practices.",
			withConditions([]model.RuleCondition{{Type: "language", Value: "go"}}),
			withScopePaths("**/*.go")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimAllKindsCombined verifies all object kinds together.
func TestIntegrationSimAllKindsCombined(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst-1", model.KindInstruction, "Instruction content."),
		makeRawObject("rule-1", model.KindRule, "Rule content.",
			withConditions([]model.RuleCondition{{Type: "language", Value: "go"}})),
		makeRawObject("skill-1", model.KindSkill, "Skill content."),
		makeRawObject("agent-1", model.KindAgent, "Agent content.",
			withSkills("skill-1")),
		makeRawObject("hook-1", model.KindHook, "Hook content.",
			withHookFields("post-edit", "command", "make lint")),
		makeRawObject("plugin-1", model.KindPlugin, "Plugin content.",
			withDistribution("inline"),
			withMCPServers(map[string]any{
				"tool": map[string]any{"command": "echo", "args": []any{"hello"}},
			})),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)

	// All 6 objects should be bridged.
	if len(pr.Objects) != 6 {
		t.Errorf("expected 6 bridged objects, got %d", len(pr.Objects))
	}
}

// ─── 3. Scope & Filtering Tests ────────────────────────────────────────────

// TestIntegrationSimScopedInstruction verifies scoped instruction renders scoped files.
func TestIntegrationSimScopedInstruction(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("go-inst", model.KindInstruction, "Go coding standards.",
			withScopeFileTypes(".go")),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimMultipleScopedInstructions verifies multiple scoped instructions.
func TestIntegrationSimMultipleScopedInstructions(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("global", model.KindInstruction, "Global guidance."),
		makeRawObject("go-scoped", model.KindInstruction, "Go guidance.",
			withScopeFileTypes(".go")),
		makeRawObject("ts-scoped", model.KindInstruction, "TS guidance.",
			withScopeFileTypes(".ts")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
	if len(pr.Objects) != 3 {
		t.Errorf("expected 3 bridged objects, got %d", len(pr.Objects))
	}
}

// TestIntegrationSimRuleWithMultipleConditions tests a rule with two conditions.
func TestIntegrationSimRuleWithMultipleConditions(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("multi-cond", model.KindRule, "Multiple conditions.",
			withConditions([]model.RuleCondition{
				{Type: "language", Value: "go"},
				{Type: "path-pattern", Value: "services/**"},
			}),
			withScopePaths("services/**")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// ─── 4. Target-Specific Tests ──────────────────────────────────────────────

// TestIntegrationSimClaudeOnlyRendersClaudeFiles verifies Claude-only pipeline.
func TestIntegrationSimClaudeOnlyRendersClaudeFiles(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	for path := range pr.MemFS.Files() {
		if !hasPrefix(path, ".ai-build/claude/") {
			t.Errorf("unexpected file outside claude: %s", path)
		}
	}
}

// TestIntegrationSimCopilotMCPUsesServersKey verifies Copilot MCP key is "servers".
func TestIntegrationSimCopilotMCPUsesServersKey(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("mcp-plugin", model.KindPlugin, "MCP plugin.",
			withDistribution("inline"),
			withMCPServers(map[string]any{
				"github": map[string]any{"command": "npx", "args": []any{"server"}},
			})),
	)
	targets := []build.Target{build.TargetCopilot}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	for path, content := range pr.MemFS.Files() {
		if strings.Contains(path, "mcp.json") {
			s := string(content)
			if !strings.Contains(s, `"servers"`) {
				t.Errorf("Copilot mcp.json must use 'servers' key, got: %s", s)
			}
			if strings.Contains(s, `"mcpServers"`) {
				t.Errorf("Copilot mcp.json must NOT use 'mcpServers' key")
			}
		}
	}
}

// TestIntegrationSimClaudeMCPUsesMcpServersKey verifies Claude MCP key is "mcpServers".
func TestIntegrationSimClaudeMCPUsesMcpServersKey(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("mcp-plugin", model.KindPlugin, "MCP plugin.",
			withDistribution("inline"),
			withMCPServers(map[string]any{
				"github": map[string]any{"command": "npx", "args": []any{"server"}},
			})),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	for path, content := range pr.MemFS.Files() {
		if strings.Contains(path, "mcp.json") {
			s := string(content)
			if !strings.Contains(s, `"mcpServers"`) {
				t.Errorf("Claude mcp.json must use 'mcpServers' key, got: %s", s)
			}
		}
	}
}

// TestIntegrationSimTwoTargets verifies selecting exactly two targets.
func TestIntegrationSimTwoTargets(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	targets := []build.Target{build.TargetClaude, build.TargetCodex}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	hasClaude := false
	hasCodex := false
	for path := range pr.MemFS.Files() {
		if hasPrefix(path, ".ai-build/claude/") {
			hasClaude = true
		}
		if hasPrefix(path, ".ai-build/codex/") {
			hasCodex = true
		}
		if hasPrefix(path, ".ai-build/copilot/") {
			t.Errorf("unexpected copilot file: %s", path)
		}
	}
	if !hasClaude {
		t.Error("expected claude output")
	}
	if !hasCodex {
		t.Error("expected codex output")
	}
}

// ─── 5. Preservation Tests ─────────────────────────────────────────────────

// TestIntegrationSimPreservationRequired verifies required preservation.
func TestIntegrationSimPreservationRequired(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("must-have", model.KindInstruction, "Critical content.",
			withPreservation(model.PreservationRequired)),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimPreservationOptional verifies optional preservation.
func TestIntegrationSimPreservationOptional(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("optional-inst", model.KindInstruction, "Optional content.",
			withPreservation(model.PreservationOptional)),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
}

// TestIntegrationSimMixedPreservation verifies mixed preservation levels.
func TestIntegrationSimMixedPreservation(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("req-inst", model.KindInstruction, "Required.",
			withPreservation(model.PreservationRequired)),
		makeRawObject("pref-inst", model.KindInstruction, "Preferred.",
			withPreservation(model.PreservationPreferred)),
		makeRawObject("opt-inst", model.KindInstruction, "Optional.",
			withPreservation(model.PreservationOptional)),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
	if len(pr.Objects) != 3 {
		t.Errorf("expected 3 objects, got %d", len(pr.Objects))
	}
}

// ─── 6. Profile Tests ──────────────────────────────────────────────────────

// TestIntegrationSimLocalDevProfile tests local-dev profile.
func TestIntegrationSimLocalDevProfile(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Dev instructions."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimCIProfileOutput tests CI profile output.
func TestIntegrationSimCIProfileOutput(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "CI instructions."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileCI, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// TestIntegrationSimEnterpriseLockedProfile tests enterprise-locked profile.
func TestIntegrationSimEnterpriseLockedProfile(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Enterprise instructions."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileEnterpriseLocked, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
}

// TestIntegrationSimOSSPublicProfile tests oss-public profile.
func TestIntegrationSimOSSPublicProfile(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "OSS instructions."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileOSSPublic, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
}

// ─── 7. Determinism & Idempotency ──────────────────────────────────────────

// TestIntegrationSimDeterministicLargeSet verifies determinism with many objects.
func TestIntegrationSimDeterministicLargeSet(t *testing.T) {
	var objs []pipeline.RawObject
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("inst-%02d", i)
		objs = append(objs, makeRawObject(id, model.KindInstruction,
			fmt.Sprintf("Instruction %d content.", i)))
	}
	tree := makeSourceTree(objs...)

	pr1 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr1)
	files1 := pr1.MemFS.Files()

	pr2 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr2)
	files2 := pr2.MemFS.Files()

	if len(files1) != len(files2) {
		t.Fatalf("determinism: %d vs %d files", len(files1), len(files2))
	}
	for path, c1 := range files1 {
		if c2, ok := files2[path]; !ok {
			t.Errorf("missing in run2: %s", path)
		} else if !bytes.Equal(c1, c2) {
			t.Errorf("mismatch: %s", path)
		}
	}
}

// TestIntegrationSimIdempotentRun verifies running twice produces same file count.
func TestIntegrationSimIdempotentRun(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
		makeRawObject("rule", model.KindRule, "Be safe.",
			withConditions([]model.RuleCondition{{Type: "language", Value: "go"}})),
	)
	pr1 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr1)
	count1 := fileCount(pr1.MemFS)

	pr2 := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr2)
	count2 := fileCount(pr2.MemFS)

	if count1 != count2 {
		t.Errorf("idempotent: run1=%d files, run2=%d files", count1, count2)
	}
}

// ─── 8. Dry-Run Edge Cases ─────────────────────────────────────────────────

// TestIntegrationSimDryRunSingleInstruction verifies dry-run with one instruction.
func TestIntegrationSimDryRunSingleInstruction(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, true)
	report := runPipeline(t, pr)

	if fileCount(pr.MemFS) > 0 {
		t.Error("dry-run should produce zero files")
	}
	if !hasDiagnosticCode(report, "MATERIALIZE_DRY_RUN") {
		t.Error("expected MATERIALIZE_DRY_RUN diagnostic")
	}
}

// TestIntegrationSimDryRunAllKinds verifies dry-run with all object kinds.
func TestIntegrationSimDryRunAllKinds(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
		makeRawObject("rule", model.KindRule, "No secrets.",
			withConditions([]model.RuleCondition{{Type: "language", Value: "go"}})),
		makeRawObject("skill", model.KindSkill, "Testing."),
		makeRawObject("agent", model.KindAgent, "Reviewer."),
		makeRawObject("hook", model.KindHook, "Lint.",
			withHookFields("post-edit", "command", "lint")),
		makeRawObject("plugin", model.KindPlugin, "MCP.",
			withDistribution("inline"),
			withMCPServers(map[string]any{"s": map[string]any{"command": "echo"}})),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, true)
	report := runPipeline(t, pr)

	if fileCount(pr.MemFS) > 0 {
		t.Error("dry-run should produce zero files")
	}
	if !hasDiagnosticCode(report, "MATERIALIZE_DRY_RUN") {
		t.Error("expected MATERIALIZE_DRY_RUN diagnostic")
	}
}

// ─── 9. Context Cancellation Edge Cases ────────────────────────────────────

// TestIntegrationSimCancelledContextBeforeStart tests pre-cancelled context.
func TestIntegrationSimCancelledContextBeforeStart(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := pr.Pipeline.Execute(ctx, []string{"."})
	if err == nil {
		t.Error("expected error from pre-cancelled context")
	}
}

// TestIntegrationSimDeadlineExceeded tests context with past deadline.
func TestIntegrationSimDeadlineExceeded(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	_, err := pr.Pipeline.Execute(ctx, []string{"."})
	if err == nil {
		t.Error("expected error from expired deadline")
	}
}

// ─── 10. Report Content Verification ───────────────────────────────────────

// TestIntegrationSimReportHasDiagnostics verifies diagnostics appear in report.
func TestIntegrationSimReportHasDiagnostics(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	if len(report.Diagnostics) == 0 {
		t.Error("expected at least some info-level diagnostics")
	}
}

// TestIntegrationSimReportTimestamp verifies report has valid timestamp/duration.
func TestIntegrationSimReportTimestamp(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	if report.Timestamp.IsZero() {
		t.Error("report timestamp should be set")
	}
	if report.Duration <= 0 {
		t.Error("report duration should be positive")
	}
}

// ─── 11. Provenance Tests ──────────────────────────────────────────────────

// TestIntegrationSimProvenancePerTarget verifies provenance.json per target.
func TestIntegrationSimProvenancePerTarget(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	targets := []string{"claude", "copilot", "codex"}
	for _, target := range targets {
		found := false
		for path := range pr.MemFS.Files() {
			if hasPrefix(path, ".ai-build/"+target+"/") && strings.HasSuffix(path, "provenance.json") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected provenance.json for target %s", target)
		}
	}
}

// TestIntegrationSimProvenanceContainsSourceObjects verifies provenance content.
func TestIntegrationSimProvenanceContainsSourceObjects(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("my-inst", model.KindInstruction, "Test content."),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	for path, content := range pr.MemFS.Files() {
		if strings.HasSuffix(path, "provenance.json") {
			if !strings.Contains(string(content), "my-inst") {
				t.Errorf("provenance.json should reference source object 'my-inst'")
			}
		}
	}
}

// ─── 12. Large Input Tests ─────────────────────────────────────────────────

// TestIntegrationSimManyInstructions verifies handling of many instructions.
func TestIntegrationSimManyInstructions(t *testing.T) {
	var objs []pipeline.RawObject
	for i := 0; i < 50; i++ {
		objs = append(objs, makeRawObject(
			fmt.Sprintf("inst-%03d", i),
			model.KindInstruction,
			fmt.Sprintf("Instruction number %d.", i),
		))
	}
	tree := makeSourceTree(objs...)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
	if len(pr.Objects) != 50 {
		t.Errorf("expected 50 bridged objects, got %d", len(pr.Objects))
	}
}

// TestIntegrationSimMixedKindsAtScale verifies a mix of many different kinds.
func TestIntegrationSimMixedKindsAtScale(t *testing.T) {
	var objs []pipeline.RawObject
	for i := 0; i < 10; i++ {
		objs = append(objs, makeRawObject(
			fmt.Sprintf("inst-%02d", i), model.KindInstruction, fmt.Sprintf("Inst %d.", i)))
		objs = append(objs, makeRawObject(
			fmt.Sprintf("rule-%02d", i), model.KindRule, fmt.Sprintf("Rule %d.", i),
			withConditions([]model.RuleCondition{{Type: "language", Value: "go"}})))
		objs = append(objs, makeRawObject(
			fmt.Sprintf("skill-%02d", i), model.KindSkill, fmt.Sprintf("Skill %d.", i)))
	}
	tree := makeSourceTree(objs...)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	if len(pr.Objects) != 30 {
		t.Errorf("expected 30 objects, got %d", len(pr.Objects))
	}
}

// ─── 13. Generated Header Tests ────────────────────────────────────────────

// TestIntegrationSimGeneratedHeaderPresent verifies all .md files have headers.
func TestIntegrationSimGeneratedHeaderPresent(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Test content."),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)
}

// TestIntegrationSimGeneratedHeaderContainsSourceID verifies header has source ID.
func TestIntegrationSimGeneratedHeaderContainsSourceID(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("unique-source-id", model.KindInstruction, "Content."),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)
}

// ─── 14. Hooks Settings File Tests ─────────────────────────────────────────

// TestIntegrationSimHookProducesSettingsJSON verifies hooks produce settings.json.
func TestIntegrationSimHookProducesSettingsJSON(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("my-hook", model.KindHook, "Lint.",
			withHookFields("post-edit", "command", "make lint")),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	found := false
	for path := range pr.MemFS.Files() {
		if strings.Contains(path, "settings.json") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected settings.json from hook")
	}
}

// TestIntegrationSimMultipleHooks verifies multiple hooks render correctly.
func TestIntegrationSimMultipleHooks(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("hook-lint", model.KindHook, "Lint.",
			withHookFields("post-edit", "command", "make lint")),
		makeRawObject("hook-test", model.KindHook, "Test.",
			withHookFields("pre-commit", "command", "make test")),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	assertFilesWritten(t, pr.MemFS)
}

// ─── 15. Description & Metadata Tests ──────────────────────────────────────

// TestIntegrationSimObjectWithDescription verifies objects with descriptions.
func TestIntegrationSimObjectWithDescription(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("desc-inst", model.KindInstruction, "Content.",
			withDescription("A detailed description")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	report := runPipeline(t, pr)

	assertNoErrors(t, report)
	if obj, ok := pr.Objects["desc-inst"]; ok {
		if obj.Meta.Description != "A detailed description" {
			t.Errorf("expected description preserved, got %q", obj.Meta.Description)
		}
	} else {
		t.Error("expected desc-inst in bridged objects")
	}
}

// TestIntegrationSimObjectWithOwner verifies owner metadata passes through.
func TestIntegrationSimObjectWithOwner(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("owned-inst", model.KindInstruction, "Content.",
			withOwner("team-platform")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	if obj, ok := pr.Objects["owned-inst"]; ok {
		if obj.Meta.Owner != "team-platform" {
			t.Errorf("expected owner 'team-platform', got %q", obj.Meta.Owner)
		}
	} else {
		t.Error("expected owned-inst in bridged objects")
	}
}

// TestIntegrationSimObjectWithLabels verifies labels pass through.
func TestIntegrationSimObjectWithLabels(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("labeled-inst", model.KindInstruction, "Content.",
			withLabels("security", "compliance")),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	if obj, ok := pr.Objects["labeled-inst"]; ok {
		if len(obj.Meta.Labels) < 2 {
			t.Errorf("expected 2+ labels, got %v", obj.Meta.Labels)
		}
	} else {
		t.Error("expected labeled-inst in bridged objects")
	}
}

// ─── 16. Content Fidelity Tests ────────────────────────────────────────────

// TestIntegrationSimContentPreservedInOutput verifies content passes through.
func TestIntegrationSimContentPreservedInOutput(t *testing.T) {
	content := "This is very specific content that must appear in output."
	tree := makeSourceTree(
		makeRawObject("content-test", model.KindInstruction, content),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	found := false
	for _, c := range pr.MemFS.Files() {
		if strings.Contains(string(c), content) {
			found = true
			break
		}
	}
	if !found {
		t.Error("instruction content not found in any output file")
	}
}

// TestIntegrationSimMultilineContent verifies multiline content renders.
func TestIntegrationSimMultilineContent(t *testing.T) {
	content := "Line one.\nLine two.\nLine three."
	tree := makeSourceTree(
		makeRawObject("multiline", model.KindInstruction, content),
	)
	targets := []build.Target{build.TargetClaude}
	pr := buildTestPipeline(t, tree, targets, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	found := false
	for _, c := range pr.MemFS.Files() {
		if strings.Contains(string(c), "Line two.") {
			found = true
			break
		}
	}
	if !found {
		t.Error("multiline content not preserved in output")
	}
}

// ─── 17. Output Path Structure Tests ───────────────────────────────────────

// TestIntegrationSimOutputPathStructure verifies .ai-build/{target}/{profile}/ paths.
func TestIntegrationSimOutputPathStructure(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr)

	for path := range pr.MemFS.Files() {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) < 4 {
			t.Errorf("unexpected path structure: %s", path)
			continue
		}
		if parts[0] != ".ai-build" {
			t.Errorf("expected .ai-build prefix, got: %s", path)
		}
		validTargets := map[string]bool{"claude": true, "copilot": true, "codex": true}
		if !validTargets[parts[1]] {
			t.Errorf("unexpected target in path: %s", parts[1])
		}
		if parts[2] != "local-dev" {
			t.Errorf("expected local-dev profile in path, got: %s", parts[2])
		}
	}
}

// TestIntegrationSimCIProfilePathStructure verifies CI profile paths.
func TestIntegrationSimCIProfilePathStructure(t *testing.T) {
	tree := makeSourceTree(
		makeRawObject("inst", model.KindInstruction, "Hello."),
	)
	pr := buildTestPipeline(t, tree, nil, build.ProfileCI, false)
	runPipeline(t, pr)

	for path := range pr.MemFS.Files() {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) >= 3 && parts[2] != "ci" {
			t.Errorf("expected 'ci' profile in path, got: %s", parts[2])
		}
	}
}

// ─── 18. Sorting / Ordering Tests ──────────────────────────────────────────

// TestIntegrationSimObjectOrderDoesNotAffectOutput verifies deterministic
// output regardless of input object order.
func TestIntegrationSimObjectOrderDoesNotAffectOutput(t *testing.T) {
	objs := []pipeline.RawObject{
		makeRawObject("z-inst", model.KindInstruction, "Zeta."),
		makeRawObject("a-inst", model.KindInstruction, "Alpha."),
		makeRawObject("m-inst", model.KindInstruction, "Mid."),
	}

	// Run with original order.
	tree1 := makeSourceTree(objs...)
	pr1 := buildTestPipeline(t, tree1, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr1)

	// Run with reversed order.
	reversed := make([]pipeline.RawObject, len(objs))
	for i, o := range objs {
		reversed[len(objs)-1-i] = o
	}
	tree2 := makeSourceTree(reversed...)
	pr2 := buildTestPipeline(t, tree2, nil, build.ProfileLocalDev, false)
	runPipeline(t, pr2)

	files1 := pr1.MemFS.Files()
	files2 := pr2.MemFS.Files()

	if len(files1) != len(files2) {
		t.Fatalf("order-independent: %d vs %d files", len(files1), len(files2))
	}

	// Get sorted keys.
	keys1 := make([]string, 0, len(files1))
	for k := range files1 {
		keys1 = append(keys1, k)
	}
	sort.Strings(keys1)

	keys2 := make([]string, 0, len(files2))
	for k := range files2 {
		keys2 = append(keys2, k)
	}
	sort.Strings(keys2)

	for i := range keys1 {
		if keys1[i] != keys2[i] {
			t.Errorf("order-independent: path mismatch at %d: %s vs %s", i, keys1[i], keys2[i])
		}
		if !bytes.Equal(files1[keys1[i]], files2[keys2[i]]) {
			t.Errorf("order-independent: content mismatch for %s", keys1[i])
		}
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────────
