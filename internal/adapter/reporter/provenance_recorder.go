package reporter

import (
	"context"
	"sync"

	portreporter "github.com/mariotoffia/goagentmeta/internal/port/reporter"
)

// Compile-time assertion.
var _ portreporter.ProvenanceRecorder = (*ProvenanceRecorder)(nil)

// ProvenanceEntry links a source canonical object to an output file with
// the lowering chain that was applied.
type ProvenanceEntry struct {
	// SourceObjectID is the canonical object that contributed to the output.
	SourceObjectID string
	// OutputPath is the generated file path.
	OutputPath string
	// Chain lists the lowering operations applied (e.g., ["skill→instruction"]).
	Chain []string
}

// ProvenanceRecorder tracks source-to-output provenance. Thread-safe.
type ProvenanceRecorder struct {
	mu      sync.Mutex
	entries []ProvenanceEntry
	bySource map[string][]ProvenanceEntry
}

// NewProvenanceRecorder creates a new empty ProvenanceRecorder.
func NewProvenanceRecorder() *ProvenanceRecorder {
	return &ProvenanceRecorder{
		bySource: make(map[string][]ProvenanceEntry),
	}
}

// Record adds a provenance entry. Safe for concurrent use.
func (r *ProvenanceRecorder) Record(_ context.Context, sourceObjectID, outputPath string, chain []string) {
	chainCopy := make([]string, len(chain))
	copy(chainCopy, chain)

	entry := ProvenanceEntry{
		SourceObjectID: sourceObjectID,
		OutputPath:     outputPath,
		Chain:          chainCopy,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)
	r.bySource[sourceObjectID] = append(r.bySource[sourceObjectID], entry)
}

// Entries returns all provenance entries in recording order.
func (r *ProvenanceRecorder) Entries() []ProvenanceEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]ProvenanceEntry, len(r.entries))
	copy(result, r.entries)
	return result
}

// EntriesForSource returns all entries for a specific source object ID.
func (r *ProvenanceRecorder) EntriesForSource(sourceObjectID string) []ProvenanceEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := r.bySource[sourceObjectID]
	result := make([]ProvenanceEntry, len(entries))
	copy(result, entries)
	return result
}
