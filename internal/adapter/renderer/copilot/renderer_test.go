package copilot_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/copilot"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/renderer"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time interface checks.
var (
	_ stage.Stage       = (*copilot.Renderer)(nil)
	_ renderer.Renderer = (*copilot.Renderer)(nil)
)

// ---------------------------------------------------------------------------
// Helper constructors
// ---------------------------------------------------------------------------

func copilotCoord() build.BuildCoordinate {
	return build.BuildCoordinate{
		Unit: build.BuildUnit{
			Target:  build.TargetCopilot,
			Profile: build.ProfileLocalDev,
		},
		OutputDir: ".ai-build/copilot/local-dev",
	}
}

func loweredGraph(objects map[string]pipeline.LoweredObject) pipeline.LoweredGraph {
	return pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/copilot/local-dev": {
				Coordinate: copilotCoord(),
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

func keptRule(id, content string, scopePaths []string) pipeline.LoweredObject {
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
		OriginalKind: model.KindRule,
		LoweredKind:  model.KindRule,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      content,
		Fields:       fields,
	}
}

func keptRuleWithConditions(id, content string, scopePaths []string, conditions []map[string]string) pipeline.LoweredObject {
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
			condsAny[i] = map[string]any{"type": c["type"], "value": c["value"]}
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

func keptCommand(id, content string, fields map[string]any) pipeline.LoweredObject {
	if fields == nil {
		fields = map[string]any{}
	}
	return pipeline.LoweredObject{
		OriginalID:   id,
		OriginalKind: model.KindCommand,
		LoweredKind:  model.KindCommand,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      content,
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
			t.Errorf("source object[%d] = %q, want %q", i, f.SourceObjects[i], e)
		}
	}
}

func unitFromResult(t *testing.T, result any) pipeline.UnitEmission {
	t.Helper()
	plan, ok := result.(pipeline.EmissionPlan)
	if !ok {
		t.Fatalf("expected EmissionPlan, got %T", result)
	}
	unit, ok := plan.Units[".ai-build/copilot/local-dev"]
	if !ok {
		t.Fatal("copilot unit not found in plan")
	}
	return unit
}

// ---------------------------------------------------------------------------
// 1. Renderer Registration Tests
// ---------------------------------------------------------------------------

func TestRendererTarget(t *testing.T) {
	r := copilot.New(nil)
	if r.Target() != build.TargetCopilot {
		t.Fatalf("expected target %q, got %q", build.TargetCopilot, r.Target())
	}
}

func TestRendererDescriptor(t *testing.T) {
	r := copilot.New(nil)
	desc := r.Descriptor()

	if desc.Name != "copilot-renderer" {
		t.Errorf("expected name %q, got %q", "copilot-renderer", desc.Name)
	}
	if desc.Phase != pipeline.PhaseRender {
		t.Errorf("expected phase %v, got %v", pipeline.PhaseRender, desc.Phase)
	}
	if desc.Order != 10 {
		t.Errorf("expected order 10, got %d", desc.Order)
	}
	if len(desc.TargetFilter) != 1 || desc.TargetFilter[0] != build.TargetCopilot {
		t.Errorf("expected target filter [copilot], got %v", desc.TargetFilter)
	}
}

func TestRendererSupportedCapabilities(t *testing.T) {
	r := copilot.New(nil)
	caps := r.SupportedCapabilities()
	if caps.Target != "copilot" {
		t.Fatalf("expected target %q, got %q", "copilot", caps.Target)
	}
	if len(caps.Surfaces) == 0 {
		t.Fatal("expected non-empty capability surfaces")
	}
	// Copilot should have handoffs as native.
	if caps.Surfaces["agents.handoffs"] != "native" {
		t.Errorf("expected agents.handoffs = native, got %q", caps.Surfaces["agents.handoffs"])
	}
}

func TestRendererFactory(t *testing.T) {
	factory := copilot.Factory(nil)
	s, err := factory()
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if s == nil {
		t.Fatal("factory returned nil stage")
	}
	if s.Descriptor().Name != "copilot-renderer" {
		t.Errorf("expected name %q, got %q", "copilot-renderer", s.Descriptor().Name)
	}
}

func TestRendererRejectsInvalidInput(t *testing.T) {
	r := copilot.New(nil)
	_, err := r.Execute(testContext(), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if !strings.Contains(err.Error(), "expected pipeline.LoweredGraph") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRendererAcceptsPointerInput(t *testing.T) {
	r := copilot.New(nil)
	graph := &pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/copilot/local-dev": {
				Coordinate: copilotCoord(),
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

func TestRootInstructionEmitsBothFiles(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1": keptInstruction("inst-1", "Always use Go modules.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// Root instructions should produce BOTH copilot-instructions.md AND AGENTS.md.
	copilotMD := findFile(unit.Files, ".github/copilot-instructions.md")
	if copilotMD == nil {
		t.Fatal(".github/copilot-instructions.md not found")
	}
	if !strings.Contains(string(copilotMD.Content), "Always use Go modules.") {
		t.Error("copilot-instructions.md missing instruction content")
	}
	if copilotMD.Layer != pipeline.LayerInstruction {
		t.Errorf("expected layer %q, got %q", pipeline.LayerInstruction, copilotMD.Layer)
	}
	assertSourceObjects(t, copilotMD, "inst-1")

	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found")
	}
	if !strings.Contains(string(agentsMD.Content), "Always use Go modules.") {
		t.Error("AGENTS.md missing instruction content")
	}
}

func TestSubtreeInstructionEmitsAGENTSMD(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-root":    keptInstruction("inst-root", "Root instruction.", nil),
		"inst-subtree": keptInstruction("inst-subtree", "Subtree instruction.", []string{"services/api"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// Root scope: both files.
	copilotMD := findFile(unit.Files, ".github/copilot-instructions.md")
	if copilotMD == nil {
		t.Fatal(".github/copilot-instructions.md not found")
	}
	agentsMD := findFile(unit.Files, "AGENTS.md")
	if agentsMD == nil {
		t.Fatal("AGENTS.md not found")
	}

	// Subtree scope: AGENTS.md in subtree.
	subtreeMD := findFile(unit.Files, "services/api/AGENTS.md")
	if subtreeMD == nil {
		t.Fatal("services/api/AGENTS.md not found")
	}
	if !strings.Contains(string(subtreeMD.Content), "Subtree instruction.") {
		t.Error("services/api/AGENTS.md missing subtree instruction")
	}
}

func TestMultipleInstructionsMergedSorted(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"z-inst": keptInstruction("z-inst", "Z instruction.", nil),
		"a-inst": keptInstruction("a-inst", "A instruction.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	copilotMD := findFile(unit.Files, ".github/copilot-instructions.md")
	if copilotMD == nil {
		t.Fatal("copilot-instructions.md not found")
	}

	content := string(copilotMD.Content)
	aIdx := strings.Index(content, "A instruction.")
	zIdx := strings.Index(content, "Z instruction.")
	if aIdx < 0 || zIdx < 0 || aIdx >= zIdx {
		t.Error("instructions not sorted by ID (A should come before Z)")
	}
}

// ---------------------------------------------------------------------------
// 3. Scoped Instructions/Rules Layer Tests (applyTo: comma-separated string)
// ---------------------------------------------------------------------------

func TestScopedRuleApplyToCommaSeparatedString(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"go-style": keptRule("go-style", "Use gofmt.", []string{"**/*.go", "**/*.mod"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	ruleFile := findFile(unit.Files, ".github/instructions/go-style.instructions.md")
	if ruleFile == nil {
		t.Fatal(".github/instructions/go-style.instructions.md not found")
	}

	content := string(ruleFile.Content)

	// CRITICAL: applyTo must be a comma-separated string, NOT a YAML array.
	if !strings.Contains(content, "applyTo:") {
		t.Error("rule missing applyTo: in frontmatter")
	}
	// Verify it's a string with comma separator.
	if !strings.Contains(content, "**/*.go, **/*.mod") {
		t.Errorf("applyTo should be comma-separated string, got content:\n%s", content)
	}
	// Ensure it's NOT a YAML array format.
	if strings.Contains(content, "  - **/*.go") {
		t.Error("applyTo should NOT be a YAML array, it should be a comma-separated string")
	}
	if !strings.Contains(content, "Use gofmt.") {
		t.Error("rule missing content body")
	}
}

func TestScopedRuleWithConditions(t *testing.T) {
	r := copilot.New(nil)
	conditions := []map[string]string{
		{"type": "language", "value": "go"},
		{"type": "generated", "value": "true"},
	}
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"gen-rule": keptRuleWithConditions("gen-rule", "Handle generated code carefully.", nil, conditions),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	ruleFile := findFile(unit.Files, ".github/instructions/gen-rule.instructions.md")
	if ruleFile == nil {
		t.Fatal(".github/instructions/gen-rule.instructions.md not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "## Conditions") {
		t.Error("rule missing conditions section")
	}
	if !strings.Contains(content, "**language**: go") {
		t.Error("rule missing language condition")
	}
}

func TestInstructionsMDExtensionForRules(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-rule": keptRule("my-rule", "Test rule.", []string{"src/**"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// Must use .instructions.md extension (not .md or .mdc).
	ruleFile := findFile(unit.Files, ".github/instructions/my-rule.instructions.md")
	if ruleFile == nil {
		t.Fatal("rule should use .instructions.md extension")
	}
}

// ---------------------------------------------------------------------------
// 4. Skills Layer Tests
// ---------------------------------------------------------------------------

func TestSkillInGitHubSkillsDir(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"iam-review": keptSkill("iam-review", "Review IAM policies.", map[string]any{
			"description":     "IAM review skill",
			"activationHints": []any{"IAM", "policy"},
			"allowedTools":    []any{"Read", "Write"},
			"resources": map[string]any{
				"references": []any{"docs/iam-guide.md"},
				"assets":     []any{"templates/iam.yaml"},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// Skills go to .github/skills/ (not .claude/skills/).
	skillFile := findFile(unit.Files, ".github/skills/iam-review/SKILL.md")
	if skillFile == nil {
		t.Fatal(".github/skills/iam-review/SKILL.md not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "name: iam-review") {
		t.Error("skill missing name")
	}
	if !strings.Contains(content, "description: IAM review skill") {
		t.Error("skill missing description")
	}
	if !strings.Contains(content, "activation_hints:") {
		t.Error("skill missing activation_hints")
	}
	// Copilot uses "tools:" not "allowed-tools:".
	if !strings.Contains(content, "tools:") {
		t.Error("skill missing tools")
	}
	if !strings.Contains(content, "Review IAM policies.") {
		t.Error("skill missing content body")
	}

	// Check sidecar assets.
	if len(unit.Assets) != 2 {
		t.Errorf("expected 2 assets, got %d", len(unit.Assets))
	}
}

func TestSkillWithPublishing(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-skill": keptSkill("my-skill", "Do things.", map[string]any{
			"publishing": map[string]any{
				"author":   "TestAuthor",
				"homepage": "https://example.com",
				"emoji":    "🔧",
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	skillFile := findFile(unit.Files, ".github/skills/my-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("skill file not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "author: TestAuthor") {
		t.Error("skill missing author")
	}
	if !strings.Contains(content, "homepage:") || !strings.Contains(content, "https://example.com") {
		t.Error("skill missing homepage")
	}
}

// ---------------------------------------------------------------------------
// 5. Agents Layer Tests (with handoffs)
// ---------------------------------------------------------------------------

func TestAgentWithAgentMDExtension(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"review-agent": keptAgent("review-agent", "You are a code review specialist.", map[string]any{
			"description": "Code review agent",
			"model":       "gpt-4o",
			"skills":      []any{"iam-review"},
			"toolPolicy": map[string]any{
				"Read":  "allow",
				"Write": "deny",
				"Bash":  "allow",
			},
			"delegation": map[string]any{
				"mayCall": []any{"deploy-agent", "test-agent"},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// CRITICAL: Copilot uses .agent.md extension.
	agentFile := findFile(unit.Files, ".github/agents/review-agent.agent.md")
	if agentFile == nil {
		t.Fatal(".github/agents/review-agent.agent.md not found (must use .agent.md extension)")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "name: review-agent") {
		t.Error("agent missing name")
	}
	if !strings.Contains(content, "description: Code review agent") {
		t.Error("agent missing description")
	}
	if !strings.Contains(content, "model: gpt-4o") {
		t.Error("agent missing model")
	}
	if !strings.Contains(content, "tools:") {
		t.Error("agent missing tools (allowed)")
	}
	if !strings.Contains(content, "disallowedTools:") {
		t.Error("agent missing disallowedTools")
	}
	// Copilot uses "agents:" for sub-agents (not "mayCall:").
	if !strings.Contains(content, "agents:") {
		t.Error("agent missing agents (sub-agent delegation list)")
	}
	if !strings.Contains(content, "- deploy-agent") {
		t.Error("agent missing delegation target")
	}
	if !strings.Contains(content, "You are a code review specialist.") {
		t.Error("agent missing body content")
	}
	assertSourceObjects(t, agentFile, "review-agent")
}

func TestAgentWithHandoffs(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"orchestrator": keptAgent("orchestrator", "You orchestrate work.", map[string]any{
			"description": "Orchestrator agent",
			"handoffs": []any{
				map[string]any{
					"label":  "Code Review",
					"agent":  "reviewer",
					"prompt": "Please review this code",
					"send":   "context",
					"model":  "gpt-4o",
				},
				map[string]any{
					"label": "Deploy",
					"agent": "deployer",
				},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	agentFile := findFile(unit.Files, ".github/agents/orchestrator.agent.md")
	if agentFile == nil {
		t.Fatal("orchestrator.agent.md not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "handoffs:") {
		t.Error("agent missing handoffs section")
	}
	if !strings.Contains(content, "label: Code Review") {
		t.Error("agent missing handoff label")
	}
	if !strings.Contains(content, "agent: reviewer") {
		t.Error("agent missing handoff agent")
	}
	if !strings.Contains(content, "prompt: Please review this code") {
		t.Error("agent missing handoff prompt")
	}
	if !strings.Contains(content, "send: context") {
		t.Error("agent missing handoff send")
	}
	if !strings.Contains(content, "model: gpt-4o") {
		t.Error("agent missing handoff model")
	}
	// Second handoff.
	if !strings.Contains(content, "label: Deploy") {
		t.Error("agent missing second handoff")
	}
}

// ---------------------------------------------------------------------------
// 6. Hooks Layer Tests (dual output)
// ---------------------------------------------------------------------------

func TestHookEmitsBothFormats(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"lint-hook": keptHook("lint-hook", "post-edit", "command", "golangci-lint run", []string{"**/*.go"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// 1. Copilot native: .github/hooks/{event}.json
	nativeFile := findFile(unit.Files, ".github/hooks/post-edit.json")
	if nativeFile == nil {
		t.Fatal(".github/hooks/post-edit.json not found")
	}
	if nativeFile.Layer != pipeline.LayerExtension {
		t.Errorf("expected layer %q, got %q", pipeline.LayerExtension, nativeFile.Layer)
	}

	var nativeHooks []map[string]any
	if err := json.Unmarshal(nativeFile.Content, &nativeHooks); err != nil {
		t.Fatalf("invalid JSON in native hooks: %v", err)
	}
	if len(nativeHooks) != 1 {
		t.Fatalf("expected 1 native hook entry, got %d", len(nativeHooks))
	}
	entry := nativeHooks[0]
	if entry["type"] != "command" {
		t.Errorf("expected hook type %q, got %q", "command", entry["type"])
	}
	if entry["command"] != "golangci-lint run" {
		t.Errorf("expected command %q, got %q", "golangci-lint run", entry["command"])
	}
	// CRITICAL: Copilot native hooks should NOT have matcher.
	if _, hasMatcher := entry["matcher"]; hasMatcher {
		t.Error("Copilot native hooks should NOT include matcher (Copilot ignores them)")
	}

	// 2. Claude compat: .claude/settings.json
	compatFile := findFile(unit.Files, ".claude/settings.json")
	if compatFile == nil {
		t.Fatal(".claude/settings.json not found for compat")
	}

	var settings map[string]any
	if err := json.Unmarshal(compatFile.Content, &settings); err != nil {
		t.Fatalf("invalid JSON in settings: %v", err)
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("settings.json missing hooks key")
	}
	postEdit, ok := hooks["post-edit"].([]any)
	if !ok || len(postEdit) == 0 {
		t.Fatal("settings.json missing post-edit hooks")
	}
	compatEntry := postEdit[0].(map[string]any)
	// Claude compat SHOULD have matcher.
	if compatEntry["matcher"] != "**/*.go" {
		t.Errorf("Claude compat hook should include matcher, got %v", compatEntry["matcher"])
	}
}

func TestMultipleHooksSameEvent(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"hook-a": keptHook("hook-a", "pre-tool-use", "command", "echo a", nil),
		"hook-b": keptHook("hook-b", "pre-tool-use", "command", "echo b", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	nativeFile := findFile(unit.Files, ".github/hooks/pre-tool-use.json")
	if nativeFile == nil {
		t.Fatal(".github/hooks/pre-tool-use.json not found")
	}

	var nativeHooks []map[string]any
	if err := json.Unmarshal(nativeFile.Content, &nativeHooks); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(nativeHooks) != 2 {
		t.Fatalf("expected 2 hooks for pre-tool-use, got %d", len(nativeHooks))
	}
}

func TestHooksMultipleEventsSeparateFiles(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"hook-post":   keptHook("hook-post", "post-edit", "command", "echo post", nil),
		"hook-pre":    keptHook("hook-pre", "pre-tool-use", "command", "echo pre", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	postFile := findFile(unit.Files, ".github/hooks/post-edit.json")
	if postFile == nil {
		t.Fatal(".github/hooks/post-edit.json not found")
	}
	preFile := findFile(unit.Files, ".github/hooks/pre-tool-use.json")
	if preFile == nil {
		t.Fatal(".github/hooks/pre-tool-use.json not found")
	}
}

// ---------------------------------------------------------------------------
// 7. Commands/Prompts Layer Tests
// ---------------------------------------------------------------------------

func TestCommandEmitsPromptMD(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"deploy-cmd": keptCommand("deploy-cmd", "Deploy the application to staging.", map[string]any{
			"name":        "deploy",
			"description": "Deploy to staging environment",
			"agent":       "deploy-agent",
			"model":       "gpt-4o",
			"tools":       []any{"Bash", "Read"},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	promptFile := findFile(unit.Files, ".github/prompts/deploy-cmd.prompt.md")
	if promptFile == nil {
		t.Fatal(".github/prompts/deploy-cmd.prompt.md not found")
	}

	content := string(promptFile.Content)
	if !strings.Contains(content, "---") {
		t.Error("prompt missing YAML frontmatter")
	}
	if !strings.Contains(content, "name: deploy") {
		t.Error("prompt missing name")
	}
	if !strings.Contains(content, "description: Deploy to staging environment") {
		t.Error("prompt missing description")
	}
	if !strings.Contains(content, "agent: deploy-agent") {
		t.Error("prompt missing agent")
	}
	if !strings.Contains(content, "model: gpt-4o") {
		t.Error("prompt missing model")
	}
	if !strings.Contains(content, "tools:") {
		t.Error("prompt missing tools")
	}
	if !strings.Contains(content, "Deploy the application to staging.") {
		t.Error("prompt missing body content")
	}
	assertSourceObjects(t, promptFile, "deploy-cmd")
}

func TestCommandDefaultsNameToID(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-cmd": keptCommand("my-cmd", "Do something.", map[string]any{
			"description": "A command",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	promptFile := findFile(unit.Files, ".github/prompts/my-cmd.prompt.md")
	if promptFile == nil {
		t.Fatal("prompt file not found")
	}

	content := string(promptFile.Content)
	if !strings.Contains(content, "name: my-cmd") {
		t.Error("command name should default to ID when no explicit name")
	}
}

// ---------------------------------------------------------------------------
// 8. MCP Layer Tests (CRITICAL: "servers" not "mcpServers")
// ---------------------------------------------------------------------------

func TestMCPConfigUsesServersKey(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"github-mcp": keptPlugin("github-mcp", map[string]any{
			"mcpServers": map[string]any{
				"github": map[string]any{
					"transport": "stdio",
					"command":   "npx",
					"args":      []any{"-y", "@modelcontextprotocol/server-github"},
					"env": map[string]any{
						"GITHUB_TOKEN": "${env:GITHUB_TOKEN}",
					},
				},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	mcpFile := findFile(unit.Files, ".vscode/mcp.json")
	if mcpFile == nil {
		t.Fatal(".vscode/mcp.json not found (should NOT be .mcp.json)")
	}
	if mcpFile.Layer != pipeline.LayerExtension {
		t.Errorf("expected layer %q, got %q", pipeline.LayerExtension, mcpFile.Layer)
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// CRITICAL: Must be "servers" NOT "mcpServers".
	if _, hasMcpServers := mcpConfig["mcpServers"]; hasMcpServers {
		t.Fatal("CRITICAL: .vscode/mcp.json must use 'servers' key, NOT 'mcpServers'")
	}

	servers, ok := mcpConfig["servers"].(map[string]any)
	if !ok {
		t.Fatal(".vscode/mcp.json missing 'servers' key")
	}

	github, ok := servers["github"].(map[string]any)
	if !ok {
		t.Fatal(".vscode/mcp.json missing github server")
	}
	if github["transport"] != "stdio" {
		t.Errorf("expected transport %q, got %q", "stdio", github["transport"])
	}
	if github["command"] != "npx" {
		t.Errorf("expected command %q, got %q", "npx", github["command"])
	}

	// Verify env is present.
	env, ok := github["env"].(map[string]any)
	if !ok || env["GITHUB_TOKEN"] != "${env:GITHUB_TOKEN}" {
		t.Error("MCP server missing env configuration")
	}
}

func TestMCPFilePathIsVSCodeMCP(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"test-mcp": keptPlugin("test-mcp", map[string]any{
			"mcpServers": map[string]any{
				"test": map[string]any{
					"transport": "stdio",
					"command":   "test-cmd",
				},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// Must NOT be .mcp.json (that's Claude).
	claudeMcp := findFile(unit.Files, ".mcp.json")
	if claudeMcp != nil {
		t.Error("Copilot should NOT emit .mcp.json (that's Claude format)")
	}

	// Must be .vscode/mcp.json.
	vscodeMcp := findFile(unit.Files, ".vscode/mcp.json")
	if vscodeMcp == nil {
		t.Fatal("Copilot must emit .vscode/mcp.json")
	}
}

func TestMCPHTTPTransport(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"http-mcp": keptPlugin("http-mcp", map[string]any{
			"mcpServers": map[string]any{
				"api-server": map[string]any{
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

	unit := unitFromResult(t, result)

	mcpFile := findFile(unit.Files, ".vscode/mcp.json")
	if mcpFile == nil {
		t.Fatal(".vscode/mcp.json not found")
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := mcpConfig["servers"].(map[string]any)
	apiServer := servers["api-server"].(map[string]any)
	if apiServer["transport"] != "http" {
		t.Errorf("expected transport %q, got %q", "http", apiServer["transport"])
	}
	if apiServer["url"] != "https://api.example.com/mcp" {
		t.Errorf("expected URL %q, got %q", "https://api.example.com/mcp", apiServer["url"])
	}
}

// ---------------------------------------------------------------------------
// 9. Plugin Layer Tests
// ---------------------------------------------------------------------------

func TestInlinePluginPackaging(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"my-plugin": keptPlugin("my-plugin", map[string]any{
			"distribution": map[string]any{"mode": "inline"},
			"artifacts": map[string]any{
				"scripts":   []any{"scripts/setup.sh"},
				"configs":   []any{"config/plugin.json"},
				"manifests": []any{"manifest.yaml"},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	if len(unit.PluginBundles) != 1 {
		t.Fatalf("expected 1 plugin bundle, got %d", len(unit.PluginBundles))
	}

	bundle := unit.PluginBundles[0]
	if bundle.PluginID != "my-plugin" {
		t.Errorf("expected plugin ID %q, got %q", "my-plugin", bundle.PluginID)
	}
	if !strings.HasPrefix(bundle.DestDir, ".claude-plugin/") {
		t.Errorf("expected .claude-plugin/ prefix, got %q", bundle.DestDir)
	}
	if len(bundle.Files) != 3 {
		t.Errorf("expected 3 files in plugin bundle, got %d", len(bundle.Files))
	}
}

func TestExternalPluginViaVSCodeMCP(t *testing.T) {
	r := copilot.New(nil)
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

	unit := unitFromResult(t, result)

	if len(unit.PluginBundles) != 0 {
		t.Errorf("external plugins should not produce bundles, got %d", len(unit.PluginBundles))
	}

	// Should have a .vscode/mcp.json entry (NOT .mcp.json).
	mcpFile := findFile(unit.Files, ".vscode/mcp.json")
	if mcpFile == nil {
		t.Fatal("external plugin should generate .vscode/mcp.json")
	}
}

func TestRegistryPluginEmitsInstallMetadata(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"reg-plugin": keptPlugin("reg-plugin", map[string]any{
			"distribution": map[string]any{
				"mode":    "registry",
				"ref":     "npm:@my/plugin",
				"version": "^1.0.0",
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	if len(unit.InstallMetadata) != 1 {
		t.Fatalf("expected 1 install metadata entry, got %d", len(unit.InstallMetadata))
	}

	entry := unit.InstallMetadata[0]
	if entry.PluginID != "reg-plugin" {
		t.Errorf("expected plugin ID %q, got %q", "reg-plugin", entry.PluginID)
	}
	if entry.Config["ref"] != "npm:@my/plugin" {
		t.Errorf("expected ref %q, got %q", "npm:@my/plugin", entry.Config["ref"])
	}
}

// ---------------------------------------------------------------------------
// 10. Provenance Tests
// ---------------------------------------------------------------------------

func TestProvenanceHeadersOnAllMarkdown(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1": keptInstruction("inst-1", "Test instruction.", nil),
		"rule-1": keptRule("rule-1", "Test rule.", []string{"**/*.go"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	for _, f := range unit.Files {
		if strings.HasSuffix(f.Path, ".md") {
			content := string(f.Content)
			if !strings.Contains(content, "<!-- generated by goagentmeta;") {
				t.Errorf("file %s missing provenance header", f.Path)
			}
			if !strings.Contains(content, "do not edit -->") {
				t.Errorf("file %s provenance header incomplete", f.Path)
			}
		}
	}
}

func TestProvenanceHeaderPreservesFrontmatter(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"go-rule": keptRule("go-rule", "Use gofmt.", []string{"**/*.go"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	ruleFile := findFile(unit.Files, ".github/instructions/go-rule.instructions.md")
	if ruleFile == nil {
		t.Fatal("rule file not found")
	}

	content := string(ruleFile.Content)
	// Files with frontmatter must start with "---" for parsers to recognize it.
	if !strings.HasPrefix(content, "---\n") {
		t.Errorf("frontmatter file must start with '---', got:\n%s", content[:min(len(content), 80)])
	}
	// Provenance must be present but AFTER the closing frontmatter.
	if !strings.Contains(content, "<!-- generated by goagentmeta;") {
		t.Error("provenance header missing")
	}
}

func TestProvenanceJSON(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1": keptInstruction("inst-1", "Test.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	provFile := findFile(unit.Files, "provenance.json")
	if provFile == nil {
		t.Fatal("provenance.json not found")
	}

	var prov map[string]any
	if err := json.Unmarshal(provFile.Content, &prov); err != nil {
		t.Fatalf("invalid provenance JSON: %v", err)
	}

	// CRITICAL: Target must be "copilot" (not "claude").
	if prov["target"] != "copilot" {
		t.Errorf("expected target %q, got %q", "copilot", prov["target"])
	}

	entries, ok := prov["entries"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatal("provenance.json missing entries")
	}
}

// ---------------------------------------------------------------------------
// 11. Determinism Tests
// ---------------------------------------------------------------------------

func TestDeterministicOutput(t *testing.T) {
	objects := map[string]pipeline.LoweredObject{
		"inst-z":    keptInstruction("inst-z", "Z instruction.", nil),
		"inst-a":    keptInstruction("inst-a", "A instruction.", nil),
		"rule-b":    keptRule("rule-b", "B rule.", []string{"**/*.ts"}),
		"rule-a":    keptRule("rule-a", "A rule.", []string{"**/*.go"}),
		"skill-c":   keptSkill("skill-c", "C skill content.", nil),
		"agent-x":   keptAgent("agent-x", "X agent prompt.", nil),
		"hook-1":    keptHook("hook-1", "post-edit", "command", "lint", nil),
		"cmd-1":     keptCommand("cmd-1", "Deploy.", nil),
		"plugin-m": keptPlugin("plugin-m", map[string]any{
			"mcpServers": map[string]any{
				"test-server": map[string]any{
					"transport": "stdio",
					"command":   "test-cmd",
				},
			},
		}),
	}

	r := copilot.New(nil)
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

	unit1 := plan1.Units[".ai-build/copilot/local-dev"]
	unit2 := plan2.Units[".ai-build/copilot/local-dev"]

	if len(unit1.Files) != len(unit2.Files) {
		t.Fatalf("file count mismatch: %d vs %d", len(unit1.Files), len(unit2.Files))
	}

	for i := range unit1.Files {
		if unit1.Files[i].Path != unit2.Files[i].Path {
			t.Errorf("file path mismatch at index %d: %q vs %q",
				i, unit1.Files[i].Path, unit2.Files[i].Path)
		}
		if string(unit1.Files[i].Content) != string(unit2.Files[i].Content) {
			t.Errorf("file content mismatch at %s", unit1.Files[i].Path)
		}
	}
}

// ---------------------------------------------------------------------------
// 12. Skipped Objects Test
// ---------------------------------------------------------------------------

func TestSkippedObjectsNotEmitted(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1":  keptInstruction("inst-1", "Kept.", nil),
		"skip-me": skippedObject("skip-me", model.KindSkill),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	for _, f := range unit.Files {
		if strings.Contains(f.Path, "skip-me") {
			t.Errorf("skipped object should not appear in output: %s", f.Path)
		}
		for _, src := range f.SourceObjects {
			if src == "skip-me" {
				t.Errorf("skipped object should not be in source objects of %s", f.Path)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 13. Non-Copilot Units Filtered Out
// ---------------------------------------------------------------------------

func TestNonCopilotUnitsFiltered(t *testing.T) {
	r := copilot.New(nil)
	graph := pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/copilot/local-dev": {
				Coordinate: copilotCoord(),
				Objects: map[string]pipeline.LoweredObject{
					"inst-1": keptInstruction("inst-1", "Copilot.", nil),
				},
			},
			".ai-build/claude/local-dev": {
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetClaude, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/claude/local-dev",
				},
				Objects: map[string]pipeline.LoweredObject{
					"inst-2": keptInstruction("inst-2", "Claude.", nil),
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
		t.Error("claude unit should not be rendered by copilot renderer")
	}
	if _, ok := plan.Units[".ai-build/copilot/local-dev"]; !ok {
		t.Error("copilot unit should be present")
	}
}

// ---------------------------------------------------------------------------
// 14. Empty Input Tests
// ---------------------------------------------------------------------------

func TestEmptyGraphProducesEmptyPlan(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	// Should only have provenance.json.
	if len(unit.Files) != 1 {
		t.Errorf("expected 1 file (provenance.json), got %d", len(unit.Files))
	}
}

// ---------------------------------------------------------------------------
// 15. YAML Frontmatter Special Character Safety
// ---------------------------------------------------------------------------

func TestSkillDescriptionWithSpecialChars(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"tricky-skill": keptSkill("tricky-skill", "Skill body.", map[string]any{
			"description": "Priority #1: critical skill",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	skillFile := findFile(unit.Files, ".github/skills/tricky-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("skill file not found")
	}

	content := string(skillFile.Content)
	if strings.Contains(content, "description: Priority #1") && !strings.Contains(content, `"Priority #1`) {
		t.Error("description with # should be YAML-quoted to prevent truncation")
	}
}

func TestAgentDescriptionWithNewline(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"nl-agent": keptAgent("nl-agent", "Agent body.", map[string]any{
			"description": "line1\nmalicious: injected",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	agentFile := findFile(unit.Files, ".github/agents/nl-agent.agent.md")
	if agentFile == nil {
		t.Fatal("agent file not found")
	}

	content := string(agentFile.Content)
	if strings.Contains(content, "malicious: injected") && !strings.Contains(content, `\n`) {
		t.Error("description with embedded newline should be escaped in YAML")
	}
}

// ---------------------------------------------------------------------------
// 16. Full Project All Object Types
// ---------------------------------------------------------------------------

func TestFullProjectAllObjectTypes(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"arch-instruction": keptInstruction("arch-instruction", "Use hexagonal architecture.", nil),
		"api-instruction":  keptInstruction("api-instruction", "Follow REST conventions.", []string{"services/api"}),
		"go-rule":          keptRule("go-rule", "Use gofmt and golangci-lint.", []string{"**/*.go"}),
		"iam-skill": keptSkill("iam-skill", "Review IAM policies thoroughly.", map[string]any{
			"description":     "IAM review skill",
			"activationHints": []any{"IAM", "security"},
		}),
		"review-agent": keptAgent("review-agent", "You specialize in code review.", map[string]any{
			"description": "Review agent",
			"skills":      []any{"iam-skill"},
			"delegation":  map[string]any{"mayCall": []any{"deploy-agent"}},
			"handoffs": []any{
				map[string]any{"label": "Deploy", "agent": "deploy-agent"},
			},
		}),
		"lint-hook":    keptHook("lint-hook", "post-edit", "command", "make lint", nil),
		"deploy-cmd":   keptCommand("deploy-cmd", "Deploy the app.", map[string]any{"description": "Deploy command"}),
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

	unit := unitFromResult(t, result)

	expectedFiles := []string{
		".github/copilot-instructions.md",
		"AGENTS.md",
		"services/api/AGENTS.md",
		".github/instructions/go-rule.instructions.md",
		".github/skills/iam-skill/SKILL.md",
		".github/agents/review-agent.agent.md",
		".github/hooks/post-edit.json",
		".claude/settings.json",
		".github/prompts/deploy-cmd.prompt.md",
		".vscode/mcp.json",
		"provenance.json",
	}

	for _, expected := range expectedFiles {
		f := findFile(unit.Files, expected)
		if f == nil {
			t.Errorf("missing expected file: %s", expected)
		}
	}

	// Verify directory list is populated.
	if len(unit.Directories) == 0 {
		t.Error("directories list should be populated")
	}

	// Verify all markdown files have provenance headers (may be before or after frontmatter).
	for _, f := range unit.Files {
		if strings.HasSuffix(f.Path, ".md") {
			content := string(f.Content)
			if !strings.Contains(content, "<!-- generated by goagentmeta;") {
				t.Errorf("file %s missing provenance header", f.Path)
			}
		}
	}

	// Verify MCP uses "servers" key.
	mcpFile := findFile(unit.Files, ".vscode/mcp.json")
	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid MCP JSON: %v", err)
	}
	if _, has := mcpConfig["servers"]; !has {
		t.Error("MCP config missing 'servers' key")
	}
	if _, has := mcpConfig["mcpServers"]; has {
		t.Error("MCP config must NOT have 'mcpServers' key")
	}

	// Verify agent has handoffs.
	agentFile := findFile(unit.Files, ".github/agents/review-agent.agent.md")
	if agentFile != nil {
		content := string(agentFile.Content)
		if !strings.Contains(content, "handoffs:") {
			t.Error("full project agent should have handoffs")
		}
	}

	// Verify scoped rule uses applyTo: comma-separated string.
	ruleFile := findFile(unit.Files, ".github/instructions/go-rule.instructions.md")
	if ruleFile != nil {
		content := string(ruleFile.Content)
		if !strings.Contains(content, "applyTo:") {
			t.Error("scoped rule should have applyTo: frontmatter")
		}
	}

	// Verify provenance target.
	provFile := findFile(unit.Files, "provenance.json")
	if provFile != nil {
		var prov map[string]any
		if err := json.Unmarshal(provFile.Content, &prov); err == nil {
			if prov["target"] != "copilot" {
				t.Errorf("provenance target should be 'copilot', got %v", prov["target"])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 17. Copilot-Specific Edge Cases
// ---------------------------------------------------------------------------

func TestApplyToSinglePath(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"single-rule": keptRule("single-rule", "Single path rule.", []string{"src/**"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	ruleFile := findFile(unit.Files, ".github/instructions/single-rule.instructions.md")
	if ruleFile == nil {
		t.Fatal("rule file not found")
	}

	content := string(ruleFile.Content)
	// Single path should use applyTo: format. Since src/** contains *, it gets quoted.
	if !strings.Contains(content, `applyTo: "src/**"`) {
		t.Errorf("single path applyTo should be quoted (contains *), got:\n%s", content)
	}
}

func TestAgentWithMCPServersField(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"mcp-agent": keptAgent("mcp-agent", "Agent with MCP servers.", map[string]any{
			"mcpServers": []any{"github", "filesystem"},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	agentFile := findFile(unit.Files, ".github/agents/mcp-agent.agent.md")
	if agentFile == nil {
		t.Fatal("agent file not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "mcp-servers:") {
		t.Error("agent missing mcp-servers in frontmatter")
	}
}

func TestRuleWithExcludeAgent(t *testing.T) {
	r := copilot.New(nil)
	fields := map[string]any{
		"scope":        map[string]any{"paths": []any{"**/*.go"}},
		"excludeAgent": []any{"test-agent", "debug-agent"},
	}
	obj := pipeline.LoweredObject{
		OriginalID:   "exclusive-rule",
		OriginalKind: model.KindRule,
		LoweredKind:  model.KindRule,
		Decision:     pipeline.LoweringDecision{Action: "kept", Safe: true},
		Content:      "Exclusive rule content.",
		Fields:       fields,
	}
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"exclusive-rule": obj,
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	ruleFile := findFile(unit.Files, ".github/instructions/exclusive-rule.instructions.md")
	if ruleFile == nil {
		t.Fatal("rule file not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "excludeAgent:") {
		t.Error("rule missing excludeAgent frontmatter")
	}
}

// ---------------------------------------------------------------------------
// 18. YAML Glob Pattern Quoting
// ---------------------------------------------------------------------------

func TestGlobPatternIsQuotedInYAML(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"glob-rule": keptRule("glob-rule", "Glob rule.", []string{"**/*.go"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	ruleFile := findFile(unit.Files, ".github/instructions/glob-rule.instructions.md")
	if ruleFile == nil {
		t.Fatal("rule file not found")
	}

	content := string(ruleFile.Content)
	// Glob patterns starting with * must be quoted in YAML to avoid alias interpretation.
	if strings.Contains(content, "applyTo: **/*.go") && !strings.Contains(content, `"**/*.go"`) {
		t.Error("glob pattern with * must be YAML-quoted to prevent alias interpretation")
	}
}

func TestProvenanceHeaderAfterFrontmatterForAgent(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"test-agent": keptAgent("test-agent", "Agent body.", map[string]any{
			"description": "Test agent",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	agentFile := findFile(unit.Files, ".github/agents/test-agent.agent.md")
	if agentFile == nil {
		t.Fatal("agent file not found")
	}

	content := string(agentFile.Content)
	// Agent files have frontmatter — must start with "---".
	if !strings.HasPrefix(content, "---\n") {
		t.Errorf("agent file with frontmatter must start with '---', got: %s", content[:min(len(content), 50)])
	}
	// Provenance header must still be present.
	if !strings.Contains(content, "<!-- generated by goagentmeta;") {
		t.Error("agent file missing provenance header")
	}
}

func TestProvenanceHeaderAfterFrontmatterForPrompt(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"test-cmd": keptCommand("test-cmd", "Command body.", map[string]any{
			"description": "Test command",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	promptFile := findFile(unit.Files, ".github/prompts/test-cmd.prompt.md")
	if promptFile == nil {
		t.Fatal("prompt file not found")
	}

	content := string(promptFile.Content)
	if !strings.HasPrefix(content, "---\n") {
		t.Errorf("prompt file with frontmatter must start with '---', got: %s", content[:min(len(content), 50)])
	}
	if !strings.Contains(content, "<!-- generated by goagentmeta;") {
		t.Error("prompt file missing provenance header")
	}
}

func TestProvenanceHeaderAfterFrontmatterForSkill(t *testing.T) {
	r := copilot.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"test-skill": keptSkill("test-skill", "Skill body.", map[string]any{
			"description": "Test skill",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unit := unitFromResult(t, result)

	skillFile := findFile(unit.Files, ".github/skills/test-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("skill file not found")
	}

	content := string(skillFile.Content)
	if !strings.HasPrefix(content, "---\n") {
		t.Errorf("skill file with frontmatter must start with '---', got: %s", content[:min(len(content), 50)])
	}
	if !strings.Contains(content, "<!-- generated by goagentmeta;") {
		t.Error("skill file missing provenance header")
	}
}
