package claude_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/claude"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ---------------------------------------------------------------------------
// Command Objects in Renderer (should warn, not render)
// ---------------------------------------------------------------------------

func TestCommandObjectsEmitWarningDiagnostic(t *testing.T) {
	r := claude.New(nil)

	// A command that somehow reached the renderer without being lowered.
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-cmd": {
			OriginalID:   "my-cmd",
			OriginalKind: model.KindCommand,
			LoweredKind:  model.KindCommand,
			Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
			Content:      "run tests",
			Fields: map[string]any{
				"action": map[string]any{"type": "command", "ref": "make test"},
			},
		},
	})

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(t.Context(), cc)

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	// Commands should NOT produce any rendered files (only provenance.json).
	for _, f := range unit.Files {
		if f.Path != "provenance.json" && strings.Contains(f.Path, "my-cmd") {
			t.Errorf("command should not be rendered, but found file: %s", f.Path)
		}
	}

	// Should have emitted a warning diagnostic about unlowered commands.
	foundWarning := false
	for _, d := range report.Diagnostics {
		if d.Code == "RENDER_UNLOWERED_COMMANDS" {
			foundWarning = true
			if d.Severity != "warning" {
				t.Errorf("expected warning severity, got %q", d.Severity)
			}
			if !strings.Contains(d.Message, "my-cmd") {
				t.Errorf("warning should mention command ID, got: %s", d.Message)
			}
			break
		}
	}
	if !foundWarning {
		t.Error("expected RENDER_UNLOWERED_COMMANDS diagnostic for command objects")
	}
}

// ---------------------------------------------------------------------------
// Nil/Empty Field Edge Cases
// ---------------------------------------------------------------------------

func TestSkillWithNilFields(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"bare-skill": {
			OriginalID:   "bare-skill",
			OriginalKind: model.KindSkill,
			LoweredKind:  model.KindSkill,
			Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
			Content:      "Minimal skill.",
			Fields:       nil,
		},
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	skillFile := findFile(unit.Files, ".claude/skills/bare-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("bare-skill SKILL.md not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "name: bare-skill") {
		t.Error("skill should have name even with nil fields")
	}
	if !strings.Contains(content, "Minimal skill.") {
		t.Error("skill body missing")
	}
}

func TestAgentWithNilFields(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"bare-agent": {
			OriginalID:   "bare-agent",
			OriginalKind: model.KindAgent,
			LoweredKind:  model.KindAgent,
			Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
			Content:      "Minimal agent.",
			Fields:       nil,
		},
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	agentFile := findFile(unit.Files, ".claude/agents/bare-agent.md")
	if agentFile == nil {
		t.Fatal("bare-agent.md not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "name: bare-agent") {
		t.Error("agent should have name even with nil fields")
	}
}

func TestHookWithEmptyEvent(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"bad-hook": {
			OriginalID:   "bad-hook",
			OriginalKind: model.KindHook,
			LoweredKind:  model.KindHook,
			Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
			Fields:       map[string]any{},
		},
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	// Hook with no event should not produce settings.json.
	settingsFile := findFile(unit.Files, ".claude/settings.json")
	if settingsFile != nil {
		t.Error("hook with empty event should not produce settings.json")
	}
}

func TestRuleWithNoPathsOrConditions(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"bare-rule": keptRule("bare-rule", "Just a rule.", nil, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	ruleFile := findFile(unit.Files, ".claude/rules/bare-rule.md")
	if ruleFile == nil {
		t.Fatal("bare-rule.md not found")
	}

	content := string(ruleFile.Content)
	// Should NOT have frontmatter when no paths.
	if strings.HasPrefix(content, "---\npaths:") {
		t.Error("rule without paths should not have paths frontmatter")
	}
	// Should NOT have conditions section.
	if strings.Contains(content, "## Conditions") {
		t.Error("rule without conditions should not have conditions section")
	}
	// Should have the content (after provenance header).
	if !strings.Contains(content, "Just a rule.") {
		t.Error("rule content missing")
	}
}

func TestInstructionWithEmptyContent(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"empty-inst": keptInstruction("empty-inst", "", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	claudeMD := findFile(unit.Files, "CLAUDE.md")
	if claudeMD == nil {
		t.Fatal("CLAUDE.md not found even for empty instruction")
	}
}

// ---------------------------------------------------------------------------
// MCP Transport Types
// ---------------------------------------------------------------------------

func TestMCPHttpTransport(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"http-mcp": keptPlugin("http-mcp", map[string]any{
			"mcpServers": map[string]any{
				"remote-api": map[string]any{
					"transport": "http",
					"url":       "https://api.example.com/mcp",
				},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	mcpFile := findFile(unit.Files, ".mcp.json")
	if mcpFile == nil {
		t.Fatal(".mcp.json not found")
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := mcpConfig["mcpServers"].(map[string]any)
	server := servers["remote-api"].(map[string]any)
	if server["transport"] != "http" {
		t.Errorf("expected transport 'http', got %q", server["transport"])
	}
	if server["url"] != "https://api.example.com/mcp" {
		t.Errorf("expected URL, got %q", server["url"])
	}
}

// ---------------------------------------------------------------------------
// Provenance Completeness
// ---------------------------------------------------------------------------

func TestProvenanceJSONMapsAllSources(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-a": keptInstruction("inst-a", "A.", nil),
		"inst-b": keptInstruction("inst-b", "B.", nil),
		"rule-c": keptRule("rule-c", "C rule.", []string{"**/*.go"}, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	provFile := findFile(unit.Files, "provenance.json")
	if provFile == nil {
		t.Fatal("provenance.json not found")
	}

	var prov map[string]any
	if err := json.Unmarshal(provFile.Content, &prov); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	entries := prov["entries"].([]any)

	// Collect all source objects mentioned in provenance.
	sources := make(map[string]bool)
	for _, e := range entries {
		entry := e.(map[string]any)
		sources[entry["sourceObject"].(string)] = true
	}

	// All input objects should be tracked.
	for _, expected := range []string{"inst-a", "inst-b", "rule-c"} {
		if !sources[expected] {
			t.Errorf("provenance.json missing source object %q", expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Provenance Header Not on JSON Files
// ---------------------------------------------------------------------------

func TestProvenanceHeaderNotOnJSONFiles(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"lint-hook": keptHook("lint-hook", "post-edit", "command", "make lint", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	for _, f := range unit.Files {
		if strings.HasSuffix(f.Path, ".json") {
			content := string(f.Content)
			if strings.Contains(content, "<!-- generated by goagentmeta") {
				t.Errorf("JSON file %s should NOT have HTML provenance header", f.Path)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Special Character Safety in Rule Paths
// ---------------------------------------------------------------------------

func TestRulePathsWithSpecialChars(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"glob-rule": keptRule("glob-rule", "Glob rule.", []string{"src/**/*.{ts,tsx}", "!**/*.test.ts"}, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	ruleFile := findFile(unit.Files, ".claude/rules/glob-rule.md")
	if ruleFile == nil {
		t.Fatal("glob-rule.md not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "src/**/*.{ts,tsx}") {
		t.Error("rule path with braces not preserved")
	}
	if !strings.Contains(content, "!**/*.test.ts") {
		t.Error("rule negation path not preserved")
	}
}

// ---------------------------------------------------------------------------
// Directories Collected Correctly
// ---------------------------------------------------------------------------

func TestDirectoriesCollectedFromAllLayers(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst":  keptInstruction("inst", "Root.", nil),
		"rule":  keptRule("rule", "Rule.", []string{"**/*.go"}, nil),
		"skill": keptSkill("skill", "Skill.", nil),
		"agent": keptAgent("agent", "Agent.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	if len(unit.Directories) == 0 {
		t.Fatal("expected directories to be populated")
	}

	expectedDirs := map[string]bool{
		".claude/rules":        true,
		".claude/skills/skill": true,
		".claude/agents":       true,
	}

	for dir := range expectedDirs {
		found := false
		for _, d := range unit.Directories {
			if d == dir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected directory %q in directories list", dir)
		}
	}
}

// ---------------------------------------------------------------------------
// sanitizeID edge cases
// ---------------------------------------------------------------------------

func TestSanitizeIDInFilePaths(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"org/my.skill": keptSkill("org/my.skill", "Skill with special ID.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	// The ID "org/my.skill" should be sanitized to "org-my-skill" in file paths.
	skillFile := findFile(unit.Files, ".claude/skills/org-my-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("skill file with sanitized ID not found")
	}
}
