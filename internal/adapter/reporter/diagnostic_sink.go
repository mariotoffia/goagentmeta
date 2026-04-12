// Package reporter provides concrete implementations of the reporter port
// interfaces: DiagnosticSink, ProvenanceRecorder, Reporter, and
// BuildReportWriter. These adapters are used by the pipeline to collect
// build events, diagnostics, provenance records, and to produce final
// human- and machine-readable build reports.
package reporter

import (
	"context"
	"sort"
	"sync"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portreporter "github.com/mariotoffia/goagentmeta/internal/port/reporter"
)

// Compile-time assertion.
var _ portreporter.DiagnosticSink = (*DiagnosticSink)(nil)

// DiagnosticSink collects compiler diagnostics in a thread-safe manner.
// Diagnostics are sorted by severity (error > warning > info), then by
// phase, then by source path when retrieved.
type DiagnosticSink struct {
	mu          sync.Mutex
	diagnostics []pipeline.Diagnostic
}

// NewDiagnosticSink creates a new empty DiagnosticSink.
func NewDiagnosticSink() *DiagnosticSink {
	return &DiagnosticSink{}
}

// Emit records a single diagnostic message. Safe for concurrent use.
func (s *DiagnosticSink) Emit(_ context.Context, d pipeline.Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diagnostics = append(s.diagnostics, d)
}

// Diagnostics returns all emitted diagnostics sorted by severity (errors
// first), then phase, then source path. Returns an empty slice (not nil)
// if no diagnostics have been emitted.
func (s *DiagnosticSink) Diagnostics() []pipeline.Diagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]pipeline.Diagnostic, len(s.diagnostics))
	copy(result, s.diagnostics)

	sort.SliceStable(result, func(i, j int) bool {
		oi, oj := severityOrder(result[i].Severity), severityOrder(result[j].Severity)
		if oi != oj {
			return oi < oj
		}
		if result[i].Phase != result[j].Phase {
			return result[i].Phase < result[j].Phase
		}
		return result[i].SourcePath < result[j].SourcePath
	})

	return result
}

// severityOrder maps severity strings to a sort order where errors come first.
func severityOrder(s string) int {
	switch s {
	case "error":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	default:
		return 3
	}
}
