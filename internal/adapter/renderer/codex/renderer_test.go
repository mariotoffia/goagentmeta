package codex_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/codex"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/renderer"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time interface checks.
var (
	_ stage.Stage       = (*codex.Renderer)(nil)
	_ renderer.Renderer = (*codex.Renderer)(nil)
)

// ---------------------------------------------------------------------------
// Helper constructors
// ---------------------------------------------------------------------------

func codexCoord() build.BuildCoordinate {
	return build.BuildCoordinate{
		Unit: build.BuildUnit{
			Target:  build.TargetCodex,
			Profile: build.ProfileLocalDev,
		},
		OutputDir: ".ai-build/codex/local-dev",
	}
}

func loweredGraph(objects map[string]pipeline.LoweredObject) pipeline.LoweredGraph {
	return pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/codex/local-dev": {
				Coordinate: codexCoord(),
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

func keptRule(id, content string, scopePaths []string, conditions []model.RuleCondition) pipeline.LoweredObject {
	fields := map[string]any{}
	if len(scopePaths) > 0 {
		pathsAny := make([]any, len(scopePaths))
		for i, p := range scopePaths {
			pathsAny[i] = p
		}
		fields["scope"] = map[string]any{"paths": pathsAny}
	}
	if len(conditions) > 0 {
		condsAny := make([]any, len(conditions))
		for i, c := range conditions {
			condsAny[i] = map[string]any{"type": c.Type, "value": c.Value}
		}
		fields["conditions"] = condsAny
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

// ---------------------------------------------------------------------------
// 1. Renderer Registration Tests
// ---------------------------------------------------------------------------

func TestRendererTarget(t *testing.T) {
	r := codex.New(nil)
	if r.Target() != build.TargetCodex {
		t.Fatalf("expected target %q, got %q", build.TargetCodex, r.Target())
	}
}

func TestRendererDescriptor(t *testing.T) {
	r := codex.New(nil)
	desc := r.Descriptor()

	if desc.Name != "codex-renderer" {
		t.Errorf("expected name %q, got %q", "codex-renderer", desc.Name)
	}
	if desc.Phase != pipeline.PhaseRender {
		t.Errorf("expected phase %v, got %v", pipeline.PhaseRender, desc.Phase)
	}
	if desc.Order != 10 {
		t.Errorf("expected order 10, got %d", desc.Order)
	}
	if len(desc.TargetFilter) != 1 || desc.TargetFilter[0] != build.TargetCodex {
		t.Errorf("expected target filter [codex], got %v", desc.TargetFilter)
	}
}

func TestRendererSupportedCapabilities(t *testing.T) {
	r := codex.New(nil)
	reg := r.SupportedCapabilities()

	if reg.Target != "codex" {
		t.Errorf("expected target %q, got %q", "codex", reg.Target)
	}
	if len(reg.Surfaces) == 0 {
		t.Error("expected non-empty surfaces")
	}

	// Verify key codex-specific capability levels.
	expected := map[string]string{
		"instructions.layeredFiles":      "native",
		"commands.explicitEntryPoints":   "lowered",
		"agents.handoffs":               "skipped",
		"skills.bundles":                "native",
		"hooks.lifecycle":               "native",
		"mcp.serverBindings":            "native",
	}
	for key, wantLevel := range expected {
		got, ok := reg.Surfaces[key]
		if !ok {
			t.Errorf("missing surface %q", key)
			continue
		}
		if string(got) != wantLevel {
			t.Errorf("surface %q: expected %q, got %q", key, wantLevel, got)
		}
	}
}

func TestRendererFactory(t *testing.T) {
	factory := codex.Factory(nil)
	s, err := factory()
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if s.Descriptor().Name != "codex-renderer" {
		t.Errorf("expected codex-renderer from factory, got %q", s.Descriptor().Name)
	}
}

// ---------------------------------------------------------------------------
// 2. Invalid Input Tests
// ---------------------------------------------------------------------------

func TestExecuteWithInvalidInput(t *testing.T) {
	r := codex.New(nil)
	_, err := r.Execute(testContext(), "not-a-graph")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestExecuteWithNilPointerInput(t *testing.T) {
	r := codex.New(nil)
	var graphPtr *pipeline.LoweredGraph
	_, err := r.Execute(testContext(), graphPtr)
	if err == nil {
		t.Fatal("expected error for nil pointer input")
	}
}

func TestExecuteWithPointerInput(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst": keptInstruction("inst", "Content.", nil),
	})

	result, err := r.Execute(testContext(), &graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	if len(plan.Units) != 1 {
		t.Errorf("expected 1 unit, got %d", len(plan.Units))
	}
}

// ---------------------------------------------------------------------------
// 3. Instruction Layer Tests
// ---------------------------------------------------------------------------

func TestRootInstruction(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"arch-inst": keptInstruction("arch-inst", "Always use Go modules.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found")
	}

	content := string(agentsMD.Content)
	if !strings.Contains(content, "Always use Go modules.") {
		t.Error("instruction content not in AGENTS.md")
	}
}

func TestSubtreeInstruction(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"root-inst": keptInstruction("root-inst", "Root instruction.", nil),
		"api-inst":  keptInstruction("api-inst", "API instruction.", []string{"services/api"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	rootMD := findFile(unit.Files, "AGENTS.md")
	if rootMD == nil {
		t.Fatal("AGENTS.md not found")
	}

	apiMD := findFile(unit.Files, "services/api/AGENTS.md")
	if apiMD == nil {
		t.Fatal("services/api/AGENTS.md not found")
	}

	if !strings.Contains(string(apiMD.Content), "API instruction.") {
		t.Error("subtree instruction content not in services/api/AGENTS.md")
	}
}

func TestMultipleInstructionsInSameScope(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"b-inst": keptInstruction("b-inst", "Second.", nil),
		"a-inst": keptInstruction("a-inst", "First.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found")
	}

	content := string(agentsMD.Content)
	aIdx := strings.Index(content, "First.")
	bIdx := strings.Index(content, "Second.")
	if aIdx < 0 || bIdx < 0 {
		t.Fatal("both instructions should be present")
	}
	if aIdx > bIdx {
		t.Error("instructions should be sorted by ID (a-inst before b-inst)")
	}
}

func TestInstructionFileNameIsAGENTSNotCLAUDE(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst": keptInstruction("inst", "Content.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		if strings.Contains(f.Path, "CLAUDE.md") {
			t.Errorf("codex renderer should produce AGENTS.md, not CLAUDE.md; found %s", f.Path)
		}
	}

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md should be produced by codex renderer")
	}
}

// ---------------------------------------------------------------------------
// 4. Rules Layer Tests
// ---------------------------------------------------------------------------

func TestRuleWithPaths(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"go-rule": keptRule("go-rule", "Use gofmt.", []string{"**/*.go"}, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	ruleFile := findFile(unit.Files, ".codex/rules/go-rule.md")
	if ruleFile == nil {
		t.Fatal(".codex/rules/go-rule.md not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "paths:") {
		t.Error("rule should have paths frontmatter")
	}
	if !strings.Contains(content, "**/*.go") {
		t.Error("rule path not present")
	}
	if !strings.Contains(content, "Use gofmt.") {
		t.Error("rule content missing")
	}
}

func TestRuleWithConditions(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"cond-rule": keptRule("cond-rule", "Use ESLint.", nil, []model.RuleCondition{
			{Type: "language", Value: "typescript"},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	ruleFile := findFile(unit.Files, ".codex/rules/cond-rule.md")
	if ruleFile == nil {
		t.Fatal(".codex/rules/cond-rule.md not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "## Conditions") {
		t.Error("conditions section missing")
	}
	if !strings.Contains(content, "**language**: typescript") {
		t.Error("condition content missing")
	}
}

func TestRuleUsesCodexPathNotClaude(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-rule": keptRule("my-rule", "A rule.", nil, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		if strings.Contains(f.Path, ".claude/") {
			t.Errorf("codex renderer should not produce .claude/ paths; found %s", f.Path)
		}
	}

	ruleFile := findFile(unit.Files, ".codex/rules/my-rule.md")
	if ruleFile == nil {
		t.Fatal(".codex/rules/my-rule.md should exist")
	}
}

// ---------------------------------------------------------------------------
// 5. Skills Layer Tests
// ---------------------------------------------------------------------------

func TestSkillGeneration(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"iam-skill": keptSkill("iam-skill", "Review IAM policies.", map[string]any{
			"description":     "IAM review skill",
			"activationHints": []any{"IAM", "security"},
			"tools":           []any{"Read"},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	skillFile := findFile(unit.Files, ".codex/skills/iam-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal(".codex/skills/iam-skill/SKILL.md not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "name: iam-skill") {
		t.Error("skill name missing")
	}
	if !strings.Contains(content, "description: IAM review skill") {
		t.Error("skill description missing")
	}
	if !strings.Contains(content, "activation_hints:") {
		t.Error("activation hints missing")
	}
	if !strings.Contains(content, "Review IAM policies.") {
		t.Error("skill body missing")
	}
}

func TestSkillUsesCodexPathNotClaude(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-skill": keptSkill("my-skill", "Skill content.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		if strings.Contains(f.Path, ".claude/skills/") {
			t.Errorf("codex renderer should use .codex/skills/, not .claude/skills/; found %s", f.Path)
		}
	}

	skillFile := findFile(unit.Files, ".codex/skills/my-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal(".codex/skills/my-skill/SKILL.md should exist")
	}
}

func TestSkillWithPublishingMetadata(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"pub-skill": keptSkill("pub-skill", "Published skill.", map[string]any{
			"publishing": map[string]any{
				"author":   "Test Author",
				"homepage": "https://example.com",
				"emoji":    "🔧",
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	skillFile := findFile(unit.Files, ".codex/skills/pub-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("pub-skill SKILL.md not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "author: Test Author") {
		t.Error("author missing from publishing metadata")
	}
	if !strings.Contains(content, "homepage:") || !strings.Contains(content, "https://example.com") {
		t.Error("homepage missing")
	}
}

// ---------------------------------------------------------------------------
// 6. Agents Layer Tests
// ---------------------------------------------------------------------------

func TestAgentGeneration(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"review-agent": keptAgent("review-agent", "You specialize in code review.", map[string]any{
			"description": "Review agent",
			"skills":      []any{"iam-skill"},
			"delegation":  map[string]any{"mayCall": []any{"deploy-agent"}},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentFile := findFile(unit.Files, ".codex/agents/review-agent.md")
	if agentFile == nil {
		t.Fatal(".codex/agents/review-agent.md not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "name: review-agent") {
		t.Error("agent name missing")
	}
	if !strings.Contains(content, "description: Review agent") {
		t.Error("agent description missing")
	}
	if !strings.Contains(content, "skills:") {
		t.Error("skills section missing")
	}
	if !strings.Contains(content, "mayCall:") {
		t.Error("delegation mayCall missing")
	}
	if !strings.Contains(content, "You specialize in code review.") {
		t.Error("agent body missing")
	}
}

func TestAgentWithTools(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"secure-agent": keptAgent("secure-agent", "Security agent.", map[string]any{
			"tools":           []any{"Read"},
			"disallowedTools": []any{"Delete", "Write"},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentFile := findFile(unit.Files, ".codex/agents/secure-agent.md")
	if agentFile == nil {
		t.Fatal("secure-agent.md not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "tools:\n  - Read") {
		t.Error("allowed tools missing")
	}
	if !strings.Contains(content, "disallowedTools:") {
		t.Error("disallowed tools section missing")
	}
}

func TestAgentUsesCodexPathNotClaude(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-agent": keptAgent("my-agent", "Agent content.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		if strings.Contains(f.Path, ".claude/agents/") {
			t.Errorf("codex renderer should use .codex/agents/, not .claude/agents/; found %s", f.Path)
		}
	}
}

// ---------------------------------------------------------------------------
// 7. Hooks Layer Tests
// ---------------------------------------------------------------------------

func TestSupportedHookEvents(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"start-hook":  keptHook("start-hook", "SessionStart", "command", "echo started", nil),
		"prompt-hook": keptHook("prompt-hook", "UserPromptSubmit", "command", "validate", nil),
		"pre-tool":    keptHook("pre-tool", "PreToolUse", "command", "pre-check", nil),
		"post-tool":   keptHook("post-tool", "PostToolUse", "command", "post-check", nil),
		"stop-hook":   keptHook("stop-hook", "Stop", "command", "cleanup", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	settingsFile := findFile(unit.Files, ".codex/settings.json")
	if settingsFile == nil {
		t.Fatal(".codex/settings.json not found")
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsFile.Content, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks := settings["hooks"].(map[string]any)
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		if _, ok := hooks[event]; !ok {
			t.Errorf("expected hook event %q in settings.json", event)
		}
	}
}

func TestUnsupportedHookEventsFiltered(t *testing.T) {
	r := codex.New(nil)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(t.Context(), cc)

	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"supported-hook":   keptHook("supported-hook", "SessionStart", "command", "echo ok", nil),
		"unsupported-hook": keptHook("unsupported-hook", "SessionEnd", "command", "echo bye", nil),
		"permission-hook":  keptHook("permission-hook", "PermissionRequest", "command", "validate", nil),
	})

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	settingsFile := findFile(unit.Files, ".codex/settings.json")
	if settingsFile == nil {
		t.Fatal(".codex/settings.json not found")
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsFile.Content, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks := settings["hooks"].(map[string]any)
	if _, ok := hooks["SessionEnd"]; ok {
		t.Error("SessionEnd should be filtered out for Codex")
	}
	if _, ok := hooks["PermissionRequest"]; ok {
		t.Error("PermissionRequest should be filtered out for Codex")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("SessionStart should be present")
	}

	// Check for diagnostic warning about unsupported events.
	foundWarning := false
	for _, d := range report.Diagnostics {
		if d.Code == "RENDER_UNSUPPORTED_HOOK_EVENT" {
			foundWarning = true
			if d.Severity != "warning" {
				t.Errorf("expected warning severity, got %q", d.Severity)
			}
			break
		}
	}
	if !foundWarning {
		t.Error("expected RENDER_UNSUPPORTED_HOOK_EVENT diagnostic")
	}
}

func TestHookWithMatcher(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"scoped-hook": keptHook("scoped-hook", "PreToolUse", "command", "lint", []string{"**/*.go"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	settingsFile := findFile(unit.Files, ".codex/settings.json")
	if settingsFile == nil {
		t.Fatal(".codex/settings.json not found")
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsFile.Content, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks := settings["hooks"].(map[string]any)
	entries := hooks["PreToolUse"].([]any)
	entry := entries[0].(map[string]any)
	if entry["matcher"] != "**/*.go" {
		t.Errorf("expected matcher '**/*.go', got %q", entry["matcher"])
	}
}

func TestHookSettingsUsesCodexPathNotClaude(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"hook": keptHook("hook", "SessionStart", "command", "echo hi", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		if f.Path == ".claude/settings.json" {
			t.Error("codex renderer should use .codex/settings.json, not .claude/settings.json")
		}
	}
}

// ---------------------------------------------------------------------------
// 8. MCP Layer Tests
// ---------------------------------------------------------------------------

func TestMCPConfiguration(t *testing.T) {
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

	mcpFile := findFile(unit.Files, ".mcp.json")
	if mcpFile == nil {
		t.Fatal(".mcp.json not found")
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

func TestMCPHttpTransport(t *testing.T) {
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

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

func TestMCPWithEnvVars(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"env-plugin": keptPlugin("env-plugin", map[string]any{
			"mcpServers": map[string]any{
				"myserver": map[string]any{
					"transport": "stdio",
					"command":   "myserver",
					"env": map[string]any{
						"API_KEY": "secret",
						"DEBUG":   "true",
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

	mcpFile := findFile(unit.Files, ".mcp.json")
	if mcpFile == nil {
		t.Fatal(".mcp.json not found")
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := mcpConfig["mcpServers"].(map[string]any)
	server := servers["myserver"].(map[string]any)
	env := server["env"].(map[string]any)
	if env["API_KEY"] != "secret" {
		t.Errorf("expected API_KEY=secret, got %q", env["API_KEY"])
	}
}

// ---------------------------------------------------------------------------
// 9. Plugin Layer Tests
// ---------------------------------------------------------------------------

func TestInlinePlugin(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-plugin": keptPlugin("my-plugin", map[string]any{
			"distribution": map[string]any{"mode": "inline"},
			"artifacts": map[string]any{
				"scripts": []any{"scripts/run.sh"},
				"configs": []any{"config/settings.yaml"},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	if len(unit.PluginBundles) == 0 {
		t.Fatal("expected plugin bundles")
	}

	bundle := unit.PluginBundles[0]
	if !strings.HasPrefix(bundle.DestDir, ".codex-plugin/") {
		t.Errorf("expected .codex-plugin/ prefix, got %q", bundle.DestDir)
	}
}

func TestRegistryPlugin(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"reg-plugin": keptPlugin("reg-plugin", map[string]any{
			"distribution": map[string]any{
				"mode":    "registry",
				"ref":     "org/plugin",
				"version": "1.2.3",
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	if len(unit.InstallMetadata) == 0 {
		t.Fatal("expected install metadata for registry plugin")
	}

	entry := unit.InstallMetadata[0]
	if entry.PluginID != "reg-plugin" {
		t.Errorf("expected plugin ID 'reg-plugin', got %q", entry.PluginID)
	}
	if entry.Config["ref"] != "org/plugin" {
		t.Errorf("expected ref 'org/plugin', got %v", entry.Config["ref"])
	}
}

func TestPluginUsesCodexPathNotClaude(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-plugin": keptPlugin("my-plugin", map[string]any{
			"distribution": map[string]any{"mode": "inline"},
			"artifacts":    map[string]any{"scripts": []any{"run.sh"}},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, bundle := range unit.PluginBundles {
		if strings.Contains(bundle.DestDir, ".claude-plugin/") {
			t.Errorf("codex renderer should use .codex-plugin/, not .claude-plugin/; found %s", bundle.DestDir)
		}
	}
}

// ---------------------------------------------------------------------------
// 10. Command Lowering Tests
// ---------------------------------------------------------------------------

func TestCommandObjectsEmitWarningDiagnostic(t *testing.T) {
	r := codex.New(nil)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(t.Context(), cc)

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

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	for _, f := range unit.Files {
		if f.Path != "provenance.json" && strings.Contains(f.Path, "my-cmd") {
			t.Errorf("command should not be rendered, but found file: %s", f.Path)
		}
	}

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
// 11. Provenance Tests
// ---------------------------------------------------------------------------

func TestProvenanceJSONHasCodexTarget(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst": keptInstruction("inst", "Content.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	provFile := findFile(unit.Files, "provenance.json")
	if provFile == nil {
		t.Fatal("provenance.json not found")
	}

	var prov map[string]any
	if err := json.Unmarshal(provFile.Content, &prov); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if prov["target"] != "codex" {
		t.Errorf("expected target 'codex' in provenance.json, got %q", prov["target"])
	}
}

func TestProvenanceJSONMapsAllSources(t *testing.T) {
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

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

func TestProvenanceHeaderNotOnJSONFiles(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"lint-hook": keptHook("lint-hook", "PreToolUse", "command", "make lint", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

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
// 12. Skipped Object Tests
// ---------------------------------------------------------------------------

func TestSkippedObjectsNotRendered(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"active-inst":  keptInstruction("active-inst", "Active.", nil),
		"skipped-inst": skippedObject("skipped-inst", model.KindInstruction),
		"skipped-rule": skippedObject("skipped-rule", model.KindRule),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found")
	}

	content := string(agentsMD.Content)
	if strings.Contains(content, "skipped-inst") {
		t.Error("skipped instruction should not appear in AGENTS.md")
	}

	// Should have no rule files for the skipped rule.
	ruleFile := findFile(unit.Files, ".codex/rules/skipped-rule.md")
	if ruleFile != nil {
		t.Error("skipped rule should not produce a file")
	}
}

// ---------------------------------------------------------------------------
// 13. Target Isolation Tests (skip non-codex units)
// ---------------------------------------------------------------------------

func TestNonCodexUnitsSkipped(t *testing.T) {
	r := codex.New(nil)
	graph := pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/codex/local-dev": {
				Coordinate: codexCoord(),
				Objects: map[string]pipeline.LoweredObject{
					"inst": keptInstruction("inst", "Codex content.", nil),
				},
			},
			".ai-build/claude/local-dev": {
				Coordinate: build.BuildCoordinate{
					Unit: build.BuildUnit{
						Target:  build.TargetClaude,
						Profile: build.ProfileLocalDev,
					},
					OutputDir: ".ai-build/claude/local-dev",
				},
				Objects: map[string]pipeline.LoweredObject{
					"inst": keptInstruction("inst", "Claude content.", nil),
				},
			},
		},
	}

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)

	if _, ok := plan.Units[".ai-build/claude/local-dev"]; ok {
		t.Error("codex renderer should not render Claude units")
	}

	if _, ok := plan.Units[".ai-build/codex/local-dev"]; !ok {
		t.Error("codex renderer should render codex units")
	}
}

// ---------------------------------------------------------------------------
// 14. Directories Collection Tests
// ---------------------------------------------------------------------------

func TestDirectoriesCollectedFromAllLayers(t *testing.T) {
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

	if len(unit.Directories) == 0 {
		t.Fatal("expected directories to be populated")
	}

	expectedDirs := map[string]bool{
		".codex/rules":        true,
		".codex/skills/skill": true,
		".codex/agents":       true,
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
// 15. sanitizeID Tests
// ---------------------------------------------------------------------------

func TestSanitizeIDInFilePaths(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"org/my.skill": keptSkill("org/my.skill", "Skill with special ID.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	skillFile := findFile(unit.Files, ".codex/skills/org-my-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("skill file with sanitized ID not found")
	}
}

// ---------------------------------------------------------------------------
// 16. Empty / Nil Edge Cases
// ---------------------------------------------------------------------------

func TestEmptyGraph(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	// Only provenance.json should be present.
	if len(unit.Files) != 1 {
		t.Errorf("expected 1 file (provenance.json) for empty graph, got %d", len(unit.Files))
	}
	if unit.Files[0].Path != "provenance.json" {
		t.Errorf("expected provenance.json, got %s", unit.Files[0].Path)
	}
}

func TestSkillWithNilFields(t *testing.T) {
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

	skillFile := findFile(unit.Files, ".codex/skills/bare-skill/SKILL.md")
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
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentFile := findFile(unit.Files, ".codex/agents/bare-agent.md")
	if agentFile == nil {
		t.Fatal("bare-agent.md not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "name: bare-agent") {
		t.Error("agent should have name even with nil fields")
	}
}

func TestHookWithEmptyEvent(t *testing.T) {
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

	settingsFile := findFile(unit.Files, ".codex/settings.json")
	if settingsFile != nil {
		t.Error("hook with empty event should not produce settings.json")
	}
}

func TestRuleWithNoPathsOrConditions(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"bare-rule": keptRule("bare-rule", "Just a rule.", nil, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	ruleFile := findFile(unit.Files, ".codex/rules/bare-rule.md")
	if ruleFile == nil {
		t.Fatal("bare-rule.md not found")
	}

	content := string(ruleFile.Content)
	if strings.HasPrefix(content, "---\npaths:") {
		t.Error("rule without paths should not have paths frontmatter")
	}
	if strings.Contains(content, "## Conditions") {
		t.Error("rule without conditions should not have conditions section")
	}
	if !strings.Contains(content, "Just a rule.") {
		t.Error("rule content missing")
	}
}

func TestInstructionWithEmptyContent(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"empty-inst": keptInstruction("empty-inst", "", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found even for empty instruction")
	}
}

// ---------------------------------------------------------------------------
// 17. Full Project Integration Test
// ---------------------------------------------------------------------------

func TestFullProject(t *testing.T) {
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
			"tools":           []any{"Read"},
		}),
		"review-agent": keptAgent("review-agent", "You specialize in code review.", map[string]any{
			"description": "Review agent",
			"skills":      []any{"iam-skill"},
			"delegation":  map[string]any{"mayCall": []any{"deploy-agent"}},
		}),
		"start-hook": keptHook("start-hook", "SessionStart", "command", "echo started", nil),
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

	// Verify all expected files exist.
	expectedFiles := []string{
		"AGENTS.md",
		"services/api/AGENTS.md",
		".codex/rules/go-rule.md",
		".codex/skills/iam-skill/SKILL.md",
		".codex/agents/review-agent.md",
		".codex/settings.json",
		".mcp.json",
		"provenance.json",
	}

	for _, path := range expectedFiles {
		if findFile(unit.Files, path) == nil {
			t.Errorf("expected file %q not found", path)
		}
	}

	// Verify NO Claude paths snuck in.
	for _, f := range unit.Files {
		if strings.Contains(f.Path, "CLAUDE.md") {
			t.Errorf("found CLAUDE.md in codex output: %s", f.Path)
		}
		if strings.Contains(f.Path, ".claude/") {
			t.Errorf("found .claude/ path in codex output: %s", f.Path)
		}
	}
}

// ---------------------------------------------------------------------------
// 18. Rule with Special Characters in Paths
// ---------------------------------------------------------------------------

func TestRulePathsWithSpecialChars(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"glob-rule": keptRule("glob-rule", "Glob rule.", []string{"src/**/*.{ts,tsx}", "!**/*.test.ts"}, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	ruleFile := findFile(unit.Files, ".codex/rules/glob-rule.md")
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
// 19. All Hooks Only Unsupported → No settings.json
// ---------------------------------------------------------------------------

func TestAllUnsupportedHooksProduceNoSettings(t *testing.T) {
	r := codex.New(nil)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(t.Context(), cc)

	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"unsupported1": keptHook("unsupported1", "SessionEnd", "command", "cleanup", nil),
		"unsupported2": keptHook("unsupported2", "PermissionRequest", "command", "check", nil),
	})

	result, err := r.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	settingsFile := findFile(unit.Files, ".codex/settings.json")
	if settingsFile != nil {
		t.Error("settings.json should not be produced when all hooks are unsupported")
	}

	foundWarning := false
	for _, d := range report.Diagnostics {
		if d.Code == "RENDER_UNSUPPORTED_HOOK_EVENT" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected RENDER_UNSUPPORTED_HOOK_EVENT diagnostic")
	}
}

// ---------------------------------------------------------------------------
// 20. Skill Sidecar Assets (QA fix: untested path)
// ---------------------------------------------------------------------------

func TestSkillWithResources(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"res-skill": keptSkill("res-skill", "Skill with resources.", map[string]any{
			"resources": map[string]any{
				"references": []any{"docs/guide.md"},
				"assets":     []any{"templates/t.yaml"},
				"scripts":    []any{"scripts/run.sh"},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	if len(unit.Assets) == 0 {
		t.Fatal("expected sidecar assets for skill with resources")
	}

	// Verify all assets use .codex/skills/ path.
	for _, asset := range unit.Assets {
		if !strings.HasPrefix(asset.DestPath, ".codex/skills/res-skill/") {
			t.Errorf("asset dest path should be under .codex/skills/res-skill/, got %q", asset.DestPath)
		}
		if strings.Contains(asset.DestPath, ".claude/") {
			t.Errorf("asset should not reference .claude/ paths, got %q", asset.DestPath)
		}
	}

	// Verify specific files are present.
	expectedFiles := map[string]bool{
		".codex/skills/res-skill/guide.md": false,
		".codex/skills/res-skill/t.yaml":   false,
		".codex/skills/res-skill/run.sh":   false,
	}
	for _, asset := range unit.Assets {
		expectedFiles[asset.DestPath] = true
	}
	for path, found := range expectedFiles {
		if !found {
			t.Errorf("expected sidecar asset %q not found", path)
		}
	}
}

// ---------------------------------------------------------------------------
// 21. YAML Injection Safety (QA fix: untested quoting)
// ---------------------------------------------------------------------------

func TestSkillDescriptionWithSpecialChars(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"special-skill": keptSkill("special-skill", "Skill body.", map[string]any{
			"description": "Handles # comments and : colons",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	skillFile := findFile(unit.Files, ".codex/skills/special-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("special-skill SKILL.md not found")
	}

	content := string(skillFile.Content)
	// Description with # should be quoted to prevent YAML comment truncation.
	if !strings.Contains(content, `"Handles # comments and`) {
		t.Error("description with # should be YAML-quoted")
	}
}

func TestAgentDescriptionWithNewline(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"nl-agent": keptAgent("nl-agent", "Agent body.", map[string]any{
			"description": "line1\nline2",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	agentFile := findFile(unit.Files, ".codex/agents/nl-agent.md")
	if agentFile == nil {
		t.Fatal("nl-agent.md not found")
	}

	content := string(agentFile.Content)
	// Newline in description should be escaped, not produce a literal newline.
	if strings.Contains(content, "description: line1\nline2") {
		t.Error("description with newline should be YAML-escaped, not literal")
	}
	if !strings.Contains(content, `\n`) {
		t.Error("expected escaped newline in description")
	}
}

// ---------------------------------------------------------------------------
// 22. MCP Server Name Collision (QA fix: untested collision)
// ---------------------------------------------------------------------------

func TestMCPServerNameCollisionFirstWins(t *testing.T) {
	r := codex.New(nil)
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
	unit := plan.Units[".ai-build/codex/local-dev"]

	mcpFile := findFile(unit.Files, ".mcp.json")
	if mcpFile == nil {
		t.Fatal(".mcp.json not found")
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := mcpConfig["mcpServers"].(map[string]any)
	server := servers["shared-server"].(map[string]any)
	// Plugins are sorted by ID; "alpha-plugin" comes first, so its command wins.
	if server["command"] != "alpha-cmd" {
		t.Errorf("expected first-writer-wins (alpha-cmd), got %q", server["command"])
	}
}

// ---------------------------------------------------------------------------
// 23. External Plugin Distribution (QA fix: untested mode)
// ---------------------------------------------------------------------------

func TestExternalPluginProducesMCPNotBundle(t *testing.T) {
	r := codex.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"ext-plugin": keptPlugin("ext-plugin", map[string]any{
			"distribution": map[string]any{
				"mode": "external",
				"ref":  "my-mcp-server",
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/codex/local-dev"]

	// External plugins should NOT produce plugin bundles.
	if len(unit.PluginBundles) > 0 {
		t.Error("external plugin should not produce plugin bundles")
	}

	// Should produce .mcp.json with the server.
	mcpFile := findFile(unit.Files, ".mcp.json")
	if mcpFile == nil {
		t.Fatal(".mcp.json not found for external plugin")
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := mcpConfig["mcpServers"].(map[string]any)
	// The server name should be the sanitized plugin ID.
	if _, ok := servers["ext-plugin"]; !ok {
		t.Error("expected MCP server entry for external plugin")
	}
	server := servers["ext-plugin"].(map[string]any)
	if server["command"] != "my-mcp-server" {
		t.Errorf("expected command 'my-mcp-server', got %q", server["command"])
	}
}

// ---------------------------------------------------------------------------
// 24. Deterministic Output (QA fix: untested determinism)
// ---------------------------------------------------------------------------

func TestDeterministicOutput(t *testing.T) {
	objects := map[string]pipeline.LoweredObject{
		"z-instruction": keptInstruction("z-instruction", "Z content.", nil),
		"a-instruction": keptInstruction("a-instruction", "A content.", nil),
		"m-rule":        keptRule("m-rule", "M rule.", []string{"**/*.go"}, nil),
		"b-skill": keptSkill("b-skill", "B skill.", map[string]any{
			"description": "B desc",
		}),
		"c-agent": keptAgent("c-agent", "C agent.", map[string]any{
			"description": "C desc",
		}),
		"hook-pre":  keptHook("hook-pre", "PreToolUse", "command", "lint", nil),
		"hook-stop": keptHook("hook-stop", "Stop", "command", "cleanup", nil),
		"mcp-plugin": keptPlugin("mcp-plugin", map[string]any{
			"mcpServers": map[string]any{
				"test-server": map[string]any{
					"transport": "stdio",
					"command":   "test-cmd",
				},
			},
		}),
	}

	r := codex.New(nil)
	graph := loweredGraph(objects)

	result1, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	result2, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	plan1 := result1.(pipeline.EmissionPlan)
	plan2 := result2.(pipeline.EmissionPlan)

	unit1 := plan1.Units[".ai-build/codex/local-dev"]
	unit2 := plan2.Units[".ai-build/codex/local-dev"]

	if len(unit1.Files) != len(unit2.Files) {
		t.Fatalf("different file counts: %d vs %d", len(unit1.Files), len(unit2.Files))
	}

	for i := range unit1.Files {
		f1 := unit1.Files[i]
		f2 := unit2.Files[i]
		if f1.Path != f2.Path {
			t.Errorf("file %d path mismatch: %q vs %q", i, f1.Path, f2.Path)
		}
		if string(f1.Content) != string(f2.Content) {
			t.Errorf("file %d (%s) content mismatch between runs", i, f1.Path)
		}
	}
}
