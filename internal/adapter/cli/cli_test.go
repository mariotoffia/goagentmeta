package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ── Helpers ──────────────────────────────────────────────────────────────

// resetCLIFlags saves and resets all global CLI state, restoring on cleanup.
func resetCLIFlags(t *testing.T) {
	t.Helper()

	savedNoColor := noColor
	savedVerbose := verbose
	savedCfgFile := cfgFile
	savedTargetsJSON := targetsJSON
	savedBuildTargets := buildTargets
	savedBuildProfile := buildProfile
	savedBuildOutput := buildOutput
	savedBuildFailFast := buildFailFast
	savedBuildDryRun := buildDryRun
	savedBuildSync := buildSync
	savedEnvNoColor := os.Getenv("NO_COLOR")

	noColor = false
	verbose = false
	cfgFile = ""
	targetsJSON = false
	buildTargets = nil
	buildProfile = "local-dev"
	buildOutput = ".ai-build"
	buildFailFast = true
	buildDryRun = false
	buildSync = "build-only"
	os.Unsetenv("NO_COLOR")

	t.Cleanup(func() {
		noColor = savedNoColor
		verbose = savedVerbose
		cfgFile = savedCfgFile
		targetsJSON = savedTargetsJSON
		buildTargets = savedBuildTargets
		buildProfile = savedBuildProfile
		buildOutput = savedBuildOutput
		buildFailFast = savedBuildFailFast
		buildDryRun = savedBuildDryRun
		buildSync = savedBuildSync
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		if savedEnvNoColor != "" {
			os.Setenv("NO_COLOR", savedEnvNoColor)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	})
}

// captureStdout redirects os.Stdout to a pipe, runs fn, and returns the
// captured output. Safe even if fn calls t.Fatal.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	outC := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	defer func() {
		w.Close()
		os.Stdout = old
	}()

	fn()

	w.Close()
	os.Stdout = old

	return <-outC
}

// chdir changes to dir and restores the original working directory on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// ── 1. resolveTargets ────────────────────────────────────────────────────

func TestResolveTargets(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []build.Target
		wantErr bool
	}{
		{
			name:  "empty input returns nil meaning all",
			input: nil,
			want:  nil,
		},
		{
			name:  "single target claude",
			input: []string{"claude"},
			want:  []build.Target{build.TargetClaude},
		},
		{
			name:  "multiple targets",
			input: []string{"claude", "codex"},
			want:  []build.Target{build.TargetClaude, build.TargetCodex},
		},
		{
			name:  "all returns nil",
			input: []string{"all"},
			want:  nil,
		},
		{
			name:    "unknown target returns error",
			input:   []string{"invalid"},
			wantErr: true,
		},
		{
			name:  "case insensitive title case",
			input: []string{"Claude"},
			want:  []build.Target{build.TargetClaude},
		},
		{
			name:  "case insensitive all caps",
			input: []string{"COPILOT"},
			want:  []build.Target{build.TargetCopilot},
		},
		{
			name:  "all four targets",
			input: []string{"claude", "cursor", "copilot", "codex"},
			want: []build.Target{
				build.TargetClaude, build.TargetCursor,
				build.TargetCopilot, build.TargetCodex,
			},
		},
		{
			name:  "whitespace is trimmed",
			input: []string{" claude "},
			want:  []build.Target{build.TargetClaude},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTargets(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d targets %v, want %d %v",
					len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("target[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ── 2. Output formatting ────────────────────────────────────────────────

func TestFormatError(t *testing.T) {
	saved := noColor
	noColor = true
	defer func() { noColor = saved }()

	got := formatError("something broke")
	want := "[ERROR] something broke"
	if got != want {
		t.Errorf("formatError = %q, want %q", got, want)
	}
}

func TestFormatWarning(t *testing.T) {
	saved := noColor
	noColor = true
	defer func() { noColor = saved }()

	got := formatWarning("heads up")
	want := "[WARN] heads up"
	if got != want {
		t.Errorf("formatWarning = %q, want %q", got, want)
	}
}

func TestFormatSummary_NoErrors(t *testing.T) {
	saved := noColor
	noColor = true
	defer func() { noColor = saved }()

	got := formatSummary(4, 10, 2, 0)
	for _, sub := range []string{"Built", "4 targets", "10 files written", "2 lowerings", "0 errors"} {
		if !strings.Contains(got, sub) {
			t.Errorf("formatSummary missing %q in %q", sub, got)
		}
	}
}

func TestFormatSummary_WithErrors(t *testing.T) {
	saved := noColor
	noColor = true
	defer func() { noColor = saved }()

	got := formatSummary(2, 5, 1, 3)
	for _, sub := range []string{"Built", "2 targets", "5 files written", "1 lowerings", "3 errors"} {
		if !strings.Contains(got, sub) {
			t.Errorf("formatSummary missing %q in %q", sub, got)
		}
	}
}

func TestColorize_NoColor(t *testing.T) {
	saved := noColor
	noColor = true
	defer func() { noColor = saved }()

	got := colorize(colorRed, "hello")
	if got != "hello" {
		t.Errorf("colorize with noColor: got %q, want %q", got, "hello")
	}
}

func TestColorize_WithColor(t *testing.T) {
	saved := noColor
	noColor = false
	defer func() { noColor = saved }()

	got := colorize(colorRed, "hello")

	expected := colorRed + "hello" + colorReset
	if got != expected {
		t.Errorf("colorize = %q, want %q", got, expected)
	}
	if !strings.Contains(got, "\033[") {
		t.Errorf("expected ANSI escape in %q", got)
	}
	if !strings.HasSuffix(got, colorReset) {
		t.Errorf("expected colorReset suffix in %q", got)
	}
}

// ── 3. normalizeHook ────────────────────────────────────────────────────

func TestNormalizeHook_Descriptor(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	if hook.Name != "normalize-objects-bridge" {
		t.Errorf("Name = %q, want %q", hook.Name, "normalize-objects-bridge")
	}
	if hook.Point != pipeline.HookAfterPhase {
		t.Errorf("Point = %q, want %q", hook.Point, pipeline.HookAfterPhase)
	}
	if hook.Phase != pipeline.PhaseNormalize {
		t.Errorf("Phase = %d, want %d", hook.Phase, pipeline.PhaseNormalize)
	}
	if hook.Handler == nil {
		t.Fatal("Handler is nil")
	}
}

func TestNormalizeHook_ValueSemanticGraph(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	sg := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"obj-1": {SourcePath: "a.yaml", Content: "hello"},
			"obj-2": {SourcePath: "b.yaml", Content: "world"},
		},
	}

	result, err := hook.Handler(context.Background(), sg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(pipeline.SemanticGraph); !ok {
		t.Fatalf("result type = %T, want pipeline.SemanticGraph", result)
	}
	if len(objects) != 2 {
		t.Fatalf("objects len = %d, want 2", len(objects))
	}
	if objects["obj-1"].SourcePath != "a.yaml" {
		t.Errorf("obj-1 SourcePath = %q, want %q", objects["obj-1"].SourcePath, "a.yaml")
	}
	if objects["obj-2"].Content != "world" {
		t.Errorf("obj-2 Content = %q, want %q", objects["obj-2"].Content, "world")
	}
}

func TestNormalizeHook_PointerSemanticGraph(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	sg := &pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"ptr-obj": {SourcePath: "c.yaml", Content: "from pointer"},
		},
	}

	result, err := hook.Handler(context.Background(), sg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(*pipeline.SemanticGraph); !ok {
		t.Fatalf("result type = %T, want *pipeline.SemanticGraph", result)
	}
	if len(objects) != 1 {
		t.Fatalf("objects len = %d, want 1", len(objects))
	}
	if objects["ptr-obj"].Content != "from pointer" {
		t.Errorf("ptr-obj Content = %q, want %q", objects["ptr-obj"].Content, "from pointer")
	}
}

func TestNormalizeHook_NonSemanticGraph(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	input := "not a semantic graph"
	result, err := hook.Handler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("result = %v, want %v", result, input)
	}
	if len(objects) != 0 {
		t.Errorf("objects should be empty, got %d entries", len(objects))
	}
}

// ── 4. wirePipeline ─────────────────────────────────────────────────────

func TestWirePipeline_Default(t *testing.T) {
	p, err := wirePipeline(buildConfig{
		profile:    build.ProfileLocalDev,
		outputDir:  t.TempDir(),
		failFast:   true,
		syncMode:   "build-only",
		reportPath: filepath.Join(t.TempDir(), "report"),
	})
	if err != nil {
		t.Fatalf("wirePipeline error: %v", err)
	}
	if p == nil {
		t.Fatal("pipeline is nil")
	}
}

func TestWirePipeline_SpecificTargets(t *testing.T) {
	p, err := wirePipeline(buildConfig{
		targets:   []build.Target{build.TargetClaude, build.TargetCodex},
		profile:   build.ProfileCI,
		outputDir: t.TempDir(),
		failFast:  false,
		syncMode:  "copy",
	})
	if err != nil {
		t.Fatalf("wirePipeline error: %v", err)
	}
	if p == nil {
		t.Fatal("pipeline is nil")
	}
}

func TestWirePipeline_DryRun(t *testing.T) {
	p, err := wirePipeline(buildConfig{
		targets:   []build.Target{build.TargetCopilot},
		profile:   build.ProfileLocalDev,
		outputDir: t.TempDir(),
		dryRun:    true,
		syncMode:  "build-only",
	})
	if err != nil {
		t.Fatalf("wirePipeline error: %v", err)
	}
	if p == nil {
		t.Fatal("pipeline is nil")
	}
}

func TestWirePipeline_SyncModes(t *testing.T) {
	for _, mode := range []string{"copy", "symlink", "build-only"} {
		t.Run(mode, func(t *testing.T) {
			p, err := wirePipeline(buildConfig{
				profile:   build.ProfileLocalDev,
				outputDir: t.TempDir(),
				syncMode:  mode,
			})
			if err != nil {
				t.Fatalf("wirePipeline(%s) error: %v", mode, err)
			}
			if p == nil {
				t.Fatalf("wirePipeline(%s) returned nil", mode)
			}
		})
	}
}

// ── 5. Cobra commands ───────────────────────────────────────────────────

func TestCmd_Version(t *testing.T) {
	resetCLIFlags(t)
	noColor = true

	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"version"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute error: %v", err)
		}
	})

	if !strings.Contains(stdout, "goagentmeta") {
		t.Errorf("version output missing 'goagentmeta': %q", stdout)
	}
	if !strings.Contains(stdout, "commit:") {
		t.Errorf("version output missing 'commit:': %q", stdout)
	}
	if !strings.Contains(stdout, "built:") {
		t.Errorf("version output missing 'built:': %q", stdout)
	}
}

func TestCmd_TargetsJSON(t *testing.T) {
	resetCLIFlags(t)
	noColor = true

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"targets", "--json"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	var targets []targetInfo
	if err := json.Unmarshal(buf.Bytes(), &targets); err != nil {
		t.Fatalf("JSON unmarshal error: %v\nraw: %s", err, buf.String())
	}
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(targets))
	}

	names := make(map[string]bool)
	for _, ti := range targets {
		names[ti.Name] = true
	}
	for _, want := range []string{"claude", "cursor", "copilot", "codex"} {
		if !names[want] {
			t.Errorf("missing target %q in JSON output", want)
		}
	}
}

func TestCmd_TargetsText(t *testing.T) {
	resetCLIFlags(t)
	noColor = true

	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"targets"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute error: %v", err)
		}
	})

	for _, want := range []string{"claude", "copilot", "codex"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("targets output missing %q:\n%s", want, stdout)
		}
	}
}

func TestCmd_Init(t *testing.T) {
	resetCLIFlags(t)
	noColor = true
	dir := t.TempDir()
	chdir(t, dir)

	captureStdout(t, func() {
		rootCmd.SetArgs([]string{"init"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute error: %v", err)
		}
	})

	for _, path := range []string{
		".ai/manifest.yaml",
		".ai/instructions/code-style.yaml",
		".ai/rules/no-secrets.yaml",
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
		}
	}

	for _, d := range []string{".ai/skills", ".ai/agents"} {
		info, err := os.Stat(filepath.Join(dir, d))
		if err != nil {
			t.Errorf("expected directory %s: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("%s should be a directory", d)
		}
	}
}

func TestCmd_InitAlreadyExists(t *testing.T) {
	resetCLIFlags(t)
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.Mkdir(filepath.Join(dir, ".ai"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	rootCmd.SetArgs([]string{"init"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when .ai/ exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists': %v", err)
	}
}

func TestCmd_BuildInvalidTarget(t *testing.T) {
	resetCLIFlags(t)

	rootCmd.SetArgs([]string{"build", "--target", "invalid"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid target")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("error should mention 'unknown target': %v", err)
	}
}

func TestCmd_UnknownSubcommand(t *testing.T) {
	resetCLIFlags(t)

	rootCmd.SetArgs([]string{"nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// ── 6. exitError ────────────────────────────────────────────────────────

func TestExitError(t *testing.T) {
	inner := errors.New("build failed")
	e := &exitError{code: 1, err: inner}

	if got := e.Error(); got != "build failed" {
		t.Errorf("Error() = %q, want %q", got, "build failed")
	}
	if e.code != 1 {
		t.Errorf("code = %d, want 1", e.code)
	}
}
