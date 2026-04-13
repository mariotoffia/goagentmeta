package codex

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderAgents generates .codex/agents/{id}.md files from agent objects.
// Each agent gets frontmatter with name, description, tools, disallowedTools,
// model, skills, hooks, and mayCall delegation.
// The body is the agent's RolePrompt content.
func renderAgents(
	agents []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(agents) == 0 {
		return nil
	}

	var files []pipeline.EmittedFile

	for _, agent := range agents {
		var content strings.Builder

		content.WriteString("---\n")
		content.WriteString(fmt.Sprintf("name: %s\n", yamlScalar(agent.OriginalID)))

		if desc := getFieldString(agent, "description"); desc != "" {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(desc)))
		}

		// Tools (allowed).
		if tools := getFieldStringSlice(agent, "tools"); len(tools) > 0 {
			content.WriteString("tools:\n")
			for _, t := range tools {
				content.WriteString(fmt.Sprintf("  - %s\n", t))
			}
		}

		// Disallowed tools.
		if denied := getFieldStringSlice(agent, "disallowedTools"); len(denied) > 0 {
			content.WriteString("disallowedTools:\n")
			for _, t := range denied {
				content.WriteString(fmt.Sprintf("  - %s\n", t))
			}
		}

		if m := getFieldString(agent, "model"); m != "" {
			content.WriteString(fmt.Sprintf("model: %s\n", yamlScalar(m)))
		}

		if skills := getFieldStringSlice(agent, "skills"); len(skills) > 0 {
			content.WriteString("skills:\n")
			for _, s := range skills {
				content.WriteString(fmt.Sprintf("  - %s\n", s))
			}
		}

		if hooks := getFieldStringSlice(agent, "hooks"); len(hooks) > 0 {
			content.WriteString("hooks:\n")
			for _, h := range hooks {
				content.WriteString(fmt.Sprintf("  - %s\n", h))
			}
		}

		// Delegation → mayCall.
		if delegation, ok := agent.Fields["delegation"].(map[string]any); ok {
			if mayCall := getStringSlice(delegation, "mayCall"); len(mayCall) > 0 {
				content.WriteString("mayCall:\n")
				for _, a := range mayCall {
					content.WriteString(fmt.Sprintf("  - %s\n", a))
				}
			}
		}

		content.WriteString("---\n\n")

		content.WriteString(agent.Content)

		filePath := fmt.Sprintf(".codex/agents/%s.md", sanitizeID(agent.OriginalID))

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{agent.OriginalID},
		})
	}

	return files
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
