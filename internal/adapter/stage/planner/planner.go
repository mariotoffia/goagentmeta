package planner

import (
	"context"
	"fmt"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion: *Stage satisfies the stage.Stage port interface.
var _ stage.Stage = (*Stage)(nil)

// Stage implements the PhasePlan pipeline stage. It expands a SemanticGraph
// into a BuildPlan by cross-multiplying normalized objects with selected
// targets and profiles, applying AppliesTo filters and scope matching.
type Stage struct{}

// New creates a new planner Stage.
func New() *Stage {
	return &Stage{}
}

// Descriptor returns the stage metadata for pipeline registration.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "planner",
		Phase: pipeline.PhasePlan,
		Order: 10,
	}
}

// Execute transforms a SemanticGraph into a BuildPlan.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	graph, ok := input.(pipeline.SemanticGraph)
	if !ok {
		graphPtr, ok := input.(*pipeline.SemanticGraph)
		if !ok || graphPtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrPlanning,
				fmt.Sprintf("expected pipeline.SemanticGraph or *pipeline.SemanticGraph, got %T", input),
				"planner",
			)
		}
		graph = *graphPtr
	}

	targets, profile := resolveTargetsAndProfile(ctx)

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "PLAN_START",
		Message:  fmt.Sprintf("planning %d objects for %d target(s), profile=%s", len(graph.Objects), len(targets), profile),
		Phase:    pipeline.PhasePlan,
	})

	var units []pipeline.BuildPlanUnit

	// Cross-multiply targets × profile to produce units.
	for _, target := range targets {
		unit := buildUnit(graph, target, profile)
		units = append(units, unit)
	}

	// Deterministic ordering: sort by target, then profile.
	sort.Slice(units, func(i, j int) bool {
		ti := string(units[i].Coordinate.Unit.Target)
		tj := string(units[j].Coordinate.Unit.Target)
		if ti != tj {
			return ti < tj
		}
		return string(units[i].Coordinate.Unit.Profile) < string(units[j].Coordinate.Unit.Profile)
	})

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "PLAN_COMPLETE",
		Message:  fmt.Sprintf("planned %d build unit(s)", len(units)),
		Phase:    pipeline.PhasePlan,
	})

	return pipeline.BuildPlan{Units: units}, nil
}

// Factory returns a StageFactory function for use with pipeline registration.
func Factory() stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(), nil
	}
}

// buildUnit creates a single BuildPlanUnit for the given target/profile by
// filtering objects from the SemanticGraph.
func buildUnit(graph pipeline.SemanticGraph, target build.Target, profile build.Profile) pipeline.BuildPlanUnit {
	// Collect and sort object IDs for deterministic iteration.
	ids := make([]string, 0, len(graph.Objects))
	for id := range graph.Objects {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var active []string
	for _, id := range ids {
		obj := graph.Objects[id]
		if !TargetFilter(obj.Meta, target) {
			continue
		}
		if !ProfileFilter(obj.Meta, profile) {
			continue
		}
		active = append(active, id)
	}

	coord := build.BuildCoordinate{
		Unit: build.BuildUnit{
			Target:  target,
			Profile: profile,
		},
		OutputDir: fmt.Sprintf(".ai-build/%s/%s", target, profile),
	}

	return pipeline.BuildPlanUnit{
		Coordinate:    coord,
		ActiveObjects: active,
	}
}

// resolveTargetsAndProfile extracts targets and profile from the CompilerContext.
// Falls back to all targets and local-dev profile.
func resolveTargetsAndProfile(ctx context.Context) ([]build.Target, build.Profile) {
	cc := compiler.CompilerFromContext(ctx)

	targets := build.AllTargets()
	profile := build.ProfileLocalDev

	if cc != nil && cc.Config != nil {
		if len(cc.Config.Targets) > 0 {
			targets = cc.Config.Targets
		}
		if cc.Config.Profile != "" {
			profile = cc.Config.Profile
		}
	}

	return targets, profile
}

// emitDiagnostic sends a diagnostic through the CompilerContext if available.
func emitDiagnostic(ctx context.Context, d pipeline.Diagnostic) {
	cc := compiler.CompilerFromContext(ctx)
	if cc == nil {
		return
	}
	if cc.Report != nil {
		cc.Report.Diagnostics = append(cc.Report.Diagnostics, d)
	}
	if cc.Config != nil && cc.Config.DiagnosticSink != nil {
		cc.Config.DiagnosticSink.Emit(ctx, d)
	}
}
