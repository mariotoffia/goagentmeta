// Package reporter defines port interfaces for build reporting, diagnostics,
// and provenance recording. Every compiler run produces a BuildReport that
// documents what was emitted, what was lowered, what was skipped, and the
// full provenance chain from source to output.
//
// These ports enable the compiler to remain agnostic about output format
// (JSON, YAML, Markdown, terminal) while ensuring all information is captured.
package reporter

import (
	"context"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// Reporter collects high-level build events during compilation.
// The pipeline orchestrator calls Reporter methods as stages execute.
type Reporter interface {
	// ReportLowering records a lowering decision for a canonical object.
	ReportLowering(ctx context.Context, record pipeline.LoweringRecord)

	// ReportSkipped records that a canonical object was skipped for a target.
	ReportSkipped(ctx context.Context, objectID string, reason string)

	// ReportFailed records that a canonical object failed to compile.
	ReportFailed(ctx context.Context, objectID string, err error)

	// ReportWarning records a non-fatal warning.
	ReportWarning(ctx context.Context, message string)
}

// DiagnosticSink receives compiler diagnostics (errors, warnings, info).
// Diagnostics are collected throughout the pipeline and included in the
// final BuildReport.
type DiagnosticSink interface {
	// Emit records a single diagnostic message.
	Emit(ctx context.Context, diagnostic pipeline.Diagnostic)

	// Diagnostics returns all diagnostics emitted so far.
	Diagnostics() []pipeline.Diagnostic
}

// ProvenanceRecorder tracks the source-to-output provenance chain.
// For every generated file, it records which canonical objects contributed
// to it and what transformations were applied.
type ProvenanceRecorder interface {
	// Record adds a provenance entry linking a source object to an output
	// file with the lowering chain applied.
	Record(ctx context.Context, sourceObjectID string, outputPath string, chain []string)
}

// BuildReportWriter serializes a completed BuildReport to a persistent
// format (JSON, YAML, Markdown). The report phase uses this port to
// write the final report to the build output directory.
type BuildReportWriter interface {
	// Write serializes the build report to the configured output format.
	Write(ctx context.Context, report pipeline.BuildReport) error
}
