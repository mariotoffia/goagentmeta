// Package reporter implements the PhaseReport pipeline stage. It assembles
// the final BuildReport from accumulated diagnostics, lowering records,
// provenance entries, and materialization results. It then delegates to
// configured BuildReportWriter(s) to persist the report.
package reporter

import (
	"context"
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"

	adapter "github.com/mariotoffia/goagentmeta/internal/adapter/reporter"
)

// Compile-time assertion.
var _ stage.Stage = (*Stage)(nil)

// Stage implements the PhaseReport pipeline stage.
type Stage struct {
	writers []writerEntry
}

type writerEntry struct {
	name   string
	writer interface {
		Write(ctx context.Context, report pipeline.BuildReport) error
	}
}

// Option configures the reporter stage.
type Option func(*Stage)

// WithJSONWriter adds a JSON report writer.
func WithJSONWriter(w *adapter.JSONReportWriter) Option {
	return func(s *Stage) {
		s.writers = append(s.writers, writerEntry{name: "json", writer: w})
	}
}

// WithMarkdownWriter adds a Markdown report writer.
func WithMarkdownWriter(w *adapter.MarkdownReportWriter) Option {
	return func(s *Stage) {
		s.writers = append(s.writers, writerEntry{name: "markdown", writer: w})
	}
}

// WithWriter adds a generic BuildReportWriter.
func WithWriter(name string, w interface {
	Write(ctx context.Context, report pipeline.BuildReport) error
}) Option {
	return func(s *Stage) {
		s.writers = append(s.writers, writerEntry{name: name, writer: w})
	}
}

// New creates a new reporter stage with the given options.
func New(opts ...Option) *Stage {
	s := &Stage{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Descriptor returns the stage descriptor for the report phase.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "reporter",
		Phase: pipeline.PhaseReport,
		Order: 10,
	}
}

// Execute assembles the final BuildReport from pipeline state and writes it
// using all configured writers. Input is MaterializationResult.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	// Accept both value and pointer.
	var matResult pipeline.MaterializationResult
	switch v := input.(type) {
	case pipeline.MaterializationResult:
		matResult = v
	case *pipeline.MaterializationResult:
		if v == nil {
			return nil, fmt.Errorf("reporter: nil MaterializationResult pointer")
		}
		matResult = *v
	default:
		return nil, fmt.Errorf("reporter: expected MaterializationResult, got %T", input)
	}

	cc := compiler.CompilerFromContext(ctx)

	report := s.assembleReport(ctx, cc, matResult)

	// If we have a CompilerContext, merge assembled data into the shared report.
	if cc != nil && cc.Report != nil {
		cc.Report.Units = report.Units
		// Merge diagnostics assembled from failed objects into the shared report.
		cc.Report.Diagnostics = mergeDiagnostics(cc.Report.Diagnostics, report.Diagnostics)
		// Merge diagnostics from sink if available.
		if cc.Config != nil && cc.Config.DiagnosticSink != nil {
			sinkDiags := cc.Config.DiagnosticSink.Diagnostics()
			if len(sinkDiags) > 0 {
				cc.Report.Diagnostics = mergeDiagnostics(cc.Report.Diagnostics, sinkDiags)
			}
		}
		report = *cc.Report
	}

	// Write via all configured writers.
	for _, w := range s.writers {
		if err := w.writer.Write(ctx, report); err != nil {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("report writer %q: %s", w.name, err),
				Phase:    pipeline.PhaseReport,
			})

			if isFailFast(ctx) {
				return nil, fmt.Errorf("report writer %q: %w", w.name, err)
			}
		}
	}

	// Also delegate to the BuildReportWriter in the config if set.
	if cc != nil && cc.Config != nil && cc.Config.ReportWriter != nil {
		if err := cc.Config.ReportWriter.Write(ctx, report); err != nil {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("config report writer: %s", err),
				Phase:    pipeline.PhaseReport,
			})
			if isFailFast(ctx) {
				return nil, fmt.Errorf("config report writer: %w", err)
			}
		}
	}

	return &report, nil
}

// assembleReport builds the BuildReport from accumulated pipeline state.
func (s *Stage) assembleReport(
	ctx context.Context,
	cc *compiler.CompilerContext,
	matResult pipeline.MaterializationResult,
) pipeline.BuildReport {
	_ = ctx

	report := pipeline.BuildReport{}

	if cc != nil && cc.Report != nil {
		report.Timestamp = cc.Report.Timestamp
		report.Duration = cc.Report.Duration
		report.Diagnostics = cc.Report.Diagnostics
	}

	if cc == nil || cc.Config == nil {
		return report
	}

	// Get accumulated data from the Reporter adapter.
	var lowerings []pipeline.LoweringRecord
	var skipped []adapter.SkippedEntry
	var failed []adapter.FailedEntry
	var warnings []string

	if r, ok := cc.Config.Reporter.(*adapter.Reporter); ok && r != nil {
		lowerings = r.LoweringRecords()
		skipped = r.SkippedEntries()
		failed = r.FailedEntries()
		warnings = r.Warnings()
	}

	// Build unit reports from materialization result.
	// The matResult itself doesn't carry per-unit breakdown, so we create
	// a single summary unit. Richer unit data comes from the report already
	// accumulated in cc.Report.Units during rendering.
	if cc.Report != nil && len(cc.Report.Units) > 0 {
		// Units were already populated by renderers; enrich with reporter data.
		for i := range cc.Report.Units {
			cc.Report.Units[i].LoweringRecords = append(
				cc.Report.Units[i].LoweringRecords, lowerings...)
			for _, se := range skipped {
				cc.Report.Units[i].SkippedObjects = append(
					cc.Report.Units[i].SkippedObjects, se.ObjectID)
			}
			cc.Report.Units[i].Warnings = append(
				cc.Report.Units[i].Warnings, warnings...)
		}
		report.Units = cc.Report.Units
	} else {
		// No pre-existing units; create a summary unit.
		skippedIDs := make([]string, 0, len(skipped))
		for _, se := range skipped {
			skippedIDs = append(skippedIDs, se.ObjectID)
		}

		report.Units = []pipeline.UnitReport{{
			EmittedFiles:    matResult.WrittenFiles,
			LoweringRecords: lowerings,
			SkippedObjects:  skippedIDs,
			Warnings:        warnings,
		}}
	}

	// Add failed objects as error diagnostics.
	for _, fe := range failed {
		report.Diagnostics = append(report.Diagnostics, pipeline.Diagnostic{
			Severity: "error",
			Message:  fe.Err,
			ObjectID: fe.ObjectID,
			Phase:    pipeline.PhaseReport,
		})
	}

	return report
}

// mergeDiagnostics combines two diagnostic slices, deduplicating by message+phase.
func mergeDiagnostics(existing, incoming []pipeline.Diagnostic) []pipeline.Diagnostic {
	seen := make(map[string]struct{}, len(existing))
	for _, d := range existing {
		key := fmt.Sprintf("%s:%d:%s", d.Severity, d.Phase, d.Message)
		seen[key] = struct{}{}
	}

	result := make([]pipeline.Diagnostic, len(existing))
	copy(result, existing)

	for _, d := range incoming {
		key := fmt.Sprintf("%s:%d:%s", d.Severity, d.Phase, d.Message)
		if _, ok := seen[key]; !ok {
			result = append(result, d)
			seen[key] = struct{}{}
		}
	}
	return result
}

func isFailFast(ctx context.Context) bool {
	cc := compiler.CompilerFromContext(ctx)
	if cc == nil || cc.Config == nil {
		return false
	}
	return cc.Config.FailFast
}

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

// Factory returns a StageFactory that creates reporter stages.
func Factory(opts ...Option) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(opts...), nil
	}
}
