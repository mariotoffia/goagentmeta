package dependency_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
)

func TestParseDependencies_FlatFormat(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `schemaVersion: 1
project:
  name: test-project

dependencies:
  test-package: "^1.0.0"
  another-pkg: "~2.1.0"
  exact-pkg: "3.0.0"

registries:
  - name: local
    type: local
    url: /tmp/packages
`)

	deps, err := dependency.ParseManifestDependencies(manifest)
	if err != nil {
		t.Fatalf("ParseManifestDependencies: %v", err)
	}

	if len(deps) != 3 {
		t.Fatalf("got %d deps, want 3", len(deps))
	}

	// Sort for deterministic comparison (YAML map order is unspecified).
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	tests := []struct {
		name    string
		version string
	}{
		{"another-pkg", "~2.1.0"},
		{"exact-pkg", "3.0.0"},
		{"test-package", "^1.0.0"},
	}

	for i, tt := range tests {
		if deps[i].Name != tt.name {
			t.Errorf("dep[%d].Name = %q, want %q", i, deps[i].Name, tt.name)
		}
		if deps[i].Version != tt.version {
			t.Errorf("dep[%d].Version = %q, want %q", i, deps[i].Version, tt.version)
		}
	}
}

func TestParseDependencies_ExtendedFormat(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  pinned-pkg:
    version: "1.2.3"
    registry: local
  simple-pkg: "^1.0.0"
`)

	deps, err := dependency.ParseManifestDependencies(manifest)
	if err != nil {
		t.Fatalf("ParseManifestDependencies: %v", err)
	}

	if len(deps) != 2 {
		t.Fatalf("got %d deps, want 2", len(deps))
	}

	// Sort for deterministic comparison.
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	// pinned-pkg comes first alphabetically.
	if deps[0].Name != "pinned-pkg" {
		t.Errorf("dep[0].Name = %q, want %q", deps[0].Name, "pinned-pkg")
	}
	if deps[0].Version != "1.2.3" {
		t.Errorf("dep[0].Version = %q, want %q", deps[0].Version, "1.2.3")
	}
	if deps[0].Registry != "local" {
		t.Errorf("dep[0].Registry = %q, want %q", deps[0].Registry, "local")
	}

	if deps[1].Name != "simple-pkg" {
		t.Errorf("dep[1].Name = %q, want %q", deps[1].Name, "simple-pkg")
	}
	if deps[1].Version != "^1.0.0" {
		t.Errorf("dep[1].Version = %q, want %q", deps[1].Version, "^1.0.0")
	}
}

func TestParseDependencies_NoDepsSection(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `schemaVersion: 1
project:
  name: test-project
`)

	deps, err := dependency.ParseManifestDependencies(manifest)
	if err != nil {
		t.Fatalf("ParseManifestDependencies: %v", err)
	}

	if len(deps) != 0 {
		t.Fatalf("got %d deps, want 0", len(deps))
	}
}

func TestParseDependencies_FileNotFound(t *testing.T) {
	_, err := dependency.ParseManifestDependencies("/nonexistent/manifest.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseDependencies_DepNamedVersion(t *testing.T) {
	// Regression: a dependency literally named "version" must not be
	// swallowed by sub-key parsing.
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  version: "^1.0.0"
  normal-pkg: "^2.0.0"
`)

	deps, err := dependency.ParseManifestDependencies(manifest)
	if err != nil {
		t.Fatalf("ParseManifestDependencies: %v", err)
	}

	if len(deps) != 2 {
		t.Fatalf("got %d deps, want 2", len(deps))
	}

	found := map[string]bool{}
	for _, d := range deps {
		found[d.Name] = true
	}
	if !found["version"] {
		t.Error("dependency named 'version' was dropped")
	}
	if !found["normal-pkg"] {
		t.Error("dependency 'normal-pkg' was dropped")
	}
}

func TestParseDependencies_InvalidYAML(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `dependencies:
  - this is not valid: [`)

	_, err := dependency.ParseManifestDependencies(manifest)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseRegistries(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `registries:
  - name: local-dev
    type: local
    url: /tmp/packages
  - name: acme-corp
    type: http
    url: https://registry.acme.com
    auth: token123
  - name: github-repo
    type: git
    url: https://github.com/acme/packages
`)

	regs, err := dependency.ParseManifestRegistries(manifest)
	if err != nil {
		t.Fatalf("ParseManifestRegistries: %v", err)
	}

	if len(regs) != 3 {
		t.Fatalf("got %d registries, want 3", len(regs))
	}

	tests := []struct {
		name    string
		regType string
		url     string
		auth    string
	}{
		{"local-dev", "local", "/tmp/packages", ""},
		{"acme-corp", "http", "https://registry.acme.com", "token123"},
		{"github-repo", "git", "https://github.com/acme/packages", ""},
	}

	for i, tt := range tests {
		if regs[i].Name != tt.name {
			t.Errorf("reg[%d].Name = %q, want %q", i, regs[i].Name, tt.name)
		}
		if regs[i].Type != tt.regType {
			t.Errorf("reg[%d].Type = %q, want %q", i, regs[i].Type, tt.regType)
		}
		if regs[i].URL != tt.url {
			t.Errorf("reg[%d].URL = %q, want %q", i, regs[i].URL, tt.url)
		}
		if regs[i].Auth != tt.auth {
			t.Errorf("reg[%d].Auth = %q, want %q", i, regs[i].Auth, tt.auth)
		}
	}
}

func TestParseRegistries_NoSection(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `schemaVersion: 1
`)

	regs, err := dependency.ParseManifestRegistries(manifest)
	if err != nil {
		t.Fatalf("ParseManifestRegistries: %v", err)
	}

	if len(regs) != 0 {
		t.Fatalf("got %d registries, want 0", len(regs))
	}
}

func TestParseManifest_BothSections(t *testing.T) {
	dir := mustTempDir(t)
	manifest := filepath.Join(dir, "manifest.yaml")
	writeFile(t, manifest, `schemaVersion: 1
dependencies:
  pkg-a: "^1.0.0"
registries:
  - name: local
    type: local
    url: /tmp
`)

	m, err := dependency.ParseManifest(manifest)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	if len(m.Dependencies) != 1 {
		t.Errorf("Dependencies len = %d, want 1", len(m.Dependencies))
	}
	if len(m.Registries) != 1 {
		t.Errorf("Registries len = %d, want 1", len(m.Registries))
	}
}
