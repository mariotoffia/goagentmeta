package copilot

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderPrompts generates .github/prompts/{id}.prompt.md files from command objects.
// Each command becomes a prompt file with YAML frontmatter containing
// description, name, agent, model, and tools.
func renderPrompts(
	commands []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(commands) == 0 {
		return nil
	}

	var files []pipeline.EmittedFile

	for _, cmd := range commands {
		var content strings.Builder

		content.WriteString("---\n")

		// Name.
		cmdName := getFieldString(cmd, "name")
		if cmdName == "" {
			cmdName = cmd.OriginalID
		}
		content.WriteString(fmt.Sprintf("name: %s\n", yamlScalar(cmdName)))

		// Description.
		if desc := getFieldString(cmd, "description"); desc != "" {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(desc)))
		}

		// Agent.
		if agent := getFieldString(cmd, "agent"); agent != "" {
			content.WriteString(fmt.Sprintf("agent: %s\n", yamlScalar(agent)))
		}

		// Model.
		if model := getFieldString(cmd, "model"); model != "" {
			content.WriteString(fmt.Sprintf("model: %s\n", yamlScalar(model)))
		}

		// Tools.
		if tools := getFieldStringSlice(cmd, "tools"); len(tools) > 0 {
			content.WriteString("tools:\n")
			for _, t := range tools {
				content.WriteString(fmt.Sprintf("  - %s\n", yamlScalar(t)))
			}
		}

		content.WriteString("---\n\n")

		// Command body.
		content.WriteString(cmd.Content)

		filePath := fmt.Sprintf(".github/prompts/%s.prompt.md", sanitizeID(cmd.OriginalID))

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{cmd.OriginalID},
		})
	}

	return files
}
