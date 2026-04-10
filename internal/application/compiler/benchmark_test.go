package compiler_test

import (
	"context"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

func BenchmarkRegistryStagesForPhase(b *testing.B) {
	reg := compiler.NewStageRegistry()
	for i := 0; i < 20; i++ {
		name := "stage-" + string(rune('A'+i))
		_ = reg.Register(newTestStage(name, pipeline.PhaseParse, i))
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = reg.StagesForPhase(pipeline.PhaseParse)
	}
}

func BenchmarkPipelineExecuteEmpty(b *testing.B) {
	p := compiler.NewPipeline()
	ctx := context.Background()
	paths := []string{"/test/.ai/"}

	b.ResetTimer()
	for b.Loop() {
		_, _ = p.Execute(ctx, paths)
	}
}

func BenchmarkPipelineExecuteWithStages(b *testing.B) {
	noop := func(_ context.Context, input any) (any, error) { return input, nil }

	p := compiler.NewPipeline(
		compiler.WithStage(&testStage{
			desc:   pipeline.StageDescriptor{Name: "parser", Phase: pipeline.PhaseParse, Order: 0},
			execFn: noop,
		}),
		compiler.WithStage(&testStage{
			desc:   pipeline.StageDescriptor{Name: "validator", Phase: pipeline.PhaseValidate, Order: 0},
			execFn: noop,
		}),
		compiler.WithStage(&testStage{
			desc:   pipeline.StageDescriptor{Name: "normalizer", Phase: pipeline.PhaseNormalize, Order: 0},
			execFn: noop,
		}),
		compiler.WithStage(&testStage{
			desc:   pipeline.StageDescriptor{Name: "planner", Phase: pipeline.PhasePlan, Order: 0},
			execFn: noop,
		}),
		compiler.WithStage(&testStage{
			desc:   pipeline.StageDescriptor{Name: "renderer", Phase: pipeline.PhaseRender, Order: 0},
			execFn: noop,
		}),
	)

	ctx := context.Background()
	paths := []string{"/test/.ai/"}

	b.ResetTimer()
	for b.Loop() {
		_, _ = p.Execute(ctx, paths)
	}
}
