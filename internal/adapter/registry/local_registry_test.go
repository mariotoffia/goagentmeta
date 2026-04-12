package registry_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

func TestLocalRegistry_Resolve(t *testing.T) {
	root := setupLocalRegistryDir(t)
	reg := registry.NewLocalRegistry(root)

	tests := []struct {
		name       string
		constraint string
		wantVer    string
		wantErr    bool
	}{
		{"any version", "*", "2.0.0", false},
		{"caret 1.x", "^1.0.0", "1.0.0", false},
		{"exact 2.0.0", "2.0.0", "2.0.0", false},
		{"exact 1.0.0", "1.0.0", "1.0.0", false},
		{"no match", "3.0.0", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := portregistry.VersionConstraint{Raw: tt.constraint}
			pkg, err := reg.Resolve(context.Background(), "test-package", c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if pkg.Version != tt.wantVer {
				t.Errorf("Resolve version = %s, want %s", pkg.Version, tt.wantVer)
			}
			if pkg.Registry != "local" {
				t.Errorf("Registry = %q, want %q", pkg.Registry, "local")
			}
			if pkg.Publisher != "test-publisher" {
				t.Errorf("Publisher = %q, want %q", pkg.Publisher, "test-publisher")
			}
			if pkg.IntegrityHash == "" {
				t.Error("IntegrityHash should not be empty")
			}
		})
	}
}

func TestLocalRegistry_Resolve_NonexistentPackage(t *testing.T) {
	root := mustTempDir(t)
	reg := registry.NewLocalRegistry(root)

	_, err := reg.Resolve(context.Background(), "nonexistent",
		portregistry.VersionConstraint{Raw: "*"})
	if err == nil {
		t.Error("expected error for non-existent package, got nil")
	}
}

func TestLocalRegistry_Resolve_ContextCancelled(t *testing.T) {
	root := setupLocalRegistryDir(t)
	reg := registry.NewLocalRegistry(root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := reg.Resolve(ctx, "test-package",
		portregistry.VersionConstraint{Raw: "*"})
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestLocalRegistry_Fetch(t *testing.T) {
	root := setupLocalRegistryDir(t)
	reg := registry.NewLocalRegistry(root)

	pkg := portregistry.ResolvedPackage{Name: "test-package", Version: "1.0.0"}
	contents, err := reg.Fetch(context.Background(), pkg)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if contents.RootDir == "" {
		t.Error("RootDir should not be empty")
	}

	// Verify expected file exists.
	_, err = os.Stat(filepath.Join(contents.RootDir, "objects", "test-instruction.yaml"))
	if err != nil {
		t.Errorf("expected test-instruction.yaml in fetched package: %v", err)
	}

	// Verify package metadata propagated.
	if contents.Package.Name != "test-package" {
		t.Errorf("Package.Name = %q, want %q", contents.Package.Name, "test-package")
	}
}

func TestLocalRegistry_Fetch_NotFound(t *testing.T) {
	root := mustTempDir(t)
	reg := registry.NewLocalRegistry(root)

	pkg := portregistry.ResolvedPackage{Name: "nonexistent", Version: "1.0.0"}
	_, err := reg.Fetch(context.Background(), pkg)
	if err == nil {
		t.Error("expected error for non-existent package, got nil")
	}
}

func TestLocalRegistry_Fetch_ContextCancelled(t *testing.T) {
	root := setupLocalRegistryDir(t)
	reg := registry.NewLocalRegistry(root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pkg := portregistry.ResolvedPackage{Name: "test-package", Version: "1.0.0"}
	_, err := reg.Fetch(ctx, pkg)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

// Resolve → Fetch round-trip with integrity verification.
func TestLocalRegistry_ResolveAndFetchWithIntegrity(t *testing.T) {
	root := setupLocalRegistryDir(t)
	reg := registry.NewLocalRegistry(root)
	verifier := registry.NewSHA256Verifier()

	pkg, err := reg.Resolve(context.Background(), "test-package",
		portregistry.VersionConstraint{Raw: "^1.0.0"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	contents, err := reg.Fetch(context.Background(), pkg)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if err := verifier.Verify(pkg, contents); err != nil {
		t.Errorf("integrity verification failed: %v", err)
	}
}

// Compile-time interface checks.
var (
	_ portregistry.PackageResolver = (*registry.LocalRegistry)(nil)
	_ portregistry.PackageFetcher  = (*registry.LocalRegistry)(nil)
)
