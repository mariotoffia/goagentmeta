package materializer_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	matstage "github.com/mariotoffia/goagentmeta/internal/adapter/stage/materializer"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func testContext() context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			Profile: build.ProfileLocalDev,
		},
		Report: &pipeline.BuildReport{},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func testContextWithFailFast() context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			Profile:  build.ProfileLocalDev,
			FailFast: true,
		},
		Report: &pipeline.BuildReport{},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func makeEmissionPlan() pipeline.EmissionPlan {
	return pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Coordinate: build.BuildCoordinate{
					Unit: build.BuildUnit{
						Target:  build.TargetClaude,
						Profile: build.ProfileLocalDev,
					},
					OutputDir: ".ai-build/claude/local-dev",
				},
				Files: []pipeline.EmittedFile{
					{
						Path:          "CLAUDE.md",
						Content:       []byte("# Claude Instructions\n\nBe helpful."),
						Layer:         pipeline.LayerInstruction,
						SourceObjects: []string{"instr-1"},
					},
					{
						Path:          ".claude/settings.json",
						Content:       []byte(`{"model": "claude-4"}`),
						Layer:         pipeline.LayerExtension,
						SourceObjects: []string{"config-1"},
					},
				},
				Directories: []string{"skills"},
				Assets: []pipeline.EmittedAsset{
					{SourcePath: "assets/logo.png", DestPath: "assets/logo.png"},
				},
				Scripts: []pipeline.EmittedScript{
					{SourcePath: "scripts/lint.sh", DestPath: "hooks/lint.sh"},
				},
				PluginBundles: []pipeline.EmittedPlugin{
					{
						PluginID: "my-plugin",
						DestDir:  "plugins/my-plugin",
						Files: []pipeline.EmittedFile{
							{Path: "manifest.json", Content: []byte(`{"name":"my-plugin"}`)},
						},
					},
				},
			},
		},
	}
}

func makeMultiUnitPlan() pipeline.EmissionPlan {
	return pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetClaude, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/claude/local-dev",
				},
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("claude instructions"), Layer: pipeline.LayerInstruction},
				},
			},
			".ai-build/copilot/local-dev": {
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetCopilot, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/copilot/local-dev",
				},
				Files: []pipeline.EmittedFile{
					{Path: "copilot-instructions.md", Content: []byte("copilot instructions"), Layer: pipeline.LayerInstruction},
				},
			},
		},
	}
}

// ─── Descriptor Tests ───────────────────────────────────────────────────────

func TestDescriptor(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)
	d := s.Descriptor()

	if d.Name != "materializer" {
		t.Errorf("expected name 'materializer', got %q", d.Name)
	}
	if d.Phase != pipeline.PhaseMaterialize {
		t.Errorf("expected phase PhaseMaterialize, got %d", d.Phase)
	}
	if d.Order != 10 {
		t.Errorf("expected order 10, got %d", d.Order)
	}
}

func TestFactory(t *testing.T) {
	mem := filesystem.NewMemFS()
	factory := matstage.Factory(mem)
	s, err := factory()
	if err != nil {
		t.Fatalf("unexpected factory error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil stage")
	}
}

// ─── Input Validation Tests ─────────────────────────────────────────────────

func TestExecute_InvalidInput_Error(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)
	_, err := s.Execute(context.Background(), "not an emission plan")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestExecute_PointerInput(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)
	plan := &pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}}
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error with pointer input: %v", err)
	}
	_, ok := result.(pipeline.MaterializationResult)
	if !ok {
		t.Fatalf("expected MaterializationResult, got %T", result)
	}
}

func TestExecute_NilPointerInput_Error(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)
	_, err := s.Execute(testContext(), (*pipeline.EmissionPlan)(nil))
	if err == nil {
		t.Fatal("expected error for nil pointer input")
	}
}

// ─── Single File Materialization Tests ──────────────────────────────────────

func TestExecute_SingleFile_Written(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello"), Layer: pipeline.LayerInstruction},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 1 {
		t.Fatalf("expected 1 written file, got %d", len(mr.WrittenFiles))
	}
	if mr.WrittenFiles[0] != ".ai-build/claude/local-dev/CLAUDE.md" {
		t.Errorf("unexpected path: %s", mr.WrittenFiles[0])
	}

	// Verify content in MemFS.
	files := mem.Files()
	content, ok := files[".ai-build/claude/local-dev/CLAUDE.md"]
	if !ok {
		t.Fatal("file not found in MemFS")
	}
	if string(content) != "hello" {
		t.Errorf("expected content 'hello', got %q", string(content))
	}
}

// ─── Directory Creation Tests ───────────────────────────────────────────────

func TestExecute_DirectoryCreatedBeforeFile(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Directories: []string{"skills", "hooks"},
				Files: []pipeline.EmittedFile{
					{Path: "skills/my-skill.md", Content: []byte("skill")},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// Output directory + 2 explicit directories = 3.
	if len(mr.CreatedDirs) < 3 {
		t.Errorf("expected at least 3 created dirs, got %d: %v", len(mr.CreatedDirs), mr.CreatedDirs)
	}

	dirs := mem.Dirs()
	dirSet := make(map[string]bool)
	for _, d := range dirs {
		dirSet[d] = true
	}
	if !dirSet[".ai-build/claude/local-dev/skills"] {
		t.Error("expected skills directory to exist")
	}
	if !dirSet[".ai-build/claude/local-dev/hooks"] {
		t.Error("expected hooks directory to exist")
	}
}

// ─── Asset and Script Materialization Tests ─────────────────────────────────

func TestExecute_AssetSymlinked(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Assets: []pipeline.EmittedAsset{
					{SourcePath: "assets/logo.png", DestPath: "assets/logo.png"},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.SymlinkedFiles) != 1 {
		t.Fatalf("expected 1 symlinked file, got %d", len(mr.SymlinkedFiles))
	}

	symlinks := mem.Symlinks()
	if target, ok := symlinks[".ai-build/claude/local-dev/assets/logo.png"]; !ok {
		t.Error("expected symlink to be created")
	} else if target != "assets/logo.png" {
		t.Errorf("expected symlink target 'assets/logo.png', got %q", target)
	}
}

func TestExecute_ScriptSymlinked(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Scripts: []pipeline.EmittedScript{
					{SourcePath: "scripts/lint.sh", DestPath: "hooks/lint.sh"},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.SymlinkedFiles) != 1 {
		t.Fatalf("expected 1 symlinked script, got %d", len(mr.SymlinkedFiles))
	}
}

// ─── Plugin Bundle Tests ────────────────────────────────────────────────────

func TestExecute_PluginBundleWritten(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				PluginBundles: []pipeline.EmittedPlugin{
					{
						PluginID: "my-plugin",
						DestDir:  "plugins/my-plugin",
						Files: []pipeline.EmittedFile{
							{Path: "manifest.json", Content: []byte(`{"name":"my-plugin"}`)},
							{Path: "index.js", Content: []byte("module.exports = {}")},
						},
					},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 2 {
		t.Fatalf("expected 2 plugin files written, got %d", len(mr.WrittenFiles))
	}

	files := mem.Files()
	if _, ok := files[".ai-build/claude/local-dev/plugins/my-plugin/manifest.json"]; !ok {
		t.Error("expected plugin manifest.json to be written")
	}
	if _, ok := files[".ai-build/claude/local-dev/plugins/my-plugin/index.js"]; !ok {
		t.Error("expected plugin index.js to be written")
	}
}

// ─── Full Plan Materialization ──────────────────────────────────────────────

func TestExecute_FullPlan_WrittenFiles(t *testing.T) {
	mem := filesystem.NewMemFS()
	// Pre-populate source files for assets/scripts so the materializer can symlink.
	_ = mem.WriteFile(context.Background(), "assets/logo.png", []byte("png"), 0o644)
	_ = mem.WriteFile(context.Background(), "scripts/lint.sh", []byte("#!/bin/sh"), 0o755)

	s := matstage.New(mem)
	plan := makeEmissionPlan()

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// 2 emitted files + 1 plugin file = 3 written files.
	if len(mr.WrittenFiles) != 3 {
		t.Errorf("expected 3 written files, got %d: %v", len(mr.WrittenFiles), mr.WrittenFiles)
	}

	// 1 asset + 1 script = 2 symlinked files.
	if len(mr.SymlinkedFiles) != 2 {
		t.Errorf("expected 2 symlinked files, got %d: %v", len(mr.SymlinkedFiles), mr.SymlinkedFiles)
	}

	// Output dir + skills + plugin dir = 3 created dirs (minimum).
	if len(mr.CreatedDirs) < 3 {
		t.Errorf("expected at least 3 created dirs, got %d: %v", len(mr.CreatedDirs), mr.CreatedDirs)
	}

	if len(mr.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(mr.Errors), mr.Errors)
	}
}

func TestExecute_MultiUnit(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)
	plan := makeMultiUnitPlan()

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 2 {
		t.Fatalf("expected 2 written files (one per unit), got %d", len(mr.WrittenFiles))
	}

	files := mem.Files()
	if _, ok := files[".ai-build/claude/local-dev/CLAUDE.md"]; !ok {
		t.Error("expected claude file")
	}
	if _, ok := files[".ai-build/copilot/local-dev/copilot-instructions.md"]; !ok {
		t.Error("expected copilot file")
	}
}

// ─── Dry-Run Tests ──────────────────────────────────────────────────────────

func TestExecute_DryRun_NoFilesWritten(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithDryRun(true))

	plan := makeEmissionPlan()

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// Dry-run should report what WOULD be written.
	if len(mr.WrittenFiles) == 0 {
		t.Error("expected dry-run to report files that would be written")
	}

	// But no actual files should exist in the filesystem.
	files := mem.Files()
	if len(files) != 0 {
		t.Errorf("expected 0 files in filesystem after dry-run, got %d", len(files))
	}

	dirs := mem.Dirs()
	if len(dirs) != 0 {
		t.Errorf("expected 0 directories in filesystem after dry-run, got %d", len(dirs))
	}

	symlinks := mem.Symlinks()
	if len(symlinks) != 0 {
		t.Errorf("expected 0 symlinks in filesystem after dry-run, got %d", len(symlinks))
	}
}

func TestExecute_DryRun_ReportsCorrectPaths(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithDryRun(true))

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
					{Path: ".claude/settings.json", Content: []byte("{}")},
				},
				Directories: []string{"skills"},
				Assets: []pipeline.EmittedAsset{
					{SourcePath: "assets/logo.png", DestPath: "assets/logo.png"},
				},
				PluginBundles: []pipeline.EmittedPlugin{
					{
						PluginID: "p1",
						DestDir:  "plugins/p1",
						Files:    []pipeline.EmittedFile{{Path: "m.json", Content: []byte("{}")}},
					},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// 2 emitted files + 1 plugin file = 3 written.
	if len(mr.WrittenFiles) != 3 {
		t.Errorf("dry-run expected 3 written files, got %d: %v", len(mr.WrittenFiles), mr.WrittenFiles)
	}

	// Output dir + skills + plugin dir = 3 dirs.
	if len(mr.CreatedDirs) != 3 {
		t.Errorf("dry-run expected 3 created dirs, got %d: %v", len(mr.CreatedDirs), mr.CreatedDirs)
	}

	// 1 asset = 1 symlinked.
	if len(mr.SymlinkedFiles) != 1 {
		t.Errorf("dry-run expected 1 symlinked file, got %d: %v", len(mr.SymlinkedFiles), mr.SymlinkedFiles)
	}
}

func TestExecute_DryRun_EmptyPlan(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithDryRun(true))

	plan := pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}}
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 0 || len(mr.CreatedDirs) != 0 || len(mr.SymlinkedFiles) != 0 {
		t.Error("expected empty result for empty plan")
	}
}

// ─── Idempotency Tests ─────────────────────────────────────────────────────

func TestExecute_Idempotent_SecondRunZeroChanges(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("same content"), Layer: pipeline.LayerInstruction},
				},
			},
		},
	}

	// First run: should write the file.
	result1, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}
	mr1 := result1.(pipeline.MaterializationResult)
	if len(mr1.WrittenFiles) != 1 {
		t.Fatalf("first run: expected 1 written file, got %d", len(mr1.WrittenFiles))
	}

	// Second run with same content: should skip (0 files changed).
	result2, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}
	mr2 := result2.(pipeline.MaterializationResult)
	if len(mr2.WrittenFiles) != 0 {
		t.Errorf("second run: expected 0 written files (idempotent), got %d: %v",
			len(mr2.WrittenFiles), mr2.WrittenFiles)
	}
}

func TestExecute_Idempotent_ChangedContent_Rewrites(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan1 := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("version 1")},
				},
			},
		},
	}

	result1, _ := s.Execute(testContext(), plan1)
	mr1 := result1.(pipeline.MaterializationResult)
	if len(mr1.WrittenFiles) != 1 {
		t.Fatalf("first run: expected 1 file, got %d", len(mr1.WrittenFiles))
	}

	plan2 := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("version 2")},
				},
			},
		},
	}

	result2, _ := s.Execute(testContext(), plan2)
	mr2 := result2.(pipeline.MaterializationResult)
	if len(mr2.WrittenFiles) != 1 {
		t.Errorf("second run: expected 1 rewrite, got %d", len(mr2.WrittenFiles))
	}

	content := mem.Files()[".ai-build/claude/local-dev/CLAUDE.md"]
	if string(content) != "version 2" {
		t.Errorf("expected 'version 2', got %q", string(content))
	}
}

// ─── Sync Mode Tests ────────────────────────────────────────────────────────

func TestExecute_SyncBuildOnly_NoRepoRootFiles(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithSyncMode(matstage.SyncBuildOnly))

	plan := makeEmissionPlan()
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// All written files should be under .ai-build/.
	for _, f := range mr.WrittenFiles {
		if len(f) < 10 || f[:10] != ".ai-build/" {
			t.Errorf("build-only: file %q should be under .ai-build/", f)
		}
	}
}

func TestExecute_SyncCopy_FilesInRepoRoot(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithSyncMode(matstage.SyncCopy))

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// Should have files both in .ai-build/ and repo root.
	files := mem.Files()
	if _, ok := files[".ai-build/claude/local-dev/CLAUDE.md"]; !ok {
		t.Error("expected file in .ai-build/")
	}
	if _, ok := files["CLAUDE.md"]; !ok {
		t.Error("expected file copied to repo root")
	}

	// The sync copy files should appear in WrittenFiles.
	hasCopied := false
	for _, f := range mr.WrittenFiles {
		if f == "CLAUDE.md" {
			hasCopied = true
		}
	}
	if !hasCopied {
		t.Error("expected CLAUDE.md in WrittenFiles from sync copy")
	}
}

func TestExecute_SyncSymlink_SymlinksInRepoRoot(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithSyncMode(matstage.SyncSymlink))

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// Should have symlinks at repo root pointing to .ai-build/.
	symlinks := mem.Symlinks()
	target, ok := symlinks["CLAUDE.md"]
	if !ok {
		t.Fatal("expected symlink at repo root CLAUDE.md")
	}
	if target != ".ai-build/claude/local-dev/CLAUDE.md" {
		t.Errorf("expected symlink target '.ai-build/claude/local-dev/CLAUDE.md', got %q", target)
	}

	// SymlinkedFiles should include the repo-root symlink.
	hasSymlink := false
	for _, f := range mr.SymlinkedFiles {
		if f == "CLAUDE.md" {
			hasSymlink = true
		}
	}
	if !hasSymlink {
		t.Error("expected CLAUDE.md in SymlinkedFiles from sync symlink")
	}
}

func TestExecute_SyncAdoptSelected_OnlyMatchingFiles(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem,
		matstage.WithSyncMode(matstage.SyncAdoptSelected),
		matstage.WithSyncPatterns([]string{"*.md"}),
	)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
					{Path: ".claude/settings.json", Content: []byte("{}")},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	files := mem.Files()
	// .md file should be in repo root.
	if _, ok := files["CLAUDE.md"]; !ok {
		t.Error("expected CLAUDE.md in repo root (matches *.md)")
	}
	// .json file should NOT be in repo root.
	if _, ok := files[".claude/settings.json"]; ok {
		t.Error("unexpected settings.json in repo root (should not match *.md)")
	}

	// Count adopted files.
	adoptedCount := 0
	for _, f := range mr.WrittenFiles {
		if f == "CLAUDE.md" {
			adoptedCount++
		}
	}
	if adoptedCount != 1 {
		t.Errorf("expected 1 adopted file, got %d", adoptedCount)
	}
}

// ─── Error Handling Tests ───────────────────────────────────────────────────

func TestExecute_EmptyPlan_NoError(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}}
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 0 {
		t.Errorf("expected 0 files, got %d", len(mr.WrittenFiles))
	}
}

func TestExecute_NilUnitsMap(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{Units: nil}
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 0 {
		t.Errorf("expected 0 files, got %d", len(mr.WrittenFiles))
	}
}

// ─── Diagnostics Tests ─────────────────────────────────────────────────────

func TestExecute_DiagnosticsEmitted(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	ctx := testContext()
	plan := makeEmissionPlan()
	_, _ = s.Execute(ctx, plan)

	cc := compiler.CompilerFromContext(ctx)
	if cc == nil || cc.Report == nil {
		t.Fatal("expected compiler context with report")
	}

	hasMaterializeStart := false
	hasMaterializeComplete := false
	for _, d := range cc.Report.Diagnostics {
		if d.Code == "MATERIALIZE_START" {
			hasMaterializeStart = true
		}
		if d.Code == "MATERIALIZE_COMPLETE" {
			hasMaterializeComplete = true
		}
	}

	if !hasMaterializeStart {
		t.Error("expected MATERIALIZE_START diagnostic")
	}
	if !hasMaterializeComplete {
		t.Error("expected MATERIALIZE_COMPLETE diagnostic")
	}
}

func TestExecute_DryRun_DiagnosticsEmitted(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithDryRun(true))

	ctx := testContext()
	plan := makeEmissionPlan()
	_, _ = s.Execute(ctx, plan)

	cc := compiler.CompilerFromContext(ctx)
	hasDryRunDiag := false
	for _, d := range cc.Report.Diagnostics {
		if d.Code == "MATERIALIZE_DRY_RUN" {
			hasDryRunDiag = true
		}
	}
	if !hasDryRunDiag {
		t.Error("expected MATERIALIZE_DRY_RUN diagnostic")
	}
}

// ─── Option Tests ───────────────────────────────────────────────────────────

func TestWithOptions(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem,
		matstage.WithDryRun(true),
		matstage.WithSyncMode(matstage.SyncCopy),
		matstage.WithSyncPatterns([]string{"*.md"}),
	)

	// Just verify it doesn't panic and returns the expected type.
	plan := pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}}
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(pipeline.MaterializationResult); !ok {
		t.Fatalf("expected MaterializationResult, got %T", result)
	}
}

// ─── Context Without CompilerContext ────────────────────────────────────────

func TestExecute_NoCompilerContext(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	// Should work without a CompilerContext, just won't emit diagnostics.
	result, err := s.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 1 {
		t.Errorf("expected 1 written file, got %d", len(mr.WrittenFiles))
	}
}

// ─── SyncMode String Tests ─────────────────────────────────────────────────

func TestSyncMode_String(t *testing.T) {
	tests := []struct {
		mode matstage.SyncMode
		want string
	}{
		{matstage.SyncBuildOnly, "build-only"},
		{matstage.SyncCopy, "copy"},
		{matstage.SyncSymlink, "symlink"},
		{matstage.SyncAdoptSelected, "adopt-selected"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("SyncMode(%q).String() = %q, want %q", string(tt.mode), got, tt.want)
		}
	}
}

// ─── Factory with Options ───────────────────────────────────────────────────

func TestFactory_WithOptions(t *testing.T) {
	mem := filesystem.NewMemFS()
	factory := matstage.Factory(mem, matstage.WithDryRun(true), matstage.WithSyncMode(matstage.SyncCopy))
	s, err := factory()
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}

	plan := pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}}
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(pipeline.MaterializationResult); !ok {
		t.Fatalf("expected MaterializationResult, got %T", result)
	}
}

// ─── Multi-Unit Sync Copy ───────────────────────────────────────────────────

func TestExecute_SyncCopy_MultiUnit(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithSyncMode(matstage.SyncCopy))
	plan := makeMultiUnitPlan()

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	files := mem.Files()
	// Both targets should have repo-root copies.
	if _, ok := files["CLAUDE.md"]; !ok {
		t.Error("expected CLAUDE.md in repo root")
	}
	if _, ok := files["copilot-instructions.md"]; !ok {
		t.Error("expected copilot-instructions.md in repo root")
	}

	_ = mr // used to avoid unused variable
}

// ─── FailFast Behavior ─────────────────────────────────────────────────────

func TestExecute_FailFast_NotTriggeredOnSuccess(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	result, err := s.Execute(testContextWithFailFast(), plan)
	if err != nil {
		t.Fatalf("unexpected error on success with fail-fast: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.WrittenFiles) != 1 {
		t.Errorf("expected 1 written file, got %d", len(mr.WrittenFiles))
	}
}

// ─── Error-Injecting Materializer Tests ─────────────────────────────────────

// errorMaterializer is a test materializer that fails on specific paths.
type errorMaterializer struct {
	failPaths map[string]bool
	mem       *filesystem.MemFS
}

func newErrorMaterializer(failPaths ...string) *errorMaterializer {
	fp := make(map[string]bool)
	for _, p := range failPaths {
		fp[p] = true
	}
	return &errorMaterializer{
		failPaths: fp,
		mem:       filesystem.NewMemFS(),
	}
}

func (e *errorMaterializer) Materialize(ctx context.Context, plan pipeline.EmissionPlan) (pipeline.MaterializationResult, error) {
	var result pipeline.MaterializationResult

	for outputDir, unit := range plan.Units {
		for _, f := range unit.Files {
			path := outputDir + "/" + f.Path
			if e.failPaths[path] {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: path,
					Err:  "injected error",
				})
				continue
			}
			result.WrittenFiles = append(result.WrittenFiles, path)
		}
	}

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("materialization completed with %d error(s)", len(result.Errors))
	}
	return result, nil
}

func TestExecute_FailFast_TriggeredOnError(t *testing.T) {
	em := newErrorMaterializer(".ai-build/claude/local-dev/CLAUDE.md")
	s := matstage.New(em)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	_, err := s.Execute(testContextWithFailFast(), plan)
	if err == nil {
		t.Fatal("expected error with fail-fast on materialization failure")
	}
}

func TestExecute_NoFailFast_ContinuesOnError(t *testing.T) {
	em := newErrorMaterializer(".ai-build/claude/local-dev/CLAUDE.md")
	s := matstage.New(em)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
					{Path: "other.md", Content: []byte("world")},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	// Without fail-fast, error from Materialize is swallowed (partial results returned).
	if err != nil {
		t.Fatalf("expected no error without fail-fast, got: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)
	if len(mr.Errors) == 0 {
		t.Error("expected errors in result for partial failure")
	}
	if len(mr.WrittenFiles) != 1 {
		t.Errorf("expected 1 successful file, got %d", len(mr.WrittenFiles))
	}
}

// ─── SyncMode with Non-Reader+Writer Materializer ──────────────────────────

// materializerOnly implements only the Materializer port (no Reader/Writer).
type materializerOnly struct{}

func (m *materializerOnly) Materialize(_ context.Context, _ pipeline.EmissionPlan) (pipeline.MaterializationResult, error) {
	return pipeline.MaterializationResult{}, nil
}

func TestExecute_SyncMode_NonReaderWriter_Error(t *testing.T) {
	mo := &materializerOnly{}
	s := matstage.New(mo, matstage.WithSyncMode(matstage.SyncCopy))

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	ctx := testContext()
	result, _ := s.Execute(ctx, plan)

	mr := result.(pipeline.MaterializationResult)
	if len(mr.Errors) == 0 {
		t.Error("expected error when materializer doesn't implement Reader+Writer for sync copy")
	}
}

func TestExecute_SyncMode_FailFast_OnSyncError(t *testing.T) {
	mo := &materializerOnly{}
	s := matstage.New(mo, matstage.WithSyncMode(matstage.SyncCopy))

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	_, err := s.Execute(testContextWithFailFast(), plan)
	if err == nil {
		t.Fatal("expected error with fail-fast on sync failure")
	}
}

// ─── Dry-Run Multi-Unit ────────────────────────────────────────────────────

func TestExecute_DryRun_MultiUnit(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem, matstage.WithDryRun(true))

	plan := makeMultiUnitPlan()
	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// Two units × 1 file each = 2 written files.
	if len(mr.WrittenFiles) != 2 {
		t.Errorf("dry-run multi-unit: expected 2 written files, got %d: %v",
			len(mr.WrittenFiles), mr.WrittenFiles)
	}

	// Two unit dirs.
	if len(mr.CreatedDirs) != 2 {
		t.Errorf("dry-run multi-unit: expected 2 created dirs, got %d: %v",
			len(mr.CreatedDirs), mr.CreatedDirs)
	}

	// No actual files.
	files := mem.Files()
	if len(files) != 0 {
		t.Errorf("dry-run should not write files, got %d", len(files))
	}
}

// ─── Adopt-Selected with Empty Patterns ────────────────────────────────────

func TestExecute_SyncAdoptSelected_EmptyPatterns_NoSync(t *testing.T) {
	mem := filesystem.NewMemFS()
	s := matstage.New(mem,
		matstage.WithSyncMode(matstage.SyncAdoptSelected),
		matstage.WithSyncPatterns(nil),
	)

	plan := pipeline.EmissionPlan{
		Units: map[string]pipeline.UnitEmission{
			".ai-build/claude/local-dev": {
				Files: []pipeline.EmittedFile{
					{Path: "CLAUDE.md", Content: []byte("hello")},
				},
			},
		},
	}

	result, err := s.Execute(testContext(), plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mr := result.(pipeline.MaterializationResult)

	// File should be in .ai-build/ but NOT in repo root.
	files := mem.Files()
	if _, ok := files[".ai-build/claude/local-dev/CLAUDE.md"]; !ok {
		t.Error("expected file in .ai-build/")
	}
	if _, ok := files["CLAUDE.md"]; ok {
		t.Error("unexpected file in repo root with empty patterns")
	}

	// No sync files should be in the result beyond the initial build.
	syncWriteCount := 0
	for _, f := range mr.WrittenFiles {
		if f == "CLAUDE.md" {
			syncWriteCount++
		}
	}
	if syncWriteCount != 0 {
		t.Errorf("expected 0 adopted files with empty patterns, got %d", syncWriteCount)
	}
}
