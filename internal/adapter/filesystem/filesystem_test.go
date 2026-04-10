package filesystem_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func ctx() context.Context { return context.Background() }

func mustTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "goagentmeta-fs-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// ─── OSReader ────────────────────────────────────────────────────────────────

func TestOSReader_ReadFile(t *testing.T) {
	dir := mustTempDir(t)
	want := []byte("hello world")
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	r := filesystem.NewOSReader()
	got, err := r.ReadFile(ctx(), path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ReadFile = %q; want %q", got, want)
	}
}

func TestOSReader_ReadFile_Missing(t *testing.T) {
	r := filesystem.NewOSReader()
	_, err := r.ReadFile(ctx(), "/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestOSReader_ReadDir(t *testing.T) {
	dir := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := filesystem.NewOSReader()
	entries, err := r.ReadDir(ctx(), dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("ReadDir returned %d entries; want 2", len(entries))
	}
}

func TestOSReader_ReadDir_Missing(t *testing.T) {
	r := filesystem.NewOSReader()
	_, err := r.ReadDir(ctx(), "/nonexistent/path/")
	if err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

func TestOSReader_Stat(t *testing.T) {
	dir := mustTempDir(t)
	path := filepath.Join(dir, "stat.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := filesystem.NewOSReader()
	info, err := r.Stat(ctx(), path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.IsDir() {
		t.Error("Stat: expected file, got dir")
	}
	if info.Size() != 4 {
		t.Errorf("Stat.Size = %d; want 4", info.Size())
	}
}

func TestOSReader_Stat_Missing(t *testing.T) {
	r := filesystem.NewOSReader()
	_, err := r.Stat(ctx(), "/nonexistent/path.txt")
	if err == nil {
		t.Error("expected error for missing path, got nil")
	}
}

func TestOSReader_Glob(t *testing.T) {
	dir := mustTempDir(t)
	for _, name := range []string{"a.yaml", "b.yaml", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	r := filesystem.NewOSReader()
	matches, err := r.Glob(ctx(), filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("Glob matched %d files; want 2", len(matches))
	}
}

func TestOSReader_Glob_NoMatches(t *testing.T) {
	dir := mustTempDir(t)
	r := filesystem.NewOSReader()
	matches, err := r.Glob(ctx(), filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("Glob returned %d matches; want 0", len(matches))
	}
}

// ─── OSWriter ────────────────────────────────────────────────────────────────

func TestOSWriter_WriteFile_CreatesParents(t *testing.T) {
	dir := mustTempDir(t)
	path := filepath.Join(dir, "a", "b", "c.txt")
	content := []byte("nested")

	w := filesystem.NewOSWriter()
	if err := w.WriteFile(ctx(), path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after WriteFile: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("file content = %q; want %q", got, content)
	}
}

func TestOSWriter_WriteFile_Overwrite(t *testing.T) {
	dir := mustTempDir(t)
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("old"), 0o644) //nolint:errcheck

	w := filesystem.NewOSWriter()
	if err := w.WriteFile(ctx(), path, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("expected overwritten content %q; got %q", "new", got)
	}
}

func TestOSWriter_MkdirAll(t *testing.T) {
	dir := mustTempDir(t)
	path := filepath.Join(dir, "x", "y", "z")

	w := filesystem.NewOSWriter()
	if err := w.MkdirAll(ctx(), path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat after MkdirAll: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestOSWriter_Remove(t *testing.T) {
	dir := mustTempDir(t)
	path := filepath.Join(dir, "rem.txt")
	os.WriteFile(path, []byte("data"), 0o644) //nolint:errcheck

	w := filesystem.NewOSWriter()
	if err := w.Remove(ctx(), path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file still exists after Remove")
	}
}

func TestOSWriter_Remove_Missing(t *testing.T) {
	w := filesystem.NewOSWriter()
	err := w.Remove(ctx(), "/nonexistent/path.txt")
	if err == nil {
		t.Error("expected error removing nonexistent file, got nil")
	}
}

func TestOSWriter_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks may require elevated privileges on Windows")
	}
	dir := mustTempDir(t)
	src := filepath.Join(dir, "src.txt")
	os.WriteFile(src, []byte("sym"), 0o644) //nolint:errcheck
	dest := filepath.Join(dir, "link.txt")

	w := filesystem.NewOSWriter()
	if err := w.Symlink(ctx(), src, dest); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	info, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("Lstat after Symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

// ─── OSMaterializer ──────────────────────────────────────────────────────────

func makeSimplePlan(outputDir string) pipeline.EmissionPlan {
	return pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			outputDir: {
				Coordinate: build.BuildCoordinate{
					Unit: build.BuildUnit{
						Target:  build.TargetClaude,
						Profile: build.ProfileLocalDev,
					},
				},
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("# Instructions\n"), Layer: pipeline.LayerInstruction},
					{Path: "sub/config.json", Content: []byte(`{"key":"val"}`), Layer: pipeline.LayerExtension},
				},
				Directories: []string{"assets"},
			},
		},
	}
}

func TestOSMaterializer_Materialize_Basic(t *testing.T) {
	dir := mustTempDir(t)
	outputDir := filepath.Join(dir, "out")
	plan := makeSimplePlan(outputDir)

	m := filesystem.NewOSMaterializer()
	result, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	if len(result.WrittenFiles) != 2 {
		t.Errorf("written files = %d; want 2", len(result.WrittenFiles))
	}

	// Verify file content on disk.
	got, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if string(got) != "# Instructions\n" {
		t.Errorf("CLAUDE.md content = %q; want %q", got, "# Instructions\n")
	}
}

func TestOSMaterializer_Materialize_Idempotent(t *testing.T) {
	dir := mustTempDir(t)
	outputDir := filepath.Join(dir, "out")
	plan := makeSimplePlan(outputDir)

	m := filesystem.NewOSMaterializer()

	// First run.
	result1, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("first Materialize: %v", err)
	}

	// Capture mod times after first run.
	stat1, _ := os.Stat(filepath.Join(outputDir, "CLAUDE.md"))

	// Second run — identical content, should be skipped.
	result2, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("second Materialize: %v", err)
	}
	if len(result2.WrittenFiles) != 0 {
		t.Errorf("second run wrote %d files; want 0 (idempotent)", len(result2.WrittenFiles))
	}

	stat2, _ := os.Stat(filepath.Join(outputDir, "CLAUDE.md"))
	if !stat1.ModTime().Equal(stat2.ModTime()) {
		t.Logf("note: mod time changed even though content identical (OS behaviour may vary)")
	}
	_ = result1
}

func TestOSMaterializer_Materialize_WithPluginBundle(t *testing.T) {
	dir := mustTempDir(t)
	outputDir := filepath.Join(dir, "out")
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			outputDir: {
				Coordinate: build.BuildCoordinate{
					Unit: build.BuildUnit{Target: build.TargetClaude},
				},
				PluginBundles: []pipeline.EmittedPlugin{
					{
						PluginID: "my-plugin",
						DestDir:  "plugins/my-plugin",
						Files: []pipeline.EmittedFile{
							{Path: "index.js", Content: []byte("// plugin\n")},
						},
					},
				},
			},
		},
	}

	m := filesystem.NewOSMaterializer()
	result, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.WrittenFiles) != 1 {
		t.Errorf("written files = %d; want 1", len(result.WrittenFiles))
	}
	pluginPath := filepath.Join(outputDir, "plugins", "my-plugin", "index.js")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Errorf("plugin file missing: %v", err)
	}
}

// ─── MemFS – Reader ───────────────────────────────────────────────────────────

func TestMemFS_ReadFile_RoundTrip(t *testing.T) {
	m := filesystem.NewMemFS()
	content := []byte("hello mem")
	if err := m.WriteFile(ctx(), "/a/b.txt", content, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := m.ReadFile(ctx(), "/a/b.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q; want %q", got, content)
	}
}

func TestMemFS_ReadFile_Missing(t *testing.T) {
	m := filesystem.NewMemFS()
	_, err := m.ReadFile(ctx(), "/missing.txt")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestMemFS_ReadFile_ReturnsCopy(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/f.txt", []byte("original"), 0o644) //nolint:errcheck
	got, _ := m.ReadFile(ctx(), "/f.txt")
	got[0] = 'X' // mutate returned slice
	got2, _ := m.ReadFile(ctx(), "/f.txt")
	if got2[0] == 'X' {
		t.Error("ReadFile returned shared reference; expected defensive copy")
	}
}

func TestMemFS_ReadDir(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/root/a.yaml", []byte("a"), 0o644)     //nolint:errcheck
	m.WriteFile(ctx(), "/root/b.yaml", []byte("b"), 0o644)     //nolint:errcheck
	m.WriteFile(ctx(), "/root/sub/c.yaml", []byte("c"), 0o644) //nolint:errcheck

	entries, err := m.ReadDir(ctx(), "/root")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	// Expect a.yaml, b.yaml, sub (dir) — 3 entries.
	if len(entries) != 3 {
		t.Errorf("ReadDir = %d entries; want 3", len(entries))
		for _, e := range entries {
			t.Logf("  %s isDir=%v", e.Name(), e.IsDir())
		}
	}
}

func TestMemFS_ReadDir_OnlyDirectChildren(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/p/a.txt", []byte{}, 0o644)     //nolint:errcheck
	m.WriteFile(ctx(), "/p/q/b.txt", []byte{}, 0o644)   //nolint:errcheck
	m.WriteFile(ctx(), "/p/q/r/c.txt", []byte{}, 0o644) //nolint:errcheck

	entries, err := m.ReadDir(ctx(), "/p")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	// Expect a.txt and q (dir) — 2 entries.
	if len(entries) != 2 {
		t.Errorf("ReadDir = %d entries; want 2", len(entries))
		for _, e := range entries {
			t.Logf("  %s isDir=%v", e.Name(), e.IsDir())
		}
	}
}

func TestMemFS_ReadDir_MissingDir(t *testing.T) {
	m := filesystem.NewMemFS()
	_, err := m.ReadDir(ctx(), "/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestMemFS_Stat_File(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/file.txt", []byte("12345"), 0o644) //nolint:errcheck

	info, err := m.Stat(ctx(), "/file.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.IsDir() {
		t.Error("Stat: expected file, got dir")
	}
	if info.Size() != 5 {
		t.Errorf("Stat.Size = %d; want 5", info.Size())
	}
}

func TestMemFS_Stat_Dir(t *testing.T) {
	m := filesystem.NewMemFS()
	m.MkdirAll(ctx(), "/mydir", 0o755) //nolint:errcheck

	info, err := m.Stat(ctx(), "/mydir")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("Stat: expected dir, got file")
	}
}

func TestMemFS_Stat_Missing(t *testing.T) {
	m := filesystem.NewMemFS()
	_, err := m.Stat(ctx(), "/ghost.txt")
	if err == nil {
		t.Error("expected error for missing path, got nil")
	}
}

func TestMemFS_Glob(t *testing.T) {
	m := filesystem.NewMemFS()
	for _, p := range []string{"/ai/a.yaml", "/ai/b.yaml", "/ai/c.md"} {
		m.WriteFile(ctx(), p, []byte("x"), 0o644) //nolint:errcheck
	}

	matches, err := m.Glob(ctx(), "/ai/*.yaml")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("Glob = %d matches; want 2", len(matches))
	}
}

func TestMemFS_Glob_NoMatches(t *testing.T) {
	m := filesystem.NewMemFS()
	matches, err := m.Glob(ctx(), "/ai/*.yaml")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("Glob = %d matches; want 0", len(matches))
	}
}

// ─── MemFS – Writer ───────────────────────────────────────────────────────────

func TestMemFS_WriteFile_RegistersParentDirs(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/a/b/c.txt", []byte("x"), 0o644) //nolint:errcheck

	dirs := m.Dirs()
	found := false
	for _, d := range dirs {
		if d == "/a/b" {
			found = true
		}
	}
	if !found {
		t.Errorf("parent dir /a/b not registered after WriteFile; dirs = %v", dirs)
	}
}

func TestMemFS_MkdirAll(t *testing.T) {
	m := filesystem.NewMemFS()
	m.MkdirAll(ctx(), "/x/y/z", 0o755) //nolint:errcheck

	info, err := m.Stat(ctx(), "/x/y/z")
	if err != nil {
		t.Fatalf("Stat after MkdirAll: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected dir")
	}
}

func TestMemFS_Symlink(t *testing.T) {
	m := filesystem.NewMemFS()
	m.Symlink(ctx(), "/old/target.txt", "/new/link.txt") //nolint:errcheck

	syms := m.Symlinks()
	if syms["/new/link.txt"] != "/old/target.txt" {
		t.Errorf("symlink not recorded; got %v", syms)
	}
}

func TestMemFS_Remove_File(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/del.txt", []byte("bye"), 0o644) //nolint:errcheck
	if err := m.Remove(ctx(), "/del.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := m.ReadFile(ctx(), "/del.txt"); err == nil {
		t.Error("file still readable after Remove")
	}
}

func TestMemFS_Remove_Symlink(t *testing.T) {
	m := filesystem.NewMemFS()
	m.Symlink(ctx(), "/src.txt", "/lnk.txt") //nolint:errcheck
	if err := m.Remove(ctx(), "/lnk.txt"); err != nil {
		t.Fatalf("Remove symlink: %v", err)
	}
	if _, ok := m.Symlinks()["/lnk.txt"]; ok {
		t.Error("symlink still exists after Remove")
	}
}

func TestMemFS_Remove_EmptyDir(t *testing.T) {
	m := filesystem.NewMemFS()
	m.MkdirAll(ctx(), "/emptydir", 0o755) //nolint:errcheck
	if err := m.Remove(ctx(), "/emptydir"); err != nil {
		t.Fatalf("Remove empty dir: %v", err)
	}
}

func TestMemFS_Remove_NonEmptyDir(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/nonempty/file.txt", []byte("x"), 0o644) //nolint:errcheck
	if err := m.Remove(ctx(), "/nonempty"); err == nil {
		t.Error("expected error removing non-empty dir, got nil")
	}
}

func TestMemFS_Remove_Missing(t *testing.T) {
	m := filesystem.NewMemFS()
	if err := m.Remove(ctx(), "/ghost.txt"); err == nil {
		t.Error("expected error removing nonexistent path, got nil")
	}
}

// ─── MemFS – Materializer ─────────────────────────────────────────────────────

func TestMemFS_Materialize_Basic(t *testing.T) {
	m := filesystem.NewMemFS()
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			"/out": {
				Coordinate: build.BuildCoordinate{
					Unit: build.BuildUnit{Target: build.TargetCopilot},
				},
				Files: []pipeline.EmittedFile{
					{Path: "copilot-instructions.md", Content: []byte("# Copilot\n"), Layer: pipeline.LayerInstruction},
				},
			},
		},
	}

	result, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Errorf("errors: %v", result.Errors)
	}
	if len(result.WrittenFiles) != 1 {
		t.Errorf("written = %d; want 1", len(result.WrittenFiles))
	}

	content, err := m.ReadFile(ctx(), "/out/copilot-instructions.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "# Copilot\n" {
		t.Errorf("content = %q; want %q", content, "# Copilot\n")
	}
}

// TestMemFS_Materialize_EmptyContent verifies that files with nil or zero-length
// content are written correctly (regression for idempotency check bug where
// bytes.Equal(nil, nil) == true caused the write to be skipped).
func TestMemFS_Materialize_EmptyContent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content []byte
	}{
		{"nil_content", nil},
		{"empty_content", []byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := filesystem.NewMemFS()
			plan := pipeline.EmissionPlan{
				Units: map[string]pipeline.UnitEmission{
					"/out": {
						Files: []pipeline.EmittedFile{
							{Path: "empty.md", Content: tc.content},
						},
					},
				},
			}
			result, err := m.Materialize(ctx(), plan)
			if err != nil {
				t.Fatalf("Materialize: %v", err)
			}
			if len(result.WrittenFiles) != 1 {
				t.Errorf("written = %d; want 1 (file with empty content must still be created)", len(result.WrittenFiles))
			}
			// Verify the file can be read back (even if empty).
			if _, err := m.ReadFile(ctx(), "/out/empty.md"); err != nil {
				t.Errorf("ReadFile after materialize: %v", err)
			}
		})
	}
}

func TestMemFS_Materialize_Idempotent(t *testing.T) {
	m := filesystem.NewMemFS()
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			"/out": {
				Files: []pipeline.EmittedFile{
					{Path: "file.md", Content: []byte("same")},
				},
			},
		},
	}

	m.Materialize(ctx(), plan) //nolint:errcheck
	result2, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("second Materialize: %v", err)
	}
	if len(result2.WrittenFiles) != 0 {
		t.Errorf("second run wrote %d files; want 0 (idempotent)", len(result2.WrittenFiles))
	}
}

func TestMemFS_Materialize_WithAssets(t *testing.T) {
	m := filesystem.NewMemFS()
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			"/out": {
				Assets: []pipeline.EmittedAsset{
					{SourcePath: "/src/logo.png", DestPath: "assets/logo.png"},
				},
			},
		},
	}

	result, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.SymlinkedFiles) != 1 {
		t.Errorf("symlinked = %d; want 1", len(result.SymlinkedFiles))
	}
	if syms := m.Symlinks(); syms["/out/assets/logo.png"] != "/src/logo.png" {
		t.Errorf("symlink not correctly recorded; symlinks = %v", syms)
	}
}

func TestMemFS_Materialize_WithPluginBundle(t *testing.T) {
	m := filesystem.NewMemFS()
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			"/out": {
				PluginBundles: []pipeline.EmittedPlugin{
					{
						PluginID: "my-plugin",
						DestDir:  "plugins/my-plugin",
						Files: []pipeline.EmittedFile{
							{Path: "index.js", Content: []byte("// entry\n")},
						},
					},
				},
			},
		},
	}

	result, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.WrittenFiles) != 1 {
		t.Errorf("written = %d; want 1", len(result.WrittenFiles))
	}
	content, err := m.ReadFile(ctx(), "/out/plugins/my-plugin/index.js")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "// entry\n" {
		t.Errorf("plugin content = %q; want %q", content, "// entry\n")
	}
}

func TestMemFS_Files_ReturnsCopies(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/a.txt", []byte("original"), 0o644) //nolint:errcheck

	files := m.Files()
	files["/a.txt"][0] = 'X' // mutate returned copy

	// Internal state must be unaffected.
	got, _ := m.ReadFile(ctx(), "/a.txt")
	if got[0] == 'X' {
		t.Error("Files() returned shared reference; expected copy")
	}
}

// ─── MemFS – Reset ────────────────────────────────────────────────────────────

func TestMemFS_Reset(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/before.txt", []byte("data"), 0o644) //nolint:errcheck
	m.MkdirAll(ctx(), "/somedir", 0o755)                     //nolint:errcheck
	m.Symlink(ctx(), "/a", "/b")                             //nolint:errcheck

	m.Reset()

	if len(m.Files()) != 0 {
		t.Error("Files not empty after Reset")
	}
	if len(m.Dirs()) != 0 {
		t.Error("Dirs not empty after Reset")
	}
	if len(m.Symlinks()) != 0 {
		t.Error("Symlinks not empty after Reset")
	}
}

// ─── Interface compliance ─────────────────────────────────────────────────────

// Compile-time checks that all implementations satisfy the port interfaces.
var (
	// OSReader must satisfy Reader.
	_ portfs.Reader = (*filesystem.OSReader)(nil)
	// OSWriter must satisfy Writer.
	_ portfs.Writer = (*filesystem.OSWriter)(nil)
	// OSMaterializer must satisfy Materializer.
	_ portfs.Materializer = (*filesystem.OSMaterializer)(nil)

	// MemFS must satisfy all three port interfaces.
	_ portfs.Reader       = (*filesystem.MemFS)(nil)
	_ portfs.Writer       = (*filesystem.MemFS)(nil)
	_ portfs.Materializer = (*filesystem.MemFS)(nil)
)

// ─── Concurrency / race detector ─────────────────────────────────────────────

func TestMemFS_ConcurrentWrites(t *testing.T) {
	m := filesystem.NewMemFS()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(n int) {
			path := filepath.Join("/concurrent", filepath.Base(t.Name())+fmt.Sprintf("%d.txt", n))
			m.WriteFile(ctx(), path, []byte("x"), 0o644) //nolint:errcheck
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestMemFS_ConcurrentReadWrite(t *testing.T) {
	m := filesystem.NewMemFS()
	m.WriteFile(ctx(), "/shared.txt", []byte("initial"), 0o644) //nolint:errcheck

	done := make(chan struct{}, 100)
	// Writers
	for i := 0; i < 50; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			p := fmt.Sprintf("/concurrent/w%d.txt", n)
			m.WriteFile(ctx(), p, []byte("data"), 0o644) //nolint:errcheck
		}(i)
	}
	// Readers
	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			m.ReadFile(ctx(), "/shared.txt") //nolint:errcheck
			m.ReadDir(ctx(), "/")            //nolint:errcheck
			m.Files()
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

// ─── Determinism ──────────────────────────────────────────────────────────────

func TestMemFS_Materialize_Deterministic(t *testing.T) {
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			"/out/alpha": {
				Files: []pipeline.EmittedFile{
					{Path: "a.md", Content: []byte("A")},
				},
			},
			"/out/beta": {
				Files: []pipeline.EmittedFile{
					{Path: "b.md", Content: []byte("B")},
				},
			},
			"/out/gamma": {
				Files: []pipeline.EmittedFile{
					{Path: "c.md", Content: []byte("C")},
				},
			},
		},
	}

	var firstRun []string
	for i := 0; i < 20; i++ {
		m := filesystem.NewMemFS()
		result, err := m.Materialize(ctx(), plan)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if i == 0 {
			firstRun = result.WrittenFiles
		} else {
			for j, f := range result.WrittenFiles {
				if f != firstRun[j] {
					t.Fatalf("run %d: non-deterministic at index %d: got %q, first had %q", i, j, f, firstRun[j])
				}
			}
		}
	}
}

func TestOSMaterializer_Materialize_Deterministic(t *testing.T) {
	dir := mustTempDir(t)
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			filepath.Join(dir, "z"): {
				Files: []pipeline.EmittedFile{{Path: "z.md", Content: []byte("Z")}},
			},
			filepath.Join(dir, "a"): {
				Files: []pipeline.EmittedFile{{Path: "a.md", Content: []byte("A")}},
			},
			filepath.Join(dir, "m"): {
				Files: []pipeline.EmittedFile{{Path: "m.md", Content: []byte("M")}},
			},
		},
	}

	var firstRun []string
	for i := 0; i < 10; i++ {
		m := filesystem.NewOSMaterializer()
		result, err := m.Materialize(ctx(), plan)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if i == 0 {
			firstRun = result.WrittenFiles
		} else {
			for j, f := range result.WrittenFiles {
				if f != firstRun[j] {
					t.Fatalf("run %d: non-deterministic at index %d: got %q, first had %q", i, j, f, firstRun[j])
				}
			}
		}
	}
}

// ─── OSMaterializer — assets/scripts ──────────────────────────────────────────

func TestOSMaterializer_Materialize_WithAssetSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks may require elevated privileges on Windows")
	}
	dir := mustTempDir(t)
	srcDir := filepath.Join(dir, "src")
	outDir := filepath.Join(dir, "out")
	srcFile := filepath.Join(srcDir, "logo.png")
	os.MkdirAll(srcDir, 0o755)                                //nolint:errcheck
	os.WriteFile(srcFile, []byte("png-data"), 0o644) //nolint:errcheck

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			outDir: {
				Assets: []pipeline.EmittedAsset{
					{SourcePath: srcFile, DestPath: "assets/logo.png"},
				},
			},
		},
	}

	m := filesystem.NewOSMaterializer()
	result, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.SymlinkedFiles) != 1 {
		t.Errorf("symlinked = %d; want 1", len(result.SymlinkedFiles))
	}

	dest := filepath.Join(outDir, "assets", "logo.png")
	info, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("Lstat dest: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestOSMaterializer_Materialize_WithScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks may require elevated privileges on Windows")
	}
	dir := mustTempDir(t)
	srcDir := filepath.Join(dir, "src")
	outDir := filepath.Join(dir, "out")
	srcFile := filepath.Join(srcDir, "hook.sh")
	os.MkdirAll(srcDir, 0o755)                              //nolint:errcheck
	os.WriteFile(srcFile, []byte("#!/bin/sh\n"), 0o755) //nolint:errcheck

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			outDir: {
				Scripts: []pipeline.EmittedScript{
					{SourcePath: srcFile, DestPath: "scripts/hook.sh"},
				},
			},
		},
	}

	m := filesystem.NewOSMaterializer()
	result, err := m.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.SymlinkedFiles) != 1 {
		t.Errorf("symlinked = %d; want 1", len(result.SymlinkedFiles))
	}
}

// ─── NewMaterializer (injectable) ─────────────────────────────────────────────

func TestNewMaterializer_WithMemFS(t *testing.T) {
	mem := filesystem.NewMemFS()
	mem.WriteFile(ctx(), "/src/asset.txt", []byte("asset-data"), 0o644) //nolint:errcheck

	mat := filesystem.NewMaterializer(mem, mem)
	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			"/out": {
				Files: []pipeline.EmittedFile{
					{Path: "file.md", Content: []byte("content")},
				},
				Assets: []pipeline.EmittedAsset{
					{SourcePath: "/src/asset.txt", DestPath: "copied/asset.txt"},
				},
			},
		},
	}

	result, err := mat.Materialize(ctx(), plan)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(result.WrittenFiles) != 1 {
		t.Errorf("written = %d; want 1", len(result.WrittenFiles))
	}

	// Asset: copyOrSymlink attempts Symlink (which MemFS supports) then fallback.
	// MemFS.Symlink always succeeds, but through the injectable writer it tries
	// the writer.Symlink first. Since MemFS Symlink records it, we check that.
	// However, the copyOrSymlink first calls writer.Remove which may fail (ok, error is discarded).
	// Then calls writer.Symlink which succeeds on MemFS.
	if len(result.SymlinkedFiles) != 1 {
		t.Errorf("symlinked = %d; want 1", len(result.SymlinkedFiles))
	}

	// Verify the file was written.
	content, err := mem.ReadFile(ctx(), "/out/file.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("content = %q; want %q", content, "content")
	}
}

// ─── Empty plan ───────────────────────────────────────────────────────────────

func TestOSMaterializer_Materialize_EmptyPlan(t *testing.T) {
	m := filesystem.NewOSMaterializer()
	result, err := m.Materialize(ctx(), pipeline.EmissionPlan{})
	if err != nil {
		t.Fatalf("Materialize empty: %v", err)
	}
	if len(result.WrittenFiles) != 0 {
		t.Errorf("expected 0 written files, got %d", len(result.WrittenFiles))
	}
}

func TestMemFS_Materialize_EmptyPlan(t *testing.T) {
	m := filesystem.NewMemFS()
	result, err := m.Materialize(ctx(), pipeline.EmissionPlan{})
	if err != nil {
		t.Fatalf("Materialize empty: %v", err)
	}
	if len(result.WrittenFiles) != 0 {
		t.Errorf("expected 0 written files, got %d", len(result.WrittenFiles))
	}
}
