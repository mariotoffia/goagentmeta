package packaging

import "github.com/mariotoffia/goagentmeta/internal/domain/build"

// PackagingConfig holds all packaging configuration from the manifest.
type PackagingConfig struct {
	VSCodeExtension *VSCodePackagingConfig `yaml:"vscode-extension,omitempty"`
	NPM             *NPMPackagingConfig    `yaml:"npm,omitempty"`
	OCI             *OCIPackagingConfig    `yaml:"oci,omitempty"`
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
