// Package plugin defines the plugin domain model. A plugin is a deployable
// extension package that provides one or more capabilities or native extension
// surfaces. Plugins are runtime-facing packaging units, distinct from skills
// (which are model-facing workflow content).
package plugin

import "github.com/mariotoffia/goagentmeta/internal/domain/model"

// Plugin is a deployable runtime provider that satisfies capabilities or
// extends a target with native tools, MCP servers, commands, or resources.
type Plugin struct {
	model.ObjectMeta

	// Distribution defines how the plugin is delivered.
	Distribution Distribution

	// Provides lists the capability IDs this plugin satisfies.
	Provides []string

	// Security declares the plugin's trust and permission requirements.
	Security PluginSecurity

	// Selection controls how the compiler selects this plugin.
	Selection SelectionMode

	// Install controls how the plugin is materialized or referenced.
	Install InstallStrategy

	// Artifacts lists the files that comprise the plugin.
	Artifacts PluginArtifacts
}

// PluginSecurity declares trust and permission requirements for a plugin.
// This is necessary because plugins are executable delivery units.
type PluginSecurity struct {
	// Trust is the trust level ("trusted", "review-required", "untrusted").
	Trust string
	// Permissions declares the plugin's operational permissions.
	Permissions PluginPermissions
}

// PluginPermissions declares the operational permissions a plugin requires.
type PluginPermissions struct {
	// Filesystem declares filesystem access ("none", "read-repo", "read-write").
	Filesystem string
	// Network declares network access ("none", "outbound", "inbound").
	Network string
	// ProcessExec indicates whether the plugin needs to execute processes.
	ProcessExec bool
	// Secrets lists the secret/environment variable names the plugin requires.
	Secrets []string
}

// PluginArtifacts lists the files that comprise a plugin.
type PluginArtifacts struct {
	// Scripts are executable files included in the plugin.
	Scripts []string
	// Configs are configuration files included in the plugin.
	Configs []string
	// Manifests are target-native manifest files.
	Manifests []string
}
