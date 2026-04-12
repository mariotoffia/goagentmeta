package capability_test

import (
	"context"
	"testing"

	capstage "github.com/mariotoffia/goagentmeta/internal/adapter/stage/capability"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	domcap "github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// --- Registry Tests ---

func TestLoadRegistry_Claude(t *testing.T) {
	capstage.ResetCache()
	reg, err := capstage.LoadRegistry("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Target != "claude" {
		t.Errorf("expected target 'claude', got %q", reg.Target)
	}
	assertSurface(t, reg, "instructions.layeredFiles", domcap.SupportNative)
	assertSurface(t, reg, "agents.handoffs", domcap.SupportSkipped)
	assertSurface(t, reg, "commands.explicitEntryPoints", domcap.SupportLowered)
}

func TestLoadRegistry_Cursor(t *testing.T) {
	capstage.ResetCache()
	reg, err := capstage.LoadRegistry("cursor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSurface(t, reg, "skills.bundles", domcap.SupportAdapted)
	assertSurface(t, reg, "instructions.layeredFiles", domcap.SupportAdapted)
	assertSurface(t, reg, "mcp.serverBindings", domcap.SupportNative)
}

func TestLoadRegistry_Copilot(t *testing.T) {
	capstage.ResetCache()
	reg, err := capstage.LoadRegistry("copilot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSurface(t, reg, "agents.handoffs", domcap.SupportNative)
	assertSurface(t, reg, "commands.explicitEntryPoints", domcap.SupportNative)
}

func TestLoadRegistry_Codex(t *testing.T) {
	capstage.ResetCache()
	reg, err := capstage.LoadRegistry("codex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSurface(t, reg, "commands.explicitEntryPoints", domcap.SupportLowered)
	assertSurface(t, reg, "agents.handoffs", domcap.SupportSkipped)
}

func TestLoadRegistry_UnknownTarget_Error(t *testing.T) {
	capstage.ResetCache()
	_, err := capstage.LoadRegistry("unknown")
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestAllRegistries(t *testing.T) {
	capstage.ResetCache()
	regs, err := capstage.AllRegistries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(regs) != 4 {
		t.Fatalf("expected 4 registries, got %d", len(regs))
	}
	for _, name := range []string{"claude", "cursor", "copilot", "codex"} {
		if _, ok := regs[name]; !ok {
			t.Errorf("missing registry for %s", name)
		}
	}
}

func TestLoadRegistry_Caching(t *testing.T) {
	capstage.ResetCache()
	r1, _ := capstage.LoadRegistry("claude")
	r2, _ := capstage.LoadRegistry("claude")
	if r1 != r2 {
		t.Error("expected cached registry to be the same pointer")
	}
}

// --- Provider Selection Tests ---

func TestSelectProvider_NativeSupport(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("claude")

	candidate := capstage.SelectProvider("instructions.layeredFiles", reg, nil, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected compatible provider for native support")
	}
	if candidate.Provider.Type != "native" {
		t.Errorf("expected type 'native', got %q", candidate.Provider.Type)
	}
	if candidate.Priority != 1 {
		t.Errorf("expected priority 1, got %d", candidate.Priority)
	}
}

func TestSelectProvider_SkippedOptional_NoError(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("cursor")

	// agents.handoffs is skipped for cursor
	candidate := capstage.SelectProvider("agents.handoffs", reg, nil, build.ProfileLocalDev)
	if candidate.Compatible {
		t.Error("expected incompatible for skipped capability with no plugin fallback")
	}
}

func TestSelectProvider_PluginFallback(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("cursor")

	plugins := []domcap.Provider{
		{ID: "my-handoff-plugin", Type: "plugin", Capabilities: []string{"agents.handoffs"}},
	}

	candidate := capstage.SelectProvider("agents.handoffs", reg, plugins, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected plugin to provide fallback")
	}
	if candidate.Provider.ID != "my-handoff-plugin" {
		t.Errorf("expected plugin ID 'my-handoff-plugin', got %q", candidate.Provider.ID)
	}
	if candidate.Priority != 2 {
		t.Errorf("expected priority 2 for plugin, got %d", candidate.Priority)
	}
}

func TestSelectProvider_EnterpriseLocked_BlocksMCP(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("cursor")

	plugins := []domcap.Provider{
		{ID: "mcp-handoff-provider", Type: "mcp", Capabilities: []string{"agents.handoffs"}},
	}

	candidate := capstage.SelectProvider("agents.handoffs", reg, plugins, build.ProfileEnterpriseLocked)
	if candidate.Compatible {
		t.Error("expected MCP provider to be blocked under enterprise-locked profile")
	}
}

func TestSelectProvider_EnterpriseLocked_AllowsPlugin(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("cursor")

	plugins := []domcap.Provider{
		{ID: "safe-plugin", Type: "plugin", Capabilities: []string{"agents.handoffs"}},
	}

	candidate := capstage.SelectProvider("agents.handoffs", reg, plugins, build.ProfileEnterpriseLocked)
	if !candidate.Compatible {
		t.Error("expected non-MCP plugin to be allowed under enterprise-locked")
	}
}

func TestSelectProvider_TwoPlugins_HighestPriorityWins(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("cursor")

	// Adversarial: script ID sorts alphabetically before plugin ID,
	// but plugin type (priority 2) should win over script (priority 4).
	plugins := []domcap.Provider{
		{ID: "a-script", Type: "script", Capabilities: []string{"agents.handoffs"}},
		{ID: "b-plugin", Type: "plugin", Capabilities: []string{"agents.handoffs"}},
	}

	candidate := capstage.SelectProvider("agents.handoffs", reg, plugins, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected compatible provider")
	}
	if candidate.Provider.Type != "plugin" {
		t.Errorf("expected plugin type (priority 2) to win over script (priority 4), got type=%q id=%q",
			candidate.Provider.Type, candidate.Provider.ID)
	}
	if candidate.Provider.ID != "b-plugin" {
		t.Errorf("expected 'b-plugin', got %q", candidate.Provider.ID)
	}
}

func TestSelectProvider_AdaptedSupport(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("cursor")

	candidate := capstage.SelectProvider("instructions.layeredFiles", reg, nil, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected adapted to be compatible")
	}
	if candidate.Priority != 1 {
		t.Errorf("expected priority 1, got %d", candidate.Priority)
	}
}

func TestSelectProvider_LoweredSupport(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("claude")

	candidate := capstage.SelectProvider("commands.explicitEntryPoints", reg, nil, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected lowered to be compatible")
	}
}

// --- RequiredCapabilities Tests ---

func TestRequiredCapabilities_Instruction(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindInstruction})
	assertContains(t, caps, "instructions.layeredFiles")
	assertContains(t, caps, "instructions.scopedSections")
}

func TestRequiredCapabilities_Skill(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindSkill})
	assertContains(t, caps, "skills.bundles")
	assertContains(t, caps, "skills.supportingFiles")
}

func TestRequiredCapabilities_Agent(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindAgent})
	assertContains(t, caps, "agents.subagents")
	assertContains(t, caps, "agents.toolPolicies")
}

func TestRequiredCapabilities_Hook(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindHook})
	assertContains(t, caps, "hooks.lifecycle")
}

func TestRequiredCapabilities_Command(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindCommand})
	assertContains(t, caps, "commands.explicitEntryPoints")
}

func TestRequiredCapabilities_Plugin(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindPlugin})
	assertContains(t, caps, "plugins.installablePackages")
}

func TestRequiredCapabilities_Capability_NoSurfaces(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindCapability})
	if len(caps) != 0 {
		t.Errorf("expected no surfaces for KindCapability, got %v", caps)
	}
}

// --- CapabilityGraph Structure Tests ---

func TestCapabilityResolver_AllUnitsPresent(t *testing.T) {
	capstage.ResetCache()

	objects := map[string]pipeline.NormalizedObject{
		"obj-1": {Meta: model.ObjectMeta{ID: "obj-1", Kind: model.KindInstruction}},
	}

	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate:    makeCoord(build.TargetClaude, build.ProfileLocalDev),
				ActiveObjects: []string{"obj-1"},
			},
			{
				Coordinate:    makeCoord(build.TargetCursor, build.ProfileLocalDev),
				ActiveObjects: []string{"obj-1"},
			},
		},
	}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(objects, nil)
	result, err := stage.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, ok := result.(pipeline.CapabilityGraph)
	if !ok {
		t.Fatalf("expected CapabilityGraph, got %T", result)
	}

	if len(graph.Units) != 2 {
		t.Fatalf("expected 2 units in graph, got %d", len(graph.Units))
	}

	// Both units should be present.
	for _, unit := range plan.Units {
		if _, ok := graph.Units[unit.Coordinate.OutputDir]; !ok {
			t.Errorf("missing unit %s in graph", unit.Coordinate.OutputDir)
		}
	}
}

func TestCapabilityResolver_ResolvedAndUnsatisfied(t *testing.T) {
	capstage.ResetCache()

	// Command on cursor: commands.explicitEntryPoints is skipped.
	objects := map[string]pipeline.NormalizedObject{
		"cmd-1": {
			Meta: model.ObjectMeta{
				ID:           "cmd-1",
				Kind:         model.KindCommand,
				Preservation: model.PreservationOptional,
			},
		},
	}

	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate:    makeCoord(build.TargetCursor, build.ProfileLocalDev),
				ActiveObjects: []string{"cmd-1"},
			},
		},
	}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(objects, nil)
	result, err := stage.Execute(ctx, plan)
	// Should NOT error because preservation is optional.
	if err != nil {
		t.Fatalf("unexpected error for optional preservation: %v", err)
	}

	graph := result.(pipeline.CapabilityGraph)
	uc := graph.Units[".ai-build/cursor/local-dev"]

	if len(uc.Unsatisfied) == 0 {
		t.Error("expected unsatisfied capabilities for cursor commands")
	}

	// Check that unsatisfied includes command surfaces.
	unsatisfiedSet := make(map[string]bool)
	for _, u := range uc.Unsatisfied {
		unsatisfiedSet[u] = true
	}
	if !unsatisfiedSet["commands.explicitEntryPoints"] {
		t.Error("expected commands.explicitEntryPoints to be unsatisfied")
	}
}

func TestCapabilityResolver_RequiredUnsatisfied_Error(t *testing.T) {
	capstage.ResetCache()

	// Command with required preservation on cursor → should error
	// because commands.explicitEntryPoints is skipped.
	objects := map[string]pipeline.NormalizedObject{
		"cmd-required": {
			Meta: model.ObjectMeta{
				ID:           "cmd-required",
				Kind:         model.KindCommand,
				Preservation: model.PreservationRequired,
			},
		},
	}

	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate:    makeCoord(build.TargetCursor, build.ProfileLocalDev),
				ActiveObjects: []string{"cmd-required"},
			},
		},
	}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(objects, nil)
	_, err := stage.Execute(ctx, plan)
	if err == nil {
		t.Fatal("expected error for required + skipped capability")
	}
}

func TestCapabilityResolver_PreferredUnsatisfied_NoError(t *testing.T) {
	capstage.ResetCache()

	objects := map[string]pipeline.NormalizedObject{
		"skill-preferred": {
			Meta: model.ObjectMeta{
				ID:           "skill-preferred",
				Kind:         model.KindSkill,
				Preservation: model.PreservationPreferred,
			},
		},
	}

	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate:    makeCoord(build.TargetCursor, build.ProfileLocalDev),
				ActiveObjects: []string{"skill-preferred"},
			},
		},
	}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(objects, nil)
	_, err := stage.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("unexpected error for preferred preservation: %v", err)
	}
}

func TestCapabilityResolver_EmptyBuildPlan(t *testing.T) {
	capstage.ResetCache()

	plan := pipeline.BuildPlan{Units: nil}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(nil, nil)
	result, err := stage.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph := result.(pipeline.CapabilityGraph)
	if len(graph.Units) != 0 {
		t.Errorf("expected 0 units, got %d", len(graph.Units))
	}
}

func TestCapabilityResolver_CandidatesPopulated(t *testing.T) {
	capstage.ResetCache()

	objects := map[string]pipeline.NormalizedObject{
		"rule-1": {Meta: model.ObjectMeta{ID: "rule-1", Kind: model.KindRule}},
	}

	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate:    makeCoord(build.TargetClaude, build.ProfileLocalDev),
				ActiveObjects: []string{"rule-1"},
			},
		},
	}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(objects, nil)
	result, _ := stage.Execute(ctx, plan)

	graph := result.(pipeline.CapabilityGraph)
	uc := graph.Units[".ai-build/claude/local-dev"]

	if len(uc.Candidates) == 0 {
		t.Error("expected candidates to be populated")
	}
	if _, ok := uc.Candidates["rules.scopedRules"]; !ok {
		t.Error("expected candidates for rules.scopedRules")
	}
}

func TestCapabilityResolver_InvalidInput_Error(t *testing.T) {
	stage := capstage.New(nil, nil)
	_, err := stage.Execute(context.Background(), "not a build plan")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestCapabilityResolver_PointerInput(t *testing.T) {
	capstage.ResetCache()

	plan := &pipeline.BuildPlan{Units: nil}
	stage := capstage.New(nil, nil)
	_, err := stage.Execute(contextWithProfile(build.ProfileLocalDev), plan)
	if err != nil {
		t.Fatalf("unexpected error with pointer input: %v", err)
	}
}

func TestCapabilityResolver_Descriptor(t *testing.T) {
	s := capstage.New(nil, nil)
	d := s.Descriptor()

	if d.Name != "capability-resolver" {
		t.Errorf("expected name 'capability-resolver', got %q", d.Name)
	}
	if d.Phase != pipeline.PhaseCapability {
		t.Errorf("expected phase PhaseCapability, got %d", d.Phase)
	}
	if d.Order != 10 {
		t.Errorf("expected order 10, got %d", d.Order)
	}
}

func TestCapabilityResolver_Factory(t *testing.T) {
	factory := capstage.Factory(nil, nil)
	s, err := factory()
	if err != nil {
		t.Fatalf("unexpected factory error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil stage")
	}
}

func TestCapabilityResolver_DefaultPreservation(t *testing.T) {
	capstage.ResetCache()

	// Object with no explicit preservation → defaults to preferred.
	objects := map[string]pipeline.NormalizedObject{
		"skill-default": {
			Meta: model.ObjectMeta{
				ID:   "skill-default",
				Kind: model.KindSkill,
				// No Preservation set.
			},
		},
	}

	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate:    makeCoord(build.TargetCursor, build.ProfileLocalDev),
				ActiveObjects: []string{"skill-default"},
			},
		},
	}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(objects, nil)
	_, err := stage.Execute(ctx, plan)
	// Should NOT error because default is "preferred", not "required".
	if err != nil {
		t.Fatalf("unexpected error for default preservation: %v", err)
	}
}

func TestCapabilityResolver_CopilotAllNative(t *testing.T) {
	capstage.ResetCache()

	// All kinds should resolve natively for copilot.
	objects := map[string]pipeline.NormalizedObject{
		"instr": {Meta: model.ObjectMeta{ID: "instr", Kind: model.KindInstruction}},
		"rule":  {Meta: model.ObjectMeta{ID: "rule", Kind: model.KindRule}},
		"skill": {Meta: model.ObjectMeta{ID: "skill", Kind: model.KindSkill}},
		"agent": {Meta: model.ObjectMeta{ID: "agent", Kind: model.KindAgent}},
		"hook":  {Meta: model.ObjectMeta{ID: "hook", Kind: model.KindHook}},
		"cmd":   {Meta: model.ObjectMeta{ID: "cmd", Kind: model.KindCommand}},
	}

	ids := make([]string, 0, len(objects))
	for id := range objects {
		ids = append(ids, id)
	}

	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate:    makeCoord(build.TargetCopilot, build.ProfileLocalDev),
				ActiveObjects: ids,
			},
		},
	}

	ctx := contextWithProfile(build.ProfileLocalDev)
	stage := capstage.New(objects, nil)
	result, err := stage.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph := result.(pipeline.CapabilityGraph)
	uc := graph.Units[".ai-build/copilot/local-dev"]

	if len(uc.Unsatisfied) != 0 {
		t.Errorf("expected no unsatisfied capabilities for copilot, got %v", uc.Unsatisfied)
	}
}

// --- Helpers ---

func makeCoord(target build.Target, profile build.Profile) build.BuildCoordinate {
	return build.BuildCoordinate{
		Unit: build.BuildUnit{
			Target:  target,
			Profile: profile,
		},
		OutputDir: ".ai-build/" + string(target) + "/" + string(profile),
	}
}

func contextWithProfile(profile build.Profile) context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{Profile: profile},
		Report: &pipeline.BuildReport{},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func assertSurface(t *testing.T, reg *domcap.CapabilityRegistry, key string, expected domcap.SupportLevel) {
	t.Helper()
	got, ok := reg.Surfaces[key]
	if !ok {
		t.Errorf("missing surface %q", key)
		return
	}
	if got != expected {
		t.Errorf("surface %q: expected %s, got %s", key, expected, got)
	}
}

func assertContains(t *testing.T, caps []string, expected string) {
	t.Helper()
	for _, c := range caps {
		if c == expected {
			return
		}
	}
	t.Errorf("expected %q in %v", expected, caps)
}
