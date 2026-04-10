// Package registry defines port interfaces for package registry access.
// The compiler uses these ports to discover, resolve, fetch, and verify
// external dependency packages from one or more registries.
//
// Registries may be public (community-operated), organizational (private),
// or git-based (resolved from git repositories by tag or commit). The
// compiler supports multiple registries in priority order.
package registry

import "context"

// PackageResolver resolves a package name and version constraint to an
// exact version from configured registries. Resolution respects registry
// priority order and profile trust policies.
type PackageResolver interface {
	// Resolve finds the best matching version for a package from configured
	// registries. Returns the resolved package metadata including exact
	// version, source registry, and integrity hash.
	Resolve(ctx context.Context, name string, constraint VersionConstraint) (ResolvedPackage, error)
}

// PackageFetcher downloads or retrieves a resolved package's contents.
// Fetched packages may be cached locally for subsequent builds.
type PackageFetcher interface {
	// Fetch retrieves the contents of a resolved package. If the package
	// is already cached and the integrity hash matches, it returns the
	// cached contents.
	Fetch(ctx context.Context, pkg ResolvedPackage) (PackageContents, error)
}

// PackageSearcher searches configured registries for packages matching
// a query. Used by CLI commands (ai-build search) and editor tooling.
type PackageSearcher interface {
	// Search finds packages matching the query across configured registries.
	Search(ctx context.Context, query string) ([]PackageMetadata, error)
}

// IntegrityVerifier verifies that a package's contents match its declared
// integrity hash. Hash mismatch fails unconditionally — this is not subject
// to preservation level.
type IntegrityVerifier interface {
	// Verify checks that the package contents match the expected hash.
	// Returns an error if the hash does not match.
	Verify(pkg ResolvedPackage, contents PackageContents) error
}

// VersionConstraint represents a semver version constraint for package
// resolution (e.g., "^1.3.0", "~2.1.0", ">=1.0.0 <3.0.0").
type VersionConstraint struct {
	// Raw is the original constraint string.
	Raw string
}

// ResolvedPackage holds the resolution result for a single package.
type ResolvedPackage struct {
	// Name is the fully qualified package name (e.g., "@acme/go-lambda-skill").
	Name string
	// Version is the exact resolved version.
	Version string
	// Registry is the name of the registry that provided this package.
	Registry string
	// IntegrityHash is the content hash for verification (e.g., "sha256:a1b2c3...").
	IntegrityHash string
	// Publisher is the identity that published this package.
	Publisher string
}

// PackageContents holds the fetched contents of a resolved package.
type PackageContents struct {
	// Package is the resolved package metadata.
	Package ResolvedPackage
	// RootDir is the local filesystem path to the package's extracted contents.
	RootDir string
}

// PackageMetadata holds discovery metadata for a package from a registry search.
type PackageMetadata struct {
	// Name is the fully qualified package name.
	Name string
	// Version is the latest available version.
	Version string
	// Publisher is the publisher identity.
	Publisher string
	// Description is a human-readable package summary.
	Description string
	// Targets lists the supported target ecosystems.
	Targets []string
	// Capabilities lists the capability IDs this package provides.
	Capabilities []string
	// License is the package license identifier.
	License string
	// Verified indicates whether the publisher is verified.
	Verified bool
}
