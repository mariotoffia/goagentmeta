package registry

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// Compile-time interface checks.
var (
	_ portregistry.PackageResolver = (*GitRegistry)(nil)
	_ portregistry.PackageFetcher  = (*GitRegistry)(nil)
)

// GitRegistry resolves and fetches packages from git repositories. Package
// names follow the URI format host/org/repo (e.g. "github.com/acme/skill").
// Versions map to git tags (with optional "v" prefix).
type GitRegistry struct {
	cacheDir    string
	urlResolver func(string) (string, error)
}

// GitOption configures a GitRegistry.
type GitOption func(*GitRegistry)

// WithGitURLResolver sets a custom function for resolving package names to
// git clone URLs. Primarily for testing with local repositories.
func WithGitURLResolver(fn func(string) (string, error)) GitOption {
	return func(r *GitRegistry) { r.urlResolver = fn }
}

// NewGitRegistry creates a git-based registry client. Fetched repos are
// cached under cacheDir.
func NewGitRegistry(cacheDir string, opts ...GitOption) *GitRegistry {
	if cacheDir == "" {
		cacheDir = filepath.Join(DefaultCacheDir(), "git")
	}
	r := &GitRegistry{
		cacheDir:    cacheDir,
		urlResolver: gitRepoURL,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve lists remote tags and finds the best version matching the
// constraint.
func (r *GitRegistry) Resolve(
	ctx context.Context,
	name string,
	constraint portregistry.VersionConstraint,
) (portregistry.ResolvedPackage, error) {
	repoURL, err := r.urlResolver(name)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: git resolve %q: %w", name, err)
	}

	tags, err := r.listTags(ctx, repoURL)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: git resolve %q: %w", name, err)
	}

	var candidates []Version
	for _, tag := range tags {
		v, err := ParseVersion(tag)
		if err != nil {
			continue
		}
		candidates = append(candidates, v)
	}

	best, found, err := BestMatch(candidates, constraint)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: git resolve %q: %w", name, err)
	}
	if !found {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: git resolve %q: no tag matching %q", name, constraint.Raw)
	}

	return portregistry.ResolvedPackage{
		Name:     name,
		Version:  best.String(),
		Registry: "git",
	}, nil
}

// Fetch clones the repository at the matching tag into the cache directory.
// If the clone already exists, it is returned directly.
func (r *GitRegistry) Fetch(
	ctx context.Context,
	pkg portregistry.ResolvedPackage,
) (portregistry.PackageContents, error) {
	repoURL, err := r.urlResolver(pkg.Name)
	if err != nil {
		return portregistry.PackageContents{},
			fmt.Errorf("registry: git fetch %q: %w", pkg.Name, err)
	}

	key := CacheKey(pkg.Name, pkg.Version)
	cloneDir := filepath.Join(r.cacheDir, key)

	// Return cached clone if it exists.
	if info, statErr := os.Stat(cloneDir); statErr == nil && info.IsDir() {
		return portregistry.PackageContents{Package: pkg, RootDir: cloneDir}, nil
	}

	// Try tag with "v" prefix first, then without.
	tag := "v" + pkg.Version
	if err := r.cloneAtTag(ctx, repoURL, tag, cloneDir); err != nil {
		tag = pkg.Version
		if err := r.cloneAtTag(ctx, repoURL, tag, cloneDir); err != nil {
			// Clean up any partial clone to prevent poisoned cache.
			os.RemoveAll(cloneDir)
			return portregistry.PackageContents{},
				fmt.Errorf("registry: git fetch %q@%s: %w", pkg.Name, pkg.Version, err)
		}
	}

	return portregistry.PackageContents{Package: pkg, RootDir: cloneDir}, nil
}

func (r *GitRegistry) listTags(ctx context.Context, repoURL string) ([]string, error) {
	out, err := gitExec(ctx, "", "ls-remote", "--tags", "--refs", repoURL)
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		tag := strings.TrimPrefix(parts[1], "refs/tags/")
		tags = append(tags, tag)
	}

	return tags, nil
}

func (r *GitRegistry) cloneAtTag(ctx context.Context, repoURL, tag, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	_, err := gitExec(ctx, "", "clone", "--depth=1", "--branch", tag, repoURL, dst)
	return err
}

// gitRepoURL converts a package name (e.g. "github.com/org/repo") to a
// clone URL.
func gitRepoURL(name string) (string, error) {
	parts := strings.SplitN(name, "/", 3)
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid git package name %q: expected host/org/repo", name)
	}
	return "https://" + name + ".git", nil
}

// gitExec runs a git command and returns its stdout.
func gitExec(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(stderr.String()), err)
	}

	return stdout.String(), nil
}
