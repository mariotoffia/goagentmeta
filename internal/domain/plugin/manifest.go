package plugin

import "github.com/mariotoffia/goagentmeta/internal/domain/marketplace"

// ComponentKind identifies a type of plugin component. It is a semi-open string
// enum: well-known values are provided as constants, but custom values are
// permitted to support future component types without schema changes.
type ComponentKind string

const (
	// ComponentSkills references skill directories containing <name>/SKILL.md.
	ComponentSkills ComponentKind = "skills"

	// ComponentCommands references flat markdown skill files or directories.
	ComponentCommands ComponentKind = "commands"

	// ComponentAgents references agent definition files.
	ComponentAgents ComponentKind = "agents"

	// ComponentHooks references hook configuration (inline or path).
	ComponentHooks ComponentKind = "hooks"

	// ComponentMcpServers references MCP server configuration (inline or path).
	ComponentMcpServers ComponentKind = "mcpServers"

	// ComponentLspServers references LSP server configuration (inline or path).
	ComponentLspServers ComponentKind = "lspServers"

	// ComponentOutputStyles references output style files or directories.
	ComponentOutputStyles ComponentKind = "outputStyles"

	// ComponentChannels references channel declarations for message injection.
	ComponentChannels ComponentKind = "channels"

	// ComponentBin references executables added to PATH.
	ComponentBin ComponentKind = "bin"
)

// Manifest describes a distributable plugin package. It is the target-native
// equivalent of Claude Code's plugin.json or similar manifests for other targets.
//
// The component system uses an open map keyed by ComponentKind so that new
// component types (channels, output-styles, etc.) can be supported without
// struct changes. Each value is component-specific: string paths, []string,
// inline objects, etc.
type Manifest struct {
	// SchemaVersion tracks manifest schema evolution.
	SchemaVersion int

	// Name is the plugin identifier (kebab-case, no spaces).
	Name string

	// Version is the plugin version (semver).
	Version string

	// Description is a brief explanation of plugin purpose.
	Description string

	// Author identifies the plugin author.
	Author marketplace.Author

	// Homepage is the plugin documentation URL.
	Homepage string

	// Repository is the source code repository URL.
	Repository string

	// License is an SPDX license identifier (e.g., "MIT", "Apache-2.0").
	License string

	// Keywords are discovery tags.
	Keywords []string

	// Components maps component kinds to their configuration. Values are
	// component-specific: string paths, []string, inline objects, etc.
	// Example: {ComponentSkills: "./custom/skills/", ComponentAgents: ["./agents/a.md"]}
	Components map[ComponentKind]any

	// Settings holds open key-value settings applied when the plugin is
	// enabled. Currently Claude Code supports the "agent" key to activate
	// a custom agent as the main thread.
	Settings map[string]any

	// UserConfig declares values that the host prompts the user for when
	// the plugin is enabled.
	UserConfig map[string]UserConfigEntry

	// Extra holds target-specific or future fields for forward-compatibility.
	Extra map[string]any
}

// UserConfigEntry describes a user-configurable value that is prompted at
// plugin enable time. Sensitive values are stored in the system keychain.
type UserConfigEntry struct {
	// Description explains what this value is for.
	Description string

	// Sensitive indicates the value should be stored securely (e.g., API tokens).
	Sensitive bool
}
