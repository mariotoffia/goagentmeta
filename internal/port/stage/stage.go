// Package stage defines the core compiler plugin SPI (Service Provider Interface).
// Every pipeline stage — whether built-in or third-party — implements the Stage
// interface. The compiler loads, orders, and dispatches stages through the pipeline.
//
// This package is a hexagonal port: it defines contracts that adapters implement.
// The domain knows about this interface; infrastructure adapters satisfy it.
package stage

import (
	"context"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// Stage is the core compiler plugin contract. Every pipeline stage
// (parser, validator, normalizer, renderer, etc.) implements this interface.
//
// The Execute method receives the IR from the previous phase and returns
// the IR for the next phase. The concrete types of input and output depend
// on the phase — for example, the parse phase receives filesystem paths
// and returns a SourceTree, while the render phase receives a LoweredGraph
// and returns an EmissionPlan.
//
// Stage implementations must be safe for sequential execution within a phase
// but are not required to be safe for concurrent use. The pipeline orchestrator
// ensures stages run sequentially within each phase.
type Stage interface {
	// Descriptor returns metadata about this stage: its name, phase,
	// ordering priority, and optional target filter. The pipeline uses
	// descriptors to order stages and apply target filtering.
	Descriptor() pipeline.StageDescriptor

	// Execute runs the stage logic. The input is the IR from the previous
	// stage or phase. The output is the IR for the next stage or phase.
	// The concrete types depend on the stage's phase.
	//
	// Context carries cancellation, deadlines, and the CompilerContext
	// for accessing shared services (diagnostics, provenance).
	Execute(ctx context.Context, input any) (any, error)
}

// StageHookHandler is a lightweight hook that runs before or after a phase,
// or between two stages. Hooks are useful for cross-cutting concerns like
// logging, metrics, or organizational policy checks without replacing a
// full stage.
//
// Unlike a Stage, a hook does not produce a new IR type — it observes or
// transforms the existing IR in place.
type StageHookHandler interface {
	// Hook returns the hook descriptor (name, point, phase).
	Hook() pipeline.StageHook
}

// StageFactory is a factory function that creates a Stage instance.
// Factories are registered with the stage registry and called during
// pipeline construction.
type StageFactory func() (Stage, error)

// StageValidator is an optional interface that stages may implement
// to support self-validation. The pipeline calls Validate during
// construction to detect configuration errors early.
type StageValidator interface {
	// Validate checks that the stage is properly configured.
	// Returns an error if the stage cannot execute.
	Validate() error
}
