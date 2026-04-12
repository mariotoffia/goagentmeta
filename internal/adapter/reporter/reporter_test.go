package reporter_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/reporter"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ---------------------------------------------------------------------------
// DiagnosticSink
// ---------------------------------------------------------------------------

func TestDiagnosticSink_EmptyReturnsEmptySlice(t *testing.T) {
	sink := reporter.NewDiagnosticSink()
	diags := sink.Diagnostics()
	if diags == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestDiagnosticSink_EmitAndRetrieve(t *testing.T) {
	sink := reporter.NewDiagnosticSink()
	ctx := context.Background()

	sink.Emit(ctx, pipeline.Diagnostic{Severity: "warning", Message: "w1", Phase: pipeline.PhaseRender})
	sink.Emit(ctx, pipeline.Diagnostic{Severity: "error", Message: "e1", Phase: pipeline.PhaseParse})
	sink.Emit(ctx, pipeline.Diagnostic{Severity: "info", Message: "i1", Phase: pipeline.PhaseReport})

	diags := sink.Diagnostics()
	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d", len(diags))
	}

	// Errors first, then warnings, then info.
	if diags[0].Severity != "error" {
		t.Errorf("first diagnostic should be error, got %s", diags[0].Severity)
	}
	if diags[1].Severity != "warning" {
		t.Errorf("second diagnostic should be warning, got %s", diags[1].Severity)
	}
	if diags[2].Severity != "info" {
		t.Errorf("third diagnostic should be info, got %s", diags[2].Severity)
	}
}

func TestDiagnosticSink_SortByPhaseWithinSeverity(t *testing.T) {
	sink := reporter.NewDiagnosticSink()
	ctx := context.Background()

	sink.Emit(ctx, pipeline.Diagnostic{Severity: "error", Message: "e2", Phase: pipeline.PhaseRender})
	sink.Emit(ctx, pipeline.Diagnostic{Severity: "error", Message: "e1", Phase: pipeline.PhaseParse})

	diags := sink.Diagnostics()
	if diags[0].Phase != pipeline.PhaseParse {
		t.Errorf("expected PhaseParse first, got %s", diags[0].Phase)
	}
	if diags[1].Phase != pipeline.PhaseRender {
		t.Errorf("expected PhaseRender second, got %s", diags[1].Phase)
	}
}

func TestDiagnosticSink_ConcurrentEmit(t *testing.T) {
	sink := reporter.NewDiagnosticSink()
	ctx := context.Background()
	const n = 100

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			sink.Emit(ctx, pipeline.Diagnostic{Severity: "info", Message: "concurrent"})
		}()
	}
	wg.Wait()

	diags := sink.Diagnostics()
	if len(diags) != n {
		t.Fatalf("expected %d diagnostics, got %d", n, len(diags))
	}
}

// ---------------------------------------------------------------------------
// ProvenanceRecorder
// ---------------------------------------------------------------------------

func TestProvenanceRecorder_EmptyEntries(t *testing.T) {
	r := reporter.NewProvenanceRecorder()
	entries := r.Entries()
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestProvenanceRecorder_RecordAndRetrieve(t *testing.T) {
	r := reporter.NewProvenanceRecorder()
	ctx := context.Background()

	r.Record(ctx, "obj-1", "/out/file1.md", []string{"skill→instruction"})
	r.Record(ctx, "obj-1", "/out/file2.md", nil)
	r.Record(ctx, "obj-2", "/out/file3.md", []string{"hook→script"})

	entries := r.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	bySource := r.EntriesForSource("obj-1")
	if len(bySource) != 2 {
		t.Fatalf("expected 2 entries for obj-1, got %d", len(bySource))
	}

	bySource2 := r.EntriesForSource("obj-2")
	if len(bySource2) != 1 {
		t.Fatalf("expected 1 entry for obj-2, got %d", len(bySource2))
	}
}

func TestProvenanceRecorder_ChainIsCopied(t *testing.T) {
	r := reporter.NewProvenanceRecorder()
	ctx := context.Background()

	chain := []string{"a", "b"}
	r.Record(ctx, "obj", "/out/f.md", chain)

	// Mutate original chain.
	chain[0] = "mutated"

	entries := r.Entries()
	if entries[0].Chain[0] != "a" {
		t.Errorf("chain should be a copy, but was mutated")
	}
}

func TestProvenanceRecorder_ConcurrentRecord(t *testing.T) {
	r := reporter.NewProvenanceRecorder()
	ctx := context.Background()
	const n = 100

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r.Record(ctx, "obj", "/out/f.md", nil)
		}()
	}
	wg.Wait()

	if len(r.Entries()) != n {
		t.Fatalf("expected %d entries, got %d", n, len(r.Entries()))
	}
}

// ---------------------------------------------------------------------------
// Reporter
// ---------------------------------------------------------------------------

func TestReporter_EmptyState(t *testing.T) {
	r := reporter.NewReporter()
	if len(r.LoweringRecords()) != 0 {
		t.Error("expected 0 lowering records")
	}
	if len(r.SkippedEntries()) != 0 {
		t.Error("expected 0 skipped entries")
	}
	if len(r.FailedEntries()) != 0 {
		t.Error("expected 0 failed entries")
	}
	if len(r.Warnings()) != 0 {
		t.Error("expected 0 warnings")
	}
}

func TestReporter_ReportLowering(t *testing.T) {
	r := reporter.NewReporter()
	ctx := context.Background()

	r.ReportLowering(ctx, pipeline.LoweringRecord{
		ObjectID: "skill-1",
		FromKind: model.KindSkill,
		ToKind:   model.KindInstruction,
		Reason:   "target does not support skills",
		Status:   "lowered",
	})

	records := r.LoweringRecords()
	if len(records) != 1 {
		t.Fatalf("expected 1 lowering record, got %d", len(records))
	}
	if records[0].ObjectID != "skill-1" {
		t.Errorf("expected ObjectID skill-1, got %s", records[0].ObjectID)
	}
}

func TestReporter_ReportSkipped(t *testing.T) {
	r := reporter.NewReporter()
	ctx := context.Background()

	r.ReportSkipped(ctx, "hook-1", "hooks not supported on this target")

	skipped := r.SkippedEntries()
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped entry, got %d", len(skipped))
	}
	if skipped[0].ObjectID != "hook-1" {
		t.Errorf("expected ObjectID hook-1, got %s", skipped[0].ObjectID)
	}
}

func TestReporter_ReportFailed(t *testing.T) {
	r := reporter.NewReporter()
	ctx := context.Background()

	r.ReportFailed(ctx, "agent-1", errors.New("compilation error"))

	failed := r.FailedEntries()
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed entry, got %d", len(failed))
	}
	if failed[0].Err != "compilation error" {
		t.Errorf("expected error message 'compilation error', got %s", failed[0].Err)
	}
}

func TestReporter_ReportWarning(t *testing.T) {
	r := reporter.NewReporter()
	ctx := context.Background()

	r.ReportWarning(ctx, "potential issue")

	warnings := r.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0] != "potential issue" {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

func TestReporter_ConcurrentAccess(t *testing.T) {
	r := reporter.NewReporter()
	ctx := context.Background()
	const n = 50

	var wg sync.WaitGroup
	wg.Add(4 * n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r.ReportLowering(ctx, pipeline.LoweringRecord{ObjectID: "obj"})
		}()
		go func() {
			defer wg.Done()
			r.ReportSkipped(ctx, "obj", "reason")
		}()
		go func() {
			defer wg.Done()
			r.ReportFailed(ctx, "obj", errors.New("err"))
		}()
		go func() {
			defer wg.Done()
			r.ReportWarning(ctx, "warn")
		}()
	}
	wg.Wait()

	if len(r.LoweringRecords()) != n {
		t.Errorf("expected %d lowerings, got %d", n, len(r.LoweringRecords()))
	}
	if len(r.SkippedEntries()) != n {
		t.Errorf("expected %d skipped, got %d", n, len(r.SkippedEntries()))
	}
	if len(r.FailedEntries()) != n {
		t.Errorf("expected %d failed, got %d", n, len(r.FailedEntries()))
	}
	if len(r.Warnings()) != n {
		t.Errorf("expected %d warnings, got %d", n, len(r.Warnings()))
	}
}
