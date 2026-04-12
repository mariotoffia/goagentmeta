package copilot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderAgents generates .github/agents/{id}.agent.md files from agent objects.
// Each agent gets frontmatter with name, description, tools, agents (subagent list),
// model, mcp-servers, handoffs, and hooks. Copilot supports handoffs natively.
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

		// Tool policy: allow/deny → tools and disallowedTools.
		if toolPolicy, ok := agent.Fields["toolPolicy"].(map[string]any); ok {
			var allowed, denied []string
			tpKeys := make([]string, 0, len(toolPolicy))
			for k := range toolPolicy {
				tpKeys = append(tpKeys, k)
			}
			sort.Strings(tpKeys)

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

		if m := getFieldString(agent, "model"); m != "" {
			content.WriteString(fmt.Sprintf("model: %s\n", yamlScalar(m)))
		}

		// Subagent delegation → agents list (Copilot uses "agents" for sub-agents).
		if delegation, ok := agent.Fields["delegation"].(map[string]any); ok {
			if mayCall := getStringSlice(delegation, "mayCall"); len(mayCall) > 0 {
				content.WriteString("agents:\n")
				for _, a := range mayCall {
					content.WriteString(fmt.Sprintf("  - %s\n", a))
				}
			}
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

		// MCP servers.
		if mcpServers := getFieldStringSlice(agent, "mcpServers"); len(mcpServers) > 0 {
			content.WriteString("mcp-servers:\n")
			for _, s := range mcpServers {
				content.WriteString(fmt.Sprintf("  - %s\n", s))
			}
		}

		// Handoffs (Copilot-native feature).
		if handoffs, ok := agent.Fields["handoffs"].([]any); ok && len(handoffs) > 0 {
			content.WriteString("handoffs:\n")
			for _, h := range handoffs {
				if hm, ok := h.(map[string]any); ok {
					content.WriteString(fmt.Sprintf("  - label: %s\n", yamlScalar(getString(hm, "label"))))
					if ag := getString(hm, "agent"); ag != "" {
						content.WriteString(fmt.Sprintf("    agent: %s\n", yamlScalar(ag)))
					}
					if prompt := getString(hm, "prompt"); prompt != "" {
						content.WriteString(fmt.Sprintf("    prompt: %s\n", yamlScalar(prompt)))
					}
					if send := getString(hm, "send"); send != "" {
						content.WriteString(fmt.Sprintf("    send: %s\n", yamlScalar(send)))
					}
					if model := getString(hm, "model"); model != "" {
						content.WriteString(fmt.Sprintf("    model: %s\n", yamlScalar(model)))
					}
				}
			}
		}

		content.WriteString("---\n\n")
		content.WriteString(agent.Content)

		// Copilot uses .agent.md extension.
		filePath := fmt.Sprintf(".github/agents/%s.agent.md", sanitizeID(agent.OriginalID))

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{agent.OriginalID},
		})
	}

	return files
}
