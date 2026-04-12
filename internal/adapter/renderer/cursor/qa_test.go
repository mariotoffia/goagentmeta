package cursor_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/cursor"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ---------------------------------------------------------------------------
// Nil/Empty Field Edge Cases
// ---------------------------------------------------------------------------

func TestSkillWithNilFields(t *testing.T) {
	r := cursor.New(nil)
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
	unit := plan.Units[".ai-build/cursor/local-dev"]

	skillFile := findFile(unit.Files, ".cursor/rules/bare-skill.mdc")
	if skillFile == nil {
		t.Fatal("bare-skill.mdc not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("skill should have alwaysApply: true with nil fields")
	}
	if !strings.Contains(content, "Minimal skill.") {
		t.Error("skill body missing")
	}
}

func TestAgentWithNilFields(t *testing.T) {
	r := cursor.New(nil)
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
	unit := plan.Units[".ai-build/cursor/local-dev"]

	agentFile := findFile(unit.Files, ".cursor/rules/bare-agent.mdc")
	if agentFile == nil {
		t.Fatal("bare-agent.mdc not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("agent should have alwaysApply: true")
	}
	if !strings.Contains(content, "Minimal agent.") {
		t.Error("agent body missing")
	}
}

func TestHookWithEmptyEvent(t *testing.T) {
	r := cursor.New(nil)
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
	unit := plan.Units[".ai-build/cursor/local-dev"]

	hooksFile := findFile(unit.Files, ".cursor/hooks.json")
	if hooksFile != nil {
		t.Error("hook with empty event should not produce hooks.json")
	}
}

func TestInstructionWithEmptyContent(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"empty-inst": keptInstruction("empty-inst", "", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found even for empty instruction")
	}
}

// ---------------------------------------------------------------------------
// MCP Transport Types
// ---------------------------------------------------------------------------

func TestMCPHttpTransport(t *testing.T) {
	r := cursor.New(nil)
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
	unit := plan.Units[".ai-build/cursor/local-dev"]

	mcpFile := findFile(unit.Files, ".cursor/mcp.json")
	if mcpFile == nil {
		t.Fatal(".cursor/mcp.json not found")
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
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-a": keptInstruction("inst-a", "A.", nil),
		"inst-b": keptInstruction("inst-b", "B.", nil),
		"rule-c": keptRule("rule-c", "C rule.", []string{"**/*.go"}, ""),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	provFile := findFile(unit.Files, "provenance.json")
	if provFile == nil {
		t.Fatal("provenance.json not found")
	}

	var prov map[string]any
	if err := json.Unmarshal(provFile.Content, &prov); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	entries := prov["entries"].([]any)
	sources := make(map[string]bool)
	for _, e := range entries {
		entry := e.(map[string]any)
		sources[entry["sourceObject"].(string)] = true
	}

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
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"mcp-hook": keptHook("mcp-hook", "beforeMCPExecution", "command", "check-auth", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

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
// Special Character Safety in Rule Globs
// ---------------------------------------------------------------------------

func TestRuleGlobsWithSpecialChars(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"glob-rule": keptRule("glob-rule", "Glob rule.", []string{"src/**/*.{ts,tsx}", "!**/*.test.ts"}, ""),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	ruleFile := findFile(unit.Files, ".cursor/rules/glob-rule.mdc")
	if ruleFile == nil {
		t.Fatal("glob-rule.mdc not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "src/**/*.{ts,tsx}") {
		t.Error("rule glob with braces not preserved")
	}
	if !strings.Contains(content, "!**/*.test.ts") {
		t.Error("rule negation glob not preserved")
	}
}

// ---------------------------------------------------------------------------
// sanitizeID edge cases
// ---------------------------------------------------------------------------

func TestSanitizeIDInFilePaths(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"org/my.skill": keptSkill("org/my.skill", "Skill with special ID.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	skillFile := findFile(unit.Files, ".cursor/rules/org-my-skill.mdc")
	if skillFile == nil {
		t.Fatal("skill file with sanitized ID not found")
	}
}

// ---------------------------------------------------------------------------
// Three supported hook events
// ---------------------------------------------------------------------------

func TestAllThreeSupportedHookEvents(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"hook-a": keptHook("hook-a", "beforeMCPExecution", "command", "cmd-a", nil),
		"hook-b": keptHook("hook-b", "afterMCPExecution", "command", "cmd-b", nil),
		"hook-c": keptHook("hook-c", "preShellCommand", "command", "cmd-c", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	hooksFile := findFile(unit.Files, ".cursor/hooks.json")
	if hooksFile == nil {
		t.Fatal(".cursor/hooks.json not found")
	}

	var hooksConfig map[string]any
	if err := json.Unmarshal(hooksFile.Content, &hooksConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks := hooksConfig["hooks"].(map[string]any)
	for _, event := range []string{"beforeMCPExecution", "afterMCPExecution", "preShellCommand"} {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing expected hook event %q", event)
		}
	}
}

// ---------------------------------------------------------------------------
// MCP server collision first-wins
// ---------------------------------------------------------------------------

func TestMCPServerCollisionFirstWins(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"alpha-plugin": keptPlugin("alpha-plugin", map[string]any{
			"mcpServers": map[string]any{
				"shared-server": map[string]any{
					"transport": "stdio",
					"command":   "alpha-cmd",
				},
			},
		}),
		"beta-plugin": keptPlugin("beta-plugin", map[string]any{
			"mcpServers": map[string]any{
				"shared-server": map[string]any{
					"transport": "stdio",
					"command":   "beta-cmd",
				},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	mcpFile := findFile(unit.Files, ".cursor/mcp.json")
	if mcpFile == nil {
		t.Fatal(".cursor/mcp.json not found")
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := mcpConfig["mcpServers"].(map[string]any)
	server := servers["shared-server"].(map[string]any)
	// "alpha-plugin" sorts before "beta-plugin", so alpha-cmd wins.
	if server["command"] != "alpha-cmd" {
		t.Errorf("expected first-wins 'alpha-cmd', got %q", server["command"])
	}
}

// ---------------------------------------------------------------------------
// Agent with no content skipped
// ---------------------------------------------------------------------------

func TestAgentWithNoContentSkipped(t *testing.T) {
	r := cursor.New(nil)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"empty-agent": keptAgent("empty-agent", "", map[string]any{
			"description": "Empty agent",
		}),
	})

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	for _, f := range unit.Files {
		if f.Path != "provenance.json" && strings.Contains(f.Path, "empty-agent") {
			t.Errorf("agent with no content should not produce file: %s", f.Path)
		}
	}

	foundDiag := false
	for _, d := range report.Diagnostics {
		if d.Code == "RENDER_AGENT_SKIPPED" {
			foundDiag = true
			break
		}
	}
	if !foundDiag {
		t.Error("expected RENDER_AGENT_SKIPPED diagnostic for empty agent")
	}
}
