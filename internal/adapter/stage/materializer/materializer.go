// Package materializer implements the PhaseMaterialize pipeline stage.
// It delegates to a filesystem.Materializer port to write the EmissionPlan
// to disk, supporting dry-run mode and multiple sync strategies.
package materializer

import (
	"context"
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion: *Stage satisfies the stage.Stage port interface.
var _ stage.Stage = (*Stage)(nil)

// Stage implements the PhaseMaterialize pipeline stage. It accepts an
// EmissionPlan and delegates to a filesystem.Materializer to write files,
// directories, symlinks, and plugin bundles to the output filesystem.
type Stage struct {
	materializer portfs.Materializer
	dryRun       bool
	syncMode     SyncMode
	syncPatterns []string // patterns for adopt-selected mode
}

// New creates a new materializer Stage with the given materializer port.
func New(m portfs.Materializer, opts ...Option) *Stage {
	s := &Stage{
		materializer: m,
		syncMode:     SyncBuildOnly,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures the materializer Stage.
type Option func(*Stage)

// WithDryRun enables dry-run mode: no actual I/O is performed.
func WithDryRun(dryRun bool) Option {
	return func(s *Stage) {
		s.dryRun = dryRun
	}
}

// WithSyncMode sets the sync strategy for output files.
func WithSyncMode(mode SyncMode) Option {
	return func(s *Stage) {
		s.syncMode = mode
	}
}

// WithSyncPatterns sets the file patterns for adopt-selected sync mode.
func WithSyncPatterns(patterns []string) Option {
	return func(s *Stage) {
		s.syncPatterns = patterns
	}
}

// Descriptor returns the stage metadata for pipeline registration.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "materializer",
		Phase: pipeline.PhaseMaterialize,
		Order: 10,
	}
}

// Execute transforms an EmissionPlan into a MaterializationResult.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	plan, ok := input.(pipeline.EmissionPlan)
	if !ok {
		planPtr, ok := input.(*pipeline.EmissionPlan)
		if !ok || planPtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrMaterialization,
				fmt.Sprintf("expected pipeline.EmissionPlan or *pipeline.EmissionPlan, got %T", input),
				"materializer",
			)
		}
		plan = *planPtr
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "MATERIALIZE_START",
		Message:  fmt.Sprintf("materializing %d build unit(s), dry-run=%v, sync=%s", len(plan.Units), s.dryRun, s.syncMode),
		Phase:    pipeline.PhaseMaterialize,
	})

	if s.dryRun {
		result := simulateMaterialization(plan)
		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "info",
			Code:     "MATERIALIZE_DRY_RUN",
			Message:  fmt.Sprintf("dry-run: would write %d file(s), create %d dir(s)", len(result.WrittenFiles), len(result.CreatedDirs)),
			Phase:    pipeline.PhaseMaterialize,
		})
		return result, nil
	}

	result, err := s.materializer.Materialize(ctx, plan)
	if err != nil {
		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "error",
			Code:     "MATERIALIZE_ERROR",
			Message:  err.Error(),
			Phase:    pipeline.PhaseMaterialize,
		})

		// Check FailFast from context.
		if isFailFast(ctx) {
			return result, pipeline.Wrap(
				pipeline.ErrMaterialization,
				"materialization failed (fail-fast)",
				"materializer",
				err,
			)
		}

		// Continue with partial results (errors already in result.Errors).
		return result, nil
	}

	// Apply sync mode post-processing.
	if s.syncMode != SyncBuildOnly {
		syncResult, syncErr := applySyncMode(ctx, s.materializer, s.syncMode, s.syncPatterns, plan)
		if syncErr != nil {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "error",
				Code:     "MATERIALIZE_SYNC_ERROR",
				Message:  syncErr.Error(),
				Phase:    pipeline.PhaseMaterialize,
			})
			// Merge individual sync errors into result.
			result.Errors = append(result.Errors, syncResult.Errors...)
			// If no individual errors were recorded, add the top-level sync error.
			if len(syncResult.Errors) == 0 {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: "",
					Err:  syncErr.Error(),
				})
			}

			if isFailFast(ctx) {
				return result, pipeline.Wrap(
					pipeline.ErrMaterialization,
					"sync failed (fail-fast)",
					"materializer",
					syncErr,
				)
			}
		}
		result.WrittenFiles = append(result.WrittenFiles, syncResult.WrittenFiles...)
		result.CreatedDirs = append(result.CreatedDirs, syncResult.CreatedDirs...)
		result.SymlinkedFiles = append(result.SymlinkedFiles, syncResult.SymlinkedFiles...)
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "MATERIALIZE_COMPLETE",
		Message: fmt.Sprintf(
			"materialized %d file(s), %d dir(s), %d symlink(s), %d error(s)",
			len(result.WrittenFiles), len(result.CreatedDirs),
			len(result.SymlinkedFiles), len(result.Errors),
		),
		Phase: pipeline.PhaseMaterialize,
	})

	return result, nil
}

// Factory returns a StageFactory function for use with pipeline registration.
func Factory(m portfs.Materializer, opts ...Option) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(m, opts...), nil
	}
}

// isFailFast checks the CompilerContext for FailFast configuration.
func isFailFast(ctx context.Context) bool {
	cc := compiler.CompilerFromContext(ctx)
	return cc != nil && cc.Config != nil && cc.Config.FailFast
}

// emitDiagnostic sends a diagnostic through the CompilerContext if available.
func emitDiagnostic(ctx context.Context, d pipeline.Diagnostic) {
	cc := compiler.CompilerFromContext(ctx)
	if cc == nil {
		return
	}
	if cc.Report != nil {
		cc.Report.Diagnostics = append(cc.Report.Diagnostics, d)
	}
	if cc.Config != nil && cc.Config.DiagnosticSink != nil {
		cc.Config.DiagnosticSink.Emit(ctx, d)
	}
}
