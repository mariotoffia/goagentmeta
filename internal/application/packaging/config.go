package packaging

import "github.com/mariotoffia/goagentmeta/internal/domain/build"

// PackagingConfig holds all packaging configuration from the manifest.
// The legacy fields (VSCodeExtension, NPM, OCI) are kept for backward
// compatibility. New packagers should use the PackageWithEmission method
// with format-keyed configs instead.
type PackagingConfig struct {
	VSCodeExtension *VSCodePackagingConfig `yaml:"vscode-extension,omitempty"`
	NPM             *NPMPackagingConfig    `yaml:"npm,omitempty"`
	OCI             *OCIPackagingConfig    `yaml:"oci,omitempty"`
	ClaudePlugin    *ClaudePluginConfig    `yaml:"claude-plugin,omitempty"`
	Marketplace     *MarketplaceConfig     `yaml:"marketplace,omitempty"`
}

// VSCodePackagingConfig controls generation of a .vsix extension archive.
type VSCodePackagingConfig struct {
	Enabled          bool           `yaml:"enabled"`
	Publisher        string         `yaml:"publisher"`
	Targets          []build.Target `yaml:"targets"`
	IncludeSkills    bool           `yaml:"includeSkills"`
	IncludeMcpConfig bool           `yaml:"includeMcpConfig"`
	Version          string         `yaml:"version,omitempty"`
	DisplayName      string         `yaml:"displayName,omitempty"`
	Description      string         `yaml:"description,omitempty"`
}

// NPMPackagingConfig controls generation of an npm-compatible tarball.
type NPMPackagingConfig struct {
	Enabled     bool           `yaml:"enabled"`
	Scope       string         `yaml:"scope"`
	Targets     []build.Target `yaml:"targets"`
	IncludeOnly string         `yaml:"includeOnly,omitempty"`
	Version     string         `yaml:"version,omitempty"`
}

// OCIPackagingConfig controls generation of an OCI image layer.
type OCIPackagingConfig struct {
	Enabled    bool           `yaml:"enabled"`
	Repository string         `yaml:"repository,omitempty"`
	Targets    []build.Target `yaml:"targets"`
	Tag        string         `yaml:"tag,omitempty"`
}

// ClaudePluginConfig controls generation of a distributable Claude Code plugin
// directory. The output is a directory conforming to Claude Code's plugin format
// (.claude-plugin/plugin.json + skills/ + agents/ + hooks/ + .mcp.json).
type ClaudePluginConfig struct {
	Enabled     bool         `yaml:"enabled"`
	Name        string       `yaml:"name"`
	Version     string       `yaml:"version,omitempty"`
	Description string       `yaml:"description,omitempty"`
	Author      AuthorConfig `yaml:"author,omitempty"`
	Homepage    string       `yaml:"homepage,omitempty"`
	Repository  string       `yaml:"repository,omitempty"`
	License     string       `yaml:"license,omitempty"`
	Keywords    []string     `yaml:"keywords,omitempty"`
}

// MarketplaceConfig controls generation of a marketplace catalog. The catalog
// lists plugins with their sources, categories, and tags for discovery.
type MarketplaceConfig struct {
	Enabled bool                     `yaml:"enabled"`
	Format  string                   `yaml:"format"`
	Name    string                   `yaml:"name"`
	Owner   OwnerConfig              `yaml:"owner"`
	Plugins []MarketplacePluginEntry `yaml:"plugins,omitempty"`
}

// MarketplacePluginEntry describes a plugin entry in the marketplace catalog.
type MarketplacePluginEntry struct {
	Name        string   `yaml:"name"`
	Source      string   `yaml:"source"`
	Description string   `yaml:"description,omitempty"`
	Version     string   `yaml:"version,omitempty"`
	Category    string   `yaml:"category,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	Keywords    []string `yaml:"keywords,omitempty"`
}

// AuthorConfig holds author information for packaging.
type AuthorConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
}

// OwnerConfig holds marketplace owner information.
type OwnerConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
}
