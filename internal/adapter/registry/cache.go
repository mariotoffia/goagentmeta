package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// DiskCache provides a local disk cache for fetched packages. Cache entries
// are stored at {dir}/{safeName}@{version}/.
type DiskCache struct {
	dir string
}

// NewDiskCache creates a cache rooted at dir. If dir is empty,
// DefaultCacheDir is used.
func NewDiskCache(dir string) *DiskCache {
	if dir == "" {
		dir = DefaultCacheDir()
	}
	return &DiskCache{dir: dir}
}

// DefaultCacheDir returns the default cache directory. It honours
// $XDG_CACHE_HOME and falls back to ~/.cache/goagentmeta/packages/.
func DefaultCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "goagentmeta", "packages")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}

	return filepath.Join(home, ".cache", "goagentmeta", "packages")
}

// Dir returns the cache root directory.
func (c *DiskCache) Dir() string { return c.dir }

// CacheKey returns the filesystem-safe cache key for a package.
func CacheKey(name, version string) string {
	safe := strings.ReplaceAll(name, "/", "__")
	return safe + "@" + version
}

// Get returns cached package contents if available. The second return value
// indicates whether the cache entry exists.
func (c *DiskCache) Get(name, version string) (portregistry.PackageContents, bool) {
	key := CacheKey(name, version)
	dir := filepath.Join(c.dir, key)

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return portregistry.PackageContents{}, false
	}

	return portregistry.PackageContents{
		Package: portregistry.ResolvedPackage{
			Name:    name,
			Version: version,
		},
		RootDir: dir,
	}, true
}

// Put stores package contents in the cache by recursively copying the source
// directory into the cache location.
func (c *DiskCache) Put(pkg portregistry.ResolvedPackage, srcDir string) error {
	key := CacheKey(pkg.Name, pkg.Version)
	dst := filepath.Join(c.dir, key)

	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("registry: cache remove %q: %w", dst, err)
	}
	if err := copyDir(srcDir, dst); err != nil {
		return fmt.Errorf("registry: cache put %q: %w", key, err)
	}

	return nil
}

// Invalidate removes a cached package entry.
func (c *DiskCache) Invalidate(name, version string) error {
	key := CacheKey(name, version)
	dir := filepath.Join(c.dir, key)

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("registry: cache invalidate %q: %w", key, err)
	}

	return nil
}

// copyDir recursively copies src to dst, preserving directory structure.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		return os.WriteFile(target, data, 0o644)
	})
}
