package reporter

import (
	"context"
	"sync"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portreporter "github.com/mariotoffia/goagentmeta/internal/port/reporter"
)

// Compile-time assertion.
var _ portreporter.Reporter = (*Reporter)(nil)

// Reporter collects high-level build events (lowering records, skips,
// failures, warnings) during pipeline execution. Thread-safe.
type Reporter struct {
	mu        sync.Mutex
	lowerings []pipeline.LoweringRecord
	skipped   []SkippedEntry
	failed    []FailedEntry
	warnings  []string
}

// SkippedEntry records a skipped object and the reason.
type SkippedEntry struct {
	ObjectID string
	Reason   string
}

// FailedEntry records a failed object and the error.
type FailedEntry struct {
	ObjectID string
	Err      string
}

// NewReporter creates a new empty Reporter.
func NewReporter() *Reporter {
	return &Reporter{}
}

// ReportLowering records a lowering decision. Safe for concurrent use.
func (r *Reporter) ReportLowering(_ context.Context, record pipeline.LoweringRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lowerings = append(r.lowerings, record)
}

// ReportSkipped records that a canonical object was skipped. Safe for concurrent use.
func (r *Reporter) ReportSkipped(_ context.Context, objectID, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skipped = append(r.skipped, SkippedEntry{ObjectID: objectID, Reason: reason})
}

// ReportFailed records that a canonical object failed to compile. Safe for concurrent use.
func (r *Reporter) ReportFailed(_ context.Context, objectID string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = append(r.failed, FailedEntry{ObjectID: objectID, Err: err.Error()})
}

// ReportWarning records a non-fatal warning. Safe for concurrent use.
func (r *Reporter) ReportWarning(_ context.Context, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warnings = append(r.warnings, message)
}

// LoweringRecords returns a copy of all accumulated lowering records.
func (r *Reporter) LoweringRecords() []pipeline.LoweringRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]pipeline.LoweringRecord, len(r.lowerings))
	copy(result, r.lowerings)
	return result
}

// SkippedEntries returns a copy of all accumulated skipped entries.
func (r *Reporter) SkippedEntries() []SkippedEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]SkippedEntry, len(r.skipped))
	copy(result, r.skipped)
	return result
}

// FailedEntries returns a copy of all accumulated failed entries.
func (r *Reporter) FailedEntries() []FailedEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]FailedEntry, len(r.failed))
	copy(result, r.failed)
	return result
}

// Warnings returns a copy of all accumulated warnings.
func (r *Reporter) Warnings() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]string, len(r.warnings))
	copy(result, r.warnings)
	return result
}
