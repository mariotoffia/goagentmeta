package registry_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// ─── Version benchmarks ─────────────────────────────────────────────────────

func BenchmarkParseVersion(b *testing.B) {
	for b.Loop() {
		registry.ParseVersion("v12.34.567")
	}
}

func BenchmarkMatchConstraint_Caret(b *testing.B) {
	v := registry.Version{Major: 1, Minor: 5, Patch: 3}
	c := portregistry.VersionConstraint{Raw: "^1.0.0"}
	for b.Loop() {
		registry.MatchConstraint(v, c)
	}
}

func BenchmarkBestMatch(b *testing.B) {
	candidates := make([]registry.Version, 100)
	for i := range candidates {
		candidates[i] = registry.Version{Major: 1, Minor: i, Patch: 0}
	}
	c := portregistry.VersionConstraint{Raw: "^1.50.0"}
	for b.Loop() {
		registry.BestMatch(candidates, c)
	}
}

// ─── Integrity benchmarks ───────────────────────────────────────────────────

func BenchmarkComputeIntegrityHash(b *testing.B) {
	dir := b.TempDir()
	for i := range 10 {
		name := filepath.Join(dir, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(name, []byte("benchmark content for file"), 0o644)
	}
	b.ResetTimer()
	for b.Loop() {
		registry.ComputeIntegrityHash(dir)
	}
}

func BenchmarkSHA256Verifier_Verify(b *testing.B) {
	dir := b.TempDir()
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("bench"), 0o644)
	hash, _ := registry.ComputeIntegrityHash(dir)

	verifier := registry.NewSHA256Verifier()
	pkg := portregistry.ResolvedPackage{Name: "bench", IntegrityHash: hash}
	contents := portregistry.PackageContents{Package: pkg, RootDir: dir}

	b.ResetTimer()
	for b.Loop() {
		verifier.Verify(pkg, contents)
	}
}

// ─── Cache benchmarks ───────────────────────────────────────────────────────

func BenchmarkDiskCache_GetMiss(b *testing.B) {
	dir := b.TempDir()
	cache := registry.NewDiskCache(dir)
	b.ResetTimer()
	for b.Loop() {
		cache.Get("nonexistent", "1.0.0")
	}
}

func BenchmarkDiskCache_GetHit(b *testing.B) {
	dir := b.TempDir()
	cache := registry.NewDiskCache(dir)

	srcDir := b.TempDir()
	os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("bench"), 0o644)
	pkg := portregistry.ResolvedPackage{Name: "bench-pkg", Version: "1.0.0"}
	cache.Put(pkg, srcDir)

	b.ResetTimer()
	for b.Loop() {
		cache.Get("bench-pkg", "1.0.0")
	}
}

// ─── Local registry benchmarks ──────────────────────────────────────────────

func BenchmarkLocalRegistry_Resolve(b *testing.B) {
	root := b.TempDir()
	for _, ver := range []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"} {
		vDir := filepath.Join(root, "bench-pkg", ver)
		os.MkdirAll(vDir, 0o755)
		os.WriteFile(filepath.Join(vDir, "package.yaml"),
			[]byte("name: bench-pkg\nversion: "+ver+"\npublisher: bench\n"), 0o644)
		os.WriteFile(filepath.Join(vDir, "data.txt"), []byte("data"), 0o644)
	}
	reg := registry.NewLocalRegistry(root)
	c := portregistry.VersionConstraint{Raw: "^1.0.0"}

	b.ResetTimer()
	for b.Loop() {
		reg.Resolve(context.Background(), "bench-pkg", c)
	}
}

// ─── HTTP registry benchmarks ───────────────────────────────────────────────

func BenchmarkHTTPRegistry_Resolve(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"name": "bench-pkg", "version": "1.0.0",
		})
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL)
	c := portregistry.VersionConstraint{Raw: "^1.0.0"}

	b.ResetTimer()
	for b.Loop() {
		reg.Resolve(context.Background(), "bench-pkg", c)
	}
}
