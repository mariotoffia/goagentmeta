package registry_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

func TestDiskCache_MissAndHit(t *testing.T) {
	cacheDir := mustTempDir(t)
	cache := registry.NewDiskCache(cacheDir)

	// Cache miss.
	_, found := cache.Get("test-pkg", "1.0.0")
	if found {
		t.Error("expected cache miss, got hit")
	}

	// Set up source.
	srcDir := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := portregistry.ResolvedPackage{Name: "test-pkg", Version: "1.0.0"}
	if err := cache.Put(pkg, srcDir); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Cache hit.
	contents, found := cache.Get("test-pkg", "1.0.0")
	if !found {
		t.Fatal("expected cache hit, got miss")
	}
	if contents.Package.Name != "test-pkg" {
		t.Errorf("cached name = %q, want %q", contents.Package.Name, "test-pkg")
	}

	// Verify file was copied.
	data, err := os.ReadFile(filepath.Join(contents.RootDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "cached" {
		t.Errorf("cached data = %q, want %q", data, "cached")
	}
}

func TestDiskCache_Invalidate(t *testing.T) {
	cacheDir := mustTempDir(t)
	cache := registry.NewDiskCache(cacheDir)

	srcDir := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := portregistry.ResolvedPackage{Name: "test-pkg", Version: "1.0.0"}
	if err := cache.Put(pkg, srcDir); err != nil {
		t.Fatal(err)
	}

	if err := cache.Invalidate("test-pkg", "1.0.0"); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	_, found := cache.Get("test-pkg", "1.0.0")
	if found {
		t.Error("expected cache miss after invalidate, got hit")
	}
}

func TestDiskCache_OverwriteExisting(t *testing.T) {
	cacheDir := mustTempDir(t)
	cache := registry.NewDiskCache(cacheDir)

	// First version.
	src1 := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(src1, "data.txt"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := portregistry.ResolvedPackage{Name: "test-pkg", Version: "1.0.0"}
	if err := cache.Put(pkg, src1); err != nil {
		t.Fatal(err)
	}

	// Overwrite with second version.
	src2 := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(src2, "data.txt"), []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(pkg, src2); err != nil {
		t.Fatal(err)
	}

	contents, found := cache.Get("test-pkg", "1.0.0")
	if !found {
		t.Fatal("expected cache hit")
	}
	data, err := os.ReadFile(filepath.Join(contents.RootDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second" {
		t.Errorf("cached data = %q, want %q", data, "second")
	}
}

func TestDiskCache_SlashInName(t *testing.T) {
	cacheDir := mustTempDir(t)
	cache := registry.NewDiskCache(cacheDir)

	srcDir := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := portregistry.ResolvedPackage{Name: "@acme/skill", Version: "1.0.0"}
	if err := cache.Put(pkg, srcDir); err != nil {
		t.Fatal(err)
	}

	_, found := cache.Get("@acme/skill", "1.0.0")
	if !found {
		t.Error("expected cache hit for name with slash")
	}
}

func TestDiskCache_DefaultDir(t *testing.T) {
	dir := registry.DefaultCacheDir()
	if dir == "" {
		t.Error("DefaultCacheDir returned empty string")
	}
}

func TestCacheKey(t *testing.T) {
	key := registry.CacheKey("@acme/skill", "1.2.3")
	if strings.Contains(key, "/") {
		t.Errorf("CacheKey should not contain /: got %q", key)
	}
	if key != "@acme__skill@1.2.3" {
		t.Errorf("CacheKey = %q, want %q", key, "@acme__skill@1.2.3")
	}
}

// Integration: cache miss → HTTP fetch → cache put → cache hit (no HTTP).
func TestCache_PreventsDuplicateFetch(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/archive") {
			fetchCount++
			w.Header().Set("Content-Type", "application/gzip")
			writeTarGz(t, w, map[string]string{"data.txt": "from-http"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"name": "test-pkg", "version": "1.0.0",
		})
	}))
	defer srv.Close()

	// Use separate dirs: HTTP extracts archives into fetchDir, cache copies
	// into cacheDir. This mirrors production where the fetcher writes to a
	// temp location and the cache owns its own storage.
	fetchDir := mustTempDir(t)
	cacheDir := mustTempDir(t)
	cache := registry.NewDiskCache(cacheDir)
	httpReg := registry.NewHTTPRegistry(srv.URL, registry.WithCacheDir(fetchDir))

	pkg := portregistry.ResolvedPackage{Name: "test-pkg", Version: "1.0.0"}

	// First: cache miss → HTTP fetch → store in cache.
	if _, found := cache.Get(pkg.Name, pkg.Version); found {
		t.Fatal("expected cache miss")
	}
	contents, err := httpReg.Fetch(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(pkg, contents.RootDir); err != nil {
		t.Fatal(err)
	}

	// Second: cache hit → no HTTP call needed.
	cached, found := cache.Get(pkg.Name, pkg.Version)
	if !found {
		t.Fatal("expected cache hit")
	}
	data, err := os.ReadFile(filepath.Join(cached.RootDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from-http" {
		t.Errorf("cached data = %q, want %q", data, "from-http")
	}

	if fetchCount != 1 {
		t.Errorf("fetch count = %d, want 1 (cache should prevent second fetch)", fetchCount)
	}
}

// Integration: integrity failure → cache invalidated.
func TestCache_IntegrityFailure_Invalidates(t *testing.T) {
	cacheDir := mustTempDir(t)
	cache := registry.NewDiskCache(cacheDir)
	verifier := registry.NewSHA256Verifier()

	srcDir := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := portregistry.ResolvedPackage{
		Name:          "test-pkg",
		Version:       "1.0.0",
		IntegrityHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	if err := cache.Put(pkg, srcDir); err != nil {
		t.Fatal(err)
	}

	cached, found := cache.Get(pkg.Name, pkg.Version)
	if !found {
		t.Fatal("expected cache hit")
	}

	// Integrity check should fail.
	err := verifier.Verify(pkg, cached)
	if err == nil {
		t.Fatal("expected integrity error")
	}

	// Invalidate on failure.
	if err := cache.Invalidate(pkg.Name, pkg.Version); err != nil {
		t.Fatal(err)
	}

	if _, found := cache.Get(pkg.Name, pkg.Version); found {
		t.Error("expected cache miss after invalidation")
	}
}
