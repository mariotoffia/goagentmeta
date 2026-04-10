package plugin

// DistributionMode controls how a plugin is delivered to the build output.
type DistributionMode string

const (
	// DistInline means plugin code and artifacts live in the repository.
	// The compiler copies or symlinks them into the build output.
	DistInline DistributionMode = "inline"

	// DistExternal means the plugin is installed and managed outside the
	// repository. The compiler emits only a target-native reference
	// (e.g., an MCP server entry in a settings file).
	DistExternal DistributionMode = "external"

	// DistRegistry means the plugin is declared as a dependency, resolved
	// from a marketplace or package registry at compile time, cached locally,
	// and either materialized or referenced depending on the target.
	DistRegistry DistributionMode = "registry"
)

// SelectionMode controls how the compiler selects a plugin during
// capability resolution.
type SelectionMode string

const (
	// SelectAutoIfSelected means the plugin is automatically selected when
	// its capabilities are required.
	SelectAutoIfSelected SelectionMode = "auto-if-selected"
	// SelectOptIn means the user must explicitly opt in to use this plugin.
	SelectOptIn SelectionMode = "opt-in"
	// SelectManualOnly means the plugin requires manual installation.
	SelectManualOnly SelectionMode = "manual-only"
	// SelectDisabled means the plugin is not available for selection.
	SelectDisabled SelectionMode = "disabled"
)

// InstallStrategy controls how the compiler materializes or references a plugin.
type InstallStrategy string

const (
	// InstallMaterialize means copy full plugin artifacts into the build output.
	InstallMaterialize InstallStrategy = "materialize"
	// InstallReferenceOnly means emit a configuration reference without
	// materializing code.
	InstallReferenceOnly InstallStrategy = "reference-only"
	// InstallAutoDetect means let the renderer choose based on target capabilities.
	InstallAutoDetect InstallStrategy = "auto-detect"
)

// Distribution describes how a plugin is delivered.
type Distribution struct {
	// Mode is the distribution mode (inline, external, or registry).
	Mode DistributionMode
	// Ref is a reference to the external source (e.g., npm package name, git URL).
	// Used when Mode is External or Registry.
	Ref string
	// Version is the version constraint for registry-resolved plugins.
	Version string
}
