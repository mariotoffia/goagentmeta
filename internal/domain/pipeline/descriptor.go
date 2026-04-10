package pipeline

import "github.com/mariotoffia/goagentmeta/internal/domain/build"

// StageDescriptor declares metadata for a pipeline stage plugin. The compiler
// uses descriptors to order stages within phases, apply target filters,
// and resolve before/after constraints.
type StageDescriptor struct {
	// Name is the unique identifier for this stage (e.g., "yaml-parser",
	// "claude-renderer", "acme-custom-lowering").
	Name string

	// Phase is the pipeline phase this stage belongs to.
	Phase Phase

	// Order is the priority within the phase. Lower values run first.
	// Stages with equal order are sorted by name for determinism.
	Order int

	// Before lists stage names that must run after this stage within
	// the same phase. This creates an ordering constraint.
	Before []string

	// After lists stage names that must run before this stage within
	// the same phase. This creates an ordering constraint.
	After []string

	// TargetFilter restricts this stage to specific targets. An empty
	// filter means the stage runs for all targets. This is primarily
	// used by renderer stages that handle a single target.
	TargetFilter []build.Target
}

// Validate checks that the descriptor has required fields set.
// Returns an error describing the first validation failure, or nil.
func (d StageDescriptor) Validate() error {
	if d.Name == "" {
		return NewCompilerError(ErrValidation, "stage descriptor: name is required", "")
	}
	if d.Phase < PhaseParse || d.Phase > PhaseReport {
		return NewCompilerError(ErrValidation, "stage descriptor: invalid phase", d.Name)
	}
	return nil
}
