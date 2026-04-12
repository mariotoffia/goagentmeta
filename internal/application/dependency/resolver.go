package dependency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
	"gopkg.in/yaml.v3"
)

// ResolutionError extends CompilerError with per-package detail.
type ResolutionError struct {
	*pipeline.CompilerError
	// Package is the package name that caused the error.
	Package string
}

// VersionSatisfier checks whether a resolved version satisfies a version
// constraint. This is injected to avoid the application layer depending
// on adapter-layer semver logic.
type VersionSatisfier func(version string, constraint string) (bool, error)

// DependencyResolver is the application-layer use case for resolving
// external package dependencies. It coordinates between registries,
// the integrity verifier, and the local cache to produce a SourceTree
// with external objects merged in.
type DependencyResolver struct {
	resolver  portregistry.PackageResolver
	fetcher   portregistry.PackageFetcher
	verifier  portregistry.IntegrityVerifier
	cache     Cache
	satisfier VersionSatisfier
	nowFunc   func() time.Time
}

// Cache abstracts the local package cache for testing.
type Cache interface {
	Get(name, version string) (portregistry.PackageContents, bool)
	Put(pkg portregistry.ResolvedPackage, srcDir string) error
}

// Option configures a DependencyResolver.
type Option func(*DependencyResolver)

// WithCache sets a custom cache implementation.
func WithCache(c Cache) Option {
	return func(r *DependencyResolver) { r.cache = c }
}

// WithNowFunc sets a custom time function (for deterministic tests).
func WithNowFunc(fn func() time.Time) Option {
	return func(r *DependencyResolver) { r.nowFunc = fn }
}

// WithVersionSatisfier sets a custom version constraint checker.
// When nil, locked versions are always assumed to satisfy the constraint.
func WithVersionSatisfier(fn VersionSatisfier) Option {
	return func(r *DependencyResolver) { r.satisfier = fn }
}

// NewDependencyResolver creates a new resolver with the given port adapters.
func NewDependencyResolver(
	resolver portregistry.PackageResolver,
	fetcher portregistry.PackageFetcher,
	verifier portregistry.IntegrityVerifier,
	opts ...Option,
) *DependencyResolver {
	dr := &DependencyResolver{
		resolver: resolver,
		fetcher:  fetcher,
		verifier: verifier,
		nowFunc:  time.Now,
	}
	for _, opt := range opts {
		opt(dr)
	}
	return dr
}

// Resolve processes all dependencies and merges external objects into the
// SourceTree. It reads the lock file, resolves missing or updated packages,
// verifies integrity, and updates the lock file when new resolutions occur.
func (dr *DependencyResolver) Resolve(
	ctx context.Context,
	tree pipeline.SourceTree,
	deps []ManifestDependency,
) (pipeline.SourceTree, error) {
	if len(deps) == 0 {
		return tree, nil
	}

	lockPath := LockFilePath(tree.RootPath)
	lockFile, err := ReadLockFile(lockPath)
	if err != nil {
		return tree, &ResolutionError{
			CompilerError: pipeline.Wrap(
				pipeline.ErrResolution,
				fmt.Sprintf("failed to read lock file: %v", err),
				"dependency-resolver",
				err,
			),
			Package: "",
		}
	}

	lockChanged := false

	for _, dep := range deps {
		if err := ctx.Err(); err != nil {
			return tree, err
		}

		resolved, contents, changed, resolveErr := dr.resolveOne(ctx, dep, lockFile)
		if resolveErr != nil {
			return tree, resolveErr
		}

		if changed {
			lockChanged = true
		}

		// Parse objects from the fetched package and merge into tree.
		externalObjects, parseErr := dr.parseExternalObjects(contents, resolved)
		if parseErr != nil {
			return tree, &ResolutionError{
				CompilerError: pipeline.Wrap(
					pipeline.ErrResolution,
					fmt.Sprintf("failed to parse external package %q: %v", dep.Name, parseErr),
					dep.Name,
					parseErr,
				),
				Package: dep.Name,
			}
		}

		tree.Objects = append(tree.Objects, externalObjects...)
	}

	if lockChanged {
		if err := WriteLockFile(lockPath, lockFile); err != nil {
			return tree, &ResolutionError{
				CompilerError: pipeline.Wrap(
					pipeline.ErrResolution,
					fmt.Sprintf("failed to write lock file: %v", err),
					"dependency-resolver",
					err,
				),
			}
		}
	}

	return tree, nil
}

// resolveOne resolves a single dependency. It checks the lock file and cache
// before hitting the registry. Returns the resolved package, its contents,
// whether the lock was changed, and any error.
func (dr *DependencyResolver) resolveOne(
	ctx context.Context,
	dep ManifestDependency,
	lockFile *LockFile,
) (portregistry.ResolvedPackage, portregistry.PackageContents, bool, error) {
	constraint := portregistry.VersionConstraint{Raw: dep.Version}

	// Check lock file for an existing resolution.
	if entry := lockFile.FindEntry(dep.Name); entry != nil {
		resolved, contents, err := dr.resolveLocked(ctx, dep, entry)
		if err == nil {
			return resolved, contents, false, nil
		}

		// If the locked version is stale (constraint mismatch), fall through
		// to fresh resolution. Any other error is returned as-is.
		if !isStaleConstraint(err) {
			return portregistry.ResolvedPackage{}, portregistry.PackageContents{}, false, err
		}
	}

	return dr.resolveFresh(ctx, dep, lockFile, constraint)
}

// resolveLocked attempts to use a lock file entry, verifying the constraint
// is still satisfied. Returns errStaleConstraint if re-resolution is needed.
func (dr *DependencyResolver) resolveLocked(
	ctx context.Context,
	dep ManifestDependency,
	entry *LockEntry,
) (portregistry.ResolvedPackage, portregistry.PackageContents, error) {
	// Verify the locked version still satisfies the declared constraint.
	if dr.satisfier != nil {
		ok, err := dr.satisfier(entry.Version, dep.Version)
		if err != nil {
			return portregistry.ResolvedPackage{}, portregistry.PackageContents{},
				&ResolutionError{
					CompilerError: pipeline.Wrap(
						pipeline.ErrResolution,
						fmt.Sprintf("constraint check failed for %q: locked=%s, constraint=%s",
							dep.Name, entry.Version, dep.Version),
						dep.Name, err,
					),
					Package: dep.Name,
				}
		}
		if !ok {
			return portregistry.ResolvedPackage{}, portregistry.PackageContents{}, errStale
		}
	}

	resolved := portregistry.ResolvedPackage{
		Name:          dep.Name,
		Version:       entry.Version,
		Registry:      entry.Registry,
		IntegrityHash: entry.Digest,
	}

	// Try cache first.
	if dr.cache != nil {
		if contents, ok := dr.cache.Get(dep.Name, entry.Version); ok {
			if err := dr.verifier.Verify(resolved, contents); err != nil {
				return portregistry.ResolvedPackage{}, portregistry.PackageContents{},
					&ResolutionError{
						CompilerError: pipeline.NewCompilerError(
							pipeline.ErrResolution,
							fmt.Sprintf(
								"integrity mismatch for %q: expected=%s, actual hash differs",
								dep.Name, entry.Digest,
							),
							dep.Name,
						),
						Package: dep.Name,
					}
			}
			return resolved, contents, nil
		}
	}

	// Lock file hit but cache miss — fetch using locked version directly.
	contents, err := dr.fetcher.Fetch(ctx, resolved)
	if err != nil {
		return portregistry.ResolvedPackage{}, portregistry.PackageContents{},
			&ResolutionError{
				CompilerError: pipeline.Wrap(
					pipeline.ErrResolution,
					fmt.Sprintf("failed to fetch locked %q@%s", dep.Name, entry.Version),
					dep.Name, err,
				),
				Package: dep.Name,
			}
	}

	if err := dr.verifier.Verify(resolved, contents); err != nil {
		return portregistry.ResolvedPackage{}, portregistry.PackageContents{},
			&ResolutionError{
				CompilerError: pipeline.NewCompilerError(
					pipeline.ErrResolution,
					fmt.Sprintf(
						"integrity mismatch for %q: expected=%s",
						dep.Name, resolved.IntegrityHash,
					),
					dep.Name,
				),
				Package: dep.Name,
			}
	}

	// Cache the fetched contents for next time.
	if dr.cache != nil {
		_ = dr.cache.Put(resolved, contents.RootDir)
	}

	return resolved, contents, nil
}

// errStale is a sentinel used internally to signal that a lock file entry
// no longer satisfies the declared constraint and fresh resolution is needed.
var errStale = fmt.Errorf("stale constraint")

func isStaleConstraint(err error) bool { return err == errStale }

// resolveFresh resolves a dependency from the registry (no lock file hit).
func (dr *DependencyResolver) resolveFresh(
	ctx context.Context,
	dep ManifestDependency,
	lockFile *LockFile,
	constraint portregistry.VersionConstraint,
) (portregistry.ResolvedPackage, portregistry.PackageContents, bool, error) {
	resolved, err := dr.resolver.Resolve(ctx, dep.Name, constraint)
	if err != nil {
		return portregistry.ResolvedPackage{}, portregistry.PackageContents{}, false,
			&ResolutionError{
				CompilerError: pipeline.Wrap(
					pipeline.ErrResolution,
					fmt.Sprintf("failed to resolve %q with constraint %q", dep.Name, dep.Version),
					dep.Name,
					err,
				),
				Package: dep.Name,
			}
	}

	// Fetch package contents.
	contents, err := dr.fetcher.Fetch(ctx, resolved)
	if err != nil {
		return portregistry.ResolvedPackage{}, portregistry.PackageContents{}, false,
			&ResolutionError{
				CompilerError: pipeline.Wrap(
					pipeline.ErrResolution,
					fmt.Sprintf("failed to fetch %q@%s", dep.Name, resolved.Version),
					dep.Name,
					err,
				),
				Package: dep.Name,
			}
	}

	// Verify integrity.
	if err := dr.verifier.Verify(resolved, contents); err != nil {
		return portregistry.ResolvedPackage{}, portregistry.PackageContents{}, false,
			&ResolutionError{
				CompilerError: pipeline.NewCompilerError(
					pipeline.ErrResolution,
					fmt.Sprintf(
						"integrity mismatch for %q: expected=%s",
						dep.Name, resolved.IntegrityHash,
					),
					dep.Name,
				),
				Package: dep.Name,
			}
	}

	// Cache the fetched contents.
	if dr.cache != nil {
		if err := dr.cache.Put(resolved, contents.RootDir); err != nil {
			// Cache put failure is not fatal — log and continue.
			_ = err
		}
	}

	// Update lock file.
	lockFile.SetEntry(LockEntry{
		Name:       dep.Name,
		Version:    resolved.Version,
		Registry:   resolved.Registry,
		Digest:     resolved.IntegrityHash,
		ResolvedAt: dr.nowFunc(),
	})

	return resolved, contents, true, nil
}

// parseExternalObjects scans the package contents directory for object files
// and creates RawObjects marked as external.
func (dr *DependencyResolver) parseExternalObjects(
	contents portregistry.PackageContents,
	resolved portregistry.ResolvedPackage,
) ([]pipeline.RawObject, error) {
	objectsDir := filepath.Join(contents.RootDir, "objects")
	entries, err := os.ReadDir(objectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read objects dir: %w", err)
	}

	var objects []pipeline.RawObject
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(objectsDir, e.Name())
		obj, err := parseRawObjectFile(path, resolved)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", e.Name(), err)
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// parseRawObjectFile reads a single YAML object file and creates a RawObject
// with basic metadata extraction and external provenance marking.
func parseRawObjectFile(
	path string,
	resolved portregistry.ResolvedPackage,
) (pipeline.RawObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pipeline.RawObject{}, fmt.Errorf("read file: %w", err)
	}

	var fields map[string]any
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return pipeline.RawObject{}, fmt.Errorf("parse YAML: %w", err)
	}
	if fields == nil {
		fields = make(map[string]any)
	}

	var meta model.ObjectMeta
	if id, ok := fields["id"].(string); ok {
		meta.ID = id
	}
	if kind, ok := fields["kind"].(string); ok {
		meta.Kind = model.Kind(kind)
	}
	if desc, ok := fields["description"].(string); ok {
		meta.Description = desc
	}

	// Mark as external with provenance.
	fields["_external"] = true
	fields["_source_package"] = resolved.Name
	fields["_source_version"] = resolved.Version
	fields["_source_registry"] = resolved.Registry

	return pipeline.RawObject{
		Meta:       meta,
		SourcePath: path,
		RawContent: string(data),
		RawFields:  fields,
	}, nil
}
