package registry

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// Compile-time interface checks.
var (
	_ portregistry.PackageResolver = (*LocalRegistry)(nil)
	_ portregistry.PackageFetcher  = (*LocalRegistry)(nil)
)

// LocalRegistry resolves and fetches packages from a local filesystem
// directory tree. Primarily for development and testing.
//
// Directory layout:
//
//	rootDir/
//	  <package-name>/
//	    <version>/
//	      package.yaml
//	      objects/
//	        ...
type LocalRegistry struct {
	rootDir string
}

// NewLocalRegistry creates a local registry backed by rootDir.
func NewLocalRegistry(rootDir string) *LocalRegistry {
	return &LocalRegistry{rootDir: rootDir}
}

// Resolve scans local directories to find the best version matching the
// constraint. The integrity hash is computed from the package directory
// contents.
func (r *LocalRegistry) Resolve(
	ctx context.Context,
	name string,
	constraint portregistry.VersionConstraint,
) (portregistry.ResolvedPackage, error) {
	if err := ctx.Err(); err != nil {
		return portregistry.ResolvedPackage{}, fmt.Errorf("registry: local resolve: %w", err)
	}

	pkgDir := filepath.Join(r.rootDir, name)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: local resolve %q: %w", name, err)
	}

	var candidates []Version
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v, err := ParseVersion(e.Name())
		if err != nil {
			continue
		}
		candidates = append(candidates, v)
	}

	best, found, err := BestMatch(candidates, constraint)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: local resolve %q: %w", name, err)
	}
	if !found {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: local resolve %q: no version matching %q", name, constraint.Raw)
	}

	versionDir := filepath.Join(pkgDir, best.String())
	manifest, err := parsePackageManifest(versionDir)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: local resolve %q: %w", name, err)
	}

	hash, err := ComputeIntegrityHash(versionDir)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: local resolve %q: %w", name, err)
	}

	return portregistry.ResolvedPackage{
		Name:          name,
		Version:       best.String(),
		Registry:      "local",
		IntegrityHash: hash,
		Publisher:     manifest.publisher,
	}, nil
}

// Fetch returns the package contents from the local directory. The RootDir
// points directly to the versioned package directory.
func (r *LocalRegistry) Fetch(
	ctx context.Context,
	pkg portregistry.ResolvedPackage,
) (portregistry.PackageContents, error) {
	if err := ctx.Err(); err != nil {
		return portregistry.PackageContents{}, fmt.Errorf("registry: local fetch: %w", err)
	}

	versionDir := filepath.Join(r.rootDir, pkg.Name, pkg.Version)
	info, err := os.Stat(versionDir)
	if err != nil || !info.IsDir() {
		return portregistry.PackageContents{},
			fmt.Errorf("registry: local fetch %q@%s: directory not found", pkg.Name, pkg.Version)
	}

	return portregistry.PackageContents{
		Package: pkg,
		RootDir: versionDir,
	}, nil
}

// packageManifest holds metadata parsed from a package.yaml file.
type packageManifest struct {
	name        string
	version     string
	publisher   string
	description string
}

// parsePackageManifest reads a simple key: value YAML file from the given
// directory. Only single-line scalar values are supported.
func parsePackageManifest(dir string) (packageManifest, error) {
	path := filepath.Join(dir, "package.yaml")
	f, err := os.Open(path)
	if err != nil {
		return packageManifest{}, fmt.Errorf("open manifest: %w", err)
	}
	defer f.Close()

	var m packageManifest
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "name":
			m.name = value
		case "version":
			m.version = value
		case "publisher":
			m.publisher = value
		case "description":
			m.description = value
		}
	}

	return m, scanner.Err()
}
