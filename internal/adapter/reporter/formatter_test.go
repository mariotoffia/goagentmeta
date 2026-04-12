package reporter_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/adapter/reporter"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

func testReport() pipeline.BuildReport {
	return pipeline.BuildReport{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Duration:  2*time.Second + 345*time.Millisecond,
		Units: []pipeline.UnitReport{
			{
				Coordinate: build.BuildCoordinate{
					Unit:      build.BuildUnit{Target: build.TargetClaude, Profile: build.ProfileLocalDev},
					OutputDir: ".ai-build/claude/local-dev",
				},
				EmittedFiles: []string{"CLAUDE.md", ".claude/settings.json"},
				LoweringRecords: []pipeline.LoweringRecord{
					{
						ObjectID:     "skill-1",
						FromKind:     model.KindSkill,
						ToKind:       model.KindInstruction,
						Reason:       "target does not support skills natively",
						Preservation: model.PreservationPreferred,
						Status:       "lowered",
					},
				},
				SkippedObjects: []string{"hook-2"},
				Warnings:       []string{"hook event not supported"},
			},
		},
		Diagnostics: []pipeline.Diagnostic{
			{Severity: "warning", Message: "deprecated syntax", Phase: pipeline.PhaseParse, SourcePath: "agents.yaml"},
			{Severity: "error", Message: "missing field", Phase: pipeline.PhaseValidate, SourcePath: "skills.yaml"},
		},
	}
}

// ---------------------------------------------------------------------------
// JSON Report Writer
// ---------------------------------------------------------------------------

func TestJSONReportWriter_Write(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewJSONReportWriter(mem, ".ai-build/report.json")

	if err := w.Write(context.Background(), testReport()); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := mem.ReadFile(context.Background(), ".ai-build/report.json")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Must be valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Check key fields exist.
	if _, ok := parsed["timestamp"]; !ok {
		t.Error("missing timestamp field")
	}
	if _, ok := parsed["duration"]; !ok {
		t.Error("missing duration field")
	}
	if _, ok := parsed["units"]; !ok {
		t.Error("missing units field")
	}
	if _, ok := parsed["diagnostics"]; !ok {
		t.Error("missing diagnostics field")
	}
}

func TestJSONReportWriter_Deterministic(t *testing.T) {
	mem1 := filesystem.NewMemFS()
	mem2 := filesystem.NewMemFS()
	w1 := reporter.NewJSONReportWriter(mem1, "report.json")
	w2 := reporter.NewJSONReportWriter(mem2, "report.json")

	report := testReport()

	if err := w1.Write(context.Background(), report); err != nil {
		t.Fatal(err)
	}
	if err := w2.Write(context.Background(), report); err != nil {
		t.Fatal(err)
	}

	data1, _ := mem1.ReadFile(context.Background(), "report.json")
	data2, _ := mem2.ReadFile(context.Background(), "report.json")

	if string(data1) != string(data2) {
		t.Error("JSON output is not deterministic")
	}
}

func TestJSONReportWriter_EmptyReport(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewJSONReportWriter(mem, "report.json")

	if err := w.Write(context.Background(), pipeline.BuildReport{
		Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	data, _ := mem.ReadFile(context.Background(), "report.json")
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestJSONReportWriter_DiagnosticsSortedByErrorFirst(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewJSONReportWriter(mem, "report.json")

	if err := w.Write(context.Background(), testReport()); err != nil {
		t.Fatal(err)
	}

	data, _ := mem.ReadFile(context.Background(), "report.json")
	var parsed struct {
		Diagnostics []struct {
			Severity string `json:"severity"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if len(parsed.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(parsed.Diagnostics))
	}
	if parsed.Diagnostics[0].Severity != "error" {
		t.Errorf("first diagnostic should be error, got %s", parsed.Diagnostics[0].Severity)
	}
}

// ---------------------------------------------------------------------------
// Markdown Report Writer
// ---------------------------------------------------------------------------

func TestMarkdownReportWriter_Write(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewMarkdownReportWriter(mem, ".ai-build/report.md")

	if err := w.Write(context.Background(), testReport()); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := mem.ReadFile(context.Background(), ".ai-build/report.md")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)

	requiredSections := []string{
		"# Build Report",
		"## Summary",
		"## Per-Unit Results",
		"## Lowerings",
		"## Skipped Objects",
		"## Diagnostics",
	}
	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("missing section: %s", section)
		}
	}
}

func TestMarkdownReportWriter_ContainsLoweringTable(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewMarkdownReportWriter(mem, "report.md")

	if err := w.Write(context.Background(), testReport()); err != nil {
		t.Fatal(err)
	}

	data, _ := mem.ReadFile(context.Background(), "report.md")
	content := string(data)

	if !strings.Contains(content, "skill-1") {
		t.Error("lowering table should contain object ID")
	}
	if !strings.Contains(content, "skill") {
		t.Error("lowering table should contain FromKind")
	}
	if !strings.Contains(content, "instruction") {
		t.Error("lowering table should contain ToKind")
	}
}

func TestMarkdownReportWriter_EmptyReport(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewMarkdownReportWriter(mem, "report.md")

	if err := w.Write(context.Background(), pipeline.BuildReport{
		Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	data, _ := mem.ReadFile(context.Background(), "report.md")
	content := string(data)

	if !strings.Contains(content, "# Build Report") {
		t.Error("should contain header even for empty report")
	}
	// Empty report should not contain unit/lowering/skipped sections.
	if strings.Contains(content, "## Per-Unit Results") {
		t.Error("empty report should not have Per-Unit Results section")
	}
}

// ---------------------------------------------------------------------------
// Support Matrix Writer
// ---------------------------------------------------------------------------

func TestSupportMatrixWriter_Write(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewSupportMatrixWriter(mem, "support-matrix.md")

	registries := []capability.CapabilityRegistry{
		{
			Target: "claude",
			Surfaces: map[string]capability.SupportLevel{
				"instructions.layeredFiles":  capability.SupportNative,
				"skills.bundledSkills":       capability.SupportNative,
				"hooks.lifecycleHooks":       capability.SupportAdapted,
				"commands.slashCommands":     capability.SupportSkipped,
			},
		},
		{
			Target: "copilot",
			Surfaces: map[string]capability.SupportLevel{
				"instructions.layeredFiles":  capability.SupportNative,
				"skills.bundledSkills":       capability.SupportLowered,
				"hooks.lifecycleHooks":       capability.SupportSkipped,
				"commands.slashCommands":     capability.SupportNative,
			},
		},
	}

	if err := w.Write(registries); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := mem.ReadFile(context.Background(), "support-matrix.md")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "# Support Matrix") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "claude") {
		t.Error("should contain claude target")
	}
	if !strings.Contains(content, "copilot") {
		t.Error("should contain copilot target")
	}
	if !strings.Contains(content, "✅") {
		t.Error("should contain native icon")
	}
	if !strings.Contains(content, "⛔") {
		t.Error("should contain skipped icon")
	}
	if !strings.Contains(content, "### Legend") {
		t.Error("should contain legend")
	}
}

func TestSupportMatrixWriter_EmptyRegistries(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewSupportMatrixWriter(mem, "support-matrix.md")

	if err := w.Write(nil); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, _ := mem.ReadFile(context.Background(), "support-matrix.md")
	content := string(data)

	if !strings.Contains(content, "No capabilities registered") {
		t.Error("should indicate no capabilities")
	}
}

func TestSupportMatrixWriter_AllTargetsShown(t *testing.T) {
	mem := filesystem.NewMemFS()
	w := reporter.NewSupportMatrixWriter(mem, "support-matrix.md")

	registries := []capability.CapabilityRegistry{
		{Target: "claude", Surfaces: map[string]capability.SupportLevel{"cap.1": capability.SupportNative}},
	}

	if err := w.Write(registries); err != nil {
		t.Fatal(err)
	}

	data, _ := mem.ReadFile(context.Background(), "support-matrix.md")
	content := string(data)

	// All four targets should appear in the header row.
	for _, target := range build.AllTargets() {
		if !strings.Contains(content, string(target)) {
			t.Errorf("missing target %s in matrix", target)
		}
	}
}
