// Package renderer defines the target renderer port. Renderers are specialized
// pipeline stages that transform lowered IR into target-native files for a
// specific target ecosystem (Claude Code, Cursor, Copilot, Codex).
//
// Renderers register as stage plugins for the render phase with a target filter.
// When the pipeline reaches the render phase, it dispatches each build unit to
// the renderer matching the unit's target.
//
// New targets are added by implementing this interface and registering the
// renderer — no compiler core changes are needed.
package renderer

import (
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Renderer is a specialized Stage for the render phase. It handles a single
// target and declares the capability support levels for that target.
//
// The renderer receives a LoweredGraph (or the subset for its target's build
// units) and produces an EmissionPlan containing the target-native files,
// configuration, and plugin references to emit.
//
// Renderers must not interpret raw source files directly. They consume
// normalized, lowered IR so that:
//   - renderer behavior stays deterministic
//   - lowering decisions are centralized
//   - provenance is uniform
//   - new targets do not re-implement core semantics
type Renderer interface {
	stage.Stage

	// Target returns the target ecosystem this renderer handles
	// (e.g., build.TargetClaude, build.TargetCursor).
	Target() build.Target

	// SupportedCapabilities returns the capability registry for this target,
	// declaring which capability surfaces are native, adapted, lowered,
	// emulated, or skipped.
	SupportedCapabilities() capability.CapabilityRegistry
}
