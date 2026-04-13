package claude_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/claude"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/renderer"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time interface checks.
var (
	_ stage.Stage       = (*claude.Renderer)(nil)
	_ renderer.Renderer = (*claude.Renderer)(nil)
)

// ---------------------------------------------------------------------------
// Helper constructors
// ---------------------------------------------------------------------------

func claudeCoord() build.BuildCoordinate {
	return build.BuildCoordinate{
		Unit: build.BuildUnit{
			Target:  build.TargetClaude,
			Profile: build.ProfileLocalDev,
		},
		OutputDir: ".ai-build/claude/local-dev",
	}
}

func loweredGraph(objects map[string]pipeline.LoweredObject) pipeline.LoweredGraph {
	return pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/claude/local-dev": {
				Coordinate: claudeCoord(),
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

// ---------------------------------------------------------------------------
// 1. Renderer Registration Tests
// ---------------------------------------------------------------------------

func TestRendererTarget(t *testing.T) {
	r := claude.New(nil)
	if r.Target() != build.TargetClaude {
		t.Fatalf("expected target %q, got %q", build.TargetClaude, r.Target())
	}
}

func TestRendererDescriptor(t *testing.T) {
	r := claude.New(nil)
	desc := r.Descriptor()

	if desc.Name != "claude-renderer" {
		t.Errorf("expected name %q, got %q", "claude-renderer", desc.Name)
	}
	if desc.Phase != pipeline.PhaseRender {
		t.Errorf("expected phase %v, got %v", pipeline.PhaseRender, desc.Phase)
	}
	if desc.Order != 10 {
		t.Errorf("expected order 10, got %d", desc.Order)
	}
	if len(desc.TargetFilter) != 1 || desc.TargetFilter[0] != build.TargetClaude {
		t.Errorf("expected target filter [claude], got %v", desc.TargetFilter)
	}
}

func TestRendererSupportedCapabilities(t *testing.T) {
	r := claude.New(nil)
	caps := r.SupportedCapabilities()
	if caps.Target != "claude" {
		t.Fatalf("expected target %q, got %q", "claude", caps.Target)
	}
	if len(caps.Surfaces) == 0 {
		t.Fatal("expected non-empty capability surfaces")
	}
}

func TestRendererFactory(t *testing.T) {
	factory := claude.Factory(nil)
	s, err := factory()
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if s == nil {
		t.Fatal("factory returned nil stage")
	}
	if s.Descriptor().Name != "claude-renderer" {
		t.Errorf("expected name %q, got %q", "claude-renderer", s.Descriptor().Name)
	}
}

func TestRendererRejectsInvalidInput(t *testing.T) {
	r := claude.New(nil)
	_, err := r.Execute(testContext(), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if !strings.Contains(err.Error(), "expected pipeline.LoweredGraph") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRendererAcceptsPointerInput(t *testing.T) {
	r := claude.New(nil)
	graph := &pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/claude/local-dev": {
				Coordinate: claudeCoord(),
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

func TestSingleInstructionCLAUDEMD(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1": keptInstruction("inst-1", "Always use Go modules.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	claudeMD := findFile(unit.Files, "CLAUDE.md")
	if claudeMD == nil {
		t.Fatal("CLAUDE.md not found")
	}

	content := string(claudeMD.Content)
	if !strings.Contains(content, "Always use Go modules.") {
		t.Errorf("CLAUDE.md content missing instruction: %s", content)
	}
	if claudeMD.Layer != pipeline.LayerInstruction {
		t.Errorf("expected layer %q, got %q", pipeline.LayerInstruction, claudeMD.Layer)
	}
	assertSourceObjects(t, claudeMD, "inst-1")
}

func TestTwoInstructionsDifferentScopesTwoFiles(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-root":    keptInstruction("inst-root", "Root instruction.", nil),
		"inst-subtree": keptInstruction("inst-subtree", "Subtree instruction.", []string{"services/api"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	rootMD := findFile(unit.Files, "CLAUDE.md")
	if rootMD == nil {
		t.Fatal("CLAUDE.md not found")
	}
	if !strings.Contains(string(rootMD.Content), "Root instruction.") {
		t.Error("CLAUDE.md missing root instruction")
	}

	subtreeMD := findFile(unit.Files, "services/api/CLAUDE.md")
	if subtreeMD == nil {
		t.Fatal("services/api/CLAUDE.md not found")
	}
	if !strings.Contains(string(subtreeMD.Content), "Subtree instruction.") {
		t.Error("services/api/CLAUDE.md missing subtree instruction")
	}
}

func TestMultipleInstructionsMergedSorted(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"z-inst": keptInstruction("z-inst", "Z instruction.", nil),
		"a-inst": keptInstruction("a-inst", "A instruction.", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	claudeMD := findFile(unit.Files, "CLAUDE.md")
	if claudeMD == nil {
		t.Fatal("CLAUDE.md not found")
	}

	content := string(claudeMD.Content)
	aIdx := strings.Index(content, "A instruction.")
	zIdx := strings.Index(content, "Z instruction.")
	if aIdx < 0 || zIdx < 0 || aIdx >= zIdx {
		t.Error("instructions not sorted by ID (A should come before Z)")
	}
}

// ---------------------------------------------------------------------------
// 3. Rules Layer Tests
// ---------------------------------------------------------------------------

func TestRuleWithPathsScope(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"go-style": keptRule("go-style", "Use gofmt.", []string{"**/*.go"}, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	ruleFile := findFile(unit.Files, ".claude/rules/go-style.md")
	if ruleFile == nil {
		t.Fatal(".claude/rules/go-style.md not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "---") {
		t.Error("rule missing YAML frontmatter")
	}
	if !strings.Contains(content, "paths:") {
		t.Error("rule missing paths: in frontmatter")
	}
	if !strings.Contains(content, "**/*.go") {
		t.Error("rule missing path glob")
	}
	if !strings.Contains(content, "Use gofmt.") {
		t.Error("rule missing content body")
	}
}

func TestRuleWithConditions(t *testing.T) {
	r := claude.New(nil)
	conditions := []model.RuleCondition{
		{Type: "language", Value: "go"},
		{Type: "generated", Value: "true"},
	}
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"gen-rule": keptRule("gen-rule", "Handle generated code carefully.", nil, conditions),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	ruleFile := findFile(unit.Files, ".claude/rules/gen-rule.md")
	if ruleFile == nil {
		t.Fatal(".claude/rules/gen-rule.md not found")
	}

	content := string(ruleFile.Content)
	if !strings.Contains(content, "## Conditions") {
		t.Error("rule missing conditions section")
	}
	if !strings.Contains(content, "**language**: go") {
		t.Error("rule missing language condition")
	}
	if !strings.Contains(content, "**generated**: true") {
		t.Error("rule missing generated condition")
	}
}

// ---------------------------------------------------------------------------
// 4. Skills Layer Tests
// ---------------------------------------------------------------------------

func TestSkillWithResources(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"iam-review": keptSkill("iam-review", "Review IAM policies.", map[string]any{
			"description":     "IAM review skill",
			"activationHints": []any{"IAM", "policy"},
			"tools":           []any{"Read", "Write"},
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

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	skillFile := findFile(unit.Files, ".claude/skills/iam-review/SKILL.md")
	if skillFile == nil {
		t.Fatal(".claude/skills/iam-review/SKILL.md not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "---") {
		t.Error("skill missing YAML frontmatter")
	}
	if !strings.Contains(content, "name: iam-review") {
		t.Error("skill missing name in frontmatter")
	}
	if !strings.Contains(content, "description: IAM review skill") {
		t.Error("skill missing description")
	}
	if !strings.Contains(content, "activation_hints:") {
		t.Error("skill missing activation_hints")
	}
	if !strings.Contains(content, "allowed-tools:") {
		t.Error("skill missing allowed-tools")
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
	r := claude.New(nil)
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

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	skillFile := findFile(unit.Files, ".claude/skills/my-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal(".claude/skills/my-skill/SKILL.md not found")
	}

	content := string(skillFile.Content)
	if !strings.Contains(content, "author: TestAuthor") {
		t.Error("skill missing author")
	}
	// homepage contains ':' so it gets YAML-quoted.
	if !strings.Contains(content, "homepage:") || !strings.Contains(content, "https://example.com") {
		t.Error("skill missing homepage")
	}
	if !strings.Contains(content, "emoji: 🔧") {
		t.Error("skill missing emoji")
	}
}

// ---------------------------------------------------------------------------
// 5. Agents Layer Tests
// ---------------------------------------------------------------------------

func TestAgentWithDelegation(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"review-agent": keptAgent("review-agent", "You are a code review specialist.", map[string]any{
			"description": "Code review agent",
			"model":       "claude-sonnet-4-20250514",
			"skills":      []any{"iam-review", "go-review"},
			"tools":           []any{"Bash", "Read"},
			"disallowedTools": []any{"Write"},
			"delegation": map[string]any{
				"mayCall": []any{"deploy-agent", "test-agent"},
			},
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	agentFile := findFile(unit.Files, ".claude/agents/review-agent.md")
	if agentFile == nil {
		t.Fatal(".claude/agents/review-agent.md not found")
	}

	content := string(agentFile.Content)
	if !strings.Contains(content, "name: review-agent") {
		t.Error("agent missing name")
	}
	if !strings.Contains(content, "description: Code review agent") {
		t.Error("agent missing description")
	}
	if !strings.Contains(content, "model: claude-sonnet-4-20250514") {
		t.Error("agent missing model")
	}
	if !strings.Contains(content, "tools:") {
		t.Error("agent missing tools (allowed)")
	}
	if !strings.Contains(content, "disallowedTools:") {
		t.Error("agent missing disallowedTools")
	}
	if !strings.Contains(content, "mayCall:") {
		t.Error("agent missing mayCall delegation")
	}
	if !strings.Contains(content, "- deploy-agent") {
		t.Error("agent missing delegation target")
	}
	if !strings.Contains(content, "You are a code review specialist.") {
		t.Error("agent missing body content")
	}
	assertSourceObjects(t, agentFile, "review-agent")
}

// ---------------------------------------------------------------------------
// 6. Hooks Layer Tests
// ---------------------------------------------------------------------------

func TestHookBlockingInSettingsJSON(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"lint-hook": keptHook("lint-hook", "post-edit", "command", "golangci-lint run", []string{"**/*.go"}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	settingsFile := findFile(unit.Files, ".claude/settings.json")
	if settingsFile == nil {
		t.Fatal(".claude/settings.json not found")
	}
	if settingsFile.Layer != pipeline.LayerExtension {
		t.Errorf("expected layer %q, got %q", pipeline.LayerExtension, settingsFile.Layer)
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsFile.Content, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("settings.json missing hooks key")
	}

	postEdit, ok := hooks["post-edit"].([]any)
	if !ok || len(postEdit) == 0 {
		t.Fatal("settings.json missing post-edit hooks")
	}

	entry := postEdit[0].(map[string]any)
	if entry["type"] != "command" {
		t.Errorf("expected hook type %q, got %q", "command", entry["type"])
	}
	if entry["command"] != "golangci-lint run" {
		t.Errorf("expected hook command %q, got %q", "golangci-lint run", entry["command"])
	}
	if entry["matcher"] != "**/*.go" {
		t.Errorf("expected hook matcher %q, got %q", "**/*.go", entry["matcher"])
	}
}

func TestMultipleHooksSameEvent(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"hook-a": keptHook("hook-a", "pre-tool-use", "command", "echo a", nil),
		"hook-b": keptHook("hook-b", "pre-tool-use", "command", "echo b", nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	settingsFile := findFile(unit.Files, ".claude/settings.json")
	if settingsFile == nil {
		t.Fatal(".claude/settings.json not found")
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsFile.Content, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["pre-tool-use"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 hooks for pre-tool-use, got %d", len(preToolUse))
	}
}

// ---------------------------------------------------------------------------
// 7. MCP Layer Tests
// ---------------------------------------------------------------------------

func TestMCPConfig(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
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
	unit := plan.Units[".ai-build/claude/local-dev"]

	mcpFile := findFile(unit.Files, ".mcp.json")
	if mcpFile == nil {
		t.Fatal(".mcp.json not found")
	}
	if mcpFile.Layer != pipeline.LayerExtension {
		t.Errorf("expected layer %q, got %q", pipeline.LayerExtension, mcpFile.Layer)
	}

	var mcpConfig map[string]any
	if err := json.Unmarshal(mcpFile.Content, &mcpConfig); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify the key is "mcpServers" (not "servers").
	servers, ok := mcpConfig["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal(".mcp.json missing mcpServers key (verify it's not 'servers')")
	}

	github, ok := servers["github"].(map[string]any)
	if !ok {
		t.Fatal(".mcp.json missing github server")
	}
	if github["transport"] != "stdio" {
		t.Errorf("expected transport %q, got %q", "stdio", github["transport"])
	}
	if github["command"] != "npx" {
		t.Errorf("expected command %q, got %q", "npx", github["command"])
	}
}

// ---------------------------------------------------------------------------
// 8. Plugin Layer Tests
// ---------------------------------------------------------------------------

func TestInlinePluginPackaging(t *testing.T) {
	r := claude.New(nil)
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

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

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

func TestExternalPluginNoLeakedPaths(t *testing.T) {
	r := claude.New(nil)
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
	unit := plan.Units[".ai-build/claude/local-dev"]

	// External plugins should be referenced via MCP config, not as bundles.
	if len(unit.PluginBundles) != 0 {
		t.Errorf("external plugins should not produce bundles, got %d", len(unit.PluginBundles))
	}

	// Should have an MCP entry.
	mcpFile := findFile(unit.Files, ".mcp.json")
	if mcpFile == nil {
		t.Fatal("external plugin should generate .mcp.json")
	}
}

// ---------------------------------------------------------------------------
// 9. Provenance Tests
// ---------------------------------------------------------------------------

func TestProvenanceHeadersOnAllMarkdown(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1": keptInstruction("inst-1", "Test instruction.", nil),
		"rule-1": keptRule("rule-1", "Test rule.", []string{"**/*.go"}, nil),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	for _, f := range unit.Files {
		if strings.HasSuffix(f.Path, ".md") {
			content := string(f.Content)
			if !strings.HasPrefix(content, "<!-- generated by goagentmeta;") {
				t.Errorf("file %s missing provenance header", f.Path)
			}
			if !strings.Contains(content, "do not edit -->") {
				t.Errorf("file %s provenance header incomplete", f.Path)
			}
		}
	}
}

func TestProvenanceJSON(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1": keptInstruction("inst-1", "Test.", nil),
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
		t.Fatalf("invalid provenance JSON: %v", err)
	}

	if prov["target"] != "claude" {
		t.Errorf("expected target %q, got %q", "claude", prov["target"])
	}

	entries, ok := prov["entries"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatal("provenance.json missing entries")
	}

	entry := entries[0].(map[string]any)
	if entry["sourceObject"] != "inst-1" {
		t.Errorf("expected source object %q, got %q", "inst-1", entry["sourceObject"])
	}
}

// ---------------------------------------------------------------------------
// 10. Determinism Tests
// ---------------------------------------------------------------------------

func TestDeterministicOutput(t *testing.T) {
	objects := map[string]pipeline.LoweredObject{
		"inst-z":  keptInstruction("inst-z", "Z instruction.", nil),
		"inst-a":  keptInstruction("inst-a", "A instruction.", nil),
		"rule-b":  keptRule("rule-b", "B rule.", []string{"**/*.ts"}, nil),
		"rule-a":  keptRule("rule-a", "A rule.", []string{"**/*.go"}, nil),
		"skill-c": keptSkill("skill-c", "C skill content.", nil),
		"agent-x": keptAgent("agent-x", "X agent prompt.", nil),
		"hook-1":  keptHook("hook-1", "post-edit", "command", "lint", nil),
		"plugin-m": keptPlugin("plugin-m", map[string]any{
			"mcpServers": map[string]any{
				"test-server": map[string]any{
					"transport": "stdio",
					"command":   "test-cmd",
				},
			},
		}),
	}

	r := claude.New(nil)
	graph := loweredGraph(objects)

	// Run twice and compare.
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

	unit1 := plan1.Units[".ai-build/claude/local-dev"]
	unit2 := plan2.Units[".ai-build/claude/local-dev"]

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
// 11. Skipped Objects Test
// ---------------------------------------------------------------------------

func TestSkippedObjectsNotEmitted(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"inst-1":  keptInstruction("inst-1", "Kept.", nil),
		"skip-me": skippedObject("skip-me", model.KindSkill),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

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
// 12. Non-Claude Units Filtered Out
// ---------------------------------------------------------------------------

func TestNonClaudeUnitsFiltered(t *testing.T) {
	r := claude.New(nil)
	graph := pipeline.LoweredGraph{
		Units: map[string]pipeline.LoweredUnit{
			".ai-build/claude/local-dev": {
				Coordinate: claudeCoord(),
				Objects: map[string]pipeline.LoweredObject{
					"inst-1": keptInstruction("inst-1", "Claude.", nil),
				},
			},
			".ai-build/cursor/local-dev": {
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetCursor, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/cursor/local-dev",
				},
				Objects: map[string]pipeline.LoweredObject{
					"inst-2": keptInstruction("inst-2", "Cursor.", nil),
				},
			},
		},
	}

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	if _, ok := plan.Units[".ai-build/cursor/local-dev"]; ok {
		t.Error("cursor unit should not be rendered by claude renderer")
	}
	if _, ok := plan.Units[".ai-build/claude/local-dev"]; !ok {
		t.Error("claude unit should be present")
	}
}

// ---------------------------------------------------------------------------
// 13. Full Project Golden Test
// ---------------------------------------------------------------------------

func TestFullProjectAllObjectTypes(t *testing.T) {
	r := claude.New(nil)
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
		"lint-hook": keptHook("lint-hook", "post-edit", "command", "make lint", nil),
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
	unit := plan.Units[".ai-build/claude/local-dev"]

	// Verify all expected file types are present.
	expectedFiles := []string{
		"CLAUDE.md",
		"services/api/CLAUDE.md",
		".claude/rules/go-rule.md",
		".claude/skills/iam-skill/SKILL.md",
		".claude/agents/review-agent.md",
		".claude/settings.json",
		".mcp.json",
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

	// Verify all markdown files have provenance headers.
	for _, f := range unit.Files {
		if strings.HasSuffix(f.Path, ".md") {
			content := string(f.Content)
			if !strings.HasPrefix(content, "<!-- generated by goagentmeta;") {
				t.Errorf("file %s missing provenance header", f.Path)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 14. Empty Input Tests
// ---------------------------------------------------------------------------

func TestEmptyGraphProducesEmptyPlan(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	// Should only have provenance.json.
	if len(unit.Files) != 1 {
		t.Errorf("expected 1 file (provenance.json), got %d", len(unit.Files))
	}
}

// ---------------------------------------------------------------------------
// 15. YAML Frontmatter Special Character Safety
// ---------------------------------------------------------------------------

func TestSkillDescriptionWithSpecialChars(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"tricky-skill": keptSkill("tricky-skill", "Skill body.", map[string]any{
			"description": "Priority #1: critical skill",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	skillFile := findFile(unit.Files, ".claude/skills/tricky-skill/SKILL.md")
	if skillFile == nil {
		t.Fatal("skill file not found")
	}

	content := string(skillFile.Content)
	// The description should be quoted to prevent YAML # truncation.
	if strings.Contains(content, "description: Priority #1") && !strings.Contains(content, `"Priority #1`) {
		t.Error("description with # should be YAML-quoted to prevent truncation")
	}
}

func TestAgentDescriptionWithNewline(t *testing.T) {
	r := claude.New(nil)
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"nl-agent": keptAgent("nl-agent", "Agent body.", map[string]any{
			"description": "line1\nmalicious: injected",
		}),
	})

	result, err := r.Execute(testContext(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	agentFile := findFile(unit.Files, ".claude/agents/nl-agent.md")
	if agentFile == nil {
		t.Fatal("agent file not found")
	}

	content := string(agentFile.Content)
	// The description with a newline must be quoted and the newline escaped.
	if strings.Contains(content, "malicious: injected") && !strings.Contains(content, `\n`) {
		t.Error("description with embedded newline should be escaped in YAML")
	}
}

// ---------------------------------------------------------------------------
// 16. Registry Plugin Install Metadata
// ---------------------------------------------------------------------------

func TestRegistryPluginEmitsInstallMetadata(t *testing.T) {
	r := claude.New(nil)
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

	plan := result.(pipeline.EmissionPlan)
	unit := plan.Units[".ai-build/claude/local-dev"]

	if len(unit.InstallMetadata) != 1 {
		t.Fatalf("expected 1 install metadata entry, got %d", len(unit.InstallMetadata))
	}

	entry := unit.InstallMetadata[0]
	if entry.PluginID != "reg-plugin" {
		t.Errorf("expected plugin ID %q, got %q", "reg-plugin", entry.PluginID)
	}
	if entry.Format != "registry-ref" {
		t.Errorf("expected format %q, got %q", "registry-ref", entry.Format)
	}
	if entry.Config["ref"] != "npm:@my/plugin" {
		t.Errorf("expected ref %q, got %q", "npm:@my/plugin", entry.Config["ref"])
	}
}

// ---------------------------------------------------------------------------
// 17. MCP Server Name Collision
// ---------------------------------------------------------------------------

func TestMCPServerNameCollisionFirstWins(t *testing.T) {
	r := claude.New(nil)
	// Use sorted IDs: "aaa-plugin" comes before "zzz-plugin".
	graph := loweredGraph(map[string]pipeline.LoweredObject{
		"aaa-plugin": keptPlugin("aaa-plugin", map[string]any{
			"mcpServers": map[string]any{
				"shared-server": map[string]any{
					"transport": "stdio",
					"command":   "first-cmd",
				},
			},
		}),
		"zzz-plugin": keptPlugin("zzz-plugin", map[string]any{
			"mcpServers": map[string]any{
				"shared-server": map[string]any{
					"transport": "stdio",
					"command":   "second-cmd",
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
	server := servers["shared-server"].(map[string]any)

	// First plugin (aaa-plugin) should win due to deterministic ordering.
	if server["command"] != "first-cmd" {
		t.Errorf("expected first plugin to win collision, got command %q", server["command"])
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func findFile(files []pipeline.EmittedFile, path string) *pipeline.EmittedFile {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}

func assertSourceObjects(t *testing.T, f *pipeline.EmittedFile, expectedIDs ...string) {
	t.Helper()
	for _, id := range expectedIDs {
		found := false
		for _, src := range f.SourceObjects {
			if src == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("file %s missing source object %q", f.Path, id)
		}
	}
}
