// Package capability defines capability contracts and resolution logic.
// A capability is an abstract contract required by authoring primitives
// (skills, agents, commands, hooks). Targets do not consume capabilities
// directly; they consume concrete providers selected by the compiler.
package capability

// Capability is an abstract contract that authoring primitives may require.
// Examples: "filesystem.read", "terminal.exec", "mcp.github".
type Capability struct {
	// ID uniquely identifies this capability (e.g., "repo.graph.query").
	ID string

	// Contract describes the capability's category and purpose.
	Contract Contract

	// Security declares the capability's permission requirements.
	Security CapabilitySecurity
}

// Contract describes what a capability provides.
type Contract struct {
	// Category classifies the capability (e.g., "tool", "mcp", "service").
	Category string
	// Description is a human-readable explanation of the capability.
	Description string
}

// CapabilitySecurity declares the permission requirements for a capability.
type CapabilitySecurity struct {
	// Network describes network access needed ("none", "outbound", "inbound").
	Network string
	// Filesystem describes filesystem access needed ("none", "read-repo", "read-write").
	Filesystem string
}

// SupportLevel classifies how a target supports a specific capability surface.
// These levels drive compiler lowering decisions.
type SupportLevel string

const (
	// SupportNative means first-class support exists in the target.
	SupportNative SupportLevel = "native"
	// SupportAdapted means same semantics, different syntax or file placement.
	SupportAdapted SupportLevel = "adapted"
	// SupportLowered means the compiler can map the concept to another primitive.
	SupportLowered SupportLevel = "lowered"
	// SupportEmulated means only an approximation exists.
	SupportEmulated SupportLevel = "emulated"
	// SupportSkipped means the capability is not available on this target.
	SupportSkipped SupportLevel = "skipped"
)

// CapabilityRegistry maps capability surfaces to their support level for a
// specific target. The registry is keyed by a two-level path:
// category (e.g., "instructions") and surface (e.g., "layeredFiles").
type CapabilityRegistry struct {
	// Target is the target this registry describes.
	Target string
	// Surfaces maps "category.surface" keys to their support level.
	Surfaces map[string]SupportLevel
}

// Provider is a concrete provider that satisfies one or more capabilities.
// Providers are selected by the compiler during capability resolution.
type Provider struct {
	// ID uniquely identifies this provider.
	ID string
	// Type classifies the provider ("native", "plugin", "mcp", "script").
	Type string
	// Capabilities lists the capability IDs this provider satisfies.
	Capabilities []string
}

// ProviderCandidate is an evaluated candidate during capability resolution.
// The compiler scores and selects candidates based on priority, compatibility,
// and profile security policy.
type ProviderCandidate struct {
	// Provider is the candidate provider.
	Provider Provider
	// Priority is the selection priority (lower is higher priority).
	Priority int
	// Compatible indicates whether the candidate passes security and profile checks.
	Compatible bool
	// Reason explains why the candidate was selected or rejected.
	Reason string
}
