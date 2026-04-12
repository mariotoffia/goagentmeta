package planner_test

import (
	"context"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/planner"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// --- Filter Tests ---

func TestTargetFilter_EmptyAppliesTo_MatchesAll(t *testing.T) {
	meta := model.ObjectMeta{AppliesTo: model.AppliesTo{}}

	for _, target := range build.AllTargets() {
		if !planner.TargetFilter(meta, target) {
			t.Errorf("expected empty AppliesTo to match target %s", target)
		}
	}
}

func TestTargetFilter_SpecificTarget_MatchesOnlyThat(t *testing.T) {
	meta := model.ObjectMeta{
		AppliesTo: model.AppliesTo{Targets: []string{"claude"}},
	}

	if !planner.TargetFilter(meta, build.TargetClaude) {
		t.Error("expected claude to match")
	}

	for _, target := range []build.Target{build.TargetCursor, build.TargetCopilot, build.TargetCodex} {
		if planner.TargetFilter(meta, target) {
			t.Errorf("expected %s not to match", target)
		}
	}
}

func TestTargetFilter_Wildcard_MatchesAll(t *testing.T) {
	meta := model.ObjectMeta{
		AppliesTo: model.AppliesTo{Targets: []string{"*"}},
	}

	for _, target := range build.AllTargets() {
		if !planner.TargetFilter(meta, target) {
			t.Errorf("expected wildcard to match target %s", target)
		}
	}
}

func TestTargetFilter_OverrideDisabled_Excludes(t *testing.T) {
	disabled := false
	meta := model.ObjectMeta{
		AppliesTo:       model.AppliesTo{},
		TargetOverrides: map[string]model.TargetOverride{"claude": {Enabled: &disabled}},
	}

	if planner.TargetFilter(meta, build.TargetClaude) {
		t.Error("expected override Enabled=false to exclude claude")
	}
	if !planner.TargetFilter(meta, build.TargetCursor) {
		t.Error("expected cursor to still match")
	}
}

func TestProfileFilter_EmptyAppliesTo_MatchesAll(t *testing.T) {
	meta := model.ObjectMeta{AppliesTo: model.AppliesTo{}}

	for _, p := range []build.Profile{build.ProfileLocalDev, build.ProfileCI, build.ProfileEnterpriseLocked, build.ProfileOSSPublic} {
		if !planner.ProfileFilter(meta, p) {
			t.Errorf("expected empty AppliesTo to match profile %s", p)
		}
	}
}

func TestProfileFilter_SpecificProfile_MatchesOnlyThat(t *testing.T) {
	meta := model.ObjectMeta{
		AppliesTo: model.AppliesTo{Profiles: []string{"ci"}},
	}

	if !planner.ProfileFilter(meta, build.ProfileCI) {
		t.Error("expected ci to match")
	}

	if planner.ProfileFilter(meta, build.ProfileLocalDev) {
		t.Error("expected local-dev not to match")
	}
}

func TestProfileFilter_Wildcard_MatchesAll(t *testing.T) {
	meta := model.ObjectMeta{
		AppliesTo: model.AppliesTo{Profiles: []string{"*"}},
	}

	for _, p := range []build.Profile{build.ProfileLocalDev, build.ProfileCI, build.ProfileEnterpriseLocked} {
		if !planner.ProfileFilter(meta, p) {
			t.Errorf("expected wildcard to match profile %s", p)
		}
	}
}

// --- Scope Matching Tests ---

func TestScopeMatch_EmptyScope_RepoWide(t *testing.T) {
	scope := model.Scope{}
	if !planner.ScopeMatch(scope, nil) {
		t.Error("expected empty scope to match (repo-wide)")
	}
	if !planner.ScopeMatch(scope, []string{"services/api"}) {
		t.Error("expected empty scope to match any build paths")
	}
}

func TestScopeMatch_PathGlob(t *testing.T) {
	scope := model.Scope{Paths: []string{"services/**"}}

	if !planner.ScopeMatch(scope, []string{"services/api"}) {
		t.Error("expected services/** to match services/api")
	}
	if planner.ScopeMatch(scope, []string{"lib/utils"}) {
		t.Error("expected services/** not to match lib/utils")
	}
}

func TestScopeMatch_RootScope_ExcludedByPaths(t *testing.T) {
	scope := model.Scope{Paths: []string{"services/**"}}
	if planner.ScopeMatch(scope, []string{}) {
		t.Error("expected scoped object to not match empty build paths (root)")
	}
}

func TestFileTypeMatch_Empty_MatchesAll(t *testing.T) {
	scope := model.Scope{}
	if !planner.FileTypeMatch(scope, nil) {
		t.Error("expected empty FileTypes to match all")
	}
}

func TestFileTypeMatch_Specific(t *testing.T) {
	scope := model.Scope{FileTypes: []string{".go", ".ts"}}

	if !planner.FileTypeMatch(scope, []string{".go"}) {
		t.Error("expected .go to match")
	}
	if planner.FileTypeMatch(scope, []string{".py"}) {
		t.Error("expected .py not to match")
	}
}

func TestFileTypeMatch_NormalizeDot(t *testing.T) {
	scope := model.Scope{FileTypes: []string{"go"}}
	if !planner.FileTypeMatch(scope, []string{".go"}) {
		t.Error("expected 'go' (without dot) to match '.go'")
	}
}

func TestLabelMatch_Empty_MatchesAll(t *testing.T) {
	scope := model.Scope{}
	if !planner.LabelMatch(scope, nil) {
		t.Error("expected empty labels to match all")
	}
}

func TestLabelMatch_Intersection(t *testing.T) {
	scope := model.Scope{Labels: []string{"backend", "api"}}

	if !planner.LabelMatch(scope, []string{"api", "web"}) {
		t.Error("expected intersection to match")
	}
	if planner.LabelMatch(scope, []string{"frontend", "web"}) {
		t.Error("expected no intersection to not match")
	}
}

// --- BuildPlan Structure Tests ---

func TestPlanner_FourTargets_OneProfile_FourUnits(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"obj-1": {
				Meta: model.ObjectMeta{ID: "obj-1", Kind: model.KindInstruction},
			},
		},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets(build.AllTargets(), build.ProfileLocalDev)

	stage := planner.New()
	result, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan, ok := result.(pipeline.BuildPlan)
	if !ok {
		t.Fatalf("expected pipeline.BuildPlan, got %T", result)
	}

	if len(plan.Units) != 4 {
		t.Fatalf("expected 4 units, got %d", len(plan.Units))
	}

	// Verify deterministic ordering (alphabetical by target).
	expectedOrder := []build.Target{build.TargetClaude, build.TargetCodex, build.TargetCopilot, build.TargetCursor}
	for i, unit := range plan.Units {
		if unit.Coordinate.Unit.Target != expectedOrder[i] {
			t.Errorf("unit[%d]: expected target %s, got %s", i, expectedOrder[i], unit.Coordinate.Unit.Target)
		}
	}
}

func TestPlanner_OutputDirFormat(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects:           map[string]pipeline.NormalizedObject{},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets([]build.Target{build.TargetClaude}, build.ProfileCI)

	stage := planner.New()
	result, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.BuildPlan)
	if len(plan.Units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(plan.Units))
	}

	expected := ".ai-build/claude/ci"
	if plan.Units[0].Coordinate.OutputDir != expected {
		t.Errorf("expected OutputDir %q, got %q", expected, plan.Units[0].Coordinate.OutputDir)
	}
}

func TestPlanner_TargetFiltering_ClaudeOnly(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"claude-only": {
				Meta: model.ObjectMeta{
					ID:        "claude-only",
					Kind:      model.KindSkill,
					AppliesTo: model.AppliesTo{Targets: []string{"claude"}},
				},
			},
			"universal": {
				Meta: model.ObjectMeta{
					ID:   "universal",
					Kind: model.KindInstruction,
				},
			},
		},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets([]build.Target{build.TargetClaude, build.TargetCursor}, build.ProfileLocalDev)

	stage := planner.New()
	result, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.BuildPlan)
	if len(plan.Units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(plan.Units))
	}

	// Claude unit should have both objects.
	claudeUnit := findUnit(plan, build.TargetClaude)
	if claudeUnit == nil {
		t.Fatal("missing claude unit")
	}
	if len(claudeUnit.ActiveObjects) != 2 {
		t.Errorf("claude unit: expected 2 active objects, got %d", len(claudeUnit.ActiveObjects))
	}

	// Cursor unit should only have the universal object.
	cursorUnit := findUnit(plan, build.TargetCursor)
	if cursorUnit == nil {
		t.Fatal("missing cursor unit")
	}
	if len(cursorUnit.ActiveObjects) != 1 {
		t.Errorf("cursor unit: expected 1 active object, got %d", len(cursorUnit.ActiveObjects))
	}
	if cursorUnit.ActiveObjects[0] != "universal" {
		t.Errorf("cursor unit: expected 'universal', got %q", cursorUnit.ActiveObjects[0])
	}
}

func TestPlanner_ProfileFiltering_CIOnly(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"ci-only": {
				Meta: model.ObjectMeta{
					ID:        "ci-only",
					Kind:      model.KindHook,
					AppliesTo: model.AppliesTo{Profiles: []string{"ci"}},
				},
			},
			"always": {
				Meta: model.ObjectMeta{ID: "always", Kind: model.KindRule},
			},
		},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets([]build.Target{build.TargetClaude}, build.ProfileLocalDev)

	stage := planner.New()
	result, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.BuildPlan)
	unit := plan.Units[0]

	if len(unit.ActiveObjects) != 1 {
		t.Fatalf("expected 1 active object for local-dev, got %d", len(unit.ActiveObjects))
	}
	if unit.ActiveObjects[0] != "always" {
		t.Errorf("expected 'always', got %q", unit.ActiveObjects[0])
	}
}

// --- Edge Case Tests ---

func TestPlanner_NoObjectsMatchFilter_EmptyUnits(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"codex-only": {
				Meta: model.ObjectMeta{
					ID:        "codex-only",
					AppliesTo: model.AppliesTo{Targets: []string{"codex"}},
				},
			},
		},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets([]build.Target{build.TargetClaude}, build.ProfileLocalDev)

	stage := planner.New()
	result, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.BuildPlan)
	if len(plan.Units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(plan.Units))
	}
	if len(plan.Units[0].ActiveObjects) != 0 {
		t.Errorf("expected 0 active objects, got %d", len(plan.Units[0].ActiveObjects))
	}
}

func TestPlanner_SingleObject_SingleTarget_MinimalPlan(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"single": {
				Meta: model.ObjectMeta{ID: "single", Kind: model.KindInstruction},
			},
		},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets([]build.Target{build.TargetClaude}, build.ProfileLocalDev)

	stage := planner.New()
	result, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.BuildPlan)
	if len(plan.Units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(plan.Units))
	}
	if len(plan.Units[0].ActiveObjects) != 1 {
		t.Errorf("expected 1 active object, got %d", len(plan.Units[0].ActiveObjects))
	}
}

func TestPlanner_EmptySemanticGraph_EmptyPlan(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects:           make(map[string]pipeline.NormalizedObject),
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets(build.AllTargets(), build.ProfileLocalDev)

	stage := planner.New()
	result, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan := result.(pipeline.BuildPlan)
	if len(plan.Units) != 4 {
		t.Fatalf("expected 4 units (one per target), got %d", len(plan.Units))
	}
	for _, unit := range plan.Units {
		if len(unit.ActiveObjects) != 0 {
			t.Errorf("expected 0 active objects, got %d", len(unit.ActiveObjects))
		}
	}
}

func TestPlanner_InvalidInput_ReturnsError(t *testing.T) {
	stage := planner.New()
	_, err := stage.Execute(context.Background(), "not a semantic graph")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestPlanner_PointerInput_Works(t *testing.T) {
	graph := &pipeline.SemanticGraph{
		Objects:           map[string]pipeline.NormalizedObject{},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets([]build.Target{build.TargetClaude}, build.ProfileLocalDev)

	stage := planner.New()
	_, err := stage.Execute(ctx, graph)
	if err != nil {
		t.Fatalf("unexpected error with pointer input: %v", err)
	}
}

func TestPlanner_DeterministicOutput(t *testing.T) {
	graph := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"z-obj": {Meta: model.ObjectMeta{ID: "z-obj", Kind: model.KindRule}},
			"a-obj": {Meta: model.ObjectMeta{ID: "a-obj", Kind: model.KindRule}},
			"m-obj": {Meta: model.ObjectMeta{ID: "m-obj", Kind: model.KindRule}},
		},
		InheritanceChains: make(map[string][]string),
		ScopeIndex:        make(map[string][]string),
	}

	ctx := contextWithTargets(build.AllTargets(), build.ProfileLocalDev)
	stage := planner.New()

	// Run multiple times to ensure determinism.
	for i := 0; i < 10; i++ {
		result, err := stage.Execute(ctx, graph)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		plan := result.(pipeline.BuildPlan)
		for _, unit := range plan.Units {
			if len(unit.ActiveObjects) != 3 {
				t.Fatalf("expected 3 objects, got %d", len(unit.ActiveObjects))
			}
			if unit.ActiveObjects[0] != "a-obj" || unit.ActiveObjects[1] != "m-obj" || unit.ActiveObjects[2] != "z-obj" {
				t.Errorf("iteration %d: objects not in sorted order: %v", i, unit.ActiveObjects)
			}
		}
	}
}

func TestPlanner_Descriptor(t *testing.T) {
	s := planner.New()
	d := s.Descriptor()

	if d.Name != "planner" {
		t.Errorf("expected name 'planner', got %q", d.Name)
	}
	if d.Phase != pipeline.PhasePlan {
		t.Errorf("expected phase PhasePlan, got %d", d.Phase)
	}
	if d.Order != 10 {
		t.Errorf("expected order 10, got %d", d.Order)
	}
}

func TestPlanner_Factory(t *testing.T) {
	factory := planner.Factory()
	s, err := factory()
	if err != nil {
		t.Fatalf("unexpected factory error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil stage")
	}
}

// --- Helpers ---

func contextWithTargets(targets []build.Target, profile build.Profile) context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			Targets: targets,
			Profile: profile,
		},
		Report: &pipeline.BuildReport{},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func findUnit(plan pipeline.BuildPlan, target build.Target) *pipeline.BuildPlanUnit {
	for i := range plan.Units {
		if plan.Units[i].Coordinate.Unit.Target == target {
			return &plan.Units[i]
		}
	}
	return nil
}
