package compiler

import (
	"context"
	"fmt"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// CompilerContext carries build-wide shared state through all pipeline stages.
// It is passed alongside context.Context to avoid the context-value anti-pattern.
type CompilerContext struct {
	// Config holds the pipeline configuration.
	Config *PipelineConfig
	// Report accumulates the build report throughout the pipeline.
	Report *pipeline.BuildReport
}

// Pipeline orchestrates the multi-phase compiler pipeline. It loads stages
// from the registry, orders them by phase and priority, dispatches IR between
// stages, calls hooks, and accumulates diagnostics into a final BuildReport.
type Pipeline struct {
	config PipelineConfig
}

// NewPipeline creates a pipeline with the given options. Default: fail-fast mode.
func NewPipeline(opts ...Option) *Pipeline {
	config := PipelineConfig{
		FailFast: true,
	}
	for _, opt := range opts {
		opt(&config)
	}
	if config.Registry == nil {
		config.Registry = NewStageRegistry()
	}
	return &Pipeline{config: config}
}

// Execute runs the full compiler pipeline. It iterates through all phases
// in order, dispatching to registered stages per phase, calling hooks,
// and accumulating diagnostics. Returns the final BuildReport.
//
// The rootPaths argument specifies the .ai/ source directory paths to compile.
func (p *Pipeline) Execute(ctx context.Context, rootPaths []string) (*pipeline.BuildReport, error) {
	report := &pipeline.BuildReport{
		Timestamp: time.Now(),
	}
	start := time.Now()

	cc := &CompilerContext{
		Config: &p.config,
		Report: report,
	}

	// The initial input to the parse phase is the list of root paths.
	var ir any = rootPaths

	for _, phase := range pipeline.AllPhases() {
		var err error
		ir, err = p.executePhase(ctx, cc, phase, ir)
		if err != nil {
			report.Duration = time.Since(start)
			return report, fmt.Errorf("pipeline phase %s: %w", phase, err)
		}
	}

	report.Duration = time.Since(start)
	return report, nil
}

// executePhase runs all stages and hooks for a single pipeline phase.
func (p *Pipeline) executePhase(
	ctx context.Context,
	cc *CompilerContext,
	phase pipeline.Phase,
	input any,
) (any, error) {
	// Run before-phase hooks.
	if err := p.runHooks(ctx, phase, pipeline.HookBeforePhase, &input); err != nil {
		return nil, fmt.Errorf("before-phase hook: %w", err)
	}

	// Get ordered stages for this phase.
	stages, err := p.config.Registry.StagesForPhase(phase)
	if err != nil {
		return nil, fmt.Errorf("stage ordering: %w", err)
	}

	// Execute each stage sequentially.
	ir := input
	for _, s := range stages {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		output, err := s.Execute(ctx, ir)
		if err != nil {
			p.emitDiagnostic(cc, pipeline.Diagnostic{
				Severity:   "error",
				Message:    err.Error(),
				Phase:      phase,
				SourcePath: s.Descriptor().Name,
			})

			if p.config.FailFast {
				return nil, fmt.Errorf("stage %s: %w", s.Descriptor().Name, err)
			}
			continue
		}

		if output != nil {
			ir = output
		}
	}

	// Run after-phase hooks.
	if err := p.runHooks(ctx, phase, pipeline.HookAfterPhase, &ir); err != nil {
		return nil, fmt.Errorf("after-phase hook: %w", err)
	}

	return ir, nil
}

// runHooks executes all hooks for the given phase and hook point.
func (p *Pipeline) runHooks(
	ctx context.Context,
	phase pipeline.Phase,
	point pipeline.HookPoint,
	ir *any,
) error {
	hooks := p.config.Registry.HooksForPhase(phase, point)
	for _, h := range hooks {
		hook := h.Hook()
		if hook.Handler == nil {
			continue
		}

		result, err := hook.Handler(ctx, *ir)
		if err != nil {
			return fmt.Errorf("hook %s: %w", hook.Name, err)
		}
		if result != nil {
			*ir = result
		}
	}
	return nil
}

// emitDiagnostic records a diagnostic if a sink is configured.
func (p *Pipeline) emitDiagnostic(cc *CompilerContext, d pipeline.Diagnostic) {
	cc.Report.Diagnostics = append(cc.Report.Diagnostics, d)
	if p.config.DiagnosticSink != nil {
		p.config.DiagnosticSink.Emit(context.Background(), d)
	}
}
