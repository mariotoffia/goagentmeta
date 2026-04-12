package capability_test

import (
	"testing"

	capstage "github.com/mariotoffia/goagentmeta/internal/adapter/stage/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	domcap "github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ---------------------------------------------------------------------------
// Emulated Support Level Tests
// ---------------------------------------------------------------------------

func TestSelectProvider_EmulatedSupport(t *testing.T) {
	capstage.ResetCache()

	// Create a synthetic registry with an emulated surface.
	reg := &domcap.CapabilityRegistry{
		Target: "test",
		Surfaces: map[string]domcap.SupportLevel{
			"skills.bundles": domcap.SupportEmulated,
		},
	}

	candidate := capstage.SelectProvider("skills.bundles", reg, nil, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected compatible provider for emulated support")
	}
	if candidate.Priority != 5 {
		t.Errorf("expected priority 5 for emulated, got %d", candidate.Priority)
	}
	if candidate.Reason != "emulated approximation" {
		t.Errorf("expected reason 'emulated approximation', got %q", candidate.Reason)
	}
}

func TestSelectProvider_EmulatedFallsBackAfterPlugins(t *testing.T) {
	capstage.ResetCache()

	// Registry with emulated support.
	reg := &domcap.CapabilityRegistry{
		Target: "test",
		Surfaces: map[string]domcap.SupportLevel{
			"skills.bundles": domcap.SupportEmulated,
		},
	}

	// A plugin providing the same capability should win over emulated.
	plugins := []domcap.Provider{
		{ID: "skill-plugin", Type: "plugin", Capabilities: []string{"skills.bundles"}},
	}

	candidate := capstage.SelectProvider("skills.bundles", reg, plugins, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected compatible provider")
	}
	if candidate.Provider.ID != "skill-plugin" {
		t.Errorf("expected plugin to win over emulated, got %q", candidate.Provider.ID)
	}
	if candidate.Priority != 2 {
		t.Errorf("expected priority 2 for plugin, got %d", candidate.Priority)
	}
}

func TestSelectProvider_EmulatedOverMCPUnderEnterpriseLocked(t *testing.T) {
	capstage.ResetCache()

	// Registry with emulated support.
	reg := &domcap.CapabilityRegistry{
		Target: "test",
		Surfaces: map[string]domcap.SupportLevel{
			"skills.bundles": domcap.SupportEmulated,
		},
	}

	// Only MCP plugin available — blocked by enterprise-locked.
	plugins := []domcap.Provider{
		{ID: "mcp-provider", Type: "mcp", Capabilities: []string{"skills.bundles"}},
	}

	candidate := capstage.SelectProvider("skills.bundles", reg, plugins, build.ProfileEnterpriseLocked)
	if !candidate.Compatible {
		t.Fatal("expected emulated fallback when MCP is blocked")
	}
	if candidate.Reason != "emulated approximation" {
		t.Errorf("expected emulated fallback, got reason %q", candidate.Reason)
	}
}

// ---------------------------------------------------------------------------
// Agent with Handoffs Capability Tests
// ---------------------------------------------------------------------------

func TestCapabilityResolver_AgentHandoffsSkippedForClaude(t *testing.T) {
	capstage.ResetCache()

	// Agent with handoffs targeting Claude. Claude has agents.handoffs: skipped.
	// But RequiredCapabilities for KindAgent only returns subagents + toolPolicies,
	// not handoffs. This tests that the resolver resolves those surfaces correctly.
	objects := map[string]pipeline.NormalizedObject{
		"my-agent": {
			Meta: model.ObjectMeta{
				ID:           "my-agent",
				Kind:         model.KindAgent,
				Preservation: model.PreservationPreferred,
			},
		},
	}

	s := capstage.New(objects, nil)
	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetClaude, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/claude/local-dev",
				},
				ActiveObjects: []string{"my-agent"},
			},
		},
	}

	result, err := s.Execute(contextWithProfile(build.ProfileLocalDev), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph := result.(pipeline.CapabilityGraph)
	uc := graph.Units[".ai-build/claude/local-dev"]

	// agents.subagents and agents.toolPolicies should be resolved (native).
	for _, cap := range []string{"agents.subagents", "agents.toolPolicies"} {
		if _, ok := uc.Resolved[cap]; !ok {
			t.Errorf("expected %q to be resolved for claude", cap)
		}
	}

	// agents.handoffs should NOT be in Required (not checked for KindAgent).
	for _, cap := range uc.Required {
		if cap == "agents.handoffs" {
			t.Error("agents.handoffs should not be in Required for a basic agent")
		}
	}
}

// ---------------------------------------------------------------------------
// RequiredCapabilities Coverage Tests
// ---------------------------------------------------------------------------

func TestRequiredCapabilities_MCP(t *testing.T) {
	// MCP surfaces come from plugins (plugins.installablePackages + capabilityProviders),
	// not from a separate "mcp" kind.
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: model.KindPlugin})
	expected := map[string]bool{
		"plugins.installablePackages": true,
		"plugins.capabilityProviders": true,
	}
	for _, cap := range caps {
		delete(expected, cap)
	}
	for missing := range expected {
		t.Errorf("missing expected capability %q for KindPlugin", missing)
	}
}

func TestRequiredCapabilities_UnknownKind(t *testing.T) {
	caps := capstage.RequiredCapabilities(model.ObjectMeta{Kind: "unknown"})
	if len(caps) != 0 {
		t.Errorf("expected no capabilities for unknown kind, got %v", caps)
	}
}

// ---------------------------------------------------------------------------
// Preservation Priority Tests
// ---------------------------------------------------------------------------

func TestCapabilityResolver_MultipleObjectsDifferentPreservation(t *testing.T) {
	capstage.ResetCache()

	// Two rules: one required, one optional. Both need rules.scopedRules.
	// The most restrictive (required) should win.
	objects := map[string]pipeline.NormalizedObject{
		"rule-optional": {
			Meta: model.ObjectMeta{
				ID:           "rule-optional",
				Kind:         model.KindRule,
				Preservation: model.PreservationOptional,
			},
		},
		"rule-required": {
			Meta: model.ObjectMeta{
				ID:           "rule-required",
				Kind:         model.KindRule,
				Preservation: model.PreservationRequired,
			},
		},
	}

	// Use cursor where rules.scopedRules is native — both should resolve.
	s := capstage.New(objects, nil)
	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetCursor, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/cursor/local-dev",
				},
				ActiveObjects: []string{"rule-optional", "rule-required"},
			},
		},
	}

	result, err := s.Execute(contextWithProfile(build.ProfileLocalDev), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph := result.(pipeline.CapabilityGraph)
	uc := graph.Units[".ai-build/cursor/local-dev"]

	if _, ok := uc.Resolved["rules.scopedRules"]; !ok {
		t.Error("expected rules.scopedRules to be resolved")
	}
}

func TestCapabilityResolver_RequiredSkillSkippedCursor_Errors(t *testing.T) {
	capstage.ResetCache()

	// Required command on cursor (commands.explicitEntryPoints = skipped) → should error.
	objects := map[string]pipeline.NormalizedObject{
		"my-cmd": {
			Meta: model.ObjectMeta{
				ID:           "my-cmd",
				Kind:         model.KindCommand,
				Preservation: model.PreservationRequired,
			},
		},
	}

	s := capstage.New(objects, nil)
	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetCursor, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/cursor/local-dev",
				},
				ActiveObjects: []string{"my-cmd"},
			},
		},
	}

	_, err := s.Execute(contextWithProfile(build.ProfileLocalDev), plan)
	if err == nil {
		t.Fatal("expected error for required command on cursor (commands.explicitEntryPoints is skipped)")
	}
}

func TestCapabilityResolver_OptionalSkillSkippedCursor_NoError(t *testing.T) {
	capstage.ResetCache()

	// Optional command on cursor → no error, just unsatisfied.
	objects := map[string]pipeline.NormalizedObject{
		"my-cmd": {
			Meta: model.ObjectMeta{
				ID:           "my-cmd",
				Kind:         model.KindCommand,
				Preservation: model.PreservationOptional,
			},
		},
	}

	s := capstage.New(objects, nil)
	plan := pipeline.BuildPlan{
		Units: []pipeline.BuildPlanUnit{
			{
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetCursor, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/cursor/local-dev",
				},
				ActiveObjects: []string{"my-cmd"},
			},
		},
	}

	result, err := s.Execute(contextWithProfile(build.ProfileLocalDev), plan)
	if err != nil {
		t.Fatalf("expected no error for optional command on cursor, got: %v", err)
	}

	graph := result.(pipeline.CapabilityGraph)
	uc := graph.Units[".ai-build/cursor/local-dev"]

	if len(uc.Unsatisfied) == 0 {
		t.Error("expected unsatisfied capabilities for optional command on cursor")
	}
}

// ---------------------------------------------------------------------------
// Script Provider Tests
// ---------------------------------------------------------------------------

func TestSelectProvider_ScriptProvider(t *testing.T) {
	capstage.ResetCache()
	reg, _ := capstage.LoadRegistry("cursor")

	plugins := []domcap.Provider{
		{ID: "script-fallback", Type: "script", Capabilities: []string{"agents.handoffs"}},
	}

	candidate := capstage.SelectProvider("agents.handoffs", reg, plugins, build.ProfileLocalDev)
	if !candidate.Compatible {
		t.Fatal("expected script provider to be compatible")
	}
	if candidate.Priority != 4 {
		t.Errorf("expected priority 4 for script, got %d", candidate.Priority)
	}
}
