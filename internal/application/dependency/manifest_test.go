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

func TestParseManifest_CompilerExternalPlugins(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string
		check     func(t *testing.T, m *dependency.Manifest)
	}{
		{
			name: "valid external plugins",
			yaml: `compiler:
  externalPlugins:
    - name: my-linter
      source: "external://path/to/linter"
      phase: validate
      order: 10
      target: go
    - name: my-renderer
      source: "external://path/to/renderer"
      phase: render
      order: 20
`,
			check: func(t *testing.T, m *dependency.Manifest) {
				t.Helper()
				if len(m.Compiler.ExternalPlugins) != 2 {
					t.Fatalf("got %d plugins, want 2", len(m.Compiler.ExternalPlugins))
				}
				p := m.Compiler.ExternalPlugins[0]
				if p.Name != "my-linter" {
					t.Errorf("Name = %q, want %q", p.Name, "my-linter")
				}
				if p.Source != "external://path/to/linter" {
					t.Errorf("Source = %q, want %q", p.Source, "external://path/to/linter")
				}
				if p.Phase != "validate" {
					t.Errorf("Phase = %q, want %q", p.Phase, "validate")
				}
				if p.Order != 10 {
					t.Errorf("Order = %d, want 10", p.Order)
				}
				if p.Target != "go" {
					t.Errorf("Target = %q, want %q", p.Target, "go")
				}
				p2 := m.Compiler.ExternalPlugins[1]
				if p2.Name != "my-renderer" {
					t.Errorf("Name = %q, want %q", p2.Name, "my-renderer")
				}
				if p2.Target != "" {
					t.Errorf("Target = %q, want empty", p2.Target)
				}
			},
		},
		{
			name: "empty compiler section",
			yaml: `compiler: {}
`,
			check: func(t *testing.T, m *dependency.Manifest) {
				t.Helper()
				if len(m.Compiler.ExternalPlugins) != 0 {
					t.Errorf("got %d plugins, want 0", len(m.Compiler.ExternalPlugins))
				}
			},
		},
		{
			name: "no compiler section (backward compatible)",
			yaml: `schemaVersion: 1
dependencies:
  pkg-a: "^1.0.0"
`,
			check: func(t *testing.T, m *dependency.Manifest) {
				t.Helper()
				if len(m.Compiler.ExternalPlugins) != 0 {
					t.Errorf("got %d plugins, want 0", len(m.Compiler.ExternalPlugins))
				}
				if len(m.Dependencies) != 1 {
					t.Errorf("got %d deps, want 1", len(m.Dependencies))
				}
			},
		},
		{
			name: "duplicate plugin names",
			yaml: `compiler:
  externalPlugins:
    - name: dup
      source: "external://a"
      phase: parse
      order: 1
    - name: dup
      source: "external://b"
      phase: render
      order: 2
`,
			wantErr:   true,
			errSubstr: "duplicate name",
		},
		{
			name: "source without external prefix",
			yaml: `compiler:
  externalPlugins:
    - name: bad-src
      source: "http://wrong"
      phase: parse
      order: 1
`,
			wantErr:   true,
			errSubstr: `source must start with`,
		},
		{
			name: "empty name",
			yaml: `compiler:
  externalPlugins:
    - name: ""
      source: "external://x"
      phase: parse
      order: 1
`,
			wantErr:   true,
			errSubstr: "name must not be empty",
		},
		{
			name: "invalid phase name",
			yaml: `compiler:
  externalPlugins:
    - name: bad-phase
      source: "external://x"
      phase: nonexistent
      order: 1
`,
			wantErr:   true,
			errSubstr: "invalid phase",
		},
		{
			name: "valid config passes validation",
			yaml: `compiler:
  externalPlugins:
    - name: ok-plugin
      source: "external://bin/ok"
      phase: lower
      order: 5
`,
			check: func(t *testing.T, m *dependency.Manifest) {
				t.Helper()
				if err := m.Compiler.Validate(); err != nil {
					t.Errorf("Validate() returned unexpected error: %v", err)
				}
			},
		},
		{
			name: "round-trip: all fields populated",
			yaml: `dependencies:
  lib-x: "^2.0.0"
registries:
  - name: corp
    type: http
    url: https://r.example.com
compiler:
  externalPlugins:
    - name: fmt-plugin
      source: "external://bin/fmt"
      phase: normalize
      order: 3
      target: typescript
`,
			check: func(t *testing.T, m *dependency.Manifest) {
				t.Helper()
				if len(m.Dependencies) != 1 {
					t.Fatalf("Dependencies len = %d, want 1", len(m.Dependencies))
				}
				if len(m.Registries) != 1 {
					t.Fatalf("Registries len = %d, want 1", len(m.Registries))
				}
				if len(m.Compiler.ExternalPlugins) != 1 {
					t.Fatalf("ExternalPlugins len = %d, want 1", len(m.Compiler.ExternalPlugins))
				}
				p := m.Compiler.ExternalPlugins[0]
				if p.Name != "fmt-plugin" || p.Source != "external://bin/fmt" ||
					p.Phase != "normalize" || p.Order != 3 || p.Target != "typescript" {
					t.Errorf("unexpected plugin values: %+v", p)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := mustTempDir(t)
			path := filepath.Join(dir, "manifest.yaml")
			writeFile(t, path, tt.yaml)

			m, err := dependency.ParseManifest(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
