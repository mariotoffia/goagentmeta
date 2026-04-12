package lowering_test

import (
	"context"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/lowering"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	domcap "github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ==========================================================================
// Preservation Policy Tests
// ==========================================================================

func TestCheckPreservation_Required_Unsafe_Error(t *testing.T) {
	decision := pipeline.LoweringDecision{
		Action: "skipped", Reason: "blocking hook", Safe: false,
	}
	result := lowering.CheckPreservation(decision, model.PreservationRequired)
	if result.Allowed {
		t.Fatal("expected required+unsafe to be blocked")
	}
	if result.Severity != "error" {
		t.Errorf("expected severity 'error', got %q", result.Severity)
	}
}

func TestCheckPreservation_Preferred_Unsafe_Warning(t *testing.T) {
	decision := pipeline.LoweringDecision{
		Action: "skipped", Reason: "skill lost", Safe: false,
	}
	result := lowering.CheckPreservation(decision, model.PreservationPreferred)
	if result.Allowed {
		t.Fatal("expected preferred+unsafe to be blocked (skipped)")
	}
	if result.Severity != "warning" {
		t.Errorf("expected severity 'warning', got %q", result.Severity)
	}
}

func TestCheckPreservation_Optional_Unsafe_Allowed(t *testing.T) {
	decision := pipeline.LoweringDecision{
		Action: "lowered", Reason: "optional lowering", Safe: false,
	}
	result := lowering.CheckPreservation(decision, model.PreservationOptional)
	if !result.Allowed {
		t.Fatal("expected optional+unsafe to be allowed")
	}
	if result.Severity != "info" {
		t.Errorf("expected severity 'info', got %q", result.Severity)
	}
}

func TestCheckPreservation_Safe_AlwaysAllowed(t *testing.T) {
	for _, pres := range []model.Preservation{
		model.PreservationRequired,
		model.PreservationPreferred,
		model.PreservationOptional,
	} {
		decision := pipeline.LoweringDecision{
			Action: "lowered", Reason: "safe lowering", Safe: true,
		}
		result := lowering.CheckPreservation(decision, pres)
		if !result.Allowed {
			t.Errorf("expected safe lowering to be allowed for preservation=%s", pres)
		}
	}
}

func TestCheckPreservation_EmptyPreservation_DefaultsToPreferred(t *testing.T) {
	decision := pipeline.LoweringDecision{
		Action: "skipped", Reason: "test", Safe: false,
	}
	result := lowering.CheckPreservation(decision, "")
	// Empty defaults to preferred → unsafe should warn and block.
	if result.Allowed {
		t.Fatal("expected empty preservation (defaults to preferred) + unsafe to be blocked")
	}
	if result.Severity != "warning" {
		t.Errorf("expected 'warning', got %q", result.Severity)
	}
}

// ==========================================================================
// Rule Lowering Tests
// ==========================================================================

func TestLowerRule_BasicContent(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:          "my-rule",
			Kind:        model.KindRule,
			Description: "Code Style",
		},
		ResolvedFields: map[string]any{
			"content": "Always use gofmt.\n",
		},
	}

	lowered := lowering.LowerRule(obj)

	if lowered.OriginalKind != model.KindRule {
		t.Errorf("expected OriginalKind=rule, got %s", lowered.OriginalKind)
	}
	if lowered.LoweredKind != model.KindInstruction {
		t.Errorf("expected LoweredKind=instruction, got %s", lowered.LoweredKind)
	}
	if !lowered.Decision.Safe {
		t.Error("rule lowering should be safe")
	}
	if lowered.Decision.Action != "lowered" {
		t.Errorf("expected action 'lowered', got %q", lowered.Decision.Action)
	}
	assertContains(t, lowered.Content, "## Code Style")
	assertContains(t, lowered.Content, "Always use gofmt.")
}

func TestLowerRule_WithConditions(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "cond-rule",
			Kind: model.KindRule,
		},
		ResolvedFields: map[string]any{
			"content": "Use strict mode.\n",
			"conditions": []model.RuleCondition{
				{Type: "fileType", Value: "*.ts"},
			},
		},
	}

	lowered := lowering.LowerRule(obj)
	assertContains(t, lowered.Content, "When fileType = *.ts:")
	assertContains(t, lowered.Content, "Use strict mode.")
}

func TestRuleLoweringRecord(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "rule-1",
			Kind:         model.KindRule,
			Preservation: model.PreservationOptional,
		},
	}

	record := lowering.RuleLoweringRecord(obj)
	if record.FromKind != model.KindRule {
		t.Errorf("expected FromKind=rule, got %s", record.FromKind)
	}
	if record.ToKind != model.KindInstruction {
		t.Errorf("expected ToKind=instruction, got %s", record.ToKind)
	}
	if record.Status != "lowered" {
		t.Errorf("expected status 'lowered', got %q", record.Status)
	}
}

// ==========================================================================
// Hook Lowering Tests
// ==========================================================================

func TestLowerHook_Blocking_NoSupport_Required_Error(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "blocking-hook",
			Kind:         model.KindHook,
			Preservation: model.PreservationRequired,
		},
		ResolvedFields: map[string]any{
			"enforcement": string(model.EnforcementBlocking),
			"effectClass": string(model.EffectValidating),
		},
	}

	// Limited target: no hook support.
	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerHook(obj, unitCaps)
	if result.Allowed {
		t.Fatal("expected blocking+required+no support → error")
	}
	if result.Severity != "error" {
		t.Errorf("expected 'error', got %q", result.Severity)
	}
	if lowered.Decision.Safe {
		t.Error("expected unsafe decision")
	}
}

func TestLowerHook_Advisory_NoSupport_Optional_Skipped(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "advisory-hook",
			Kind:         model.KindHook,
			Preservation: model.PreservationOptional,
		},
		ResolvedFields: map[string]any{
			"enforcement": string(model.EnforcementAdvisory),
			"effectClass": string(model.EffectObserving),
		},
	}

	// Limited target: no hook or command support.
	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerHook(obj, unitCaps)
	if !result.Allowed {
		t.Fatalf("expected optional+advisory to be allowed, got: %s", result.Message)
	}
	// Should be skipped but allowed.
	if lowered.Decision.Action != "skipped" {
		t.Errorf("expected action 'skipped', got %q", lowered.Decision.Action)
	}
}

func TestLowerHook_Blocking_NativeSupport_PassThrough(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "native-hook",
			Kind: model.KindHook,
		},
		ResolvedFields: map[string]any{
			"enforcement": string(model.EnforcementBlocking),
		},
	}

	// Claude: native hook support.
	unitCaps := claudeUnitCaps()

	lowered, result := lowering.LowerHook(obj, unitCaps)
	if !result.Allowed {
		t.Fatal("expected native hooks to pass through")
	}
	if lowered.Decision.Action != "kept" {
		t.Errorf("expected 'kept', got %q", lowered.Decision.Action)
	}
	if lowered.LoweredKind != model.KindHook {
		t.Errorf("expected LoweredKind=hook, got %s", lowered.LoweredKind)
	}
}

func TestLowerHook_Advisory_WithCommandSupport_LoweredToCommand(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "advisory-cmd-hook",
			Kind:         model.KindHook,
			Preservation: model.PreservationOptional,
		},
		ResolvedFields: map[string]any{
			"enforcement": string(model.EnforcementAdvisory),
			"effectClass": string(model.EffectObserving),
			"actionRef":   "./scripts/lint.sh",
		},
	}

	// Target with command support but no hook support.
	unitCaps := noHooksWithCommandsCaps()

	lowered, result := lowering.LowerHook(obj, unitCaps)
	if !result.Allowed {
		t.Fatalf("expected advisory hook to be lowered to command, got: %s", result.Message)
	}
	if lowered.LoweredKind != model.KindCommand {
		t.Errorf("expected LoweredKind=command, got %s", lowered.LoweredKind)
	}
	assertContains(t, lowered.Content, "./scripts/lint.sh")
}

func TestLowerHook_SetupEffect_NoCommandSupport_DocumentedAsManualStep(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "setup-hook",
			Kind:         model.KindHook,
			Preservation: model.PreservationOptional,
		},
		ResolvedFields: map[string]any{
			"enforcement": string(model.EnforcementAdvisory),
			"effectClass": string(model.EffectSetup),
			"actionRef":   "./scripts/setup.sh",
		},
	}

	// Limited target: no hook or command support.
	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerHook(obj, unitCaps)
	if !result.Allowed {
		t.Fatalf("expected setup hook to be documented, got: %s", result.Message)
	}
	if lowered.LoweredKind != model.KindInstruction {
		t.Errorf("expected LoweredKind=instruction, got %s", lowered.LoweredKind)
	}
	if lowered.Decision.Action != "lowered" {
		t.Errorf("expected action 'lowered', got %q", lowered.Decision.Action)
	}
	assertContains(t, lowered.Content, "Manual Step")
	assertContains(t, lowered.Content, "./scripts/setup.sh")
}

func TestLowerHook_ReportingEffect_NoCommandSupport_DocumentedAsManualStep(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "reporting-hook",
			Kind:         model.KindHook,
			Preservation: model.PreservationOptional,
		},
		ResolvedFields: map[string]any{
			"enforcement": string(model.EnforcementAdvisory),
			"effectClass": string(model.EffectReporting),
			"actionRef":   "./scripts/report.sh",
		},
	}

	// Limited target: no hook or command support.
	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerHook(obj, unitCaps)
	if !result.Allowed {
		t.Fatalf("expected reporting hook to be documented, got: %s", result.Message)
	}
	if lowered.LoweredKind != model.KindInstruction {
		t.Errorf("expected LoweredKind=instruction, got %s", lowered.LoweredKind)
	}
	assertContains(t, lowered.Content, "reporting")
}

// ==========================================================================
// Skill Lowering Tests
// ==========================================================================

func TestLowerSkill_NativeSupport_PassThrough(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "my-skill",
			Kind: model.KindSkill,
		},
	}

	unitCaps := claudeUnitCaps()

	lowered, result := lowering.LowerSkill(obj, unitCaps)
	if !result.Allowed {
		t.Fatal("expected native skill support to pass through")
	}
	if lowered.Decision.Action != "kept" {
		t.Errorf("expected 'kept', got %q", lowered.Decision.Action)
	}
}

func TestLowerSkill_LimitedTarget_Preferred_LoweredToRule(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "cursor-skill",
			Kind:         model.KindSkill,
			Preservation: model.PreservationPreferred,
			Description:  "A helpful skill",
		},
		ResolvedFields: map[string]any{
			"content": "Always validate inputs.\n",
		},
	}

	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerSkill(obj, unitCaps)
	// Preferred + unsafe → should be blocked (skipped).
	if result.Allowed {
		t.Fatal("expected preferred+unsafe to block")
	}
	if result.Severity != "warning" {
		t.Errorf("expected warning, got %q", result.Severity)
	}
	if lowered.Decision.Action != "skipped" {
		t.Errorf("expected 'skipped', got %q", lowered.Decision.Action)
	}
}

func TestLowerSkill_LimitedTarget_Required_Error(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "required-skill",
			Kind:         model.KindSkill,
			Preservation: model.PreservationRequired,
		},
	}

	unitCaps := limitedTargetUnitCaps()

	_, result := lowering.LowerSkill(obj, unitCaps)
	if result.Allowed {
		t.Fatal("expected required+limited target skill to produce error")
	}
	if result.Severity != "error" {
		t.Errorf("expected 'error', got %q", result.Severity)
	}
}

func TestLowerSkill_LimitedTarget_Optional_LoweredToRule(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "opt-skill",
			Kind:         model.KindSkill,
			Preservation: model.PreservationOptional,
			Description:  "Optional skill",
		},
		ResolvedFields: map[string]any{
			"content": "Skill content here.\n",
		},
	}

	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerSkill(obj, unitCaps)
	if !result.Allowed {
		t.Fatalf("expected optional skill to be allowed, got: %s", result.Message)
	}
	if lowered.LoweredKind != model.KindRule {
		t.Errorf("expected LoweredKind=rule, got %s", lowered.LoweredKind)
	}
	assertContains(t, lowered.Content, "Skill: opt-skill")
	assertContains(t, lowered.Content, "Skill content here.")
}

// ==========================================================================
// Agent Lowering Tests
// ==========================================================================

func TestLowerAgent_NativeSupport_PassThrough(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "my-agent",
			Kind: model.KindAgent,
		},
	}

	unitCaps := claudeUnitCaps()

	lowered, result := lowering.LowerAgent(obj, unitCaps)
	if !result.Allowed {
		t.Fatal("expected native agent support to pass through")
	}
	if lowered.Decision.Action != "kept" {
		t.Errorf("expected 'kept', got %q", lowered.Decision.Action)
	}
}

func TestLowerAgent_LimitedTarget_Optional_LoweredToRule(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "limited-agent",
			Kind:         model.KindAgent,
			Preservation: model.PreservationOptional,
		},
		ResolvedFields: map[string]any{
			"rolePrompt": "You are a code reviewer.\n",
		},
	}

	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerAgent(obj, unitCaps)
	if !result.Allowed {
		t.Fatalf("expected optional agent to be allowed, got: %s", result.Message)
	}
	if lowered.LoweredKind != model.KindRule {
		t.Errorf("expected LoweredKind=rule, got %s", lowered.LoweredKind)
	}
	assertContains(t, lowered.Content, "Agent: limited-agent")
	assertContains(t, lowered.Content, "You are a code reviewer.")
}

func TestLowerAgent_LimitedTarget_Required_Error(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "req-agent",
			Kind:         model.KindAgent,
			Preservation: model.PreservationRequired,
		},
	}

	unitCaps := limitedTargetUnitCaps()

	_, result := lowering.LowerAgent(obj, unitCaps)
	if result.Allowed {
		t.Fatal("expected required+limited target agent to produce error")
	}
	if result.Severity != "error" {
		t.Errorf("expected 'error', got %q", result.Severity)
	}
}

// ==========================================================================
// Command Lowering Tests
// ==========================================================================

func TestLowerCommand_Copilot_NativePassThrough(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "my-cmd",
			Kind: model.KindCommand,
		},
	}

	unitCaps := copilotUnitCaps()

	lowered, result := lowering.LowerCommand(obj, unitCaps, "copilot")
	if !result.Allowed {
		t.Fatal("expected copilot commands to pass through natively")
	}
	if lowered.Decision.Action != "kept" {
		t.Errorf("expected 'kept', got %q", lowered.Decision.Action)
	}
}

func TestLowerCommand_Claude_LoweredToSkill(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:          "claude-cmd",
			Kind:        model.KindCommand,
			Description: "Run tests",
		},
		ResolvedFields: map[string]any{
			"actionRef": "./scripts/test.sh",
		},
	}

	unitCaps := claudeUnitCaps()

	lowered, result := lowering.LowerCommand(obj, unitCaps, "claude")
	if !result.Allowed {
		t.Fatalf("expected claude command to be lowered, got: %s", result.Message)
	}
	if lowered.LoweredKind != model.KindSkill {
		t.Errorf("expected LoweredKind=skill, got %s", lowered.LoweredKind)
	}
	if !lowered.Decision.Safe {
		t.Error("command→skill should be safe for lowered support")
	}
	assertContains(t, lowered.Content, "Command: claude-cmd")
	assertContains(t, lowered.Content, "./scripts/test.sh")
}

func TestLowerCommand_Cursor_Skipped(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "cursor-cmd",
			Kind:         model.KindCommand,
			Preservation: model.PreservationOptional,
		},
	}

	unitCaps := cursorUnitCaps()

	lowered, result := lowering.LowerCommand(obj, unitCaps, "cursor")
	if !result.Allowed {
		t.Fatalf("expected optional to be allowed, got: %s", result.Message)
	}
	if lowered.Decision.Action != "skipped" {
		t.Errorf("expected 'skipped', got %q", lowered.Decision.Action)
	}
}

func TestLowerCommand_Cursor_Required_Error(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "req-cmd",
			Kind:         model.KindCommand,
			Preservation: model.PreservationRequired,
		},
	}

	unitCaps := cursorUnitCaps()

	_, result := lowering.LowerCommand(obj, unitCaps, "cursor")
	if result.Allowed {
		t.Fatal("expected required+cursor command to produce error")
	}
	if result.Severity != "error" {
		t.Errorf("expected 'error', got %q", result.Severity)
	}
}

func TestLowerCommand_PluginProvider_PassThrough(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "plugin-cmd",
			Kind: model.KindCommand,
		},
	}

	// Create caps where command is resolved via a plugin provider (not native).
	unitCaps := pipeline.UnitCapabilities{
		Coordinate: build.BuildCoordinate{
			Unit:      build.BuildUnit{Target: "custom", Profile: build.ProfileLocalDev},
			OutputDir: ".ai-build/custom/local-dev",
		},
		Required: []string{"commands.explicitEntryPoints"},
		Resolved: map[string]domcap.Provider{
			"commands.explicitEntryPoints": {
				ID:           "my-plugin",
				Type:         "plugin",
				Capabilities: []string{"commands.explicitEntryPoints"},
			},
		},
	}

	lowered, result := lowering.LowerCommand(obj, unitCaps, "custom")
	if !result.Allowed {
		t.Fatalf("expected plugin-provided command to pass through, got: %s", result.Message)
	}
	if lowered.Decision.Action != "kept" {
		t.Errorf("expected 'kept', got %q", lowered.Decision.Action)
	}
}

// ==========================================================================
// Plugin Lowering Tests
// ==========================================================================

func TestLowerPlugin_NativeSupport_PassThrough(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "my-plugin",
			Kind: model.KindPlugin,
		},
	}

	unitCaps := claudeUnitCaps()

	lowered, result := lowering.LowerPlugin(obj, unitCaps, "claude")
	if !result.Allowed {
		t.Fatal("expected claude plugins to pass through natively")
	}
	if lowered.Decision.Action != "kept" {
		t.Errorf("expected 'kept', got %q", lowered.Decision.Action)
	}
}

func TestLowerPlugin_LimitedTarget_WithMCP_LoweredToMCPRef(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:   "mcp-plugin",
			Kind: model.KindPlugin,
		},
		ResolvedFields: map[string]any{
			"provides": []string{"mcp.serverBindings"},
		},
	}

	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerPlugin(obj, unitCaps, "limited")
	if !result.Allowed {
		t.Fatalf("expected MCP plugin on limited target to be lowered, got: %s", result.Message)
	}
	if lowered.Decision.Action != "lowered" {
		t.Errorf("expected 'lowered', got %q", lowered.Decision.Action)
	}
	assertContains(t, lowered.Content, "MCP server reference")
}

func TestLowerPlugin_LimitedTarget_NoMCP_Required_Error(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "no-mcp-plugin",
			Kind:         model.KindPlugin,
			Preservation: model.PreservationRequired,
		},
		ResolvedFields: map[string]any{},
	}

	unitCaps := limitedTargetUnitCaps()

	_, result := lowering.LowerPlugin(obj, unitCaps, "limited")
	if result.Allowed {
		t.Fatal("expected required+limited target plugin without MCP to produce error")
	}
	if result.Severity != "error" {
		t.Errorf("expected 'error', got %q", result.Severity)
	}
}

func TestLowerPlugin_LimitedTarget_NoMCP_Optional_Skipped(t *testing.T) {
	obj := pipeline.NormalizedObject{
		Meta: model.ObjectMeta{
			ID:           "opt-plugin",
			Kind:         model.KindPlugin,
			Preservation: model.PreservationOptional,
		},
		ResolvedFields: map[string]any{},
	}

	unitCaps := limitedTargetUnitCaps()

	lowered, result := lowering.LowerPlugin(obj, unitCaps, "limited")
	if !result.Allowed {
		t.Fatalf("expected optional plugin to be allowed, got: %s", result.Message)
	}
	if lowered.Decision.Action != "skipped" {
		t.Errorf("expected 'skipped', got %q", lowered.Decision.Action)
	}
}

// ==========================================================================
// Lowering Engine Integration Tests
// ==========================================================================

func TestEngine_Descriptor(t *testing.T) {
	s := lowering.New(nil)
	d := s.Descriptor()
	if d.Name != "lowering-engine" {
		t.Errorf("expected name 'lowering-engine', got %q", d.Name)
	}
	if d.Phase != pipeline.PhaseLower {
		t.Errorf("expected phase PhaseLower, got %d", d.Phase)
	}
	if d.Order != 10 {
		t.Errorf("expected order 10, got %d", d.Order)
	}
}

func TestEngine_InvalidInput_Error(t *testing.T) {
	s := lowering.New(nil)
	_, err := s.Execute(context.Background(), "not a capability graph")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestEngine_PointerInput(t *testing.T) {
	graph := &pipeline.CapabilityGraph{Units: make(map[string]pipeline.UnitCapabilities)}
	s := lowering.New(nil)
	_, err := s.Execute(testCtx(), graph)
	if err != nil {
		t.Fatalf("unexpected error with pointer input: %v", err)
	}
}

func TestEngine_EmptyGraph(t *testing.T) {
	graph := pipeline.CapabilityGraph{Units: make(map[string]pipeline.UnitCapabilities)}
	s := lowering.New(nil)
	result, err := s.Execute(testCtx(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lg, ok := result.(pipeline.LoweredGraph)
	if !ok {
		t.Fatalf("expected LoweredGraph, got %T", result)
	}
	if len(lg.Units) != 0 {
		t.Errorf("expected 0 units, got %d", len(lg.Units))
	}
}

func TestEngine_AllObjectsProduceLoweringRecords(t *testing.T) {
	objects := map[string]pipeline.NormalizedObject{
		"instr-1": {
			Meta:           model.ObjectMeta{ID: "instr-1", Kind: model.KindInstruction},
			ResolvedFields: map[string]any{},
		},
		"rule-1": {
			Meta:           model.ObjectMeta{ID: "rule-1", Kind: model.KindRule},
			ResolvedFields: map[string]any{"content": "rule content"},
		},
	}

	graph := pipeline.CapabilityGraph{
		Units: map[string]pipeline.UnitCapabilities{
			".ai-build/claude/local-dev": claudeUnitCapsWithObjects(),
		},
	}

	recorder := &mockReporter{}
	ctx := testCtxWithReporter(recorder)
	s := lowering.New(objects)
	result, err := s.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lg := result.(pipeline.LoweredGraph)
	unit := lg.Units[".ai-build/claude/local-dev"]

	// All objects should be present.
	if len(unit.Objects) < 2 {
		t.Errorf("expected at least 2 objects, got %d", len(unit.Objects))
	}

	// Reporter should have received lowering records for each object.
	if len(recorder.records) < 2 {
		t.Errorf("expected at least 2 lowering records, got %d", len(recorder.records))
	}
}

func TestEngine_LimitedTargetSkillRequired_ProducesError(t *testing.T) {
	objects := map[string]pipeline.NormalizedObject{
		"req-skill": {
			Meta: model.ObjectMeta{
				ID:           "req-skill",
				Kind:         model.KindSkill,
				Preservation: model.PreservationRequired,
			},
			ResolvedFields: map[string]any{},
		},
	}

	graph := pipeline.CapabilityGraph{
		Units: map[string]pipeline.UnitCapabilities{
			".ai-build/limited/local-dev": limitedTargetUnitCaps(),
		},
	}

	s := lowering.New(objects)
	_, err := s.Execute(testCtx(), graph)
	if err == nil {
		t.Fatal("expected error for required skill on limited target")
	}
}

func TestEngine_LimitedTargetBlockingHookRequired_ProducesError(t *testing.T) {
	objects := map[string]pipeline.NormalizedObject{
		"blocking-hook": {
			Meta: model.ObjectMeta{
				ID:           "blocking-hook",
				Kind:         model.KindHook,
				Preservation: model.PreservationRequired,
			},
			ResolvedFields: map[string]any{
				"enforcement": string(model.EnforcementBlocking),
				"effectClass": string(model.EffectValidating),
			},
		},
	}

	graph := pipeline.CapabilityGraph{
		Units: map[string]pipeline.UnitCapabilities{
			".ai-build/limited/local-dev": limitedTargetUnitCaps(),
		},
	}

	s := lowering.New(objects)
	_, err := s.Execute(testCtx(), graph)
	if err == nil {
		t.Fatal("expected error for blocking hook with required on limited target")
	}
}

func TestEngine_CopilotAllNative_NoErrors(t *testing.T) {
	objects := map[string]pipeline.NormalizedObject{
		"instr": {
			Meta:           model.ObjectMeta{ID: "instr", Kind: model.KindInstruction},
			ResolvedFields: map[string]any{},
		},
		"rule": {
			Meta:           model.ObjectMeta{ID: "rule", Kind: model.KindRule},
			ResolvedFields: map[string]any{"content": "rule"},
		},
		"skill": {
			Meta:           model.ObjectMeta{ID: "skill", Kind: model.KindSkill},
			ResolvedFields: map[string]any{},
		},
		"agent": {
			Meta:           model.ObjectMeta{ID: "agent", Kind: model.KindAgent},
			ResolvedFields: map[string]any{},
		},
		"hook": {
			Meta:           model.ObjectMeta{ID: "hook", Kind: model.KindHook},
			ResolvedFields: map[string]any{},
		},
		"cmd": {
			Meta:           model.ObjectMeta{ID: "cmd", Kind: model.KindCommand},
			ResolvedFields: map[string]any{},
		},
		"plugin": {
			Meta:           model.ObjectMeta{ID: "plugin", Kind: model.KindPlugin},
			ResolvedFields: map[string]any{},
		},
	}

	graph := pipeline.CapabilityGraph{
		Units: map[string]pipeline.UnitCapabilities{
			".ai-build/copilot/local-dev": copilotUnitCaps(),
		},
	}

	s := lowering.New(objects)
	result, err := s.Execute(testCtx(), graph)
	if err != nil {
		t.Fatalf("unexpected error for copilot (all native): %v", err)
	}

	lg := result.(pipeline.LoweredGraph)
	unit := lg.Units[".ai-build/copilot/local-dev"]

	if len(unit.Objects) != 7 {
		t.Errorf("expected 7 objects in copilot unit, got %d", len(unit.Objects))
	}

	// All should be kept or safely lowered.
	for id, obj := range unit.Objects {
		if obj.Decision.Action == "failed" {
			t.Errorf("object %s failed unexpectedly: %s", id, obj.Decision.Reason)
		}
	}
}

func TestEngine_MultipleUnits(t *testing.T) {
	objects := map[string]pipeline.NormalizedObject{
		"rule-1": {
			Meta:           model.ObjectMeta{ID: "rule-1", Kind: model.KindRule},
			ResolvedFields: map[string]any{"content": "universal rule"},
		},
	}

	graph := pipeline.CapabilityGraph{
		Units: map[string]pipeline.UnitCapabilities{
			".ai-build/claude/local-dev": claudeUnitCapsWithObjects(),
			".ai-build/cursor/local-dev": cursorUnitCaps(),
		},
	}

	s := lowering.New(objects)
	result, err := s.Execute(testCtx(), graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lg := result.(pipeline.LoweredGraph)
	if len(lg.Units) != 2 {
		t.Errorf("expected 2 units, got %d", len(lg.Units))
	}
}

func TestEngine_Factory(t *testing.T) {
	factory := lowering.Factory(nil)
	s, err := factory()
	if err != nil {
		t.Fatalf("unexpected factory error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil stage")
	}
}

func TestEngine_Deterministic(t *testing.T) {
	objects := map[string]pipeline.NormalizedObject{
		"rule-a": {
			Meta:           model.ObjectMeta{ID: "rule-a", Kind: model.KindRule},
			ResolvedFields: map[string]any{"content": "a"},
		},
		"rule-b": {
			Meta:           model.ObjectMeta{ID: "rule-b", Kind: model.KindRule},
			ResolvedFields: map[string]any{"content": "b"},
		},
		"rule-c": {
			Meta:           model.ObjectMeta{ID: "rule-c", Kind: model.KindRule},
			ResolvedFields: map[string]any{"content": "c"},
		},
	}

	graph := pipeline.CapabilityGraph{
		Units: map[string]pipeline.UnitCapabilities{
			".ai-build/claude/local-dev": claudeUnitCapsWithObjects(),
		},
	}

	s := lowering.New(objects)

	// Run multiple times and verify same output.
	var firstResult pipeline.LoweredGraph
	for i := 0; i < 5; i++ {
		result, err := s.Execute(testCtx(), graph)
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		lg := result.(pipeline.LoweredGraph)
		if i == 0 {
			firstResult = lg
			continue
		}
		// Compare object count and keys.
		unit1 := firstResult.Units[".ai-build/claude/local-dev"]
		unit2 := lg.Units[".ai-build/claude/local-dev"]
		if len(unit1.Objects) != len(unit2.Objects) {
			t.Fatalf("run %d: non-deterministic: got %d vs %d objects", i, len(unit1.Objects), len(unit2.Objects))
		}
		for id, obj1 := range unit1.Objects {
			obj2, ok := unit2.Objects[id]
			if !ok {
				t.Fatalf("run %d: missing object %s", i, id)
			}
			if obj1.Decision.Action != obj2.Decision.Action {
				t.Fatalf("run %d: object %s: action %s vs %s", i, id, obj1.Decision.Action, obj2.Decision.Action)
			}
			if obj1.Content != obj2.Content {
				t.Fatalf("run %d: object %s: different content", i, id)
			}
		}
	}
}

func TestEngine_DiagnosticsEmitted(t *testing.T) {
	objects := map[string]pipeline.NormalizedObject{
		"rule-1": {
			Meta:           model.ObjectMeta{ID: "rule-1", Kind: model.KindRule},
			ResolvedFields: map[string]any{"content": "r"},
		},
	}

	graph := pipeline.CapabilityGraph{
		Units: map[string]pipeline.UnitCapabilities{
			".ai-build/claude/local-dev": claudeUnitCapsWithObjects(),
		},
	}

	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: &pipeline.BuildReport{},
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	s := lowering.New(objects)
	_, err := s.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least LOWERING_START and LOWERING_COMPLETE diagnostics.
	foundStart := false
	foundComplete := false
	for _, d := range cc.Report.Diagnostics {
		if d.Code == "LOWERING_START" {
			foundStart = true
		}
		if d.Code == "LOWERING_COMPLETE" {
			foundComplete = true
		}
	}
	if !foundStart {
		t.Error("expected LOWERING_START diagnostic")
	}
	if !foundComplete {
		t.Error("expected LOWERING_COMPLETE diagnostic")
	}
}

// ==========================================================================
// Helpers
// ==========================================================================

func testCtx() context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: &pipeline.BuildReport{},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func testCtxWithReporter(r *mockReporter) context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			Reporter: r,
		},
		Report: &pipeline.BuildReport{},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

type mockReporter struct {
	records  []pipeline.LoweringRecord
	skipped  []string
	warnings []string
}

func (m *mockReporter) ReportLowering(_ context.Context, record pipeline.LoweringRecord) {
	m.records = append(m.records, record)
}

func (m *mockReporter) ReportSkipped(_ context.Context, objectID string, _ string) {
	m.skipped = append(m.skipped, objectID)
}

func (m *mockReporter) ReportFailed(_ context.Context, _ string, _ error) {}

func (m *mockReporter) ReportWarning(_ context.Context, msg string) {
	m.warnings = append(m.warnings, msg)
}

func nativeProvider(target, surface string) domcap.Provider {
	return domcap.Provider{
		ID:           target + "/native/" + surface,
		Type:         "native",
		Capabilities: []string{surface},
	}
}

func loweredProvider(target, surface string) domcap.Provider {
	return domcap.Provider{
		ID:           target + "/lowered/" + surface,
		Type:         "native",
		Capabilities: []string{surface},
	}
}

func adaptedProvider(target, surface string) domcap.Provider {
	return domcap.Provider{
		ID:           target + "/adapted/" + surface,
		Type:         "native",
		Capabilities: []string{surface},
	}
}

func claudeUnitCaps() pipeline.UnitCapabilities {
	return pipeline.UnitCapabilities{
		Coordinate: build.BuildCoordinate{
			Unit:      build.BuildUnit{Target: build.TargetClaude, Profile: build.ProfileLocalDev},
			OutputDir: ".ai-build/claude/local-dev",
		},
		Required: []string{
			"instructions.layeredFiles", "instructions.scopedSections",
			"rules.scopedRules",
			"skills.bundles", "skills.supportingFiles",
			"agents.subagents", "agents.toolPolicies",
			"hooks.lifecycle", "hooks.blockingValidation",
			"commands.explicitEntryPoints",
			"plugins.installablePackages", "plugins.capabilityProviders",
			"mcp.serverBindings",
		},
		Resolved: map[string]domcap.Provider{
			"instructions.layeredFiles":    nativeProvider("claude", "instructions.layeredFiles"),
			"instructions.scopedSections":  nativeProvider("claude", "instructions.scopedSections"),
			"rules.scopedRules":            nativeProvider("claude", "rules.scopedRules"),
			"skills.bundles":               nativeProvider("claude", "skills.bundles"),
			"skills.supportingFiles":       nativeProvider("claude", "skills.supportingFiles"),
			"agents.subagents":             nativeProvider("claude", "agents.subagents"),
			"agents.toolPolicies":          nativeProvider("claude", "agents.toolPolicies"),
			"hooks.lifecycle":              nativeProvider("claude", "hooks.lifecycle"),
			"hooks.blockingValidation":     nativeProvider("claude", "hooks.blockingValidation"),
			"commands.explicitEntryPoints": loweredProvider("claude", "commands.explicitEntryPoints"),
			"plugins.installablePackages":  nativeProvider("claude", "plugins.installablePackages"),
			"plugins.capabilityProviders":  nativeProvider("claude", "plugins.capabilityProviders"),
			"mcp.serverBindings":           nativeProvider("claude", "mcp.serverBindings"),
		},
	}
}

func claudeUnitCapsWithObjects() pipeline.UnitCapabilities {
	uc := claudeUnitCaps()
	return uc
}

func cursorUnitCaps() pipeline.UnitCapabilities {
	return pipeline.UnitCapabilities{
		Coordinate: build.BuildCoordinate{
			Unit:      build.BuildUnit{Target: build.TargetCursor, Profile: build.ProfileLocalDev},
			OutputDir: ".ai-build/cursor/local-dev",
		},
		Required: []string{
			"instructions.layeredFiles", "instructions.scopedSections",
			"rules.scopedRules",
			"skills.bundles", "skills.supportingFiles",
			"agents.subagents", "agents.toolPolicies",
			"hooks.lifecycle", "hooks.blockingValidation",
			"commands.explicitEntryPoints",
			"plugins.installablePackages", "plugins.capabilityProviders",
			"mcp.serverBindings",
		},
		Resolved: map[string]domcap.Provider{
			"instructions.layeredFiles":   nativeProvider("cursor", "instructions.layeredFiles"),
			"instructions.scopedSections": nativeProvider("cursor", "instructions.scopedSections"),
			"rules.scopedRules":           nativeProvider("cursor", "rules.scopedRules"),
			"skills.bundles":              adaptedProvider("cursor", "skills.bundles"),
			"skills.supportingFiles":      adaptedProvider("cursor", "skills.supportingFiles"),
			"agents.subagents":            adaptedProvider("cursor", "agents.subagents"),
			"agents.toolPolicies":         adaptedProvider("cursor", "agents.toolPolicies"),
			"hooks.lifecycle":             nativeProvider("cursor", "hooks.lifecycle"),
			"hooks.blockingValidation":    nativeProvider("cursor", "hooks.blockingValidation"),
			"plugins.installablePackages": nativeProvider("cursor", "plugins.installablePackages"),
			"plugins.capabilityProviders": nativeProvider("cursor", "plugins.capabilityProviders"),
			"mcp.serverBindings":          nativeProvider("cursor", "mcp.serverBindings"),
		},
		Unsatisfied: []string{
			"agents.handoffs",
			"commands.explicitEntryPoints",
		},
	}
}

func limitedTargetUnitCaps() pipeline.UnitCapabilities {
	return pipeline.UnitCapabilities{
		Coordinate: build.BuildCoordinate{
			Unit:      build.BuildUnit{Target: build.TargetCursor, Profile: build.ProfileLocalDev},
			OutputDir: ".ai-build/limited/local-dev",
		},
		Required: []string{
			"instructions.layeredFiles", "instructions.scopedSections",
			"rules.scopedRules",
			"skills.bundles", "skills.supportingFiles",
			"agents.subagents", "agents.toolPolicies",
			"hooks.lifecycle", "hooks.blockingValidation",
			"commands.explicitEntryPoints",
			"plugins.installablePackages", "plugins.capabilityProviders",
			"mcp.serverBindings",
		},
		Resolved: map[string]domcap.Provider{
			"instructions.layeredFiles":   nativeProvider("limited", "instructions.layeredFiles"),
			"instructions.scopedSections": nativeProvider("limited", "instructions.scopedSections"),
			"rules.scopedRules":           nativeProvider("limited", "rules.scopedRules"),
			"mcp.serverBindings":          nativeProvider("limited", "mcp.serverBindings"),
		},
		Unsatisfied: []string{
			"skills.bundles", "skills.supportingFiles",
			"agents.subagents", "agents.toolPolicies", "agents.handoffs",
			"hooks.lifecycle", "hooks.blockingValidation",
			"commands.explicitEntryPoints",
			"plugins.installablePackages", "plugins.capabilityProviders",
		},
	}
}

func copilotUnitCaps() pipeline.UnitCapabilities {
	return pipeline.UnitCapabilities{
		Coordinate: build.BuildCoordinate{
			Unit:      build.BuildUnit{Target: build.TargetCopilot, Profile: build.ProfileLocalDev},
			OutputDir: ".ai-build/copilot/local-dev",
		},
		Required: []string{
			"instructions.layeredFiles", "instructions.scopedSections",
			"rules.scopedRules",
			"skills.bundles", "skills.supportingFiles",
			"agents.subagents", "agents.toolPolicies", "agents.handoffs",
			"hooks.lifecycle", "hooks.blockingValidation",
			"commands.explicitEntryPoints",
			"plugins.installablePackages", "plugins.capabilityProviders",
			"mcp.serverBindings",
		},
		Resolved: map[string]domcap.Provider{
			"instructions.layeredFiles":    nativeProvider("copilot", "instructions.layeredFiles"),
			"instructions.scopedSections":  nativeProvider("copilot", "instructions.scopedSections"),
			"rules.scopedRules":            nativeProvider("copilot", "rules.scopedRules"),
			"skills.bundles":               nativeProvider("copilot", "skills.bundles"),
			"skills.supportingFiles":       nativeProvider("copilot", "skills.supportingFiles"),
			"agents.subagents":             nativeProvider("copilot", "agents.subagents"),
			"agents.toolPolicies":          nativeProvider("copilot", "agents.toolPolicies"),
			"agents.handoffs":              nativeProvider("copilot", "agents.handoffs"),
			"hooks.lifecycle":              nativeProvider("copilot", "hooks.lifecycle"),
			"hooks.blockingValidation":     nativeProvider("copilot", "hooks.blockingValidation"),
			"commands.explicitEntryPoints": nativeProvider("copilot", "commands.explicitEntryPoints"),
			"plugins.installablePackages":  nativeProvider("copilot", "plugins.installablePackages"),
			"plugins.capabilityProviders":  nativeProvider("copilot", "plugins.capabilityProviders"),
			"mcp.serverBindings":           nativeProvider("copilot", "mcp.serverBindings"),
		},
	}
}

func noHooksWithCommandsCaps() pipeline.UnitCapabilities {
	return pipeline.UnitCapabilities{
		Coordinate: build.BuildCoordinate{
			Unit:      build.BuildUnit{Target: "custom", Profile: build.ProfileLocalDev},
			OutputDir: ".ai-build/custom/local-dev",
		},
		Required: []string{
			"hooks.lifecycle", "hooks.blockingValidation",
			"commands.explicitEntryPoints",
		},
		Resolved: map[string]domcap.Provider{
			"commands.explicitEntryPoints": nativeProvider("custom", "commands.explicitEntryPoints"),
		},
		Unsatisfied: []string{
			"hooks.lifecycle", "hooks.blockingValidation",
		},
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !containsStr(haystack, needle) {
		t.Errorf("expected content to contain %q, got:\n%s", needle, haystack)
	}
}

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(haystack) > 0 && containsSubstr(haystack, needle))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
