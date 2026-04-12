// Package pipeline defines pipeline stage interfaces, IR types, and compiler
// plugin contracts. The compiler pipeline is a sequence of ordered phases,
// each containing one or more stage plugins. IR types flow between stages
// as the compiler transforms source input into target output.
package pipeline

// Phase identifies a pipeline phase. Phases execute in a fixed order.
// Each phase may contain one or more stage plugins that run in priority order.
type Phase int

const (
	// PhaseParse reads source files and produces a SourceTree.
	PhaseParse Phase = iota
	// PhaseValidate validates the SourceTree against schemas.
	PhaseValidate
	// PhaseResolve resolves external dependencies from registries.
	PhaseResolve
	// PhaseNormalize normalizes the source tree into a SemanticGraph.
	PhaseNormalize
	// PhasePlan expands the semantic graph into a BuildPlan.
	PhasePlan
	// PhaseCapability resolves capabilities and selects providers.
	PhaseCapability
	// PhaseLower lowers unsupported concepts for each target.
	PhaseLower
	// PhaseRender renders lowered IR into target-native files.
	PhaseRender
	// PhaseMaterialize writes rendered files to disk.
	PhaseMaterialize
	// PhaseReport generates provenance and build reports.
	PhaseReport
)

// String returns the human-readable name for the phase.
func (p Phase) String() string {
	switch p {
	case PhaseParse:
		return "parse"
	case PhaseValidate:
		return "validate"
	case PhaseResolve:
		return "resolve"
	case PhaseNormalize:
		return "normalize"
	case PhasePlan:
		return "plan"
	case PhaseCapability:
		return "capability"
	case PhaseLower:
		return "lower"
	case PhaseRender:
		return "render"
	case PhaseMaterialize:
		return "materialize"
	case PhaseReport:
		return "report"
	default:
		return "unknown"
	}
}

// AllPhases returns all phases in execution order.
func AllPhases() []Phase {
	return []Phase{
		PhaseParse,
		PhaseValidate,
		PhaseResolve,
		PhaseNormalize,
		PhasePlan,
		PhaseCapability,
		PhaseLower,
		PhaseRender,
		PhaseMaterialize,
		PhaseReport,
	}
}

// PhaseCount is the total number of pipeline phases.
const PhaseCount = 10

// phaseNames maps phase string names to Phase values.
var phaseNames = func() map[string]Phase {
	m := make(map[string]Phase, PhaseCount)
	for _, p := range AllPhases() {
		m[p.String()] = p
	}
	return m
}()

// ParsePhase converts a phase name string to a Phase value.
// It returns the phase and true if the name is valid, or zero and false otherwise.
func ParsePhase(name string) (Phase, bool) {
	p, ok := phaseNames[name]
	return p, ok
}

// ValidPhaseNames returns a sorted slice of all valid phase name strings.
func ValidPhaseNames() []string {
	names := make([]string, 0, PhaseCount)
	for _, p := range AllPhases() {
		names = append(names, p.String())
	}
	return names
}
