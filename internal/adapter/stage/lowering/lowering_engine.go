package lowering

import (
	"context"
	"fmt"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion: *Stage satisfies the stage.Stage port interface.
var _ stage.Stage = (*Stage)(nil)

// Stage implements the PhaseLower pipeline stage. It transforms canonical objects
// that a target does not natively support into equivalent (or approximate) native
// forms, recording all decisions in LoweringRecords.
type Stage struct {
	// objects is a reference to normalized objects from the SemanticGraph.
	objects map[string]pipeline.NormalizedObject
}

// New creates a new lowering engine Stage.
func New(objects map[string]pipeline.NormalizedObject) *Stage {
	return &Stage{objects: objects}
}

// Descriptor returns the stage metadata for pipeline registration.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "lowering-engine",
		Phase: pipeline.PhaseLower,
		Order: 10,
	}
}

// Execute transforms a CapabilityGraph into a LoweredGraph.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	capGraph, ok := input.(pipeline.CapabilityGraph)
	if !ok {
		capPtr, ok := input.(*pipeline.CapabilityGraph)
		if !ok || capPtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrLowering,
				fmt.Sprintf("expected pipeline.CapabilityGraph or *pipeline.CapabilityGraph, got %T", input),
				"lowering-engine",
			)
		}
		capGraph = *capPtr
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "LOWERING_START",
		Message:  fmt.Sprintf("lowering objects for %d unit(s)", len(capGraph.Units)),
		Phase:    pipeline.PhaseLower,
	})

	graph := pipeline.LoweredGraph{
		Units: make(map[string]pipeline.LoweredUnit, len(capGraph.Units)),
	}

	var lowerErrors []string

	// Sort unit keys for deterministic processing.
	unitKeys := make([]string, 0, len(capGraph.Units))
	for k := range capGraph.Units {
		unitKeys = append(unitKeys, k)
	}
	sort.Strings(unitKeys)

	for _, unitKey := range unitKeys {
		unitCaps := capGraph.Units[unitKey]
		loweredUnit, errs := s.lowerUnit(ctx, unitKey, unitCaps)
		graph.Units[unitKey] = loweredUnit
		lowerErrors = append(lowerErrors, errs...)
	}

	// Emit errors for failed lowerings.
	for _, errMsg := range lowerErrors {
		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "error",
			Code:     "LOWERING",
			Message:  errMsg,
			Phase:    pipeline.PhaseLower,
		})
	}

	if len(lowerErrors) > 0 {
		return graph, pipeline.NewCompilerError(
			pipeline.ErrLowering,
			fmt.Sprintf("%d lowering failure(s)", len(lowerErrors)),
			"lowering-engine",
		)
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "LOWERING_COMPLETE",
		Message:  fmt.Sprintf("lowering complete for %d unit(s)", len(graph.Units)),
		Phase:    pipeline.PhaseLower,
	})

	return graph, nil
}

// Factory returns a StageFactory function for use with pipeline registration.
func Factory(objects map[string]pipeline.NormalizedObject) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(objects), nil
	}
}

// lowerUnit processes all active objects in a single build unit.
func (s *Stage) lowerUnit(
	ctx context.Context,
	unitKey string,
	unitCaps pipeline.UnitCapabilities,
) (pipeline.LoweredUnit, []string) {
	target := string(unitCaps.Coordinate.Unit.Target)

	lu := pipeline.LoweredUnit{
		Coordinate: unitCaps.Coordinate,
		Objects:    make(map[string]pipeline.LoweredObject),
	}

	// Collect object IDs from the Required + Resolved capabilities.
	// We need to process all objects that contribute to this unit.
	objectIDs := s.objectIDsForUnit(unitCaps)

	// Sort for deterministic processing.
	sort.Strings(objectIDs)

	var errs []string

	for _, objID := range objectIDs {
		obj, ok := s.objects[objID]
		if !ok {
			continue
		}

		lowered, record, err := s.lowerObject(ctx, obj, unitCaps, target)
		lu.Objects[objID] = lowered

		// Report every lowering decision.
		reportLowering(ctx, record)

		if err != "" {
			errs = append(errs, err)
		}
	}

	return lu, errs
}

// lowerObject applies the appropriate lowering function for a single object.
// Returns the lowered object, a lowering record, and an optional error message.
func (s *Stage) lowerObject(
	ctx context.Context,
	obj pipeline.NormalizedObject,
	unitCaps pipeline.UnitCapabilities,
	target string,
) (pipeline.LoweredObject, pipeline.LoweringRecord, string) {
	switch obj.Meta.Kind {
	case model.KindInstruction:
		// Instructions always pass through (all targets support instructions).
		kept := keepObject(obj, model.KindInstruction, "instructions always supported")
		record := pipeline.LoweringRecord{
			ObjectID:     obj.Meta.ID,
			FromKind:     model.KindInstruction,
			ToKind:       model.KindInstruction,
			Reason:       "kept (instructions always supported)",
			Preservation: obj.Meta.Preservation,
			Status:       "kept",
		}
		return kept, record, ""

	case model.KindRule:
		lowered := LowerRule(obj)
		record := RuleLoweringRecord(obj)
		return lowered, record, ""

	case model.KindHook:
		lowered, presResult := LowerHook(obj, unitCaps)
		record := HookLoweringRecord(lowered)
		if !presResult.Allowed && presResult.Severity == "error" {
			return lowered, record, fmt.Sprintf(
				"unit %s: %s", unitCaps.Coordinate.OutputDir, presResult.Message,
			)
		}
		if presResult.Severity == "warning" {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "warning",
				Code:     "LOWERING_WARN",
				Message:  presResult.Message,
				ObjectID: obj.Meta.ID,
				Phase:    pipeline.PhaseLower,
			})
		}
		return lowered, record, ""

	case model.KindSkill:
		lowered, presResult := LowerSkill(obj, unitCaps)
		record := SkillLoweringRecord(lowered)
		if !presResult.Allowed && presResult.Severity == "error" {
			return lowered, record, fmt.Sprintf(
				"unit %s: %s", unitCaps.Coordinate.OutputDir, presResult.Message,
			)
		}
		if presResult.Severity == "warning" {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "warning",
				Code:     "LOWERING_WARN",
				Message:  presResult.Message,
				ObjectID: obj.Meta.ID,
				Phase:    pipeline.PhaseLower,
			})
		}
		return lowered, record, ""

	case model.KindAgent:
		lowered, presResult := LowerAgent(obj, unitCaps)
		record := AgentLoweringRecord(lowered)
		if !presResult.Allowed && presResult.Severity == "error" {
			return lowered, record, fmt.Sprintf(
				"unit %s: %s", unitCaps.Coordinate.OutputDir, presResult.Message,
			)
		}
		if presResult.Severity == "warning" {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "warning",
				Code:     "LOWERING_WARN",
				Message:  presResult.Message,
				ObjectID: obj.Meta.ID,
				Phase:    pipeline.PhaseLower,
			})
		}
		return lowered, record, ""

	case model.KindCommand:
		lowered, presResult := LowerCommand(obj, unitCaps, target)
		record := CommandLoweringRecord(lowered)
		if !presResult.Allowed && presResult.Severity == "error" {
			return lowered, record, fmt.Sprintf(
				"unit %s: %s", unitCaps.Coordinate.OutputDir, presResult.Message,
			)
		}
		if presResult.Severity == "warning" {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "warning",
				Code:     "LOWERING_WARN",
				Message:  presResult.Message,
				ObjectID: obj.Meta.ID,
				Phase:    pipeline.PhaseLower,
			})
		}
		return lowered, record, ""

	case model.KindPlugin:
		lowered, presResult := LowerPlugin(obj, unitCaps, target)
		record := PluginLoweringRecord(lowered)
		if !presResult.Allowed && presResult.Severity == "error" {
			return lowered, record, fmt.Sprintf(
				"unit %s: %s", unitCaps.Coordinate.OutputDir, presResult.Message,
			)
		}
		if presResult.Severity == "warning" {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "warning",
				Code:     "LOWERING_WARN",
				Message:  presResult.Message,
				ObjectID: obj.Meta.ID,
				Phase:    pipeline.PhaseLower,
			})
		}
		return lowered, record, ""

	default:
		// Unknown kind → pass through with record.
		kept := keepObject(obj, obj.Meta.Kind, "unknown kind, passed through")
		record := pipeline.LoweringRecord{
			ObjectID:     obj.Meta.ID,
			FromKind:     obj.Meta.Kind,
			ToKind:       obj.Meta.Kind,
			Reason:       "unknown kind, passed through",
			Preservation: obj.Meta.Preservation,
			Status:       "kept",
		}
		return kept, record, ""
	}
}

// objectIDsForUnit extracts all object IDs that should be processed for a unit.
// It derives this from the objects map, matching objects whose capabilities
// are in the unit's Required set.
func (s *Stage) objectIDsForUnit(unitCaps pipeline.UnitCapabilities) []string {
	// Build the set of capability surfaces this unit deals with.
	surfaceSet := make(map[string]bool, len(unitCaps.Required))
	for _, r := range unitCaps.Required {
		surfaceSet[r] = true
	}
	// Also include resolved surfaces not in Required.
	for surface := range unitCaps.Resolved {
		surfaceSet[surface] = true
	}
	// And unsatisfied.
	for _, u := range unitCaps.Unsatisfied {
		surfaceSet[u] = true
	}

	// Find all objects whose required capabilities overlap with this unit's surfaces.
	seen := make(map[string]bool)
	var ids []string

	for objID, obj := range s.objects {
		caps := requiredCapabilities(obj.Meta)
		for _, cap := range caps {
			if surfaceSet[cap] {
				if !seen[objID] {
					seen[objID] = true
					ids = append(ids, objID)
				}
				break
			}
		}
	}

	return ids
}

// keepObject creates a LoweredObject that passes through unchanged.
func keepObject(obj pipeline.NormalizedObject, kind model.Kind, reason string) pipeline.LoweredObject {
	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: kind,
		LoweredKind:  kind,
		Decision: pipeline.LoweringDecision{
			Action:       "kept",
			Reason:       reason,
			Preservation: obj.Meta.Preservation,
			Safe:         true,
		},
		Content: objectContent(obj),
		Fields:  obj.ResolvedFields,
	}
}

func objectContent(obj pipeline.NormalizedObject) string {
	if obj.Content != "" {
		return obj.Content
	}
	if content, ok := obj.ResolvedFields["content"].(string); ok {
		return content
	}
	return ""
}

// requiredCapabilities mirrors the capability stage's mapping.
func requiredCapabilities(meta model.ObjectMeta) []string {
	switch meta.Kind {
	case model.KindInstruction:
		return []string{"instructions.layeredFiles", "instructions.scopedSections"}
	case model.KindRule:
		return []string{"rules.scopedRules"}
	case model.KindSkill:
		return []string{"skills.bundles", "skills.supportingFiles"}
	case model.KindAgent:
		return []string{"agents.subagents", "agents.toolPolicies"}
	case model.KindHook:
		return []string{"hooks.lifecycle", "hooks.blockingValidation"}
	case model.KindCommand:
		return []string{"commands.explicitEntryPoints"}
	case model.KindPlugin:
		return []string{"plugins.installablePackages", "plugins.capabilityProviders"}
	default:
		return nil
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

// reportLowering sends a lowering record through the Reporter if available.
func reportLowering(ctx context.Context, record pipeline.LoweringRecord) {
	cc := compiler.CompilerFromContext(ctx)
	if cc == nil || cc.Config == nil || cc.Config.Reporter == nil {
		return
	}
	cc.Config.Reporter.ReportLowering(ctx, record)
}
