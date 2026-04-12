package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	portreporter "github.com/mariotoffia/goagentmeta/internal/port/reporter"
)

// Compile-time assertion.
var _ portreporter.BuildReportWriter = (*JSONReportWriter)(nil)

// JSONReportWriter writes a BuildReport as deterministic JSON to a file.
type JSONReportWriter struct {
	writer     portfs.Writer
	outputPath string
}

// NewJSONReportWriter creates a writer that emits report.json at the given path.
func NewJSONReportWriter(w portfs.Writer, outputPath string) *JSONReportWriter {
	return &JSONReportWriter{writer: w, outputPath: outputPath}
}

// jsonReport is the serialization structure for deterministic JSON output.
// Fields are ordered alphabetically by convention.
type jsonReport struct {
	Diagnostics []jsonDiagnostic `json:"diagnostics"`
	Duration    string           `json:"duration"`
	Timestamp   string           `json:"timestamp"`
	Units       []jsonUnit       `json:"units"`
}

type jsonUnit struct {
	EmittedFiles      []string          `json:"emitted_files"`
	LoweringRecords   []jsonLowering    `json:"lowering_records"`
	OutputDir         string            `json:"output_dir"`
	Profile           string            `json:"profile"`
	SelectedProviders map[string]string `json:"selected_providers,omitempty"`
	SkippedObjects    []string          `json:"skipped_objects"`
	Target            string            `json:"target"`
	Warnings          []string          `json:"warnings"`
}

type jsonLowering struct {
	FromKind     string `json:"from_kind"`
	ObjectID     string `json:"object_id"`
	Preservation string `json:"preservation"`
	Reason       string `json:"reason"`
	Status       string `json:"status"`
	ToKind       string `json:"to_kind"`
	ToPath       string `json:"to_path"`
}

type jsonDiagnostic struct {
	Code       string `json:"code,omitempty"`
	Message    string `json:"message"`
	ObjectID   string `json:"object_id,omitempty"`
	Phase      string `json:"phase"`
	Severity   string `json:"severity"`
	SourcePath string `json:"source_path,omitempty"`
}

// Write serializes the BuildReport as JSON and writes it to disk.
func (w *JSONReportWriter) Write(ctx context.Context, report pipeline.BuildReport) error {
	jr := jsonReport{
		Timestamp:   report.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
		Duration:    report.Duration.String(),
		Diagnostics: make([]jsonDiagnostic, 0, len(report.Diagnostics)),
		Units:       make([]jsonUnit, 0, len(report.Units)),
	}

	// Deterministic diagnostic ordering.
	diags := make([]pipeline.Diagnostic, len(report.Diagnostics))
	copy(diags, report.Diagnostics)
	sort.SliceStable(diags, func(i, j int) bool {
		oi, oj := severityOrder(diags[i].Severity), severityOrder(diags[j].Severity)
		if oi != oj {
			return oi < oj
		}
		if diags[i].Phase != diags[j].Phase {
			return diags[i].Phase < diags[j].Phase
		}
		return diags[i].SourcePath < diags[j].SourcePath
	})

	for _, d := range diags {
		jr.Diagnostics = append(jr.Diagnostics, jsonDiagnostic{
			Severity:   d.Severity,
			Code:       d.Code,
			Message:    d.Message,
			SourcePath: d.SourcePath,
			ObjectID:   d.ObjectID,
			Phase:      d.Phase.String(),
		})
	}

	// Deterministic unit ordering by output dir.
	units := make([]pipeline.UnitReport, len(report.Units))
	copy(units, report.Units)
	sort.SliceStable(units, func(i, j int) bool {
		return units[i].Coordinate.OutputDir < units[j].Coordinate.OutputDir
	})

	for _, u := range units {
		emitted := make([]string, len(u.EmittedFiles))
		copy(emitted, u.EmittedFiles)
		sort.Strings(emitted)

		skipped := make([]string, len(u.SkippedObjects))
		copy(skipped, u.SkippedObjects)
		sort.Strings(skipped)

		warnings := make([]string, len(u.Warnings))
		copy(warnings, u.Warnings)

		lowerings := make([]jsonLowering, 0, len(u.LoweringRecords))
		for _, lr := range u.LoweringRecords {
			lowerings = append(lowerings, jsonLowering{
				ObjectID:     lr.ObjectID,
				FromKind:     string(lr.FromKind),
				ToKind:       string(lr.ToKind),
				ToPath:       lr.ToPath,
				Reason:       lr.Reason,
				Preservation: string(lr.Preservation),
				Status:       lr.Status,
			})
		}

		providers := u.SelectedProviders
		if providers == nil {
			providers = make(map[string]string)
		}

		jr.Units = append(jr.Units, jsonUnit{
			Target:            string(u.Coordinate.Unit.Target),
			Profile:           string(u.Coordinate.Unit.Profile),
			OutputDir:         u.Coordinate.OutputDir,
			EmittedFiles:      emitted,
			LoweringRecords:   lowerings,
			SelectedProviders: providers,
			SkippedObjects:    skipped,
			Warnings:          warnings,
		})
	}

	data, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report JSON: %w", err)
	}

	data = append(data, '\n')

	if err := w.writer.WriteFile(ctx, w.outputPath, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", w.outputPath, err)
	}

	return nil
}
