package cursor

import (
	"context"
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderSkills lowers skills into .cursor/rules/{skill-id}.mdc files.
// Cursor has no native skill concept, so skills with content are inlined
// into rule files. Skills without meaningful content are skipped with a
// diagnostic.
func renderSkills(
	ctx context.Context,
	skills []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(skills) == 0 {
		return nil
	}

	var files []pipeline.EmittedFile

	for _, skill := range skills {
		if skill.Content == "" {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "info",
				Code:     "RENDER_SKILL_SKIPPED",
				Message:  fmt.Sprintf("skill %q has no content — skipped (no Cursor native skill support)", skill.OriginalID),
				ObjectID: skill.OriginalID,
				Phase:    pipeline.PhaseRender,
			})
			continue
		}

		var content strings.Builder

		desc := getFieldString(skill, "description")
		hints := getFieldStringSlice(skill, "activationHints")

		content.WriteString("---\n")

		// Skills are lowered with alwaysApply: true since they don't have
		// natural glob scopes.
		content.WriteString("alwaysApply: true\n")

		// Use description or construct one from skill metadata.
		if desc != "" {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(desc)))
		} else if len(hints) > 0 {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar("Skill: "+strings.Join(hints, ", "))))
		}

		content.WriteString("---\n\n")

		// Add skill content body.
		content.WriteString(skill.Content)

		skillID := sanitizeID(skill.OriginalID)
		filePath := fmt.Sprintf(".cursor/rules/%s.mdc", skillID)

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{skill.OriginalID},
		})

		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "info",
			Code:     "RENDER_SKILL_LOWERED",
			Message:  fmt.Sprintf("skill %q lowered into Cursor rule at %s", skill.OriginalID, filePath),
			ObjectID: skill.OriginalID,
			Phase:    pipeline.PhaseRender,
		})
	}

	return files
}
