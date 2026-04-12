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
	"time"

	"github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

func TestHTTPRegistry_Resolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/packages/") {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"name":           "test-pkg",
			"version":        "1.2.3",
			"registry":       "test-registry",
			"integrity_hash": "sha256:abc123",
			"publisher":      "test-pub",
		})
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL)
	pkg, err := reg.Resolve(context.Background(), "test-pkg",
		portregistry.VersionConstraint{Raw: "^1.0.0"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if pkg.Name != "test-pkg" {
		t.Errorf("Name = %q, want %q", pkg.Name, "test-pkg")
	}
	if pkg.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", pkg.Version, "1.2.3")
	}
	if pkg.Registry != "test-registry" {
		t.Errorf("Registry = %q, want %q", pkg.Registry, "test-registry")
	}
	if pkg.IntegrityHash != "sha256:abc123" {
		t.Errorf("IntegrityHash = %q, want %q", pkg.IntegrityHash, "sha256:abc123")
	}
	if pkg.Publisher != "test-pub" {
		t.Errorf("Publisher = %q, want %q", pkg.Publisher, "test-pub")
	}
}

func TestHTTPRegistry_Resolve_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL)
	_, err := reg.Resolve(context.Background(), "nonexistent",
		portregistry.VersionConstraint{})
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

func TestHTTPRegistry_Resolve_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	reg := registry.NewHTTPRegistry(srv.URL, registry.WithMaxRetries(1))
	_, err := reg.Resolve(ctx, "test-pkg", portregistry.VersionConstraint{})
	if err == nil {
		t.Error("expected error for timeout, got nil")
	}
}

func TestHTTPRegistry_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		writeTarGz(t, w, map[string]string{
			"data.txt":      "hello from registry",
			"sub/inner.txt": "nested file",
		})
	}))
	defer srv.Close()

	cacheDir := mustTempDir(t)
	reg := registry.NewHTTPRegistry(srv.URL, registry.WithCacheDir(cacheDir))

	pkg := portregistry.ResolvedPackage{Name: "test-pkg", Version: "1.0.0"}
	contents, err := reg.Fetch(context.Background(), pkg)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(contents.RootDir, "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello from registry" {
		t.Errorf("fetched data = %q, want %q", data, "hello from registry")
	}

	inner, err := os.ReadFile(filepath.Join(contents.RootDir, "sub", "inner.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(inner) != "nested file" {
		t.Errorf("nested file = %q, want %q", inner, "nested file")
	}
}

func TestHTTPRegistry_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			http.NotFound(w, r)
			return
		}
		if q := r.URL.Query().Get("q"); q != "test" {
			t.Errorf("search query = %q, want %q", q, "test")
		}
		json.NewEncoder(w).Encode([]portregistry.PackageMetadata{
			{Name: "pkg1", Version: "1.0.0", Description: "first"},
			{Name: "pkg2", Version: "2.0.0", Description: "second"},
		})
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL)
	results, err := reg.Search(context.Background(), "test")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search returned %d results, want 2", len(results))
	}
	if results[0].Name != "pkg1" {
		t.Errorf("results[0].Name = %q, want %q", results[0].Name, "pkg1")
	}
	if results[1].Description != "second" {
		t.Errorf("results[1].Description = %q, want %q", results[1].Description, "second")
	}
}

func TestHTTPRegistry_AuthToken(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]string{
			"name": "test-pkg", "version": "1.0.0",
		})
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL, registry.WithAuthToken("secret-token"))
	_, _ = reg.Resolve(context.Background(), "test-pkg", portregistry.VersionConstraint{})

	if gotToken != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", gotToken, "Bearer secret-token")
	}
}

func TestHTTPRegistry_RetryOnServerError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"name": "test-pkg", "version": "1.0.0",
		})
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL, registry.WithMaxRetries(3))
	pkg, err := reg.Resolve(context.Background(), "test-pkg", portregistry.VersionConstraint{})
	if err != nil {
		t.Fatalf("Resolve with retries: %v", err)
	}
	if pkg.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", pkg.Version, "1.0.0")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestHTTPRegistry_MaxRetriesExceeded(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL, registry.WithMaxRetries(2))
	_, err := reg.Resolve(context.Background(), "test-pkg", portregistry.VersionConstraint{})
	if err == nil {
		t.Error("expected error after max retries, got nil")
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("error should mention max retries, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestHTTPRegistry_NoRetryOn4xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	reg := registry.NewHTTPRegistry(srv.URL, registry.WithMaxRetries(3))
	_, err := reg.Resolve(context.Background(), "test-pkg", portregistry.VersionConstraint{})
	if err == nil {
		t.Error("expected error for 400, got nil")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (4xx should not retry)", attempts)
	}
}

// Compile-time interface checks.
var (
	_ portregistry.PackageResolver = (*registry.HTTPRegistry)(nil)
	_ portregistry.PackageFetcher  = (*registry.HTTPRegistry)(nil)
	_ portregistry.PackageSearcher = (*registry.HTTPRegistry)(nil)
)
