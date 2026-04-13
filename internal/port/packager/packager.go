// Package packager defines the port interface for packaging compiled output
// into distributable artifacts. Packagers transform emission plans and
// materialized files into target-native distribution formats such as VS Code
// extensions (.vsix), npm packages (.tgz), Claude Code plugins, or OCI images.
//
// This package is a hexagonal port: it defines contracts that adapters implement.
// New packaging formats are added by implementing the Packager interface and
// registering with a PackagerRegistry — no changes to the core pipeline are needed.
package packager

import (
	"context"
	"fmt"
	"sync"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// Format identifies a packaging output format. It is a semi-open string enum:
// well-known values are provided as constants, but custom values are permitted
// to support future or third-party packaging formats without code changes.
type Format string

const (
	// FormatVSIX produces a VS Code extension archive (.vsix).
	FormatVSIX Format = "vsix"
	// FormatNPM produces an npm package tarball (.tgz).
	FormatNPM Format = "npm"
	// FormatOCI produces an OCI container image artifact.
	FormatOCI Format = "oci"
	// FormatClaudePlugin produces a distributable Claude Code plugin directory.
	FormatClaudePlugin Format = "claude-plugin"
	// FormatMarketplace produces a marketplace catalog (e.g., marketplace.json).
	FormatMarketplace Format = "marketplace"
)

// Packager transforms compiled emission plans and materialized files into a
// distributable artifact in a specific format.
type Packager interface {
	// Format returns the packaging format this packager produces.
	Format() Format

	// Targets returns the build targets this packager supports. A nil or empty
	// slice means the packager is target-agnostic.
	Targets() []build.Target

	// Package produces distributable artifacts from the emission plan and
	// materialized files. The Config field in PackagerInput carries a
	// packager-specific configuration struct that each implementation
	// type-asserts internally.
	Package(ctx context.Context, input PackagerInput) (*PackagerOutput, error)
}

// PackagerInput provides everything a packager needs to produce artifacts.
type PackagerInput struct {
	// EmissionPlan is the structured compilation output containing files,
	// plugin bundles, install metadata, and per-unit coordinates.
	EmissionPlan pipeline.EmissionPlan

	// MaterializationResult lists the files and directories that were
	// actually written to disk during materialization.
	MaterializationResult pipeline.MaterializationResult

	// Config carries a packager-specific configuration struct. Each Packager
	// implementation type-asserts this to its own config type. Passing nil
	// means "use defaults".
	Config any

	// OutputDir is the base directory where artifacts should be written.
	OutputDir string
}

// PackagerOutput holds the artifacts produced by a packager.
type PackagerOutput struct {
	Artifacts []PackagedArtifact
}

// PackagedArtifact describes a single distributable artifact produced by a
// packager. The Metadata map provides an open extension point for
// format-specific key-value data (same pattern as model.TargetOverride.Extra).
type PackagedArtifact struct {
	// Format identifies which packager produced this artifact.
	Format Format

	// Path is the filesystem path to the produced artifact.
	Path string

	// Targets lists the build targets included in this artifact.
	Targets []build.Target

	// Metadata carries format-specific key-value data. Examples:
	// "publisher" for VSIX, "scope" for npm, "pluginName" for Claude plugins.
	Metadata map[string]string
}

// PackagerRegistry manages packager registrations and provides lookup by format.
// Implementations must be safe for concurrent use.
type PackagerRegistry interface {
	// Register adds a packager to the registry. Returns an error if a packager
	// for the same format is already registered.
	Register(p Packager) error

	// MustRegister is like Register but panics on error.
	MustRegister(p Packager)

	// ByFormat returns the packager registered for the given format, or false
	// if none is registered.
	ByFormat(f Format) (Packager, bool)

	// All returns all registered packagers in registration order.
	All() []Packager
}

// DefaultRegistry is a thread-safe in-memory PackagerRegistry.
type DefaultRegistry struct {
	mu       sync.RWMutex
	idx      map[Format]int
	packgers []Packager
}

// NewRegistry creates an empty DefaultRegistry.
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		idx: make(map[Format]int),
	}
}

// Register adds a packager. Returns an error if the format is already registered.
func (r *DefaultRegistry) Register(p Packager) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f := p.Format()
	if _, exists := r.idx[f]; exists {
		return fmt.Errorf("packager already registered for format %q", f)
	}

	r.idx[f] = len(r.packgers)
	r.packgers = append(r.packgers, p)

	return nil
}

// MustRegister is like Register but panics on error.
func (r *DefaultRegistry) MustRegister(p Packager) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

// ByFormat returns the packager for the given format.
func (r *DefaultRegistry) ByFormat(f Format) (Packager, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	i, ok := r.idx[f]
	if !ok {
		return nil, false
	}

	return r.packgers[i], true
}

// All returns all registered packagers in registration order.
func (r *DefaultRegistry) All() []Packager {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Packager, len(r.packgers))
	copy(out, r.packgers)

	return out
}

// Compile-time assertion.
var _ PackagerRegistry = (*DefaultRegistry)(nil)
