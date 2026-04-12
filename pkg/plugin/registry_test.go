package plugin_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
	"github.com/mariotoffia/goagentmeta/pkg/plugin"
)

// stubStage is a minimal Stage implementation for testing.
type stubStage struct {
	desc pipeline.StageDescriptor
}

func (s *stubStage) Descriptor() pipeline.StageDescriptor { return s.desc }
func (s *stubStage) Execute(_ context.Context, input any) (any, error) {
	return input, nil
}

// stubRenderer is a minimal Renderer implementation for testing.
type stubRenderer struct {
	stubStage
	target build.Target
}

func (r *stubRenderer) Target() build.Target                        { return r.target }
func (r *stubRenderer) SupportedCapabilities() capability.CapabilityRegistry {
	return capability.CapabilityRegistry{}
}

func newStage(name string, phase pipeline.Phase, order int) *stubStage {
	return &stubStage{desc: pipeline.StageDescriptor{Name: name, Phase: phase, Order: order}}
}

func newRenderer(name string, target build.Target) *stubRenderer {
	return &stubRenderer{
		stubStage: stubStage{desc: pipeline.StageDescriptor{
			Name:  name,
			Phase: pipeline.PhaseRender,
			Order: 0,
		}},
		target: target,
	}
}

func TestRegisterStage(t *testing.T) {
	tests := []struct {
		name    string
		stages  []*stubStage
		wantErr bool
	}{
		{
			name:    "single stage succeeds",
			stages:  []*stubStage{newStage("parse-yaml", pipeline.PhaseParse, 10)},
			wantErr: false,
		},
		{
			name: "duplicate stage name returns error",
			stages: []*stubStage{
				newStage("parse-yaml", pipeline.PhaseParse, 10),
				newStage("parse-yaml", pipeline.PhaseParse, 20),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := plugin.NewRegistry()
			var err error
			for _, s := range tt.stages {
				if err = reg.RegisterStage(s); err != nil {
					break
				}
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("RegisterStage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMustRegisterStagePanicsOnDuplicate(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.MustRegisterStage(newStage("dup-stage", pipeline.PhaseParse, 0))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate MustRegisterStage")
		}
	}()

	reg.MustRegisterStage(newStage("dup-stage", pipeline.PhaseParse, 0))
}

func TestStagesOrderedByPhaseThenOrder(t *testing.T) {
	reg := plugin.NewRegistry()
	// Register out of order.
	reg.MustRegisterStage(newStage("render-claude", pipeline.PhaseRender, 20))
	reg.MustRegisterStage(newStage("parse-yaml", pipeline.PhaseParse, 10))
	reg.MustRegisterStage(newStage("render-cursor", pipeline.PhaseRender, 10))
	reg.MustRegisterStage(newStage("validate-schema", pipeline.PhaseValidate, 5))
	reg.MustRegisterStage(newStage("parse-json", pipeline.PhaseParse, 20))

	stages := reg.Stages()
	if len(stages) != 5 {
		t.Fatalf("got %d stages, want 5", len(stages))
	}

	want := []struct {
		name  string
		phase pipeline.Phase
		order int
	}{
		{"parse-yaml", pipeline.PhaseParse, 10},
		{"parse-json", pipeline.PhaseParse, 20},
		{"validate-schema", pipeline.PhaseValidate, 5},
		{"render-cursor", pipeline.PhaseRender, 10},
		{"render-claude", pipeline.PhaseRender, 20},
	}

	for i, w := range want {
		d := stages[i].Descriptor()
		if d.Name != w.name || d.Phase != w.phase || d.Order != w.order {
			t.Errorf("stages[%d] = {%s, %v, %d}, want {%s, %v, %d}",
				i, d.Name, d.Phase, d.Order, w.name, w.phase, w.order)
		}
	}
}

func TestRegisterRenderer(t *testing.T) {
	tests := []struct {
		name      string
		renderers []*stubRenderer
		wantErr   bool
	}{
		{
			name:      "single renderer succeeds",
			renderers: []*stubRenderer{newRenderer("claude-renderer", build.TargetClaude)},
			wantErr:   false,
		},
		{
			name: "duplicate target returns error",
			renderers: []*stubRenderer{
				newRenderer("claude-v1", build.TargetClaude),
				newRenderer("claude-v2", build.TargetClaude),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := plugin.NewRegistry()
			var err error
			for _, r := range tt.renderers {
				if err = reg.RegisterRenderer(r); err != nil {
					break
				}
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("RegisterRenderer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStageByName(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.MustRegisterStage(newStage("my-stage", pipeline.PhaseParse, 0))

	tests := []struct {
		name  string
		query string
		found bool
	}{
		{name: "found", query: "my-stage", found: true},
		{name: "not found", query: "no-such-stage", found: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := reg.StageByName(tt.query)
			if (s != nil) != tt.found {
				t.Fatalf("StageByName(%q) found = %v, want %v", tt.query, s != nil, tt.found)
			}
		})
	}
}

func TestRendererByTarget(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.MustRegisterRenderer(newRenderer("claude-rend", build.TargetClaude))

	tests := []struct {
		name   string
		target build.Target
		found  bool
	}{
		{name: "found", target: build.TargetClaude, found: true},
		{name: "not found", target: build.TargetCursor, found: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := reg.RendererByTarget(tt.target)
			if (r != nil) != tt.found {
				t.Fatalf("RendererByTarget(%q) found = %v, want %v", tt.target, r != nil, tt.found)
			}
		})
	}
}

func TestConcurrentRegistration(t *testing.T) {
	reg := plugin.NewRegistry()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)

	errs := make([]error, n)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			s := newStage(fmt.Sprintf("stage-%d", idx), pipeline.PhaseParse, idx)
			errs[idx] = reg.RegisterStage(s)
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent RegisterStage(%d) failed: %v", i, err)
		}
	}

	stages := reg.Stages()
	if len(stages) != n {
		t.Fatalf("got %d stages, want %d", len(stages), n)
	}
}

func TestWithStageValidator(t *testing.T) {
	reg := plugin.NewRegistry(
		plugin.WithStageValidator(func(s stage.Stage) error {
			if s.Descriptor().Order < 0 {
				return fmt.Errorf("negative order not allowed")
			}
			return nil
		}),
	)

	// Valid stage.
	if err := reg.RegisterStage(newStage("ok", pipeline.PhaseParse, 10)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid stage rejected by validator.
	if err := reg.RegisterStage(newStage("bad", pipeline.PhaseParse, -1)); err == nil {
		t.Fatal("expected error from validator, got nil")
	}
}
