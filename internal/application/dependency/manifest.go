// Package dependency implements the dependency resolution use case for the
// compiler pipeline. It resolves external package dependencies declared in
// manifest.yaml, manages a lock file for reproducible builds, and merges
// external objects into the SourceTree.
package dependency

import (
	"fmt"
	"os"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"gopkg.in/yaml.v3"
)

// ManifestDependency represents a single dependency declaration from
// manifest.yaml. Each dependency specifies a package name and a semver
// version constraint.
type ManifestDependency struct {
	// Name is the fully qualified package name (e.g., "@acme/go-lambda-skill").
	Name string
	// Version is the semver constraint string (e.g., "^1.0.0", "~2.1.0").
	Version string
	// Registry optionally restricts resolution to a named registry.
	// Empty means use all configured registries in priority order.
	Registry string
}

// RegistryConfig represents a registry configuration from manifest.yaml.
type RegistryConfig struct {
	// Name is the unique identifier for this registry (e.g., "local", "acme-corp").
	Name string `yaml:"name"`
	// Type is the registry type: "local", "http", or "git".
	Type string `yaml:"type"`
	// URL is the base URL or filesystem path for the registry.
	URL string `yaml:"url"`
	// Auth is an optional authentication token.
	Auth string `yaml:"auth,omitempty"`
}

// CompilerConfig holds compiler pipeline configuration from the manifest.
type CompilerConfig struct {
	ExternalPlugins []ExternalPluginEntry `yaml:"externalPlugins"`
}

// ExternalPluginEntry describes an external process plugin that communicates
// over stdin/stdout with JSON serialization.
type ExternalPluginEntry struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`         // Must start with "external://"
	Phase  string `yaml:"phase"`          // Must be a valid pipeline phase name
	Order  int    `yaml:"order"`
	Target string `yaml:"target,omitempty"` // Optional target filter
}

// Validate checks that the ExternalPluginEntry has valid field values.
func (e *ExternalPluginEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("external plugin: name must not be empty")
	}
	if !strings.HasPrefix(e.Source, "external://") {
		return fmt.Errorf("external plugin %q: source must start with %q, got %q", e.Name, "external://", e.Source)
	}
	if _, ok := pipeline.ParsePhase(e.Phase); !ok {
		return fmt.Errorf("external plugin %q: invalid phase %q (valid: %s)",
			e.Name, e.Phase, strings.Join(pipeline.ValidPhaseNames(), ", "))
	}
	return nil
}

// Validate checks that all external plugin entries are valid and that names are unique.
func (c *CompilerConfig) Validate() error {
	seen := make(map[string]struct{}, len(c.ExternalPlugins))
	for i := range c.ExternalPlugins {
		entry := &c.ExternalPlugins[i]
		if err := entry.Validate(); err != nil {
			return err
		}
		if _, dup := seen[entry.Name]; dup {
			return fmt.Errorf("external plugin %q: duplicate name", entry.Name)
		}
		seen[entry.Name] = struct{}{}
	}
	return nil
}

// Manifest is the top-level manifest.yaml structure. Only the fields
// needed by the dependency resolver are decoded; the rest is ignored.
type Manifest struct {
	Dependencies map[string]dependencyEntry `yaml:"dependencies"`
	Registries   []RegistryConfig           `yaml:"registries"`
	Compiler     CompilerConfig             `yaml:"compiler"`
}

// dependencyEntry supports both short and extended dependency formats:
//
//	# Short: version constraint string
//	test-package: "^1.0.0"
//
//	# Extended: object with version and optional registry
//	pinned-pkg:
//	  version: "1.2.3"
//	  registry: local
type dependencyEntry struct {
	Version  string
	Registry string
}

// UnmarshalYAML implements a custom unmarshaller that accepts both a plain
// string (short form) and a mapping (extended form).
func (d *dependencyEntry) UnmarshalYAML(value *yaml.Node) error {
	// Short form: bare string value.
	if value.Kind == yaml.ScalarNode {
		d.Version = value.Value
		return nil
	}

	// Extended form: mapping with version and optional registry.
	if value.Kind == yaml.MappingNode {
		var ext struct {
			Version  string `yaml:"version"`
			Registry string `yaml:"registry"`
		}
		if err := value.Decode(&ext); err != nil {
			return err
		}
		d.Version = ext.Version
		d.Registry = ext.Registry
		return nil
	}

	return fmt.Errorf("dependency entry: expected string or mapping, got %v", value.Kind)
}

// ParseManifest reads a manifest.yaml file and returns the parsed dependency
// and registry configuration sections.
func ParseManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dependency: read manifest %q: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("dependency: parse manifest %q: %w", path, err)
	}

	if err := m.Compiler.Validate(); err != nil {
		return nil, fmt.Errorf("dependency: manifest %q: %w", path, err)
	}

	return &m, nil
}

// ParseManifestDependencies reads the dependencies section from a
// manifest.yaml file. Supports both short and extended dependency formats.
func ParseManifestDependencies(path string) ([]ManifestDependency, error) {
	m, err := ParseManifest(path)
	if err != nil {
		return nil, err
	}

	var deps []ManifestDependency
	for name, entry := range m.Dependencies {
		deps = append(deps, ManifestDependency{
			Name:     name,
			Version:  entry.Version,
			Registry: entry.Registry,
		})
	}

	return deps, nil
}

// ParseManifestRegistries reads the registries section from a manifest.yaml file.
func ParseManifestRegistries(path string) ([]RegistryConfig, error) {
	m, err := ParseManifest(path)
	if err != nil {
		return nil, err
	}

	return m.Registries, nil
}
