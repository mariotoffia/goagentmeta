// Package marketplace defines the domain model for plugin marketplaces.
// A marketplace is a catalog of distributable plugins with metadata for
// discovery, versioning, and installation. The types here are target-agnostic;
// target-specific serialization (e.g., Claude Code marketplace.json) is handled
// by adapter implementations of the Generator port.
//
// Extensibility follows the codebase conventions:
//   - Semi-open string enums (type Foo string + constants) for source types
//   - map[string]any escape hatches (Components, Extra) for forward-compatibility
//   - SchemaVersion for catalog schema evolution
package marketplace

// Marketplace represents a complete plugin catalog. It is the domain aggregate
// root for marketplace operations.
type Marketplace struct {
	// SchemaVersion tracks catalog schema evolution. Consumers that encounter
	// a version higher than they understand should warn but still attempt to
	// parse known fields.
	SchemaVersion int

	// Name is the marketplace identifier (kebab-case, no spaces).
	Name string

	// Owner identifies the marketplace maintainer.
	Owner Owner

	// Metadata holds optional catalog-level information.
	Metadata Metadata

	// Plugins lists the available plugin entries.
	Plugins []PluginEntry
}

// Owner identifies a marketplace maintainer.
type Owner struct {
	Name  string
	Email string
}

// Metadata holds optional catalog-level information.
type Metadata struct {
	// Description is a brief marketplace description.
	Description string

	// Version is the marketplace catalog version (semver).
	Version string

	// PluginRoot is a base directory prepended to relative plugin source paths.
	PluginRoot string

	// Extra holds forward-compatible metadata key-value pairs.
	Extra map[string]string
}

// PluginEntry describes a single plugin in the marketplace catalog.
type PluginEntry struct {
	// Name is the plugin identifier (kebab-case, no spaces).
	Name string

	// Source describes where to fetch the plugin.
	Source Source

	// Description is a brief plugin description.
	Description string

	// Version is the plugin version (semver).
	Version string

	// Author identifies the plugin author.
	Author Author

	// Homepage is the plugin documentation URL.
	Homepage string

	// Repository is the source code repository URL.
	Repository string

	// License is an SPDX license identifier (e.g., "MIT", "Apache-2.0").
	License string

	// Keywords are tags for plugin discovery and categorization.
	Keywords []string

	// Category classifies the plugin (e.g., "code-intelligence",
	// "external-integrations", "development-workflows", "output-styles").
	Category string

	// Tags provide additional searchability.
	Tags []string

	// Strict controls whether plugin.json is the authority for component
	// definitions. nil means default (true).
	Strict *bool

	// Components declares plugin components using an open map. Keys are
	// component kind strings (e.g., "skills", "agents", "hooks",
	// "mcpServers", "lspServers", "outputStyles", "channels"). Values are
	// component-specific: string paths, []string, inline objects, etc.
	// New component types are supported without schema changes.
	Components map[string]any

	// Extra holds target-specific or future fields for forward-compatibility.
	Extra map[string]any
}

// Author identifies a plugin author.
type Author struct {
	Name  string
	Email string
	URL   string
}
