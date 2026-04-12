package reporter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/capability"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
)

// SupportMatrixWriter generates a support-matrix.md showing capability
// support levels across all targets.
type SupportMatrixWriter struct {
	writer     portfs.Writer
	outputPath string
}

// NewSupportMatrixWriter creates a writer that emits support-matrix.md.
func NewSupportMatrixWriter(w portfs.Writer, outputPath string) *SupportMatrixWriter {
	return &SupportMatrixWriter{writer: w, outputPath: outputPath}
}

// Write generates the support matrix markdown from the given registries.
func (w *SupportMatrixWriter) Write(registries []capability.CapabilityRegistry) error {
	var b strings.Builder

	b.WriteString("# Support Matrix\n\n")
	b.WriteString("Capability support levels per target.\n\n")

	// Collect all unique capability surface keys across all registries.
	surfaceSet := make(map[string]struct{})
	for _, reg := range registries {
		for surface := range reg.Surfaces {
			surfaceSet[surface] = struct{}{}
		}
	}

	surfaces := make([]string, 0, len(surfaceSet))
	for s := range surfaceSet {
		surfaces = append(surfaces, s)
	}
	sort.Strings(surfaces)

	if len(surfaces) == 0 {
		b.WriteString("_No capabilities registered._\n")
		return w.write(b.String())
	}

	// Build target list from all known targets.
	targets := build.AllTargets()

	// Build registry lookup by target.
	regByTarget := make(map[string]capability.CapabilityRegistry)
	for _, reg := range registries {
		regByTarget[reg.Target] = reg
	}

	// Header row.
	b.WriteString("| Capability |")
	for _, t := range targets {
		b.WriteString(fmt.Sprintf(" %s |", t))
	}
	b.WriteString("\n")

	// Separator.
	b.WriteString("|---|")
	for range targets {
		b.WriteString("---|")
	}
	b.WriteString("\n")

	// Data rows.
	for _, surface := range surfaces {
		b.WriteString(fmt.Sprintf("| `%s` |", surface))
		for _, t := range targets {
			reg, ok := regByTarget[string(t)]
			if !ok {
				b.WriteString(" — |")
				continue
			}
			level, ok := reg.Surfaces[surface]
			if !ok {
				b.WriteString(" — |")
				continue
			}
			b.WriteString(fmt.Sprintf(" %s %s |", supportIcon(level), level))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n### Legend\n\n")
	b.WriteString("| Icon | Level | Meaning |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| ✅ | native | First-class support |\n")
	b.WriteString("| 🔄 | adapted | Same semantics, different syntax |\n")
	b.WriteString("| ⬇️ | lowered | Mapped to another primitive |\n")
	b.WriteString("| 🔁 | emulated | Approximation only |\n")
	b.WriteString("| ⛔ | skipped | Not available |\n")

	return w.write(b.String())
}

func (w *SupportMatrixWriter) write(content string) error {
	if err := w.writer.WriteFile(context.Background(), w.outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", w.outputPath, err)
	}
	return nil
}

func supportIcon(level capability.SupportLevel) string {
	switch level {
	case capability.SupportNative:
		return "✅"
	case capability.SupportAdapted:
		return "🔄"
	case capability.SupportLowered:
		return "⬇️"
	case capability.SupportEmulated:
		return "🔁"
	case capability.SupportSkipped:
		return "⛔"
	default:
		return "•"
	}
}
