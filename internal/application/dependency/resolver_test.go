package dependency_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// --- mocks ---

type mockResolver struct {
	resolveFunc func(ctx context.Context, name string, constraint portregistry.VersionConstraint) (portregistry.ResolvedPackage, error)
}

func (m *mockResolver) Resolve(ctx context.Context, name string, constraint portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
	return m.resolveFunc(ctx, name, constraint)
}

type mockFetcher struct {
	fetchFunc func(ctx context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error)
}

func (m *mockFetcher) Fetch(ctx context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
	return m.fetchFunc(ctx, pkg)
}

type mockVerifier struct {
	verifyFunc func(pkg portregistry.ResolvedPackage, contents portregistry.PackageContents) error
}

func (m *mockVerifier) Verify(pkg portregistry.ResolvedPackage, contents portregistry.PackageContents) error {
	return m.verifyFunc(pkg, contents)
}

type mockCache struct {
	store map[string]portregistry.PackageContents
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]portregistry.PackageContents)}
}

func (m *mockCache) Get(name, version string) (portregistry.PackageContents, bool) {
	key := name + "@" + version
	c, ok := m.store[key]
	return c, ok
}

func (m *mockCache) Put(pkg portregistry.ResolvedPackage, srcDir string) error {
	key := pkg.Name + "@" + pkg.Version
	m.store[key] = portregistry.PackageContents{
		Package: pkg,
		RootDir: srcDir,
	}
	return nil
}

// --- test helpers ---

func setupPackageDir(t *testing.T, rootDir, name, version string, objects map[string]string) string {
	t.Helper()
	pkgDir := filepath.Join(rootDir, "packages", name, version)
	objDir := filepath.Join(pkgDir, "objects")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(pkgDir, "package.yaml"),
		"name: "+name+"\nversion: "+version+"\npublisher: test\n")

	for fileName, content := range objects {
		writeFile(t, filepath.Join(objDir, fileName), content)
	}

	return pkgDir
}

func makeTree(rootDir, manifestPath string, objects ...pipeline.RawObject) pipeline.SourceTree {
	return pipeline.SourceTree{
		RootPath:      rootDir,
		Objects:       objects,
		SchemaVersion: 1,
		ManifestPath:  manifestPath,
	}
}

func fixedTime() time.Time {
	return time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
}

// --- tests ---

func TestResolver_NoDependencies(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `schemaVersion: 1
`)

	tree := makeTree(dir, manifest)

	resolver := dependency.NewDependencyResolver(
		&mockResolver{},
		&mockFetcher{},
		&mockVerifier{verifyFunc: func(pkg portregistry.ResolvedPackage, c portregistry.PackageContents) error {
			return nil
		}},
	)

	result, err := resolver.Resolve(context.Background(), tree, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(result.Objects) != 0 {
		t.Errorf("Objects len = %d, want 0", len(result.Objects))
	}
}

func TestResolver_LocalRegistry_MergesObjects(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "test-package", "1.0.0", map[string]string{
		"test-instruction.yaml": "kind: instruction\nid: ext-instruction\ndescription: from external\n",
	})

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-package: "^1.0.0"
`)

	tree := makeTree(dir, manifest)

	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{
				Name:          name,
				Version:       "1.0.0",
				Registry:      "local",
				IntegrityHash: "sha256:abc123",
			}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{
				Package: pkg,
				RootDir: pkgDir,
			}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
	)

	deps := []dependency.ManifestDependency{
		{Name: "test-package", Version: "^1.0.0"},
	}

	result, err := resolver.Resolve(context.Background(), tree, deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(result.Objects) != 1 {
		t.Fatalf("Objects len = %d, want 1", len(result.Objects))
	}

	obj := result.Objects[0]
	if obj.Meta.ID != "ext-instruction" {
		t.Errorf("Meta.ID = %q, want %q", obj.Meta.ID, "ext-instruction")
	}
	if obj.Meta.Kind != "instruction" {
		t.Errorf("Meta.Kind = %q, want %q", obj.Meta.Kind, "instruction")
	}

	// Check external provenance markers.
	if ext, ok := obj.RawFields["_external"].(bool); !ok || !ext {
		t.Error("_external marker not set")
	}
	if src, ok := obj.RawFields["_source_package"].(string); !ok || src != "test-package" {
		t.Errorf("_source_package = %v, want test-package", obj.RawFields["_source_package"])
	}
	if ver, ok := obj.RawFields["_source_version"].(string); !ok || ver != "1.0.0" {
		t.Errorf("_source_version = %v, want 1.0.0", obj.RawFields["_source_version"])
	}
}

func TestResolver_WritesLockFile(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "test-package", "1.0.0", map[string]string{
		"obj.yaml": "kind: instruction\nid: obj-1\n",
	})

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-package: "^1.0.0"
`)

	tree := makeTree(dir, manifest)

	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{
				Name:          name,
				Version:       "1.0.0",
				Registry:      "local",
				IntegrityHash: "sha256:abc123",
			}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
	)

	deps := []dependency.ManifestDependency{{Name: "test-package", Version: "^1.0.0"}}

	_, err := resolver.Resolve(context.Background(), tree, deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Verify lock file was written.
	lockPath := dependency.LockFilePath(dir)
	lf, err := dependency.ReadLockFile(lockPath)
	if err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}

	if len(lf.Dependencies) != 1 {
		t.Fatalf("lock entries = %d, want 1", len(lf.Dependencies))
	}

	entry := lf.Dependencies[0]
	if entry.Name != "test-package" {
		t.Errorf("entry.Name = %q, want test-package", entry.Name)
	}
	if entry.Version != "1.0.0" {
		t.Errorf("entry.Version = %q, want 1.0.0", entry.Version)
	}
	if entry.Digest != "sha256:abc123" {
		t.Errorf("entry.Digest = %q, want sha256:abc123", entry.Digest)
	}
}

func TestResolver_LockFileRead_SkipsResolution(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "test-package", "1.0.0", map[string]string{
		"obj.yaml": "kind: instruction\nid: cached-obj\n",
	})

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-package: "^1.0.0"
`)

	// Pre-create lock file.
	lockPath := dependency.LockFilePath(dir)
	lf := &dependency.LockFile{
		SchemaVersion: 1,
		Dependencies: []dependency.LockEntry{
			{
				Name:       "test-package",
				Version:    "1.0.0",
				Registry:   "local",
				Digest:     "sha256:abc123",
				ResolvedAt: fixedTime(),
			},
		},
	}
	if err := dependency.WriteLockFile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	// Setup cache with the package contents.
	cache := newMockCache()
	cache.store["test-package@1.0.0"] = portregistry.PackageContents{
		Package: portregistry.ResolvedPackage{
			Name:          "test-package",
			Version:       "1.0.0",
			IntegrityHash: "sha256:abc123",
		},
		RootDir: pkgDir,
	}

	resolverCalled := false
	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, _ string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			resolverCalled = true
			return portregistry.ResolvedPackage{}, errors.New("should not be called")
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, _ portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			t.Fatal("fetcher should not be called")
			return portregistry.PackageContents{}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
		dependency.WithCache(cache),
	)

	deps := []dependency.ManifestDependency{{Name: "test-package", Version: "^1.0.0"}}
	tree := makeTree(dir, manifest)

	result, err := resolver.Resolve(context.Background(), tree, deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolverCalled {
		t.Error("resolver should not have been called (lock file + cache hit)")
	}

	if len(result.Objects) != 1 {
		t.Fatalf("Objects len = %d, want 1", len(result.Objects))
	}
	if result.Objects[0].Meta.ID != "cached-obj" {
		t.Errorf("Meta.ID = %q, want cached-obj", result.Objects[0].Meta.ID)
	}
}

func TestResolver_IntegrityFailure(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "bad-package", "1.0.0", map[string]string{
		"obj.yaml": "kind: instruction\nid: obj-1\n",
	})

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  bad-package: "^1.0.0"
`)

	tree := makeTree(dir, manifest)

	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{
				Name:          name,
				Version:       "1.0.0",
				Registry:      "local",
				IntegrityHash: "sha256:expected",
			}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return errors.New("hash mismatch")
		}},
	)

	deps := []dependency.ManifestDependency{{Name: "bad-package", Version: "^1.0.0"}}

	_, err := resolver.Resolve(context.Background(), tree, deps)
	if err == nil {
		t.Fatal("expected integrity error")
	}

	var resErr *dependency.ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected ResolutionError, got %T", err)
	}
	if resErr.Package != "bad-package" {
		t.Errorf("Package = %q, want bad-package", resErr.Package)
	}
	if resErr.CompilerError.Code != pipeline.ErrResolution {
		t.Errorf("Code = %q, want RESOLUTION", resErr.CompilerError.Code)
	}
}

func TestResolver_MissingPackage(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  missing-package: "^1.0.0"
`)

	tree := makeTree(dir, manifest)

	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{}, errors.New("package not found")
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, _ portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
	)

	deps := []dependency.ManifestDependency{{Name: "missing-package", Version: "^1.0.0"}}

	_, err := resolver.Resolve(context.Background(), tree, deps)
	if err == nil {
		t.Fatal("expected resolution error for missing package")
	}

	var resErr *dependency.ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected ResolutionError, got %T", err)
	}
	if resErr.Package != "missing-package" {
		t.Errorf("Package = %q, want missing-package", resErr.Package)
	}
}

func TestResolver_ContextCancellation(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-package: "^1.0.0"
`)

	tree := makeTree(dir, manifest)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, _ string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{}, nil
		}},
		&mockFetcher{},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
	)

	deps := []dependency.ManifestDependency{{Name: "test-package", Version: "^1.0.0"}}

	_, err := resolver.Resolve(ctx, tree, deps)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestResolver_PackageWithNoObjects(t *testing.T) {
	dir := mustTempDir(t)

	// Create a package directory with no objects subdir.
	pkgDir := filepath.Join(dir, "packages", "empty-pkg", "1.0.0")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(pkgDir, "package.yaml"), "name: empty-pkg\nversion: 1.0.0\n")

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  empty-pkg: "1.0.0"
`)

	tree := makeTree(dir, manifest)

	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{
				Name:     name,
				Version:  "1.0.0",
				Registry: "local",
			}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
	)

	deps := []dependency.ManifestDependency{{Name: "empty-pkg", Version: "1.0.0"}}

	result, err := resolver.Resolve(context.Background(), tree, deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(result.Objects) != 0 {
		t.Errorf("Objects len = %d, want 0", len(result.Objects))
	}
}

func TestResolver_MultipleDependencies(t *testing.T) {
	dir := mustTempDir(t)

	pkg1Dir := setupPackageDir(t, dir, "pkg-a", "1.0.0", map[string]string{
		"a.yaml": "kind: instruction\nid: obj-a\n",
	})
	pkg2Dir := setupPackageDir(t, dir, "pkg-b", "2.0.0", map[string]string{
		"b.yaml": "kind: rule\nid: obj-b\n",
	})

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  pkg-a: "^1.0.0"
  pkg-b: "^2.0.0"
`)

	tree := makeTree(dir, manifest)

	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			switch name {
			case "pkg-a":
				return portregistry.ResolvedPackage{Name: name, Version: "1.0.0", Registry: "local"}, nil
			case "pkg-b":
				return portregistry.ResolvedPackage{Name: name, Version: "2.0.0", Registry: "local"}, nil
			}
			return portregistry.ResolvedPackage{}, errors.New("unknown")
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			switch pkg.Name {
			case "pkg-a":
				return portregistry.PackageContents{Package: pkg, RootDir: pkg1Dir}, nil
			case "pkg-b":
				return portregistry.PackageContents{Package: pkg, RootDir: pkg2Dir}, nil
			}
			return portregistry.PackageContents{}, errors.New("unknown")
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
	)

	deps := []dependency.ManifestDependency{
		{Name: "pkg-a", Version: "^1.0.0"},
		{Name: "pkg-b", Version: "^2.0.0"},
	}

	result, err := resolver.Resolve(context.Background(), tree, deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(result.Objects) != 2 {
		t.Fatalf("Objects len = %d, want 2", len(result.Objects))
	}

	ids := map[string]bool{}
	for _, obj := range result.Objects {
		ids[obj.Meta.ID] = true
	}
	if !ids["obj-a"] {
		t.Error("missing obj-a")
	}
	if !ids["obj-b"] {
		t.Error("missing obj-b")
	}
}

func TestResolver_LockFileWithoutCache_FetchesDirectly(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "test-package", "1.0.0", map[string]string{
		"obj.yaml": "kind: instruction\nid: locked-obj\n",
	})

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-package: "^1.0.0"
`)

	// Pre-create lock file.
	lockPath := dependency.LockFilePath(dir)
	lf := &dependency.LockFile{
		SchemaVersion: 1,
		Dependencies: []dependency.LockEntry{
			{
				Name:       "test-package",
				Version:    "1.0.0",
				Registry:   "local",
				Digest:     "sha256:abc123",
				ResolvedAt: fixedTime(),
			},
		},
	}
	if err := dependency.WriteLockFile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	resolverCalled := false
	fetcherCalled := false

	// No cache is set — the resolver should fetch using the locked version
	// directly, without calling PackageResolver.Resolve.
	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, _ string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			resolverCalled = true
			return portregistry.ResolvedPackage{}, errors.New("should not be called")
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			fetcherCalled = true
			if pkg.Version != "1.0.0" {
				t.Errorf("fetcher called with version %q, want 1.0.0", pkg.Version)
			}
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
	)

	deps := []dependency.ManifestDependency{{Name: "test-package", Version: "^1.0.0"}}
	tree := makeTree(dir, manifest)

	result, err := resolver.Resolve(context.Background(), tree, deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolverCalled {
		t.Error("PackageResolver.Resolve should not be called when lock file is available")
	}
	if !fetcherCalled {
		t.Error("PackageFetcher.Fetch should be called for lock-file-hit + cache-miss")
	}
	if len(result.Objects) != 1 {
		t.Fatalf("Objects len = %d, want 1", len(result.Objects))
	}
}

func TestResolver_VersionSatisfier_StaleConstraint(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "test-package", "2.0.0", map[string]string{
		"obj.yaml": "kind: instruction\nid: new-obj\n",
	})

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-package: "^2.0.0"
`)

	// Pre-create lock file with an OLD version that no longer satisfies ^2.0.0.
	lockPath := dependency.LockFilePath(dir)
	lf := &dependency.LockFile{
		SchemaVersion: 1,
		Dependencies: []dependency.LockEntry{
			{
				Name:       "test-package",
				Version:    "1.0.0",
				Registry:   "local",
				Digest:     "sha256:old",
				ResolvedAt: fixedTime(),
			},
		},
	}
	if err := dependency.WriteLockFile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	resolverCalled := false
	resolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			resolverCalled = true
			return portregistry.ResolvedPackage{
				Name: name, Version: "2.0.0", Registry: "local", IntegrityHash: "sha256:new",
			}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{verifyFunc: func(_ portregistry.ResolvedPackage, _ portregistry.PackageContents) error {
			return nil
		}},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
		dependency.WithVersionSatisfier(func(version, constraint string) (bool, error) {
			// 1.0.0 does NOT satisfy ^2.0.0.
			if version == "1.0.0" && constraint == "^2.0.0" {
				return false, nil
			}
			return true, nil
		}),
	)

	deps := []dependency.ManifestDependency{{Name: "test-package", Version: "^2.0.0"}}
	tree := makeTree(dir, manifest)

	result, err := resolver.Resolve(context.Background(), tree, deps)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if !resolverCalled {
		t.Error("resolver should be called when locked version is stale")
	}
	if len(result.Objects) != 1 {
		t.Fatalf("Objects len = %d, want 1", len(result.Objects))
	}
	if result.Objects[0].Meta.ID != "new-obj" {
		t.Errorf("Meta.ID = %q, want new-obj", result.Objects[0].Meta.ID)
	}
}
