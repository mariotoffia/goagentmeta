package compiler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

func TestPipelineExecuteEmpty(t *testing.T) {
	p := compiler.NewPipeline()
	report, err := p.Execute(context.Background(), []string{"/test/.ai/"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if report == nil {
		t.Fatal("Execute() returned nil report")
	}
	if report.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestPipelineExecuteStageOrder(t *testing.T) {
	var order []string

	makeStage := func(name string, phase pipeline.Phase) *testStage {
		s := newTestStage(name, phase, 0)
		s.execFn = func(_ context.Context, input any) (any, error) {
			order = append(order, name)
			return input, nil
		}
		return s
	}

	p := compiler.NewPipeline(
		compiler.WithStage(makeStage("reporter", pipeline.PhaseReport)),
		compiler.WithStage(makeStage("parser", pipeline.PhaseParse)),
		compiler.WithStage(makeStage("validator", pipeline.PhaseValidate)),
	)

	_, err := p.Execute(context.Background(), []string{"/test/"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expected := []string{"parser", "validator", "reporter"}
	if len(order) != len(expected) {
		t.Fatalf("executed %d stages, want %d", len(order), len(expected))
	}
	for i, name := range expected {
		if order[i] != name {
			t.Errorf("order[%d] = %q, want %q", i, order[i], name)
		}
	}
}

func TestPipelineFailFast(t *testing.T) {
	stageFail := newTestStage("fail-stage", pipeline.PhaseParse, 0)
	stageFail.execFn = func(_ context.Context, _ any) (any, error) {
		return nil, errors.New("stage failed")
	}

	stageAfter := newTestStage("should-not-run", pipeline.PhaseValidate, 0)
	ran := false
	stageAfter.execFn = func(_ context.Context, input any) (any, error) {
		ran = true
		return input, nil
	}

	p := compiler.NewPipeline(
		compiler.WithStage(stageFail),
		compiler.WithStage(stageAfter),
		compiler.WithFailFast(true),
	)

	report, err := p.Execute(context.Background(), []string{"/test/"})
	if err == nil {
		t.Fatal("expected error from failing stage")
	}
	if ran {
		t.Error("stage after failure should not have run in fail-fast mode")
	}
	if report == nil {
		t.Fatal("report should be non-nil even on failure")
	}
	if len(report.Diagnostics) == 0 {
		t.Error("expected diagnostics from the failed stage")
	}
}

func TestPipelineAccumulateMode(t *testing.T) {
	stageFail := newTestStage("fail-stage", pipeline.PhaseParse, 10)
	stageFail.execFn = func(_ context.Context, _ any) (any, error) {
		return nil, errors.New("stage failed")
	}

	stageOK := newTestStage("ok-stage", pipeline.PhaseParse, 20)
	ran := false
	stageOK.execFn = func(_ context.Context, input any) (any, error) {
		ran = true
		return input, nil
	}

	p := compiler.NewPipeline(
		compiler.WithStage(stageFail),
		compiler.WithStage(stageOK),
		compiler.WithFailFast(false),
	)

	_, err := p.Execute(context.Background(), []string{"/test/"})
	if err != nil {
		t.Fatalf("Execute() in accumulate mode should not return error, got %v", err)
	}
	if !ran {
		t.Error("ok-stage should have run in accumulate mode")
	}
}

func TestPipelineHooks(t *testing.T) {
	var hookOrder []string

	beforeHook := &testHookHandler{
		hook: pipeline.StageHook{
			Name:  "before-parse",
			Point: pipeline.HookBeforePhase,
			Phase: pipeline.PhaseParse,
			Handler: func(_ context.Context, ir any) (any, error) {
				hookOrder = append(hookOrder, "before")
				return ir, nil
			},
		},
	}

	afterHook := &testHookHandler{
		hook: pipeline.StageHook{
			Name:  "after-parse",
			Point: pipeline.HookAfterPhase,
			Phase: pipeline.PhaseParse,
			Handler: func(_ context.Context, ir any) (any, error) {
				hookOrder = append(hookOrder, "after")
				return ir, nil
			},
		},
	}

	stageMiddle := newTestStage("parser", pipeline.PhaseParse, 0)
	stageMiddle.execFn = func(_ context.Context, input any) (any, error) {
		hookOrder = append(hookOrder, "stage")
		return input, nil
	}

	p := compiler.NewPipeline(
		compiler.WithStage(stageMiddle),
		compiler.WithHook(beforeHook),
		compiler.WithHook(afterHook),
	)

	_, err := p.Execute(context.Background(), []string{"/test/"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expected := []string{"before", "stage", "after"}
	if len(hookOrder) != len(expected) {
		t.Fatalf("hook order = %v, want %v", hookOrder, expected)
	}
	for i, name := range expected {
		if hookOrder[i] != name {
			t.Errorf("hookOrder[%d] = %q, want %q", i, hookOrder[i], name)
		}
	}
}

func TestPipelineContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	s := newTestStage("parser", pipeline.PhaseParse, 0)
	p := compiler.NewPipeline(compiler.WithStage(s))

	_, err := p.Execute(ctx, []string{"/test/"})
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestPipelineIRPassthrough(t *testing.T) {
	s1 := newTestStage("s1", pipeline.PhaseParse, 10)
	s1.execFn = func(_ context.Context, _ any) (any, error) {
		return "parsed-ir", nil
	}

	s2 := newTestStage("s2", pipeline.PhaseValidate, 0)
	var received any
	s2.execFn = func(_ context.Context, input any) (any, error) {
		received = input
		return input, nil
	}

	p := compiler.NewPipeline(
		compiler.WithStage(s1),
		compiler.WithStage(s2),
	)

	_, err := p.Execute(context.Background(), []string{"/test/"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if received != "parsed-ir" {
		t.Errorf("s2 received %v, want %q", received, "parsed-ir")
	}
}

// testHookHandler implements stage.StageHookHandler for testing.
type testHookHandler struct {
	hook pipeline.StageHook
}

func (h *testHookHandler) Hook() pipeline.StageHook { return h.hook }
