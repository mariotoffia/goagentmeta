package pipeline

import "github.com/mariotoffia/goagentmeta/internal/domain/model"

// SourceTree is the aggregate root produced by the parse phase. It contains
// the raw parsed representation of all files under the .ai/ source tree,
// before normalization, dependency resolution, or semantic analysis.
type SourceTree struct {
	// RootPath is the absolute path to the .ai/ directory.
	RootPath string

	// Objects holds every parsed canonical object with raw field values.
	Objects []RawObject

	// SchemaVersion is the schema version declared in manifest.yaml.
	SchemaVersion int

	// ManifestPath is the path to the manifest.yaml file.
	ManifestPath string
}

// RawObject is a single parsed canonical object before normalization.
// It carries the raw YAML/Markdown fields and the source file path.
type RawObject struct {
	// Meta holds the parsed common envelope fields.
	Meta model.ObjectMeta

	// SourcePath is the filesystem path where this object was defined.
	SourcePath string

	// RawContent holds the unparsed content body (markdown, YAML fragments).
	RawContent string

	// RawFields holds arbitrary YAML fields not captured by ObjectMeta.
	// This allows forward compatibility with schema extensions.
	RawFields map[string]any
}

// SemanticGraph is the normalized representation of the source tree.
// It has resolved object IDs, inheritance chains, scope selectors,
// relative paths, and default values. Renderers consume this IR,
// never the raw source.
type SemanticGraph struct {
	// Objects holds all normalized canonical objects indexed by ID.
	Objects map[string]NormalizedObject

	// InheritanceChains maps object IDs to their resolved inheritance chain
	// (ordered from most specific to most general).
	InheritanceChains map[string][]string

	// ScopeIndex maps scope paths to the object IDs that apply at that path.
	ScopeIndex map[string][]string
}

// NormalizedObject is a canonical object after normalization. All inheritance
// is resolved, defaults are applied, and references are validated.
type NormalizedObject struct {
	// Meta holds the fully resolved metadata.
	Meta model.ObjectMeta

	// SourcePath is the original source file path.
	SourcePath string

	// Content holds the resolved content (with inheritance applied).
	Content string

	// ResolvedFields holds all fields after inheritance and default resolution.
	ResolvedFields map[string]any
}
