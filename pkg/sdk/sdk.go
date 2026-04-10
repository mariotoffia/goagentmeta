// Package sdk provides the public compiler plugin SDK for third-party authors.
// It re-exports the key types needed to build custom pipeline stages, renderers,
// and hooks without depending on internal packages.
//
// Third-party plugin authors import this package to:
//   - implement the Stage interface for custom pipeline stages
//   - implement the Renderer interface for new target ecosystems
//   - create StageDescriptors with the NewDescriptor builder
//   - reference Phase constants and IR types
//
// Example usage:
//
//	import "github.com/mariotoffia/goagentmeta/pkg/sdk"
//
//	type MyStage struct{}
//
//	func (s *MyStage) Descriptor() sdk.StageDescriptor {
//	    return sdk.NewDescriptor("my-stage", sdk.PhaseLower, 50)
//	}
//
//	func (s *MyStage) Execute(ctx context.Context, input any) (any, error) {
//	    // custom lowering logic
//	    return input, nil
//	}
package sdk

import (
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// Re-export Phase constants for plugin authors.
const (
	PhaseParse       = pipeline.PhaseParse
	PhaseValidate    = pipeline.PhaseValidate
	PhaseResolve     = pipeline.PhaseResolve
	PhaseNormalize   = pipeline.PhaseNormalize
	PhasePlan        = pipeline.PhasePlan
	PhaseCapability  = pipeline.PhaseCapability
	PhaseLower       = pipeline.PhaseLower
	PhaseRender      = pipeline.PhaseRender
	PhaseMaterialize = pipeline.PhaseMaterialize
	PhaseReport      = pipeline.PhaseReport
)

// Re-export type aliases for plugin authors.
type (
	// Phase identifies a pipeline phase.
	Phase = pipeline.Phase

	// StageDescriptor declares metadata for a pipeline stage plugin.
	StageDescriptor = pipeline.StageDescriptor

	// Target identifies the vendor or ecosystem being emitted for.
	Target = build.Target

	// SourceTree is the parse-phase output IR.
	SourceTree = pipeline.SourceTree
	// SemanticGraph is the normalize-phase output IR.
	SemanticGraph = pipeline.SemanticGraph
	// BuildPlan is the plan-phase output IR.
	BuildPlan = pipeline.BuildPlan
	// CapabilityGraph is the capability-phase output IR.
	CapabilityGraph = pipeline.CapabilityGraph
	// LoweredGraph is the lower-phase output IR.
	LoweredGraph = pipeline.LoweredGraph
	// EmissionPlan is the render-phase output IR.
	EmissionPlan = pipeline.EmissionPlan
	// MaterializationResult is the materialize-phase output IR.
	MaterializationResult = pipeline.MaterializationResult
	// BuildReport is the report-phase output IR and the final pipeline result.
	BuildReport = pipeline.BuildReport
	// Diagnostic is a compiler diagnostic message.
	Diagnostic = pipeline.Diagnostic
	// CompilerError is a structured error for compiler operations.
	CompilerError = pipeline.CompilerError
)

// NewDescriptor creates a StageDescriptor with the given name, phase, and order.
// Use the returned descriptor's Before/After/TargetFilter fields for
// ordering constraints and target filtering.
func NewDescriptor(name string, phase Phase, order int) StageDescriptor {
	return StageDescriptor{
		Name:  name,
		Phase: phase,
		Order: order,
	}
}

// NewDescriptorWithTarget creates a StageDescriptor filtered to a specific target.
// This is a convenience for renderer stages that handle a single target.
func NewDescriptorWithTarget(name string, phase Phase, order int, target Target) StageDescriptor {
	return StageDescriptor{
		Name:         name,
		Phase:        phase,
		Order:        order,
		TargetFilter: []Target{target},
	}
}
