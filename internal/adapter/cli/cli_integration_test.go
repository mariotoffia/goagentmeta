package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ── Test stage helper ────────────────────────────────────────────────────

type testStage struct {
	name    string
	phase   pipeline.Phase
	order   int
	execFn  func(ctx context.Context, input any) (any, error)
	targets []build.Target
}

func (s *testStage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:         s.name,
		Phase:        s.phase,
		Order:        s.order,
		TargetFilter: s.targets,
	}
}

func (s *testStage) Execute(ctx context.Context, input any) (any, error) {
	if s.execFn != nil {
		return s.execFn(ctx, input)
	}
	return input, nil
}

// testHookHandler wraps a StageHook into the StageHookHandler interface.
type testHookHandler struct {
	hook pipeline.StageHook
}

func (h *testHookHandler) Hook() pipeline.StageHook { return h.hook }

// ── Group 1: Pipeline Wiring Integration ─────────────────────────────────

func TestWirePipelineDefaultConfig(t *testing.T) {
	p, err := wirePipeline(buildConfig{
		profile:   build.ProfileLocalDev,
		outputDir: t.TempDir(),
		failFast:  true,
		syncMode:  "build-only",
	})
	if err != nil {
		t.Fatalf("wirePipeline error: %v", err)
	}
	if p == nil {
		t.Fatal("pipeline is nil")
	}
}

func TestWirePipelineAllSyncModes(t *testing.T) {
	for _, mode := range []string{"build-only", "copy", "symlink"} {
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

func TestWirePipelineDryRunMode(t *testing.T) {
	p, err := wirePipeline(buildConfig{
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

func TestWirePipelineSpecificTargets(t *testing.T) {
	p, err := wirePipeline(buildConfig{
		targets:   []build.Target{build.TargetClaude, build.TargetCopilot},
		profile:   build.ProfileCI,
		outputDir: t.TempDir(),
		syncMode:  "copy",
	})
	if err != nil {
		t.Fatalf("wirePipeline error: %v", err)
	}
	if p == nil {
		t.Fatal("pipeline is nil")
	}
}

func TestWirePipelineSingleTarget(t *testing.T) {
	p, err := wirePipeline(buildConfig{
		targets:   []build.Target{build.TargetCodex},
		profile:   build.ProfileLocalDev,
		outputDir: t.TempDir(),
		syncMode:  "build-only",
	})
	if err != nil {
		t.Fatalf("wirePipeline error: %v", err)
	}
	if p == nil {
		t.Fatal("pipeline is nil")
	}
}

// ── Group 2: Normalize Hook Simulation ───────────────────────────────────

func TestNormalizeHookPopulatesObjects(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	sg := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"obj-a": {Meta: model.ObjectMeta{ID: "obj-a", Kind: model.KindInstruction}, SourcePath: "a.yaml", Content: "alpha"},
			"obj-b": {Meta: model.ObjectMeta{ID: "obj-b", Kind: model.KindRule}, SourcePath: "b.yaml", Content: "beta"},
			"obj-c": {Meta: model.ObjectMeta{ID: "obj-c", Kind: model.KindSkill}, SourcePath: "c.yaml", Content: "gamma"},
		},
	}

	result, err := hook.Handler(context.Background(), sg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(pipeline.SemanticGraph); !ok {
		t.Fatalf("result type = %T, want pipeline.SemanticGraph", result)
	}
	if len(objects) != 3 {
		t.Fatalf("objects len = %d, want 3", len(objects))
	}
	for _, id := range []string{"obj-a", "obj-b", "obj-c"} {
		if _, ok := objects[id]; !ok {
			t.Errorf("missing object %q in shared map", id)
		}
	}
	if objects["obj-a"].Content != "alpha" {
		t.Errorf("obj-a Content = %q, want %q", objects["obj-a"].Content, "alpha")
	}
}

func TestNormalizeHookPointerInput(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	sg := &pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"ptr-1": {SourcePath: "p1.yaml", Content: "first"},
			"ptr-2": {SourcePath: "p2.yaml", Content: "second"},
			"ptr-3": {SourcePath: "p3.yaml", Content: "third"},
		},
	}

	result, err := hook.Handler(context.Background(), sg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(*pipeline.SemanticGraph); !ok {
		t.Fatalf("result type = %T, want *pipeline.SemanticGraph", result)
	}
	if len(objects) != 3 {
		t.Fatalf("objects len = %d, want 3", len(objects))
	}
}

func TestNormalizeHookNonSemanticInput(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	input := "just a string, not a SemanticGraph"
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

func TestNormalizeHookIdempotent(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	h := &normalizeHook{objects: objects}
	hook := h.Hook()

	sg1 := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"obj-1": {Content: "first-version"},
			"obj-2": {Content: "only-in-first"},
		},
	}
	if _, err := hook.Handler(context.Background(), sg1); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("after first call: len = %d, want 2", len(objects))
	}

	sg2 := pipeline.SemanticGraph{
		Objects: map[string]pipeline.NormalizedObject{
			"obj-1": {Content: "second-version"},
			"obj-3": {Content: "only-in-second"},
		},
	}
	if _, err := hook.Handler(context.Background(), sg2); err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(objects) != 3 {
		t.Fatalf("after second call: len = %d, want 3", len(objects))
	}
	if objects["obj-1"].Content != "second-version" {
		t.Errorf("obj-1 Content = %q, want %q", objects["obj-1"].Content, "second-version")
	}
	if objects["obj-2"].Content != "only-in-first" {
		t.Errorf("obj-2 Content = %q, want %q", objects["obj-2"].Content, "only-in-first")
	}
	if objects["obj-3"].Content != "only-in-second" {
		t.Errorf("obj-3 Content = %q, want %q", objects["obj-3"].Content, "only-in-second")
	}
}

func TestNormalizeHookDescriptor(t *testing.T) {
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
		t.Errorf("Phase = %d, want %d (PhaseNormalize)", hook.Phase, pipeline.PhaseNormalize)
	}
	if hook.Handler == nil {
		t.Fatal("Handler is nil")
	}
}

// ── Group 3: End-to-End Pipeline Simulations ─────────────────────────────

func TestSimulateFullPipelineFlow(t *testing.T) {
	var executionOrder []string

	record := func(name string) {
		executionOrder = append(executionOrder, name)
	}

	objects := make(map[string]pipeline.NormalizedObject)

	p := compiler.NewPipeline(
		// Parse: receives []string root paths, returns SourceTree
		compiler.WithStage(&testStage{
			name: "test-parse", phase: pipeline.PhaseParse, order: 10,
			execFn: func(_ context.Context, input any) (any, error) {
				record("parse")
				paths, ok := input.([]string)
				if !ok {
					return nil, fmt.Errorf("parse: expected []string, got %T", input)
				}
				return pipeline.SourceTree{
					RootPath: paths[0],
					Objects: []pipeline.RawObject{
						{Meta: model.ObjectMeta{ID: "raw-1", Kind: model.KindInstruction}, SourcePath: "a.yaml", RawContent: "content-1"},
						{Meta: model.ObjectMeta{ID: "raw-2", Kind: model.KindRule}, SourcePath: "b.yaml", RawContent: "content-2"},
					},
				}, nil
			},
		}),
		// Validate
		compiler.WithStage(&testStage{
			name: "test-validate", phase: pipeline.PhaseValidate, order: 10,
			execFn: func(_ context.Context, input any) (any, error) {
				record("validate")
				return input, nil
			},
		}),
		// Resolve
		compiler.WithStage(&testStage{
			name: "test-resolve", phase: pipeline.PhaseResolve, order: 10,
			execFn: func(_ context.Context, input any) (any, error) {
				record("resolve")
				return input, nil
			},
		}),
		// Normalize: produces SemanticGraph
		compiler.WithStage(&testStage{
			name: "test-normalize", phase: pipeline.PhaseNormalize, order: 10,
			execFn: func(_ context.Context, input any) (any, error) {
				record("normalize")
				return pipeline.SemanticGraph{
					Objects: map[string]pipeline.NormalizedObject{
						"norm-1": {Meta: model.ObjectMeta{ID: "norm-1", Kind: model.KindInstruction}, SourcePath: "a.yaml", Content: "normalized-1"},
						"norm-2": {Meta: model.ObjectMeta{ID: "norm-2", Kind: model.KindRule}, SourcePath: "b.yaml", Content: "normalized-2"},
					},
				}, nil
			},
		}),
		// Hook after normalize
		compiler.WithHook(&normalizeHook{objects: objects}),
		// Plan
		compiler.WithStage(&testStage{
			name: "test-plan", phase: pipeline.PhasePlan, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				record("plan")
				return pipeline.BuildPlan{
					Units: []pipeline.BuildPlanUnit{
						{Coordinate: build.BuildCoordinate{Unit: build.BuildUnit{Target: build.TargetClaude}}, ActiveObjects: []string{"norm-1", "norm-2"}},
					},
				}, nil
			},
		}),
		// Capability
		compiler.WithStage(&testStage{
			name: "test-capability", phase: pipeline.PhaseCapability, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				record("capability")
				return pipeline.CapabilityGraph{
					Units: map[string]pipeline.UnitCapabilities{
						"claude": {},
					},
				}, nil
			},
		}),
		// Lower
		compiler.WithStage(&testStage{
			name: "test-lower", phase: pipeline.PhaseLower, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				record("lower")
				return pipeline.LoweredGraph{
					Units: map[string]pipeline.LoweredUnit{
						"claude": {},
					},
				}, nil
			},
		}),
		// Render
		compiler.WithStage(&testStage{
			name: "test-render", phase: pipeline.PhaseRender, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				record("render")
				return pipeline.EmissionPlan{
					Units: map[string]pipeline.UnitEmission{
						"claude": {Files: []pipeline.EmittedFile{{Path: "out.md", Content: []byte("hello")}}},
					},
				}, nil
			},
		}),
		// Materialize
		compiler.WithStage(&testStage{
			name: "test-materialize", phase: pipeline.PhaseMaterialize, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				record("materialize")
				return pipeline.MaterializationResult{
					WrittenFiles: []string{"out.md"},
				}, nil
			},
		}),
		// Report
		compiler.WithStage(&testStage{
			name: "test-report", phase: pipeline.PhaseReport, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				record("report")
				return &pipeline.BuildReport{
					Timestamp: time.Now(),
					Units: []pipeline.UnitReport{
						{EmittedFiles: []string{"out.md"}},
					},
				}, nil
			},
		}),
	)

	report, err := p.Execute(context.Background(), []string{"."})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}

	// Verify execution order
	wantOrder := []string{"parse", "validate", "resolve", "normalize", "plan", "capability", "lower", "render", "materialize", "report"}
	if len(executionOrder) != len(wantOrder) {
		t.Fatalf("executed %d stages %v, want %d %v", len(executionOrder), executionOrder, len(wantOrder), wantOrder)
	}
	for i, got := range executionOrder {
		if got != wantOrder[i] {
			t.Errorf("stage[%d] = %q, want %q", i, got, wantOrder[i])
		}
	}

	// Verify the normalize hook populated the shared objects map
	if len(objects) != 2 {
		t.Errorf("normalized objects len = %d, want 2", len(objects))
	}
}

func TestSimulateMultiTargetPipeline(t *testing.T) {
	var renderCount int32

	makeRenderStage := func(target build.Target) *testStage {
		return &testStage{
			name:    fmt.Sprintf("render-%s", target),
			phase:   pipeline.PhaseRender,
			order:   10,
			targets: []build.Target{target},
			execFn: func(_ context.Context, input any) (any, error) {
				atomic.AddInt32(&renderCount, 1)
				// Each renderer passes through or extends the emission plan.
				if ep, ok := input.(pipeline.EmissionPlan); ok {
					return ep, nil
				}
				return pipeline.EmissionPlan{
					Units: map[string]pipeline.UnitEmission{},
				}, nil
			},
		}
	}

	p := compiler.NewPipeline(
		compiler.WithStage(&testStage{name: "sim-parse", phase: pipeline.PhaseParse, order: 10}),
		compiler.WithStage(&testStage{name: "sim-validate", phase: pipeline.PhaseValidate, order: 10}),
		compiler.WithStage(&testStage{name: "sim-resolve", phase: pipeline.PhaseResolve, order: 10}),
		compiler.WithStage(&testStage{
			name: "sim-normalize", phase: pipeline.PhaseNormalize, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				return pipeline.SemanticGraph{Objects: map[string]pipeline.NormalizedObject{}}, nil
			},
		}),
		compiler.WithStage(&testStage{name: "sim-plan", phase: pipeline.PhasePlan, order: 10}),
		compiler.WithStage(&testStage{name: "sim-capability", phase: pipeline.PhaseCapability, order: 10}),
		compiler.WithStage(&testStage{
			name: "sim-lower", phase: pipeline.PhaseLower, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				return pipeline.LoweredGraph{Units: map[string]pipeline.LoweredUnit{}}, nil
			},
		}),
		// Three renderers — one per target
		compiler.WithStage(makeRenderStage(build.TargetClaude)),
		compiler.WithStage(makeRenderStage(build.TargetCopilot)),
		compiler.WithStage(makeRenderStage(build.TargetCodex)),
		compiler.WithStage(&testStage{name: "sim-materialize", phase: pipeline.PhaseMaterialize, order: 10}),
		compiler.WithStage(&testStage{name: "sim-report", phase: pipeline.PhaseReport, order: 10}),
	)

	report, err := p.Execute(context.Background(), []string{"."})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}

	count := atomic.LoadInt32(&renderCount)
	if count != 3 {
		t.Errorf("render count = %d, want 3", count)
	}
}

func TestSimulateNormalizeHookBridgesData(t *testing.T) {
	objects := make(map[string]pipeline.NormalizedObject)
	var capabilitySawObjects bool

	p := compiler.NewPipeline(
		compiler.WithStage(&testStage{name: "bridge-parse", phase: pipeline.PhaseParse, order: 10}),
		compiler.WithStage(&testStage{name: "bridge-validate", phase: pipeline.PhaseValidate, order: 10}),
		compiler.WithStage(&testStage{name: "bridge-resolve", phase: pipeline.PhaseResolve, order: 10}),
		// Normalize: produce SemanticGraph with objects
		compiler.WithStage(&testStage{
			name: "bridge-normalize", phase: pipeline.PhaseNormalize, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				return pipeline.SemanticGraph{
					Objects: map[string]pipeline.NormalizedObject{
						"bridged-obj": {
							Meta:       model.ObjectMeta{ID: "bridged-obj", Kind: model.KindInstruction},
							SourcePath: "bridge.yaml",
							Content:    "bridged content",
						},
					},
				}, nil
			},
		}),
		// Hook: normalizeHook populates shared map
		compiler.WithHook(&normalizeHook{objects: objects}),
		compiler.WithStage(&testStage{name: "bridge-plan", phase: pipeline.PhasePlan, order: 10}),
		// Capability: read from shared map
		compiler.WithStage(&testStage{
			name: "bridge-capability", phase: pipeline.PhaseCapability, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				if obj, ok := objects["bridged-obj"]; ok && obj.Content == "bridged content" {
					capabilitySawObjects = true
				}
				return pipeline.CapabilityGraph{}, nil
			},
		}),
		compiler.WithStage(&testStage{name: "bridge-lower", phase: pipeline.PhaseLower, order: 10}),
		compiler.WithStage(&testStage{name: "bridge-render", phase: pipeline.PhaseRender, order: 10}),
		compiler.WithStage(&testStage{name: "bridge-materialize", phase: pipeline.PhaseMaterialize, order: 10}),
		compiler.WithStage(&testStage{name: "bridge-report", phase: pipeline.PhaseReport, order: 10}),
	)

	_, err := p.Execute(context.Background(), []string{"."})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !capabilitySawObjects {
		t.Error("capability stage did not see bridged objects from normalize hook")
	}
	if len(objects) != 1 {
		t.Errorf("objects len = %d, want 1", len(objects))
	}
}

func TestSimulatePipelineFailFast(t *testing.T) {
	stageErr := errors.New("intentional stage failure")

	p := compiler.NewPipeline(
		compiler.WithFailFast(true),
		compiler.WithStage(&testStage{name: "ff-parse", phase: pipeline.PhaseParse, order: 10}),
		compiler.WithStage(&testStage{
			name: "ff-validate", phase: pipeline.PhaseValidate, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				return nil, stageErr
			},
		}),
		// These stages should NOT execute due to fail-fast
		compiler.WithStage(&testStage{name: "ff-resolve", phase: pipeline.PhaseResolve, order: 10}),
	)

	_, err := p.Execute(context.Background(), []string{"."})
	if err == nil {
		t.Fatal("expected error from failing stage")
	}
	if !strings.Contains(err.Error(), "intentional stage failure") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "intentional stage failure")
	}
}

func TestSimulatePipelineCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	p := compiler.NewPipeline(
		compiler.WithStage(&testStage{
			name: "cancel-parse", phase: pipeline.PhaseParse, order: 10,
			execFn: func(_ context.Context, input any) (any, error) {
				// Cancel the context during the parse phase.
				cancel()
				return input, nil
			},
		}),
		// Validate has a stage that should see the cancelled context.
		compiler.WithStage(&testStage{
			name: "cancel-validate", phase: pipeline.PhaseValidate, order: 10,
			execFn: func(ctx context.Context, input any) (any, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					return input, nil
				}
			},
		}),
		compiler.WithStage(&testStage{name: "cancel-resolve", phase: pipeline.PhaseResolve, order: 10}),
	)

	_, err := p.Execute(ctx, []string{"."})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestSimulateDryRunScenario(t *testing.T) {
	var materializeInput any

	p := compiler.NewPipeline(
		compiler.WithStage(&testStage{name: "dry-parse", phase: pipeline.PhaseParse, order: 10}),
		compiler.WithStage(&testStage{name: "dry-validate", phase: pipeline.PhaseValidate, order: 10}),
		compiler.WithStage(&testStage{name: "dry-resolve", phase: pipeline.PhaseResolve, order: 10}),
		compiler.WithStage(&testStage{name: "dry-normalize", phase: pipeline.PhaseNormalize, order: 10}),
		compiler.WithStage(&testStage{name: "dry-plan", phase: pipeline.PhasePlan, order: 10}),
		compiler.WithStage(&testStage{name: "dry-capability", phase: pipeline.PhaseCapability, order: 10}),
		compiler.WithStage(&testStage{name: "dry-lower", phase: pipeline.PhaseLower, order: 10}),
		compiler.WithStage(&testStage{
			name: "dry-render", phase: pipeline.PhaseRender, order: 10,
			execFn: func(_ context.Context, _ any) (any, error) {
				return pipeline.EmissionPlan{
					Units: map[string]pipeline.UnitEmission{
						"dry-unit": {
							Files: []pipeline.EmittedFile{
								{Path: "dry-output.md", Content: []byte("dry run content")},
							},
						},
					},
				}, nil
			},
		}),
		// Materializer in dry-run: records what it sees but writes nothing
		compiler.WithStage(&testStage{
			name: "dry-materialize", phase: pipeline.PhaseMaterialize, order: 10,
			execFn: func(_ context.Context, input any) (any, error) {
				materializeInput = input
				return pipeline.MaterializationResult{
					WrittenFiles: nil, // dry run: nothing written
				}, nil
			},
		}),
		compiler.WithStage(&testStage{name: "dry-report", phase: pipeline.PhaseReport, order: 10}),
	)

	report, err := p.Execute(context.Background(), []string{"."})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if materializeInput == nil {
		t.Fatal("materialize stage was not called")
	}
	ep, ok := materializeInput.(pipeline.EmissionPlan)
	if !ok {
		t.Fatalf("materialize input type = %T, want pipeline.EmissionPlan", materializeInput)
	}
	if len(ep.Units) != 1 {
		t.Errorf("emission plan units = %d, want 1", len(ep.Units))
	}
}

// ── Group 4: CLI Command Simulation ──────────────────────────────────────

func TestInitCreatesDirectoryStructure(t *testing.T) {
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

	// Verify directory structure
	expectedDirs := []string{
		".ai",
		".ai/instructions",
		".ai/rules",
		".ai/skills",
		".ai/agents",
	}
	for _, d := range expectedDirs {
		info, err := os.Stat(filepath.Join(dir, d))
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s should be a directory", d)
		}
	}

	// Verify files
	expectedFiles := []string{
		".ai/manifest.yaml",
		".ai/instructions/code-style.yaml",
		".ai/rules/no-secrets.yaml",
	}
	for _, f := range expectedFiles {
		info, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			t.Errorf("expected file %s to exist: %v", f, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("%s should be a file, not a directory", f)
		}
		if info.Size() == 0 {
			t.Errorf("%s should not be empty", f)
		}
	}
}

func TestInitFailsWhenDirExists(t *testing.T) {
	resetCLIFlags(t)
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.Mkdir(filepath.Join(dir, ".ai"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	rootCmd.SetArgs([]string{"init"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when .ai/ already exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists': %v", err)
	}
}

func TestTargetsOutputContainsAllTargets(t *testing.T) {
	resetCLIFlags(t)
	noColor = true

	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"targets"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute error: %v", err)
		}
	})

	for _, target := range []string{"claude", "cursor", "copilot", "codex"} {
		if !strings.Contains(stdout, target) {
			t.Errorf("targets output missing %q:\n%s", target, stdout)
		}
	}
}

func TestTargetsJSONOutput(t *testing.T) {
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

	// Verify each entry has a status
	for _, ti := range targets {
		if ti.Status == "" {
			t.Errorf("target %q has empty status", ti.Name)
		}
	}
}

func TestVersionOutput(t *testing.T) {
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

func TestBuildUnknownTarget(t *testing.T) {
	resetCLIFlags(t)

	rootCmd.SetArgs([]string{"build", "--target", "invalid"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("error should mention 'unknown target': %v", err)
	}
}

func TestBuildMultipleTargets(t *testing.T) {
	got, err := resolveTargets([]string{"claude", "copilot"})
	if err != nil {
		t.Fatalf("resolveTargets error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d targets, want 2", len(got))
	}
	if got[0] != build.TargetClaude {
		t.Errorf("target[0] = %q, want %q", got[0], build.TargetClaude)
	}
	if got[1] != build.TargetCopilot {
		t.Errorf("target[1] = %q, want %q", got[1], build.TargetCopilot)
	}
}
