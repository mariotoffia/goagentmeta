package claude

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderAgents generates .claude/agents/{id}.md files from agent objects.
// Each agent gets frontmatter with name, description, tools, disallowedTools,
// model, permissionMode, skills, mcpServers, hooks, and mayCall delegation.
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

		// Generate YAML frontmatter.
		content.WriteString("---\n")
		content.WriteString(fmt.Sprintf("name: %s\n", yamlScalar(agent.OriginalID)))

		if desc := getFieldString(agent, "description"); desc != "" {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(desc)))
		}

		// Tool policy: allow/deny/ask → tools and disallowedTools.
		if toolPolicy, ok := agent.Fields["toolPolicy"].(map[string]any); ok {
			var allowed, denied []string
			// Sort keys for determinism.
			tpKeys := make([]string, 0, len(toolPolicy))
			for k := range toolPolicy {
				tpKeys = append(tpKeys, k)
			}
			sortStrings(tpKeys)

			for _, tool := range tpKeys {
				if decision, ok := toolPolicy[tool].(string); ok {
					switch decision {
					case "allow":
						allowed = append(allowed, tool)
					case "deny":
						denied = append(denied, tool)
					}
				}
			}
			if len(allowed) > 0 {
				content.WriteString("tools:\n")
				for _, t := range allowed {
					content.WriteString(fmt.Sprintf("  - %s\n", t))
				}
			}
			if len(denied) > 0 {
				content.WriteString("disallowedTools:\n")
				for _, t := range denied {
					content.WriteString(fmt.Sprintf("  - %s\n", t))
				}
			}
		}

		// Model.
		if m := getFieldString(agent, "model"); m != "" {
			content.WriteString(fmt.Sprintf("model: %s\n", yamlScalar(m)))
		}

		// Skills.
		if skills := getFieldStringSlice(agent, "skills"); len(skills) > 0 {
			content.WriteString("skills:\n")
			for _, s := range skills {
				content.WriteString(fmt.Sprintf("  - %s\n", s))
			}
		}

		// Hooks.
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

		// Agent body: the RolePrompt content.
		content.WriteString(agent.Content)

		filePath := fmt.Sprintf(".claude/agents/%s.md", sanitizeID(agent.OriginalID))

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
