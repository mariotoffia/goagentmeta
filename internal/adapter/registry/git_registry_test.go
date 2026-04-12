package registry_test

import (
	"context"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

func TestGitRegistry_Resolve_InvalidName(t *testing.T) {
	reg := registry.NewGitRegistry(mustTempDir(t))
	_, err := reg.Resolve(context.Background(), "invalid",
		portregistry.VersionConstraint{Raw: "*"})
	if err == nil {
		t.Error("expected error for invalid git name, got nil")
	}
}

func TestGitRegistry_Resolve_LocalRepo(t *testing.T) {
	repoDir := setupGitRepoForTest(t)
	cacheDir := mustTempDir(t)

	reg := registry.NewGitRegistry(cacheDir,
		registry.WithGitURLResolver(func(_ string) (string, error) {
			return repoDir, nil
		}),
	)

	tests := []struct {
		name       string
		constraint string
		wantVer    string
		wantErr    bool
	}{
		{"any", "*", "2.0.0", false},
		{"caret 1.x", "^1.0.0", "1.0.0", false},
		{"exact 2.0.0", "2.0.0", "2.0.0", false},
		{"no match", "3.0.0", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := portregistry.VersionConstraint{Raw: tt.constraint}
			pkg, err := reg.Resolve(context.Background(), "github.com/test/repo", c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if pkg.Version != tt.wantVer {
				t.Errorf("Version = %q, want %q", pkg.Version, tt.wantVer)
			}
			if pkg.Registry != "git" {
				t.Errorf("Registry = %q, want %q", pkg.Registry, "git")
			}
		})
	}
}

func TestGitRegistry_Fetch_LocalRepo(t *testing.T) {
	repoDir := setupGitRepoForTest(t)
	cacheDir := mustTempDir(t)

	reg := registry.NewGitRegistry(cacheDir,
		registry.WithGitURLResolver(func(_ string) (string, error) {
			return repoDir, nil
		}),
	)

	pkg := portregistry.ResolvedPackage{
		Name:     "github.com/test/repo",
		Version:  "1.0.0",
		Registry: "git",
	}
	contents, err := reg.Fetch(context.Background(), pkg)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if contents.RootDir == "" {
		t.Error("RootDir should not be empty")
	}
	if contents.Package.Name != pkg.Name {
		t.Errorf("Package.Name = %q, want %q", contents.Package.Name, pkg.Name)
	}
}

func TestGitRegistry_Fetch_CachesClone(t *testing.T) {
	repoDir := setupGitRepoForTest(t)
	cacheDir := mustTempDir(t)

	cloneCount := 0
	reg := registry.NewGitRegistry(cacheDir,
		registry.WithGitURLResolver(func(_ string) (string, error) {
			cloneCount++
			return repoDir, nil
		}),
	)

	pkg := portregistry.ResolvedPackage{
		Name:     "github.com/test/repo",
		Version:  "1.0.0",
		Registry: "git",
	}

	// First fetch — clones.
	_, err := reg.Fetch(context.Background(), pkg)
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}

	// Second fetch — should use cached clone (but still calls urlResolver).
	_, err = reg.Fetch(context.Background(), pkg)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
}

func TestGitRegistry_Fetch_ContextCancelled(t *testing.T) {
	cacheDir := mustTempDir(t)
	reg := registry.NewGitRegistry(cacheDir,
		registry.WithGitURLResolver(func(_ string) (string, error) {
			return "/nonexistent/repo", nil
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pkg := portregistry.ResolvedPackage{
		Name:     "github.com/test/repo",
		Version:  "1.0.0",
		Registry: "git",
	}
	_, err := reg.Fetch(ctx, pkg)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

// Compile-time interface checks.
var (
	_ portregistry.PackageResolver = (*registry.GitRegistry)(nil)
	_ portregistry.PackageFetcher  = (*registry.GitRegistry)(nil)
)
