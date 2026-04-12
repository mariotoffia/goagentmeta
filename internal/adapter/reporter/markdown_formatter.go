package reporter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	portreporter "github.com/mariotoffia/goagentmeta/internal/port/reporter"
)

// Compile-time assertion.
var _ portreporter.BuildReportWriter = (*MarkdownReportWriter)(nil)

// MarkdownReportWriter writes a BuildReport as human-readable Markdown.
type MarkdownReportWriter struct {
	writer     portfs.Writer
	outputPath string
}

// NewMarkdownReportWriter creates a writer that emits report.md at the given path.
func NewMarkdownReportWriter(w portfs.Writer, outputPath string) *MarkdownReportWriter {
	return &MarkdownReportWriter{writer: w, outputPath: outputPath}
}

// Write serializes the BuildReport as Markdown and writes it to disk.
func (w *MarkdownReportWriter) Write(ctx context.Context, report pipeline.BuildReport) error {
	var b strings.Builder

	w.writeSummary(&b, report)
	w.writeUnits(&b, report)
	w.writeLowerings(&b, report)
	w.writeSkipped(&b, report)
	w.writeDiagnostics(&b, report)

	if err := w.writer.WriteFile(ctx, w.outputPath, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("write %s: %w", w.outputPath, err)
	}

	return nil
}

func (w *MarkdownReportWriter) writeSummary(b *strings.Builder, report pipeline.BuildReport) {
	b.WriteString("# Build Report\n\n")
	b.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", report.Timestamp.UTC().Format("2006-01-02T15:04:05Z")))
	b.WriteString(fmt.Sprintf("**Duration:** %s\n\n", report.Duration))

	totalFiles := 0
	totalLowerings := 0
	totalSkipped := 0
	totalWarnings := 0
	for _, u := range report.Units {
		totalFiles += len(u.EmittedFiles)
		totalLowerings += len(u.LoweringRecords)
		totalSkipped += len(u.SkippedObjects)
		totalWarnings += len(u.Warnings)
	}

	errors := 0
	warnings := 0
	for _, d := range report.Diagnostics {
		switch d.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		}
	}

	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("| Metric | Count |\n"))
	b.WriteString(fmt.Sprintf("|---|---|\n"))
	b.WriteString(fmt.Sprintf("| Build Units | %d |\n", len(report.Units)))
	b.WriteString(fmt.Sprintf("| Emitted Files | %d |\n", totalFiles))
	b.WriteString(fmt.Sprintf("| Lowerings | %d |\n", totalLowerings))
	b.WriteString(fmt.Sprintf("| Skipped Objects | %d |\n", totalSkipped))
	b.WriteString(fmt.Sprintf("| Warnings | %d |\n", totalWarnings))
	b.WriteString(fmt.Sprintf("| Errors | %d |\n", errors))
	b.WriteString(fmt.Sprintf("| Diagnostics (warnings) | %d |\n", warnings))
	b.WriteString("\n")
}

func (w *MarkdownReportWriter) writeUnits(b *strings.Builder, report pipeline.BuildReport) {
	if len(report.Units) == 0 {
		return
	}

	b.WriteString("## Per-Unit Results\n\n")

	units := make([]pipeline.UnitReport, len(report.Units))
	copy(units, report.Units)
	sort.SliceStable(units, func(i, j int) bool {
		return units[i].Coordinate.OutputDir < units[j].Coordinate.OutputDir
	})

	for _, u := range units {
		b.WriteString(fmt.Sprintf("### %s / %s\n\n",
			u.Coordinate.Unit.Target, u.Coordinate.Unit.Profile))
		b.WriteString(fmt.Sprintf("**Output:** `%s`\n\n", u.Coordinate.OutputDir))

		b.WriteString(fmt.Sprintf("- Files emitted: %d\n", len(u.EmittedFiles)))
		b.WriteString(fmt.Sprintf("- Lowerings: %d\n", len(u.LoweringRecords)))
		b.WriteString(fmt.Sprintf("- Skipped: %d\n", len(u.SkippedObjects)))
		b.WriteString(fmt.Sprintf("- Warnings: %d\n", len(u.Warnings)))

		if len(u.EmittedFiles) > 0 {
			b.WriteString("\n**Emitted files:**\n\n")
			sorted := make([]string, len(u.EmittedFiles))
			copy(sorted, u.EmittedFiles)
			sort.Strings(sorted)
			for _, f := range sorted {
				b.WriteString(fmt.Sprintf("- `%s`\n", f))
			}
		}
		b.WriteString("\n")
	}
}

func (w *MarkdownReportWriter) writeLowerings(b *strings.Builder, report pipeline.BuildReport) {
	var allLowerings []pipeline.LoweringRecord
	for _, u := range report.Units {
		allLowerings = append(allLowerings, u.LoweringRecords...)
	}

	if len(allLowerings) == 0 {
		return
	}

	b.WriteString("## Lowerings\n\n")
	b.WriteString("| Object | From | To | Reason | Preservation | Status |\n")
	b.WriteString("|---|---|---|---|---|---|\n")

	for _, lr := range allLowerings {
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s |\n",
			lr.ObjectID,
			lr.FromKind,
			lr.ToKind,
			lr.Reason,
			lr.Preservation,
			lr.Status,
		))
	}
	b.WriteString("\n")
}

func (w *MarkdownReportWriter) writeSkipped(b *strings.Builder, report pipeline.BuildReport) {
	var allSkipped []string
	for _, u := range report.Units {
		allSkipped = append(allSkipped, u.SkippedObjects...)
	}

	if len(allSkipped) == 0 {
		return
	}

	b.WriteString("## Skipped Objects\n\n")
	for _, s := range allSkipped {
		b.WriteString(fmt.Sprintf("- `%s`\n", s))
	}
	b.WriteString("\n")
}

func (w *MarkdownReportWriter) writeDiagnostics(b *strings.Builder, report pipeline.BuildReport) {
	if len(report.Diagnostics) == 0 {
		return
	}

	b.WriteString("## Diagnostics\n\n")
	b.WriteString("| Severity | Phase | Message | Source |\n")
	b.WriteString("|---|---|---|---|\n")

	diags := make([]pipeline.Diagnostic, len(report.Diagnostics))
	copy(diags, report.Diagnostics)
	sort.SliceStable(diags, func(i, j int) bool {
		oi, oj := severityOrder(diags[i].Severity), severityOrder(diags[j].Severity)
		if oi != oj {
			return oi < oj
		}
		return diags[i].Phase < diags[j].Phase
	})

	for _, d := range diags {
		icon := diagnosticIcon(d.Severity)
		source := d.SourcePath
		if d.ObjectID != "" {
			source = d.ObjectID
		}
		b.WriteString(fmt.Sprintf("| %s %s | %s | %s | `%s` |\n",
			icon, d.Severity, d.Phase, d.Message, source))
	}
	b.WriteString("\n")
}

func diagnosticIcon(severity string) string {
	switch severity {
	case "error":
		return "❌"
	case "warning":
		return "⚠️"
	case "info":
		return "ℹ️"
	default:
		return "•"
	}
}
