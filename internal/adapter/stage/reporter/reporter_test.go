package reporter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	adapter "github.com/mariotoffia/goagentmeta/internal/adapter/reporter"
	stagerep "github.com/mariotoffia/goagentmeta/internal/adapter/stage/reporter"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portstage "github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion.
var _ portstage.Stage = (*stagerep.Stage)(nil)

func testContext() context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: &pipeline.BuildReport{
			Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

func testContextWithReporter() (context.Context, *adapter.Reporter, *adapter.DiagnosticSink) {
	r := adapter.NewReporter()
	sink := adapter.NewDiagnosticSink()
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			Reporter:       r,
			DiagnosticSink: sink,
		},
		Report: &pipeline.BuildReport{
			Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
	}
	return compiler.ContextWithCompiler(context.Background(), cc), r, sink
}

func testContextWithFailFast() context.Context {
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{FailFast: true},
		Report: &pipeline.BuildReport{
			Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
	}
	return compiler.ContextWithCompiler(context.Background(), cc)
}

// ---------------------------------------------------------------------------
// Descriptor
// ---------------------------------------------------------------------------

func TestDescriptor(t *testing.T) {
	s := stagerep.New()
	d := s.Descriptor()

	if d.Name != "reporter" {
		t.Errorf("expected name 'reporter', got %q", d.Name)
	}
	if d.Phase != pipeline.PhaseReport {
		t.Errorf("expected phase PhaseReport, got %v", d.Phase)
	}
	if d.Order != 10 {
		t.Errorf("expected order 10, got %d", d.Order)
	}
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

func TestFactory(t *testing.T) {
	factory := stagerep.Factory()
	s, err := factory()
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if s.Descriptor().Name != "reporter" {
		t.Error("factory produced wrong stage")
	}
}

// ---------------------------------------------------------------------------
// Execute — input validation
// ---------------------------------------------------------------------------

func TestExecute_InvalidInput(t *testing.T) {
	s := stagerep.New()
	_, err := s.Execute(testContext(), "not a MaterializationResult")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestExecute_NilPointerInput(t *testing.T) {
	s := stagerep.New()
	var ptr *pipeline.MaterializationResult
	_, err := s.Execute(testContext(), ptr)
	if err == nil {
		t.Fatal("expected error for nil pointer")
	}
}

func TestExecute_ValueInput(t *testing.T) {
	s := stagerep.New()
	result, err := s.Execute(testContext(), pipeline.MaterializationResult{
		WrittenFiles: []string{"file1.md"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestExecute_PointerInput(t *testing.T) {
	s := stagerep.New()
	result, err := s.Execute(testContext(), &pipeline.MaterializationResult{
		WrittenFiles: []string{"file1.md"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Execute — report assembly
// ---------------------------------------------------------------------------

func TestExecute_AssemblesReportFromReporter(t *testing.T) {
	ctx, rpt, _ := testContextWithReporter()

	rpt.ReportLowering(ctx, pipeline.LoweringRecord{
		ObjectID: "skill-1",
		FromKind: model.KindSkill,
		ToKind:   model.KindInstruction,
		Status:   "lowered",
	})
	rpt.ReportSkipped(ctx, "hook-1", "not supported")
	rpt.ReportWarning(ctx, "some warning")

	s := stagerep.New()
	result, err := s.Execute(ctx, pipeline.MaterializationResult{
		WrittenFiles: []string{"out.md"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report := result.(*pipeline.BuildReport)
	if len(report.Units) == 0 {
		t.Fatal("expected at least one unit")
	}

	unit := report.Units[0]
	if len(unit.LoweringRecords) == 0 {
		t.Error("expected lowering records in unit")
	}
	if len(unit.SkippedObjects) == 0 {
		t.Error("expected skipped objects in unit")
	}
	if len(unit.Warnings) == 0 {
		t.Error("expected warnings in unit")
	}
}

func TestExecute_FailedObjectsBecomeDiagnostics(t *testing.T) {
	ctx, rpt, _ := testContextWithReporter()

	rpt.ReportFailed(ctx, "agent-1", errors.New("compilation error"))

	s := stagerep.New()
	result, err := s.Execute(ctx, pipeline.MaterializationResult{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report := result.(*pipeline.BuildReport)
	found := false
	for _, d := range report.Diagnostics {
		if d.ObjectID == "agent-1" && d.Severity == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error diagnostic for failed object")
	}
}

// ---------------------------------------------------------------------------
// Execute — with writers
// ---------------------------------------------------------------------------

func TestExecute_WritesJSON(t *testing.T) {
	mem := filesystem.NewMemFS()
	jsonW := adapter.NewJSONReportWriter(mem, ".ai-build/report.json")

	s := stagerep.New(stagerep.WithJSONWriter(jsonW))
	_, err := s.Execute(testContext(), pipeline.MaterializationResult{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := mem.ReadFile(context.Background(), ".ai-build/report.json")
	if err != nil {
		t.Fatalf("expected report.json to be written: %v", err)
	}
	if len(data) == 0 {
		t.Error("report.json is empty")
	}
}

func TestExecute_WritesMarkdown(t *testing.T) {
	mem := filesystem.NewMemFS()
	mdW := adapter.NewMarkdownReportWriter(mem, ".ai-build/report.md")

	s := stagerep.New(stagerep.WithMarkdownWriter(mdW))
	_, err := s.Execute(testContext(), pipeline.MaterializationResult{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := mem.ReadFile(context.Background(), ".ai-build/report.md")
	if err != nil {
		t.Fatalf("expected report.md to be written: %v", err)
	}
	if len(data) == 0 {
		t.Error("report.md is empty")
	}
}

func TestExecute_MultipleWriters(t *testing.T) {
	mem := filesystem.NewMemFS()
	jsonW := adapter.NewJSONReportWriter(mem, "report.json")
	mdW := adapter.NewMarkdownReportWriter(mem, "report.md")

	s := stagerep.New(
		stagerep.WithJSONWriter(jsonW),
		stagerep.WithMarkdownWriter(mdW),
	)
	_, err := s.Execute(testContext(), pipeline.MaterializationResult{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := mem.ReadFile(context.Background(), "report.json"); err != nil {
		t.Error("report.json not written")
	}
	if _, err := mem.ReadFile(context.Background(), "report.md"); err != nil {
		t.Error("report.md not written")
	}
}

// ---------------------------------------------------------------------------
// Execute — writer errors
// ---------------------------------------------------------------------------

type failWriter struct{}

func (w *failWriter) Write(_ context.Context, _ pipeline.BuildReport) error {
	return errors.New("write failed")
}

func TestExecute_WriterError_FailFast(t *testing.T) {
	s := stagerep.New(stagerep.WithWriter("fail", &failWriter{}))
	_, err := s.Execute(testContextWithFailFast(), pipeline.MaterializationResult{})
	if err == nil {
		t.Fatal("expected error from failing writer with fail-fast")
	}
}

func TestExecute_WriterError_ContinueOnError(t *testing.T) {
	mem := filesystem.NewMemFS()
	jsonW := adapter.NewJSONReportWriter(mem, "report.json")

	s := stagerep.New(
		stagerep.WithWriter("fail", &failWriter{}),
		stagerep.WithJSONWriter(jsonW),
	)

	_, err := s.Execute(testContext(), pipeline.MaterializationResult{})
	if err != nil {
		t.Fatalf("expected no error in non-fail-fast mode, got: %v", err)
	}

	// The JSON writer should still have been called.
	if _, err := mem.ReadFile(context.Background(), "report.json"); err != nil {
		t.Error("report.json should still be written despite other writer failure")
	}
}

// ---------------------------------------------------------------------------
// Execute — no CompilerContext
// ---------------------------------------------------------------------------

func TestExecute_NoCompilerContext(t *testing.T) {
	s := stagerep.New()
	result, err := s.Execute(context.Background(), pipeline.MaterializationResult{
		WrittenFiles: []string{"file.md"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Execute — with pre-existing units
// ---------------------------------------------------------------------------

func TestExecute_PreExistingUnitsEnriched(t *testing.T) {
	ctx, rpt, _ := testContextWithReporter()

	// Simulate renderers populating units.
	cc := compiler.CompilerFromContext(ctx)
	cc.Report.Units = []pipeline.UnitReport{
		{
			Coordinate: build.BuildCoordinate{
				Unit:      build.BuildUnit{Target: build.TargetClaude, Profile: build.ProfileLocalDev},
				OutputDir: ".ai-build/claude/local-dev",
			},
			EmittedFiles: []string{"CLAUDE.md"},
		},
	}

	rpt.ReportLowering(ctx, pipeline.LoweringRecord{
		ObjectID: "skill-1",
		Status:   "lowered",
	})
	rpt.ReportWarning(ctx, "some warning")

	s := stagerep.New()
	result, err := s.Execute(ctx, pipeline.MaterializationResult{})
	if err != nil {
		t.Fatal(err)
	}

	report := result.(*pipeline.BuildReport)
	if len(report.Units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(report.Units))
	}
	if len(report.Units[0].LoweringRecords) == 0 {
		t.Error("expected lowering records to be merged into existing unit")
	}
	if len(report.Units[0].Warnings) == 0 {
		t.Error("expected warnings to be merged into existing unit")
	}
}

// ---------------------------------------------------------------------------
// Execute — empty plan
// ---------------------------------------------------------------------------

func TestExecute_EmptyMaterializationResult(t *testing.T) {
	s := stagerep.New()
	result, err := s.Execute(testContext(), pipeline.MaterializationResult{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report := result.(*pipeline.BuildReport)
	if report == nil {
		t.Fatal("expected non-nil report")
	}
}

// ---------------------------------------------------------------------------
// Execute — DiagnosticSink merge
// ---------------------------------------------------------------------------

func TestExecute_DiagnosticsFromSinkMerged(t *testing.T) {
	ctx, _, sink := testContextWithReporter()

	sink.Emit(ctx, pipeline.Diagnostic{
		Severity: "warning",
		Message:  "from-sink",
		Phase:    pipeline.PhaseRender,
	})

	s := stagerep.New()
	result, err := s.Execute(ctx, pipeline.MaterializationResult{})
	if err != nil {
		t.Fatal(err)
	}

	report := result.(*pipeline.BuildReport)
	found := false
	for _, d := range report.Diagnostics {
		if d.Message == "from-sink" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected diagnostic from sink to be merged into report")
	}
}
