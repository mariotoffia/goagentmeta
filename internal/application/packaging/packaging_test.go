package packaging_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/application/packaging"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
)

// Compile-time interface checks.
var (
	_ portfs.Reader = (*filesystem.MemFS)(nil)
	_ portfs.Writer = (*filesystem.MemFS)(nil)
)

func seedFiles(t *testing.T, memfs *filesystem.MemFS) pipeline.MaterializationResult {
	t.Helper()
	ctx := context.Background()
	files := map[string]string{
		".ai-build/copilot/local-dev/instructions.md": "# Copilot Instructions",
		".ai-build/copilot/local-dev/config.yaml":     "mode: assist",
		".ai-build/cursor/local-dev/rules.md":         "# Cursor Rules",
		".ai-build/claude/local-dev/CLAUDE.md":        "# Claude Config",
	}
	var written []string
	for p, content := range files {
		if err := memfs.WriteFile(ctx, p, []byte(content), 0o644); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
		written = append(written, p)
	}
	return pipeline.MaterializationResult{WrittenFiles: written}
}

func TestPackage_VSCodeEnabled(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		VSCodeExtension: &packaging.VSCodePackagingConfig{
			Enabled:   true,
			Publisher: "test-pub",
			Targets:   []build.Target{build.TargetCopilot, build.TargetCursor},
			Version:   "1.2.3",
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	ctx := context.Background()
	pkgResult, err := svc.Package(ctx, result, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgResult.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(pkgResult.Artifacts))
	}

	art := pkgResult.Artifacts[0]
	if art.Type != "vsix" {
		t.Errorf("expected type vsix, got %s", art.Type)
	}
	if art.Metadata["publisher"] != "test-pub" {
		t.Errorf("expected publisher test-pub, got %s", art.Metadata["publisher"])
	}
	if art.Metadata["version"] != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", art.Metadata["version"])
	}

	// Verify the vsix is a valid ZIP with required entries.
	vsixData, err := memfs.ReadFile(ctx, art.Path)
	if err != nil {
		t.Fatalf("read vsix: %v", err)
	}
	entries := zipEntries(t, vsixData)

	for _, required := range []string{
		"[Content_Types].xml",
		"extension.vsixmanifest",
		"extension/package.json",
	} {
		if _, ok := entries[required]; !ok {
			t.Errorf("missing required vsix entry: %s", required)
		}
	}

	// Verify package.json is valid JSON with expected fields.
	pkgJSON := entries["extension/package.json"]
	var pkg map[string]any
	if err := json.Unmarshal(pkgJSON, &pkg); err != nil {
		t.Fatalf("invalid package.json: %v", err)
	}
	if got := pkg["publisher"]; got != "test-pub" {
		t.Errorf("package.json publisher = %v, want test-pub", got)
	}
	if got := pkg["version"]; got != "1.2.3" {
		t.Errorf("package.json version = %v, want 1.2.3", got)
	}

	// Verify content files are included under extension/ prefix.
	hasContent := false
	for name := range entries {
		if strings.HasPrefix(name, "extension/local-dev/") {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("no content files found under extension/ in vsix")
	}
}

func TestPackage_NPMEnabled(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		NPM: &packaging.NPMPackagingConfig{
			Enabled: true,
			Scope:   "@acme",
			Targets: []build.Target{build.TargetClaude, build.TargetCursor},
			Version: "2.0.0",
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	ctx := context.Background()
	pkgResult, err := svc.Package(ctx, result, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgResult.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(pkgResult.Artifacts))
	}

	art := pkgResult.Artifacts[0]
	if art.Type != "npm" {
		t.Errorf("expected type npm, got %s", art.Type)
	}
	if art.Metadata["scope"] != "@acme" {
		t.Errorf("expected scope @acme, got %s", art.Metadata["scope"])
	}

	// Verify the tarball is valid tar.gz with required entries.
	tgzData, err := memfs.ReadFile(ctx, art.Path)
	if err != nil {
		t.Fatalf("read tarball: %v", err)
	}
	entries := tarEntries(t, tgzData)

	if _, ok := entries["package/package.json"]; !ok {
		t.Error("missing package/package.json in tarball")
	}

	// Verify package.json content.
	var pkg map[string]any
	if err := json.Unmarshal(entries["package/package.json"], &pkg); err != nil {
		t.Fatalf("invalid package.json: %v", err)
	}
	if got := pkg["name"]; got != "@acme/ai-agent-config" {
		t.Errorf("package.json name = %v, want @acme/ai-agent-config", got)
	}
	if got := pkg["version"]; got != "2.0.0" {
		t.Errorf("package.json version = %v, want 2.0.0", got)
	}

	// Verify content files are included under package/ prefix.
	hasContent := false
	for name := range entries {
		if strings.HasPrefix(name, "package/local-dev/") {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("no content files found under package/ in tarball")
	}
}

func TestPackage_OCIReturnsNotImplemented(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		OCI: &packaging.OCIPackagingConfig{
			Enabled: true,
			Targets: []build.Target{build.TargetClaude},
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	_, err := svc.Package(context.Background(), result, config)
	if err == nil {
		t.Fatal("expected error for OCI packaging")
	}
	if !errors.Is(err, packaging.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}

func TestPackage_DisabledPackagersSkipped(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		VSCodeExtension: &packaging.VSCodePackagingConfig{
			Enabled:   false,
			Publisher: "test",
			Targets:   []build.Target{build.TargetCopilot},
		},
		NPM: &packaging.NPMPackagingConfig{
			Enabled: false,
			Scope:   "@test",
			Targets: []build.Target{build.TargetClaude},
		},
		OCI: &packaging.OCIPackagingConfig{
			Enabled: false,
			Targets: []build.Target{build.TargetClaude},
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	pkgResult, err := svc.Package(context.Background(), result, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgResult.Artifacts) != 0 {
		t.Errorf("expected 0 artifacts for disabled packagers, got %d", len(pkgResult.Artifacts))
	}
}

func TestPackage_EmptyMaterializationResult(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := pipeline.MaterializationResult{}

	config := packaging.PackagingConfig{
		VSCodeExtension: &packaging.VSCodePackagingConfig{
			Enabled:   true,
			Publisher: "test",
			Targets:   []build.Target{build.TargetCopilot},
		},
		NPM: &packaging.NPMPackagingConfig{
			Enabled: true,
			Scope:   "@test",
			Targets: []build.Target{build.TargetClaude},
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	pkgResult, err := svc.Package(context.Background(), result, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both should produce artifacts (with only the generated metadata files).
	if len(pkgResult.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(pkgResult.Artifacts))
	}
}

func TestPackage_EmptyConfig(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	svc := packaging.NewPackagingService(memfs, memfs)
	pkgResult, err := svc.Package(context.Background(), result, packaging.PackagingConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgResult.Artifacts) != 0 {
		t.Errorf("expected 0 artifacts for empty config, got %d", len(pkgResult.Artifacts))
	}
}

func TestPackage_MultiplePackagers(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		VSCodeExtension: &packaging.VSCodePackagingConfig{
			Enabled:   true,
			Publisher: "multi",
			Targets:   []build.Target{build.TargetCopilot},
		},
		NPM: &packaging.NPMPackagingConfig{
			Enabled: true,
			Scope:   "@multi",
			Targets: []build.Target{build.TargetClaude},
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	pkgResult, err := svc.Package(context.Background(), result, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgResult.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(pkgResult.Artifacts))
	}

	types := map[string]bool{}
	for _, a := range pkgResult.Artifacts {
		types[a.Type] = true
	}
	if !types["vsix"] {
		t.Error("missing vsix artifact")
	}
	if !types["npm"] {
		t.Error("missing npm artifact")
	}
}

func TestPackage_WithOutputDir(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		VSCodeExtension: &packaging.VSCodePackagingConfig{
			Enabled:   true,
			Publisher: "test",
			Targets:   []build.Target{build.TargetCopilot},
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs, packaging.WithOutputDir("custom/output"))
	pkgResult, err := svc.Package(context.Background(), result, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgResult.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(pkgResult.Artifacts))
	}
	if !strings.HasPrefix(pkgResult.Artifacts[0].Path, "custom/output/") {
		t.Errorf("expected path under custom/output/, got %s", pkgResult.Artifacts[0].Path)
	}
}

func TestPackage_DefaultVersion(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		VSCodeExtension: &packaging.VSCodePackagingConfig{
			Enabled:   true,
			Publisher: "test",
			Targets:   []build.Target{build.TargetCopilot},
			// Version omitted — should default to 0.0.1.
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	pkgResult, err := svc.Package(context.Background(), result, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkgResult.Artifacts[0].Metadata["version"] != "0.0.1" {
		t.Errorf("expected default version 0.0.1, got %s", pkgResult.Artifacts[0].Metadata["version"])
	}
}

func TestPackage_VSIXDeterministicOrder(t *testing.T) {
	memfs := filesystem.NewMemFS()
	result := seedFiles(t, memfs)

	config := packaging.PackagingConfig{
		VSCodeExtension: &packaging.VSCodePackagingConfig{
			Enabled:   true,
			Publisher: "test",
			Targets:   []build.Target{build.TargetCopilot, build.TargetCursor},
		},
	}

	svc := packaging.NewPackagingService(memfs, memfs)
	ctx := context.Background()

	// Run twice and compare archive entries order.
	var entryOrders [2][]string
	for i := range 2 {
		memfs.Reset()
		result = seedFiles(t, memfs)
		pkgResult, err := svc.Package(ctx, result, config)
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		data, err := memfs.ReadFile(ctx, pkgResult.Artifacts[0].Path)
		if err != nil {
			t.Fatalf("run %d: read vsix: %v", i, err)
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("run %d: open zip: %v", i, err)
		}
		for _, f := range zr.File {
			entryOrders[i] = append(entryOrders[i], f.Name)
		}
	}

	if len(entryOrders[0]) != len(entryOrders[1]) {
		t.Fatalf("different entry counts: %d vs %d", len(entryOrders[0]), len(entryOrders[1]))
	}
	for i := range entryOrders[0] {
		if entryOrders[0][i] != entryOrders[1][i] {
			t.Errorf("entry order mismatch at %d: %s vs %s", i, entryOrders[0][i], entryOrders[1][i])
		}
	}
}

// --- helpers ---

func zipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	entries := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		entries[f.Name] = content
	}
	return entries
}

func tarEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	entries := make(map[string][]byte)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar entry %s: %v", header.Name, err)
		}
		entries[header.Name] = content
	}
	return entries
}
