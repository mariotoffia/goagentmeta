package resolver_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/resolver"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// --- mocks ---

type mockResolver struct {
	resolveFunc func(ctx context.Context, name string, constraint portregistry.VersionConstraint) (portregistry.ResolvedPackage, error)
}

func (m *mockResolver) Resolve(ctx context.Context, name string, constraint portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, name, constraint)
	}
	return portregistry.ResolvedPackage{}, nil
}

type mockFetcher struct {
	fetchFunc func(ctx context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error)
}

func (m *mockFetcher) Fetch(ctx context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, pkg)
	}
	return portregistry.PackageContents{}, nil
}

type mockVerifier struct {
	verifyFunc func(pkg portregistry.ResolvedPackage, contents portregistry.PackageContents) error
}

func (m *mockVerifier) Verify(pkg portregistry.ResolvedPackage, contents portregistry.PackageContents) error {
	if m.verifyFunc != nil {
		return m.verifyFunc(pkg, contents)
	}
	return nil
}

// --- helpers ---

func mustTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "goagentmeta-stage-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupPackageDir(t *testing.T, rootDir, name, version string) string {
	t.Helper()
	pkgDir := filepath.Join(rootDir, "packages", name, version)
	objDir := filepath.Join(pkgDir, "objects")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(pkgDir, "package.yaml"),
		"name: "+name+"\nversion: "+version+"\npublisher: test\n")
	writeFile(t, filepath.Join(objDir, "obj.yaml"),
		"kind: instruction\nid: ext-obj\ndescription: external\n")
	return pkgDir
}

func contextWithProfile(profile build.Profile) context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			Profile: profile,
		},
		Report: &pipeline.BuildReport{},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func fixedTime() time.Time {
	return time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
}

// --- tests ---

func TestStage_Descriptor(t *testing.T) {
	depResolver := dependency.NewDependencyResolver(
		&mockResolver{}, &mockFetcher{}, &mockVerifier{},
	)
	stage := resolver.New(depResolver)
	desc := stage.Descriptor()

	if desc.Name != "dependency-resolver" {
		t.Errorf("Name = %q, want dependency-resolver", desc.Name)
	}
	if desc.Phase != pipeline.PhaseResolve {
		t.Errorf("Phase = %v, want PhaseResolve", desc.Phase)
	}
	if desc.Order != 10 {
		t.Errorf("Order = %d, want 10", desc.Order)
	}
}

func TestStage_NoDependencies_PassThrough(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `schemaVersion: 1
project:
  name: test
`)

	tree := pipeline.SourceTree{
		RootPath:      dir,
		ManifestPath:  manifest,
		SchemaVersion: 1,
	}

	depResolver := dependency.NewDependencyResolver(
		&mockResolver{}, &mockFetcher{}, &mockVerifier{},
	)
	stage := resolver.New(depResolver)

	ctx := contextWithProfile(build.ProfileLocalDev)
	result, err := stage.Execute(ctx, tree)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	resultTree, ok := result.(pipeline.SourceTree)
	if !ok {
		t.Fatalf("result type = %T, want pipeline.SourceTree", result)
	}
	if len(resultTree.Objects) != 0 {
		t.Errorf("Objects len = %d, want 0", len(resultTree.Objects))
	}
}

func TestStage_WithDependencies_MergesObjects(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "test-pkg", "1.0.0")

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-pkg: "^1.0.0"
registries:
  - name: local
    type: local
    url: /tmp
`)

	tree := pipeline.SourceTree{
		RootPath:      dir,
		ManifestPath:  manifest,
		SchemaVersion: 1,
	}

	depResolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{
				Name: name, Version: "1.0.0", Registry: "local",
			}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
	)

	stage := resolver.New(depResolver)
	ctx := contextWithProfile(build.ProfileLocalDev)

	result, err := stage.Execute(ctx, tree)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	resultTree := result.(pipeline.SourceTree)
	if len(resultTree.Objects) != 1 {
		t.Fatalf("Objects len = %d, want 1", len(resultTree.Objects))
	}

	obj := resultTree.Objects[0]
	if obj.Meta.ID != "ext-obj" {
		t.Errorf("Meta.ID = %q, want ext-obj", obj.Meta.ID)
	}
}

func TestStage_EnterpriseLocked_BlocksExternalRegistry(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-pkg: "^1.0.0"
registries:
  - name: acme-corp
    type: http
    url: https://registry.acme.com
`)

	tree := pipeline.SourceTree{
		RootPath:      dir,
		ManifestPath:  manifest,
		SchemaVersion: 1,
	}

	depResolver := dependency.NewDependencyResolver(
		&mockResolver{}, &mockFetcher{}, &mockVerifier{},
	)

	stage := resolver.New(depResolver)
	ctx := contextWithProfile(build.ProfileEnterpriseLocked)

	_, err := stage.Execute(ctx, tree)
	if err == nil {
		t.Fatal("expected error for enterprise-locked profile with external registry")
	}

	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T: %v", err, err)
	}
	if compErr.Code != pipeline.ErrResolution {
		t.Errorf("Code = %q, want RESOLUTION", compErr.Code)
	}
}

func TestStage_EnterpriseLocked_AllowsLocalOnly(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "local-pkg", "1.0.0")

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  local-pkg: "^1.0.0"
registries:
  - name: dev
    type: local
    url: /tmp/packages
`)

	tree := pipeline.SourceTree{
		RootPath:      dir,
		ManifestPath:  manifest,
		SchemaVersion: 1,
	}

	depResolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{
				Name: name, Version: "1.0.0", Registry: "local",
			}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
	)

	stage := resolver.New(depResolver)
	ctx := contextWithProfile(build.ProfileEnterpriseLocked)

	result, err := stage.Execute(ctx, tree)
	if err != nil {
		t.Fatalf("Execute: %v (enterprise-locked should allow local registries)", err)
	}

	resultTree := result.(pipeline.SourceTree)
	if len(resultTree.Objects) != 1 {
		t.Fatalf("Objects len = %d, want 1", len(resultTree.Objects))
	}
}

func TestStage_InvalidInput(t *testing.T) {
	depResolver := dependency.NewDependencyResolver(
		&mockResolver{}, &mockFetcher{}, &mockVerifier{},
	)
	stage := resolver.New(depResolver)

	_, err := stage.Execute(context.Background(), "not a source tree")
	if err == nil {
		t.Fatal("expected error for invalid input type")
	}

	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T", err)
	}
	if compErr.Code != pipeline.ErrResolution {
		t.Errorf("Code = %q, want RESOLUTION", compErr.Code)
	}
}

func TestStage_PointerInput(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `schemaVersion: 1
`)

	tree := &pipeline.SourceTree{
		RootPath:      dir,
		ManifestPath:  manifest,
		SchemaVersion: 1,
	}

	depResolver := dependency.NewDependencyResolver(
		&mockResolver{}, &mockFetcher{}, &mockVerifier{},
	)
	stage := resolver.New(depResolver)

	result, err := stage.Execute(context.Background(), tree)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, ok := result.(pipeline.SourceTree); !ok {
		t.Fatalf("result type = %T, want pipeline.SourceTree", result)
	}
}

func TestStage_EmitsDiagnostics(t *testing.T) {
	dir := mustTempDir(t)
	pkgDir := setupPackageDir(t, dir, "test-pkg", "1.0.0")

	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  test-pkg: "^1.0.0"
registries:
  - name: dev
    type: local
    url: /tmp
`)

	tree := pipeline.SourceTree{
		RootPath:      dir,
		ManifestPath:  manifest,
		SchemaVersion: 1,
	}

	depResolver := dependency.NewDependencyResolver(
		&mockResolver{resolveFunc: func(_ context.Context, name string, _ portregistry.VersionConstraint) (portregistry.ResolvedPackage, error) {
			return portregistry.ResolvedPackage{Name: name, Version: "1.0.0", Registry: "local"}, nil
		}},
		&mockFetcher{fetchFunc: func(_ context.Context, pkg portregistry.ResolvedPackage) (portregistry.PackageContents, error) {
			return portregistry.PackageContents{Package: pkg, RootDir: pkgDir}, nil
		}},
		&mockVerifier{},
		dependency.WithNowFunc(func() time.Time { return fixedTime() }),
	)

	stage := resolver.New(depResolver)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{Profile: build.ProfileLocalDev},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, err := stage.Execute(ctx, tree)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(report.Diagnostics) < 2 {
		t.Fatalf("Diagnostics len = %d, want >= 2 (start + complete)", len(report.Diagnostics))
	}

	foundStart := false
	foundComplete := false
	for _, d := range report.Diagnostics {
		if d.Code == "RESOLVE_START" {
			foundStart = true
		}
		if d.Code == "RESOLVE_COMPLETE" {
			foundComplete = true
		}
	}
	if !foundStart {
		t.Error("missing RESOLVE_START diagnostic")
	}
	if !foundComplete {
		t.Error("missing RESOLVE_COMPLETE diagnostic")
	}
}

func TestFactory(t *testing.T) {
	depResolver := dependency.NewDependencyResolver(
		&mockResolver{}, &mockFetcher{}, &mockVerifier{},
	)

	factory := resolver.Factory(depResolver)
	stage, err := factory()
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if stage == nil {
		t.Fatal("Factory returned nil stage")
	}
	if stage.Descriptor().Name != "dependency-resolver" {
		t.Errorf("Name = %q, want dependency-resolver", stage.Descriptor().Name)
	}
}

func TestStage_EnterpriseLocked_NoRegistries_BlocksDeps(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	// Dependencies declared, but no registries section at all.
	writeFile(t, manifest, `dependencies:
  some-pkg: "^1.0.0"
`)

	tree := pipeline.SourceTree{
		RootPath:      dir,
		ManifestPath:  manifest,
		SchemaVersion: 1,
	}

	depResolver := dependency.NewDependencyResolver(
		&mockResolver{}, &mockFetcher{}, &mockVerifier{},
	)
	stage := resolver.New(depResolver)
	ctx := contextWithProfile(build.ProfileEnterpriseLocked)

	_, err := stage.Execute(ctx, tree)
	if err == nil {
		t.Fatal("expected error when enterprise-locked profile has deps but no registries")
	}

	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T: %v", err, err)
	}
	if compErr.Code != pipeline.ErrResolution {
		t.Errorf("Code = %q, want RESOLUTION", compErr.Code)
	}
}
