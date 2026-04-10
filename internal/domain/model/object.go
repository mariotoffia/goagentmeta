// Package model defines the canonical object types that form the source
// language of the goagentmeta compiler. These are pure domain entities and
// value objects with no infrastructure dependencies.
//
// Every canonical object shares a common envelope ([ObjectMeta]) that carries
// identity, versioning, scoping, preservation semantics, and override surface.
// Concrete object types (Instruction, Rule, Skill, Agent, Hook, Command) embed
// ObjectMeta and add kind-specific fields.
package model

// Kind identifies the type of a canonical object in the source tree.
type Kind string

const (
	// KindInstruction represents always-on guidance and context.
	KindInstruction Kind = "instruction"
	// KindRule represents scoped or conditional policy.
	KindRule Kind = "rule"
	// KindSkill represents a reusable, model-facing workflow bundle.
	KindSkill Kind = "skill"
	// KindAgent represents a specialized delegate or orchestration wrapper.
	KindAgent Kind = "agent"
	// KindHook represents deterministic lifecycle automation.
	KindHook Kind = "hook"
	// KindCommand represents an explicit user-invoked entry point.
	KindCommand Kind = "command"
	// KindCapability represents an abstract contract required by authoring primitives.
	KindCapability Kind = "capability"
	// KindPlugin represents a deployable extension package.
	KindPlugin Kind = "plugin"
)

// Preservation controls how the compiler handles unsupported or unsafe lowering
// for a canonical object. Every object carries a preservation level.
type Preservation string

const (
	// PreservationRequired means unsupported or unsafe lowering fails the build.
	PreservationRequired Preservation = "required"
	// PreservationPreferred means lower when safe, otherwise warn and skip.
	PreservationPreferred Preservation = "preferred"
	// PreservationOptional means may skip with reporting.
	PreservationOptional Preservation = "optional"
)

// Scope identifies where a canonical object applies within the repository.
// Scopes support repository root, subtree paths, globs, file types, and labels.
type Scope struct {
	// Paths are the filesystem paths or globs where this object applies.
	// An empty slice means repository root.
	Paths []string
	// FileTypes restricts applicability to specific file extensions (e.g., ".go", ".ts").
	FileTypes []string
	// Labels are semantic tags for bounded-context or domain scoping.
	Labels []string
}

// AppliesTo constrains which targets and profiles a canonical object is active for.
type AppliesTo struct {
	// Targets lists target identifiers (e.g., "claude", "cursor").
	// A wildcard ["*"] means all targets.
	Targets []string
	// Profiles lists profile identifiers (e.g., "local-dev", "ci").
	// A wildcard ["*"] means all profiles.
	Profiles []string
}

// TargetOverride carries delta-based adjustments for a specific target.
// Overrides may adjust syntax, file placement, packaging hints, enablement,
// or optional target-native features. They must not silently redefine
// canonical meaning.
type TargetOverride struct {
	// Enabled controls whether the object is emitted for this target.
	// nil means inherit from the base object.
	Enabled *bool
	// Syntax holds target-specific syntax adjustments.
	Syntax map[string]string
	// Placement holds target-specific file placement hints.
	Placement map[string]string
	// Extra holds arbitrary target-specific key-value pairs.
	Extra map[string]string
}

// ObjectMeta is the common metadata envelope shared by every canonical object.
// It carries identity, versioning, scoping, preservation semantics, inheritance,
// and the override surface.
type ObjectMeta struct {
	// ID is the unique identifier for this object within the source tree.
	ID string
	// Kind identifies the object type.
	Kind Kind
	// Version is the schema version for this object (e.g., 1).
	Version int
	// Description is a human-readable summary of this object's purpose.
	Description string
	// PackageVersion is the semantic version of the distributable package
	// (e.g., "1.1.3"). Distinct from Version which is the schema version.
	PackageVersion string
	// License is the SPDX license identifier for this object (e.g., "MIT",
	// "Apache-2.0"). Applies to skills, plugins, and other distributable objects.
	License string
	// Scope defines where this object applies in the repository.
	Scope Scope
	// AppliesTo constrains which targets and profiles this object is active for.
	AppliesTo AppliesTo
	// Extends lists object IDs that this object inherits from.
	Extends []string
	// Preservation controls how the compiler handles unsupported lowering.
	Preservation Preservation
	// Labels are arbitrary tags for grouping and selection.
	Labels []string
	// Owner identifies the team or individual responsible for this object.
	Owner string
	// TargetOverrides holds delta-based adjustments keyed by target name.
	TargetOverrides map[string]TargetOverride
}
