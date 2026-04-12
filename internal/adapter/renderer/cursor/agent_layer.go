package cursor

import (
	"context"
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderAgents lowers agents into .cursor/rules/{agent-id}.mdc files.
// Cursor has no native agent/sub-agent concept, so agents with a
// rolePrompt are merged into rule files. Agents with complex
// delegation/handoff config are skipped with a diagnostic.
func renderAgents(
	ctx context.Context,
	agents []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(agents) == 0 {
		return nil
	}

	var files []pipeline.EmittedFile

	for _, agent := range agents {
		// Skip agents with complex delegation that can't be meaningfully lowered.
		if hasComplexDelegation(agent) {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "warning",
				Code:     "RENDER_AGENT_SKIPPED",
				Message:  fmt.Sprintf("agent %q has complex delegation/handoff config — skipped (no Cursor native agent support)", agent.OriginalID),
				ObjectID: agent.OriginalID,
				Phase:    pipeline.PhaseRender,
			})
			continue
		}

		if agent.Content == "" {
			emitDiagnostic(ctx, pipeline.Diagnostic{
				Severity: "info",
				Code:     "RENDER_AGENT_SKIPPED",
				Message:  fmt.Sprintf("agent %q has no rolePrompt content — skipped", agent.OriginalID),
				ObjectID: agent.OriginalID,
				Phase:    pipeline.PhaseRender,
			})
			continue
		}

		var content strings.Builder

		desc := getFieldString(agent, "description")

		content.WriteString("---\n")
		content.WriteString("alwaysApply: true\n")

		if desc != "" {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(desc)))
		}

		content.WriteString("---\n\n")

		// Merge agent rolePrompt as rule content.
		content.WriteString(agent.Content)

		agentID := sanitizeID(agent.OriginalID)
		filePath := fmt.Sprintf(".cursor/rules/%s.mdc", agentID)

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{agent.OriginalID},
		})

		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "info",
			Code:     "RENDER_AGENT_LOWERED",
			Message:  fmt.Sprintf("agent %q rolePrompt lowered into Cursor rule at %s", agent.OriginalID, filePath),
			ObjectID: agent.OriginalID,
			Phase:    pipeline.PhaseRender,
		})
	}

	return files
}

// hasComplexDelegation returns true if an agent has delegation with
// handoffs that can't be meaningfully represented as a Cursor rule.
func hasComplexDelegation(agent pipeline.LoweredObject) bool {
	if agent.Fields == nil {
		return false
	}

	if handoffs, ok := agent.Fields["handoffs"].([]any); ok && len(handoffs) > 0 {
		return true
	}

	return false
}
