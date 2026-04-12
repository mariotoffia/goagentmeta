package cursor_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/cursor"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/renderer"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time interface checks.
var (
	_ stage.Stage       = (*cursor.Renderer)(nil)
	_ renderer.Renderer = (*cursor.Renderer)(nil)
)

// ---------------------------------------------------------------------------
// Helper constructors
// ---------------------------------------------------------------------------

func cursorCoord() build.BuildCoordinate {
	return build.BuildCoordinate{
		Unit: build.BuildUnit{
			Target:  build.TargetCursor,
			Profile: build.ProfileLocalDev,
		},
		OutputDir: ".ai-build/cursor/local-dev",
	}
}

func loweredGraph(objects map[string]pipeline.LoweredObject) pipeline.LoweredGraph {
	return pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/cursor/local-dev": {
				Coordinate: cursorCoord(),
				Objects:    objects,
			},
		},
	}
}

func keptInstruction(id, content string, scopePaths []string) pipeline.LoweredObject {
	fields := map[string]any{}
	if len(scopePaths) > 0 {
		pathsAny := make([]any, len(scopePaths))
		for i, p := range scopePaths {
			pathsAny[i] = p
		}
		fields["scope"] = map[string]any{"paths": pathsAny}
	}
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: model.KindInstruction,
		LoweredKind:  model.KindInstruction,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      content,
		Fields:       fields,
	}
}

func keptRule(id, content string, scopePaths []string, desc string) pipeline.LoweredObject {
	fields := map[string]any{}
	if len(scopePaths) > 0 {
		pathsAny := make([]any, len(scopePaths))
		for i, p := range scopePaths {
			pathsAny[i] = p
		}
		fields["scope"] = map[string]any{"paths": pathsAny}
	}
	if desc != "" {
		fields["description"] = desc
	}
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: model.KindRule,
		LoweredKind:  model.KindRule,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      content,
		Fields:       fields,
	}
}

func keptSkill(id, content string, fields map[string]any) pipeline.LoweredObject {
	if fields == nil {
		fields = map[string]any{}
	}
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: model.KindSkill,
		LoweredKind:  model.KindSkill,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      content,
		Fields:       fields,
	}
}

func keptAgent(id, content string, fields map[string]any) pipeline.LoweredObject {
	if fields == nil {
		fields = map[string]any{}
	}
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: model.KindAgent,
		LoweredKind:  model.KindAgent,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      content,
		Fields:       fields,
	}
}

func keptHook(id string, event, actionType, actionRef string, scopePaths []string) pipeline.LoweredObject {
	fields := map[string]any{
		"event":  event,
		"action": map[string]any{"type": actionType, "ref": actionRef},
	}
	if len(scopePaths) > 0 {
		pathsAny := make([]any, len(scopePaths))
		for i, p := range scopePaths {
			pathsAny[i] = p
		}
		fields["scope"] = map[string]any{"paths": pathsAny}
	}
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: model.KindHook,
		LoweredKind:  model.KindHook,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      "",
		Fields:       fields,
	}
}

func keptPlugin(id string, fields map[string]any) pipeline.LoweredObject {
	if fields == nil {
		fields = map[string]any{}
	}
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: model.KindPlugin,
		LoweredKind:  model.KindPlugin,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      "",
		Fields:       fields,
	}
}

func skippedObject(id string, kind model.Kind) pipeline.LoweredObject {
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: kind,
		LoweredKind:  kind,
		Decision:     pipeline.LoweringDecision{Action: "skipped"},
	}
}

func testContext() context.Context {
	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func findFile(files []pipeline.EmittedFile, path string) *pipeline.EmittedFile {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}

func assertSourceObjects(t *testing.T, f *pipeline.EmittedFile, expected ...string) {
	t.Helper()
	if len(f.SourceObjects) != len(expected) {
		t.Errorf("expected %d source objects, got %d: %v", len(expected), len(f.SourceObjects), f.SourceObjects)
		return
	}
	for i, e := range expected {
		if f.SourceObjects[i] != e {
			t.Errorf("source object[%d]: expected %q, got %q", i, e, f.SourceObjects[i])
		}
	}
}

// ---------------------------------------------------------------------------
// 1. Renderer Registration Tests
// ---------------------------------------------------------------------------

func TestRendererTarget(t *testing.T) {
	r := cursor.New(nil)
	if r.Target() != build.TargetCursor {
		t.Fatalf("expected target %q, got %q", build.TargetCursor, r.Target())
	}
}

func TestRendererDescriptor(t *testing.T) {
	r := cursor.New(nil)
	desc := r.Descriptor()

	if desc.Name != "cursor-renderer" {
		t.Errorf("expected name %q, got %q", "cursor-renderer", desc.Name)
	}
	if desc.Phase != pipeline.PhaseRender {
		t.Errorf("expected phase %v, got %v", pipeline.PhaseRender, desc.Phase)
	}
	if desc.Order != 10 {
		t.Errorf("expected order 10, got %d", desc.Order)
	}
	if len(desc.TargetFilter) != 1 || desc.TargetFilter[0] != build.TargetCursor {
		t.Errorf("expected target filter [cursor], got %v", desc.TargetFilter)
	}
}

func TestRendererSupportedCapabilities(t *testing.T) {
	r := cursor.New(nil)
	caps := r.SupportedCapabilities()
	if caps.Target != "cursor" {
		t.Fatalf("expected target %q, got %q", "cursor", caps.Target)
	}
	if len(caps.Surfaces) == 0 {
		t.Fatal("expected non-empty capability surfaces")
	}
}

func TestRendererFactory(t *testing.T) {
	factory := cursor.Factory(nil)
	s, err := factory()
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if s == nil {
		t.Fatal("factory returned nil stage")
	}
	if s.Descriptor().Name != "cursor-renderer" {
		t.Errorf("expected name %q, got %q", "cursor-renderer", s.Descriptor().Name)
	}
}

func TestRendererRejectsInvalidInput(t *testing.T) {
	r := cursor.New(nil)
	_, err := r.Execute(testContext(), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if !strings.Contains(err.Error(), "expected pipeline.LoweredGraph") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRendererAcceptsPointerInput(t *testing.T) {
	r := cursor.New(nil)
	graph := &pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/cursor/local-dev": {
				Coordinate: cursorCoord(),
				Objects:    map[string]pipeline.LoweredObject{},
			},
		},
	}
	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	plan, ok := result.(pipeline.EmissionPlan)
	if !ok {
		t.Fatalf("expected EmissionPlan, got %T", result)
	}
	if len(plan.Units) != 1 {
		t.Errorf("expected 1 unit, got %d", len(plan.Units))
	}
}

// ---------------------------------------------------------------------------
// 2. Instruction Layer Tests
// ---------------------------------------------------------------------------

func TestSingleInstructionAGENTSMD(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1": keptInstruction("inst-1", "Always use Go modules.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found")
	}

	content := string(agentsMD.Content)
	if !strings.Contains(content, "Always use Go modules.") {
		t.Errorf("AGENTS.md content missing instruction: %s", content)
	}
	if agentsMD.Layer != pipeline.LayerInstruction {
		t.Errorf("expected layer %q, got %q", pipeline.LayerInstruction, agentsMD.Layer)
	}
	assertSourceObjects(t, agentsMD, "inst-1")
}

func TestTwoInstructionsDifferentScopesTwoFiles(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-root":    keptInstruction("inst-root", "Root instruction.", nil),
		"inst-subtree": keptInstruction("inst-subtree", "Subtree instruction.", []string{"services/api"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	rootMD := findFile(unit.Files, "AGENTS.md")
	if rootMD == nil {
		t.Fatal("AGENTS.md not found")
	}
	if !strings.Contains(string(rootMD.Content), "Root instruction.") {
		t.Error("AGENTS.md missing root instruction")
	}

	subtreeMD := findFile(unit.Files, "services/api/AGENTS.md")
	if subtreeMD == nil {
		t.Fatal("services/api/AGENTS.md not found")
	}
	if !strings.Contains(string(subtreeMD.Content), "Subtree instruction.") {
		t.Error("services/api/AGENTS.md missing subtree instruction")
	}
}

func TestMultipleInstructionsMergedSorted(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"z-inst": keptInstruction("z-inst", "Z instruction.", nil),
		"a-inst": keptInstruction("a-inst", "A instruction.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found")
	}

	content := string(agentsMD.Content)
	aIdx := strings.Index(content, "A instruction.")
	zIdx := strings.Index(content, "Z instruction.")
	if aIdx < 0 || zIdx < 0 || aIdx >= zIdx {
		t.Error("instructions not sorted by ID (A should come before Z)")
	}
}

// ---------------------------------------------------------------------------
// 3. Rules Layer Tests
// ---------------------------------------------------------------------------

func TestRuleWithGlobs(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"go-style": keptRule("go-style", "Use gofmt.", []string{"**/*.go"}, "Go style rule"),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	ruleFile := findFile(unit.Files, ".cursor/rules/go-style.mdc")
	if ruleFile == nil {
		t.Fatal(".cursor/rules/go-style.mdc not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "---") {
		t.Error("rule missing YAML frontmatter")
	}
	if !strings.Contains(content, "globs:") {
		t.Error("rule missing globs: in frontmatter")
	}
	if !strings.Contains(content, "**/*.go") {
		t.Error("rule missing path glob")
	}
	if !strings.Contains(content, "alwaysApply: false") {
		t.Error("rule with globs should have alwaysApply: false")
	}
	if !strings.Contains(content, "description: Go style rule") {
		t.Error("rule missing description")
	}
	if !strings.Contains(content, "Use gofmt.") {
		t.Error("rule missing content body")
	}
}

func TestRuleAlwaysApplyWhenNoScopeOrDescription(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"bare-rule": keptRule("bare-rule", "Just a rule.", nil, ""),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	ruleFile := findFile(unit.Files, ".cursor/rules/bare-rule.mdc")
	if ruleFile == nil {
		t.Fatal(".cursor/rules/bare-rule.mdc not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("rule with no scope/description should have alwaysApply: true")
	}
}

func TestRuleApplyIntelligentlyWithDescriptionOnly(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"smart-rule": keptRule("smart-rule", "Be smart.", nil, "Apply when writing tests"),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	ruleFile := findFile(unit.Files, ".cursor/rules/smart-rule.mdc")
	if ruleFile == nil {
		t.Fatal(".cursor/rules/smart-rule.mdc not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "alwaysApply: false") {
		t.Error("rule with description-only should have alwaysApply: false")
	}
	if !strings.Contains(content, "description: Apply when writing tests") {
		t.Error("rule missing description for Apply Intelligently mode")
	}
}

// ---------------------------------------------------------------------------
// 4. Skills Layer Tests (Lowered)
// ---------------------------------------------------------------------------

func TestSkillLoweredIntoRule(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"iam-review": keptSkill("iam-review", "Review IAM policies.", map[string]any{
			"description":     "IAM review skill",
			"activationHints": []any{"IAM", "policy"},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	skillFile := findFile(unit.Files, ".cursor/rules/iam-review.mdc")
	if skillFile == nil {
		t.Fatal(".cursor/rules/iam-review.mdc not found (skill should be lowered into rule)")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("lowered skill should have alwaysApply: true")
	}
	if !strings.Contains(content, "description: IAM review skill") {
		t.Error("lowered skill missing description")
	}
	if !strings.Contains(content, "Review IAM policies.") {
		t.Error("lowered skill missing content body")
	}
}

func TestSkillWithNoContentSkipped(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"empty-skill": keptSkill("empty-skill", "", map[string]any{
			"description": "Empty skill",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	for _, f := range unit.Files {
		if f.Path != "provenance.json" && strings.Contains(f.Path, "empty-skill") {
			t.Errorf("skill with no content should not produce file, but found: %s", f.Path)
		}
	}
}

// ---------------------------------------------------------------------------
// 5. Agents Layer Tests (Lowered)
// ---------------------------------------------------------------------------

func TestAgentLoweredIntoRule(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"review-agent": keptAgent("review-agent", "You specialize in code review.", map[string]any{
			"description": "Code review agent",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	agentFile := findFile(unit.Files, ".cursor/rules/review-agent.mdc")
	if agentFile == nil {
		t.Fatal(".cursor/rules/review-agent.mdc not found (agent should be lowered into rule)")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("lowered agent should have alwaysApply: true")
	}
	if !strings.Contains(content, "description: Code review agent") {
		t.Error("lowered agent missing description")
	}
	if !strings.Contains(content, "You specialize in code review.") {
		t.Error("lowered agent missing rolePrompt content")
	}
}

func TestAgentWithHandoffsSkipped(t *testing.T) {
	r := cursor.New(nil)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"complex-agent": keptAgent("complex-agent", "Complex agent.", map[string]any{
			"description": "Complex",
			"handoffs": []any{
				map[string]any{"label": "deploy", "agent": "deploy-agent"},
			},
		}),
	})

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	for _, f := range unit.Files {
		if f.Path != "provenance.json" && strings.Contains(f.Path, "complex-agent") {
			t.Errorf("agent with handoffs should be skipped, but found: %s", f.Path)
		}
	}

	// Should have warning diagnostic.
	foundWarning := false
	for _, d := range report.Diagnostics {
		if d.Code == "RENDER_AGENT_SKIPPED" && d.Severity == "warning" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected RENDER_AGENT_SKIPPED warning for agent with handoffs")
	}
}

// ---------------------------------------------------------------------------
// 6. Hooks Layer Tests
// ---------------------------------------------------------------------------

func TestHookSupportedEvent(t *testing.T) {
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

	hooksFile := findFile(unit.Files, ".cursor/hooks.json")
	if hooksFile == nil {
		t.Fatal(".cursor/hooks.json not found")
	}

	var hooksConfig map[string]any
	if err := json.Unmarshal(hooksFile.Content, &hooksConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks := hooksConfig["hooks"].(map[string]any)
	if _, ok := hooks["beforeMCPExecution"]; !ok {
		t.Error("expected beforeMCPExecution in hooks")
	}
}

func TestHookUnsupportedEventSkipped(t *testing.T) {
	r := cursor.New(nil)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"unsupported-hook": keptHook("unsupported-hook", "post-edit", "command", "make lint", nil),
	})

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	// Should not produce hooks file for unsupported events.
	hooksFile := findFile(unit.Files, ".cursor/hooks.json")
	if hooksFile != nil {
		t.Error("should not produce hooks.json for unsupported event")
	}

	// Should have diagnostic about unsupported event.
	foundDiag := false
	for _, d := range report.Diagnostics {
		if d.Code == "RENDER_HOOK_UNSUPPORTED" {
			foundDiag = true
			break
		}
	}
	if !foundDiag {
		t.Error("expected RENDER_HOOK_UNSUPPORTED diagnostic")
	}
}

func TestMultipleHooksSameEvent(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"hook-a": keptHook("hook-a", "beforeMCPExecution", "command", "cmd-a", nil),
		"hook-b": keptHook("hook-b", "beforeMCPExecution", "command", "cmd-b", nil),
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
	entries := hooks["beforeMCPExecution"].([]any)
	if len(entries) != 2 {
		t.Errorf("expected 2 hook entries, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// 7. MCP Layer Tests
// ---------------------------------------------------------------------------

func TestMCPServerConfig(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
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
	server := servers["github"].(map[string]any)
	if server["transport"] != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", server["transport"])
	}
	if server["command"] != "npx" {
		t.Errorf("expected command 'npx', got %q", server["command"])
	}
}

// ---------------------------------------------------------------------------
// 8. Plugin Layer Tests
// ---------------------------------------------------------------------------

func TestRegistryPluginInstallMetadata(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"ext-plugin": keptPlugin("ext-plugin", map[string]any{
			"distribution": map[string]any{
				"mode":    "registry",
				"ref":     "cursor.ext-plugin",
				"version": "1.0.0",
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	if len(unit.InstallMetadata) != 1 {
		t.Fatalf("expected 1 install entry, got %d", len(unit.InstallMetadata))
	}

	entry := unit.InstallMetadata[0]
	if entry.PluginID != "ext-plugin" {
		t.Errorf("expected plugin ID 'ext-plugin', got %q", entry.PluginID)
	}
	if entry.Format != "cursor-marketplace" {
		t.Errorf("expected format 'cursor-marketplace', got %q", entry.Format)
	}
}

// ---------------------------------------------------------------------------
// 9. Provenance Tests
// ---------------------------------------------------------------------------

func TestProvenanceHeaderOnMDC(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"go-style": keptRule("go-style", "Use gofmt.", []string{"**/*.go"}, ""),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	ruleFile := findFile(unit.Files, ".cursor/rules/go-style.mdc")
	if ruleFile == nil {
		t.Fatal("go-style.mdc not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "<!-- generated by goagentmeta") {
		t.Error("mdc file should have provenance header")
	}
}

func TestProvenanceJSON(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-a": keptInstruction("inst-a", "A.", nil),
		"rule-b": keptRule("rule-b", "B.", []string{"**/*.go"}, ""),
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

	if prov["target"] != "cursor" {
		t.Errorf("expected target 'cursor', got %q", prov["target"])
	}

	entries := prov["entries"].([]any)
	sources := make(map[string]bool)
	for _, e := range entries {
		entry := e.(map[string]any)
		sources[entry["sourceObject"].(string)] = true
	}
	for _, expected := range []string{"inst-a", "rule-b"} {
		if !sources[expected] {
			t.Errorf("provenance.json missing source object %q", expected)
		}
	}
}

// ---------------------------------------------------------------------------
// 10. Determinism Tests
// ---------------------------------------------------------------------------

func TestDeterministicOutput(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst":  keptInstruction("inst", "Instruction.", nil),
		"rule":  keptRule("rule", "Rule.", []string{"**/*.go"}, "A rule"),
		"skill": keptSkill("skill", "Skill body.", map[string]any{"description": "A skill"}),
		"agent": keptAgent("agent", "Agent body.", map[string]any{"description": "An agent"}),
	})

	result1, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("run 1 error: %v", err)
	}
	result2, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("run 2 error: %v", err)
	}

	plan1 := result1.(pipeline.EmissionPlan)
	plan2 := result2.(pipeline.EmissionPlan)

	unit1 := plan1.Units[".ai-build/cursor/local-dev"]
	unit2 := plan2.Units[".ai-build/cursor/local-dev"]

	if len(unit1.Files) != len(unit2.Files) {
		t.Fatalf("file count differs: %d vs %d", len(unit1.Files), len(unit2.Files))
	}

	for i := range unit1.Files {
		if unit1.Files[i].Path != unit2.Files[i].Path {
			t.Errorf("path[%d] differs: %q vs %q", i, unit1.Files[i].Path, unit2.Files[i].Path)
		}
		if string(unit1.Files[i].Content) != string(unit2.Files[i].Content) {
			t.Errorf("content[%d] differs for %s", i, unit1.Files[i].Path)
		}
	}
}

// ---------------------------------------------------------------------------
// 11. Skipped Objects
// ---------------------------------------------------------------------------

func TestSkippedObjectsNotEmitted(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst":         keptInstruction("inst", "Kept.", nil),
		"skipped-rule": skippedObject("skipped-rule", model.KindRule),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	for _, f := range unit.Files {
		if strings.Contains(f.Path, "skipped-rule") {
			t.Errorf("skipped object should not produce a file: %s", f.Path)
		}
	}
}

// ---------------------------------------------------------------------------
// 12. Non-Cursor Units Filtered
// ---------------------------------------------------------------------------

func TestNonCursorUnitsFiltered(t *testing.T) {
	r := cursor.New(nil)
	graph := pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/claude/local-dev": {
				Coordinate: build.BuildCoordinate{
					Unit: build.BuildUnit{
						Target:  build.TargetClaude,
						Profile: build.ProfileLocalDev,
					},
					OutputDir: ".ai-build/claude/local-dev",
				},
				Objects: map[string]pipeline.LoweredObject{
					"inst": keptInstruction("inst", "Claude instruction.", nil),
				},
			},
		},
	}

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	if len(plan.Units) != 0 {
		t.Errorf("expected 0 units for non-cursor graph, got %d", len(plan.Units))
	}
}

// ---------------------------------------------------------------------------
// 13. Commands Skipped
// ---------------------------------------------------------------------------

func TestCommandsSkipped(t *testing.T) {
	r := cursor.New(nil)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-cmd": {
			OriginalID:   "my-cmd",
			OriginalKind: model.KindCommand,
			LoweredKind:  model.KindCommand,
			Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
			Content:      "run tests",
			Fields:       map[string]any{"action": map[string]any{"type": "command", "ref": "make test"}},
		},
	})

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	for _, f := range unit.Files {
		if f.Path != "provenance.json" && strings.Contains(f.Path, "my-cmd") {
			t.Errorf("command should not be rendered: %s", f.Path)
		}
	}

	foundDiag := false
	for _, d := range report.Diagnostics {
		if d.Code == "RENDER_COMMANDS_SKIPPED" {
			foundDiag = true
			if !strings.Contains(d.Message, "my-cmd") {
				t.Errorf("diagnostic should mention command ID, got: %s", d.Message)
			}
			break
		}
	}
	if !foundDiag {
		t.Error("expected RENDER_COMMANDS_SKIPPED diagnostic")
	}
}

// ---------------------------------------------------------------------------
// 14. Empty Graph
// ---------------------------------------------------------------------------

func TestEmptyGraphProducesProvenance(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	if len(unit.Files) != 1 {
		t.Fatalf("expected 1 file (provenance.json), got %d", len(unit.Files))
	}
	if unit.Files[0].Path != "provenance.json" {
		t.Errorf("expected provenance.json, got %s", unit.Files[0].Path)
	}
}

// ---------------------------------------------------------------------------
// 15. Directories Collected
// ---------------------------------------------------------------------------

func TestDirectoriesCollected(t *testing.T) {
	r := cursor.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst":  keptInstruction("inst", "Root.", nil),
		"rule":  keptRule("rule", "Rule.", []string{"**/*.go"}, ""),
		"skill": keptSkill("skill", "Skill body.", map[string]any{"description": "A skill"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/cursor/local-dev"]

	if len(unit.Directories) == 0 {
		t.Fatal("expected directories to be populated")
	}

	found := false
	for _, d := range unit.Directories {
		if d == ".cursor/rules" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing expected directory .cursor/rules")
	}
}
