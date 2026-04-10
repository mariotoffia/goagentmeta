package compiler_test

import (
	"context"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// testStage is a minimal Stage implementation for testing.
type testStage struct {
	desc   pipeline.StageDescriptor
	execFn func(ctx context.Context, input any) (any, error)
}

func newTestStage(name string, phase pipeline.Phase, order int) *testStage {
	return &testStage{
		desc: pipeline.StageDescriptor{Name: name, Phase: phase, Order: order},
		execFn: func(_ context.Context, input any) (any, error) {
			return input, nil
		},
	}
}

func (s *testStage) Descriptor() pipeline.StageDescriptor { return s.desc }
func (s *testStage) Execute(ctx context.Context, input any) (any, error) {
	return s.execFn(ctx, input)
}

// Compile-time interface verification.
var _ stage.Stage = (*testStage)(nil)

func TestRegistryRegister(t *testing.T) {
	reg := compiler.NewStageRegistry()

	s := newTestStage("parser", pipeline.PhaseParse, 0)
	if err := reg.Register(s); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Duplicate registration should fail.
	if err := reg.Register(s); err == nil {
		t.Error("Register() duplicate should fail, got nil")
	}
}

func TestRegistryRejectInvalidDescriptor(t *testing.T) {
	reg := compiler.NewStageRegistry()

	s := &testStage{
		desc:   pipeline.StageDescriptor{Name: "", Phase: pipeline.PhaseParse},
		execFn: func(_ context.Context, input any) (any, error) { return input, nil },
	}
	if err := reg.Register(s); err == nil {
		t.Error("Register() with empty name should fail, got nil")
	}
}

func TestRegistryStagesForPhaseOrdering(t *testing.T) {
	reg := compiler.NewStageRegistry()

	// Register in reverse order.
	stages := []struct {
		name  string
		order int
	}{
		{"c-stage", 30},
		{"a-stage", 10},
		{"b-stage", 20},
	}
	for _, s := range stages {
		if err := reg.Register(newTestStage(s.name, pipeline.PhaseParse, s.order)); err != nil {
			t.Fatalf("Register(%q) error = %v", s.name, err)
		}
	}

	ordered, err := reg.StagesForPhase(pipeline.PhaseParse)
	if err != nil {
		t.Fatalf("StagesForPhase() error = %v", err)
	}
	if len(ordered) != 3 {
		t.Fatalf("StagesForPhase() returned %d stages, want 3", len(ordered))
	}

	expectedOrder := []string{"a-stage", "b-stage", "c-stage"}
	for i, s := range ordered {
		if s.Descriptor().Name != expectedOrder[i] {
			t.Errorf("stage[%d] = %q, want %q", i, s.Descriptor().Name, expectedOrder[i])
		}
	}
}

func TestRegistryBeforeAfterConstraints(t *testing.T) {
	reg := compiler.NewStageRegistry()

	// b must run before a (even though a has lower order).
	a := &testStage{
		desc: pipeline.StageDescriptor{
			Name: "a", Phase: pipeline.PhaseParse, Order: 10,
			After: []string{"b"},
		},
		execFn: func(_ context.Context, input any) (any, error) { return input, nil },
	}
	b := &testStage{
		desc: pipeline.StageDescriptor{
			Name: "b", Phase: pipeline.PhaseParse, Order: 20,
			Before: []string{"a"},
		},
		execFn: func(_ context.Context, input any) (any, error) { return input, nil },
	}

	if err := reg.Register(a); err != nil {
		t.Fatalf("Register(a) error = %v", err)
	}
	if err := reg.Register(b); err != nil {
		t.Fatalf("Register(b) error = %v", err)
	}

	ordered, err := reg.StagesForPhase(pipeline.PhaseParse)
	if err != nil {
		t.Fatalf("StagesForPhase() error = %v", err)
	}

	if ordered[0].Descriptor().Name != "b" {
		t.Errorf("expected b first, got %q", ordered[0].Descriptor().Name)
	}
	if ordered[1].Descriptor().Name != "a" {
		t.Errorf("expected a second, got %q", ordered[1].Descriptor().Name)
	}
}

func TestRegistryEmptyPhase(t *testing.T) {
	reg := compiler.NewStageRegistry()
	stages, err := reg.StagesForPhase(pipeline.PhaseRender)
	if err != nil {
		t.Fatalf("StagesForPhase() error = %v", err)
	}
	if len(stages) != 0 {
		t.Errorf("expected 0 stages for empty phase, got %d", len(stages))
	}
}

func TestRegistryPhaseFiltering(t *testing.T) {
	reg := compiler.NewStageRegistry()

	_ = reg.Register(newTestStage("parser", pipeline.PhaseParse, 0))
	_ = reg.Register(newTestStage("validator", pipeline.PhaseValidate, 0))
	_ = reg.Register(newTestStage("renderer", pipeline.PhaseRender, 0))

	parseStages, err := reg.StagesForPhase(pipeline.PhaseParse)
	if err != nil {
		t.Fatalf("StagesForPhase(Parse) error = %v", err)
	}
	if len(parseStages) != 1 || parseStages[0].Descriptor().Name != "parser" {
		t.Errorf("expected [parser], got %v", stageNames(parseStages))
	}
}

func stageNames(stages []stage.Stage) []string {
	names := make([]string, len(stages))
	for i, s := range stages {
		names[i] = s.Descriptor().Name
	}
	return names
}
