package compiler

import (
	"github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/port/reporter"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// PipelineConfig holds the resolved configuration for a pipeline execution.
type PipelineConfig struct {
	// Registry is the stage registry containing all stages and hooks.
	Registry *StageRegistry

	// Reporter receives high-level build events.
	Reporter reporter.Reporter

	// DiagnosticSink receives compiler diagnostics.
	DiagnosticSink reporter.DiagnosticSink

	// ProvenanceRecorder tracks source-to-output provenance.
	ProvenanceRecorder reporter.ProvenanceRecorder

	// ReportWriter serializes the final build report.
	ReportWriter reporter.BuildReportWriter

	// FSReader provides read-only filesystem access.
	FSReader filesystem.Reader

	// FSWriter provides write filesystem access.
	FSWriter filesystem.Writer

	// Materializer writes emission plans to disk.
	Materializer filesystem.Materializer

	// FailFast stops the pipeline on the first stage error.
	// When false, the pipeline accumulates diagnostics and continues.
	FailFast bool
}

// Option configures a Pipeline during construction.
type Option func(*PipelineConfig)

// WithRegistry sets the stage registry.
func WithRegistry(r *StageRegistry) Option {
	return func(c *PipelineConfig) { c.Registry = r }
}

// WithStage registers a stage in the pipeline's registry.
// If no registry is set, one is created.
func WithStage(s stage.Stage) Option {
	return func(c *PipelineConfig) {
		if c.Registry == nil {
			c.Registry = NewStageRegistry()
		}
		// Ignore registration error here; it will surface during Execute.
		_ = c.Registry.Register(s)
	}
}

// WithHook registers a stage hook in the pipeline's registry.
func WithHook(h stage.StageHookHandler) Option {
	return func(c *PipelineConfig) {
		if c.Registry == nil {
			c.Registry = NewStageRegistry()
		}
		c.Registry.RegisterHook(h)
	}
}

// WithReporter sets the build event reporter.
func WithReporter(r reporter.Reporter) Option {
	return func(c *PipelineConfig) { c.Reporter = r }
}

// WithDiagnosticSink sets the diagnostic sink.
func WithDiagnosticSink(d reporter.DiagnosticSink) Option {
	return func(c *PipelineConfig) { c.DiagnosticSink = d }
}

// WithProvenanceRecorder sets the provenance recorder.
func WithProvenanceRecorder(p reporter.ProvenanceRecorder) Option {
	return func(c *PipelineConfig) { c.ProvenanceRecorder = p }
}

// WithReportWriter sets the build report writer.
func WithReportWriter(w reporter.BuildReportWriter) Option {
	return func(c *PipelineConfig) { c.ReportWriter = w }
}

// WithFSReader sets the filesystem reader.
func WithFSReader(r filesystem.Reader) Option {
	return func(c *PipelineConfig) { c.FSReader = r }
}

// WithFSWriter sets the filesystem writer.
func WithFSWriter(w filesystem.Writer) Option {
	return func(c *PipelineConfig) { c.FSWriter = w }
}

// WithMaterializer sets the materializer.
func WithMaterializer(m filesystem.Materializer) Option {
	return func(c *PipelineConfig) { c.Materializer = m }
}

// WithFailFast sets fail-fast mode (default: true).
func WithFailFast(ff bool) Option {
	return func(c *PipelineConfig) { c.FailFast = ff }
}
