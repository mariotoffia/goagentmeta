package pipeline

import (
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

// BuildPlan expands normalized objects into concrete build units.
// Each build unit is a (target, profile, scopes) tuple with its own
// set of active objects.
type BuildPlan struct {
	// Units holds the expanded build units with their active objects.
	Units []BuildPlanUnit
}

// BuildPlanUnit is a single build unit with its resolved set of active
// canonical objects.
type BuildPlanUnit struct {
	// Coordinate identifies this build unit (target + profile + scopes + output dir).
	Coordinate build.BuildCoordinate

	// ActiveObjects lists the IDs of canonical objects active in this unit.
	ActiveObjects []string
}

// CapabilityGraph holds the results of capability resolution for all build
// units. For each unit, it records which capabilities are required, which
// providers were selected, and which capabilities remain unsatisfied.
type CapabilityGraph struct {
	// Units maps build unit output dirs to their capability resolution results.
	Units map[string]UnitCapabilities
}

// UnitCapabilities holds capability resolution results for a single build unit.
type UnitCapabilities struct {
	// Coordinate identifies the build unit.
	Coordinate build.BuildCoordinate

	// Required lists all capability IDs required by active objects.
	Required []string

	// Resolved maps capability IDs to their selected provider.
	Resolved map[string]capability.Provider

	// Unsatisfied lists capability IDs with no compatible provider.
	Unsatisfied []string

	// Candidates maps capability IDs to all evaluated candidates
	// (for provenance and reporting).
	Candidates map[string][]capability.ProviderCandidate
}

// LoweredGraph holds the target-specific lowered representation of
// canonical objects. Each object carries its lowering decision and
// provenance back to the normalized source.
type LoweredGraph struct {
	// Units maps build unit output dirs to their lowered objects.
	Units map[string]LoweredUnit
}

// LoweredUnit holds lowered objects for a single build unit.
type LoweredUnit struct {
	// Coordinate identifies the build unit.
	Coordinate build.BuildCoordinate

	// Objects holds the lowered objects indexed by their original ID.
	Objects map[string]LoweredObject
}

// LoweredObject is a canonical object after target-specific lowering.
// It preserves provenance back to the source object and records the
// lowering decision.
type LoweredObject struct {
	// OriginalID is the object's ID in the semantic graph.
	OriginalID string

	// OriginalKind is the object's kind before lowering.
	OriginalKind model.Kind

	// LoweredKind is the object's kind after lowering (may equal OriginalKind
	// if no lowering was needed).
	LoweredKind model.Kind

	// Decision describes the lowering decision.
	Decision LoweringDecision

	// Content holds the lowered content ready for rendering.
	Content string

	// Fields holds the lowered fields.
	Fields map[string]any
}

// LoweringDecision records what lowering was applied and why.
type LoweringDecision struct {
	// Action is the lowering action taken ("kept", "lowered", "emulated", "skipped").
	Action string
	// Reason explains why the lowering was performed.
	Reason string
	// Preservation is the object's preservation level.
	Preservation model.Preservation
	// Safe indicates whether the lowering is semantically safe.
	Safe bool
}
