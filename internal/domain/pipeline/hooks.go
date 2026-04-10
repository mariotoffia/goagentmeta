package pipeline

import "context"

// HookPoint identifies when a stage hook runs relative to a phase.
type HookPoint string

const (
	// HookBeforePhase runs before all stages in a phase.
	HookBeforePhase HookPoint = "before-phase"
	// HookAfterPhase runs after all stages in a phase.
	HookAfterPhase HookPoint = "after-phase"
	// HookTransform receives and may modify the IR between two stages.
	HookTransform HookPoint = "transform"
)

// StageHook describes a lightweight hook attached to the pipeline.
// Hooks are useful for cross-cutting concerns like logging, metrics,
// or organizational policy checks without replacing a full stage.
type StageHook struct {
	// Name uniquely identifies this hook.
	Name string
	// Point identifies when the hook runs.
	Point HookPoint
	// Phase is the pipeline phase this hook is attached to.
	Phase Phase
	// Handler is the function called when the hook fires.
	Handler StageHookFunc
}

// StageHookFunc is the function signature for stage hooks.
// It receives the current context and the IR flowing through the pipeline.
// For BeforePhase/AfterPhase hooks, ir is the phase input/output.
// For Transform hooks, ir is the IR between two stages and the returned
// value replaces it.
type StageHookFunc func(ctx context.Context, ir any) (any, error)
