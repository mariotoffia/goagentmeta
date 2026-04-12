package capability

import (
	"context"
	"fmt"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	domcap "github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion: *Stage satisfies the stage.Stage port interface.
var _ stage.Stage = (*Stage)(nil)

// Stage implements the PhaseCapability pipeline stage. It resolves required
// capabilities for each BuildPlanUnit to concrete providers, producing a
// CapabilityGraph.
type Stage struct {
	// plugins are externally configured providers available for capability
	// resolution (e.g., from manifest or plugin registry).
	plugins []domcap.Provider

	// objects is a reference to normalized objects from the SemanticGraph,
	// injected at construction or looked up from context.
	objects map[string]pipeline.NormalizedObject
}

// New creates a new capability resolver Stage.
func New(objects map[string]pipeline.NormalizedObject, plugins []domcap.Provider) *Stage {
	return &Stage{
		objects: objects,
		plugins: plugins,
	}
}

// Descriptor returns the stage metadata for pipeline registration.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "capability-resolver",
		Phase: pipeline.PhaseCapability,
		Order: 10,
	}
}

// Execute transforms a BuildPlan into a CapabilityGraph.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	plan, ok := input.(pipeline.BuildPlan)
	if !ok {
		planPtr, ok := input.(*pipeline.BuildPlan)
		if !ok || planPtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrCapability,
				fmt.Sprintf("expected pipeline.BuildPlan or *pipeline.BuildPlan, got %T", input),
				"capability-resolver",
			)
		}
		plan = *planPtr
	}

	profile := resolveProfile(ctx)

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "CAPABILITY_START",
		Message:  fmt.Sprintf("resolving capabilities for %d build unit(s)", len(plan.Units)),
		Phase:    pipeline.PhaseCapability,
	})

	graph := pipeline.CapabilityGraph{
		Units: make(map[string]pipeline.UnitCapabilities, len(plan.Units)),
	}

	var capErrors []string

	for _, unit := range plan.Units {
		uc, errs := s.resolveUnit(ctx, unit, profile)
		graph.Units[unit.Coordinate.OutputDir] = uc
		capErrors = append(capErrors, errs...)
	}

	// Emit errors for required but unsatisfied capabilities.
	for _, errMsg := range capErrors {
		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "error",
			Code:     "CAPABILITY",
			Message:  errMsg,
			Phase:    pipeline.PhaseCapability,
		})
	}

	if len(capErrors) > 0 {
		return graph, pipeline.NewCompilerError(
			pipeline.ErrCapability,
			fmt.Sprintf("%d required capability(ies) unsatisfied", len(capErrors)),
			"capability-resolver",
		)
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "CAPABILITY_COMPLETE",
		Message:  fmt.Sprintf("resolved capabilities for %d unit(s)", len(graph.Units)),
		Phase:    pipeline.PhaseCapability,
	})

	return graph, nil
}

// Factory returns a StageFactory function for use with pipeline registration.
func Factory(objects map[string]pipeline.NormalizedObject, plugins []domcap.Provider) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(objects, plugins), nil
	}
}

// resolveUnit resolves capabilities for a single build plan unit.
// Returns the UnitCapabilities and any error messages for required but
// unsatisfied capabilities.
func (s *Stage) resolveUnit(
	ctx context.Context,
	unit pipeline.BuildPlanUnit,
	profile build.Profile,
) (pipeline.UnitCapabilities, []string) {
	target := string(unit.Coordinate.Unit.Target)

	registry, err := LoadRegistry(target)
	if err != nil {
		return pipeline.UnitCapabilities{
			Coordinate:  unit.Coordinate,
			Unsatisfied: []string{"*"},
		}, []string{fmt.Sprintf("failed to load registry for %s: %v", target, err)}
	}

	// Collect all required capabilities from active objects.
	capSet := make(map[string]model.Preservation) // surface → worst preservation
	for _, objID := range unit.ActiveObjects {
		obj, ok := s.objects[objID]
		if !ok {
			continue
		}

		caps := RequiredCapabilities(obj.Meta)
		for _, cap := range caps {
			pres := obj.Meta.Preservation
			if pres == "" {
				pres = model.PreservationPreferred
			}
			// Keep the most restrictive preservation level.
			if existing, ok := capSet[cap]; !ok || preservationPriority(pres) > preservationPriority(existing) {
				capSet[cap] = pres
			}
		}
	}

	// Sort capability keys for deterministic processing.
	capKeys := make([]string, 0, len(capSet))
	for k := range capSet {
		capKeys = append(capKeys, k)
	}
	sort.Strings(capKeys)

	uc := pipeline.UnitCapabilities{
		Coordinate: unit.Coordinate,
		Required:   capKeys,
		Resolved:   make(map[string]domcap.Provider),
		Candidates: make(map[string][]domcap.ProviderCandidate),
	}

	var errs []string

	for _, capSurface := range capKeys {
		preservation := capSet[capSurface]
		candidate := SelectProvider(capSurface, registry, s.plugins, profile)
		uc.Candidates[capSurface] = []domcap.ProviderCandidate{candidate}

		if candidate.Compatible {
			uc.Resolved[capSurface] = candidate.Provider
		} else {
			uc.Unsatisfied = append(uc.Unsatisfied, capSurface)

			switch preservation {
			case model.PreservationRequired:
				errs = append(errs, fmt.Sprintf(
					"unit %s: required capability %q unsatisfied for target %s",
					unit.Coordinate.OutputDir, capSurface, target,
				))
			case model.PreservationPreferred:
				emitDiagnostic(ctx, pipeline.Diagnostic{
					Severity: "warning",
					Code:     "CAPABILITY_PREFERRED_SKIP",
					Message:  fmt.Sprintf("preferred capability %q unsatisfied for %s, skipping", capSurface, target),
					Phase:    pipeline.PhaseCapability,
				})
			case model.PreservationOptional:
				emitDiagnostic(ctx, pipeline.Diagnostic{
					Severity: "info",
					Code:     "CAPABILITY_OPTIONAL_SKIP",
					Message:  fmt.Sprintf("optional capability %q unsatisfied for %s, skipping", capSurface, target),
					Phase:    pipeline.PhaseCapability,
				})
			}
		}
	}

	return uc, errs
}

// resolveProfile extracts the build profile from the CompilerContext.
func resolveProfile(ctx context.Context) build.Profile {
	cc := compiler.CompilerFromContext(ctx)
	if cc != nil && cc.Config != nil && cc.Config.Profile != "" {
		return cc.Config.Profile
	}
	return build.ProfileLocalDev
}

// preservationPriority returns a numeric priority for preservation levels
// (higher = more restrictive).
func preservationPriority(p model.Preservation) int {
	switch p {
	case model.PreservationRequired:
		return 3
	case model.PreservationPreferred:
		return 2
	case model.PreservationOptional:
		return 1
	default:
		return 0
	}
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
