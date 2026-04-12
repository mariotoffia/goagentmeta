package codex

import (
	"encoding/json"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderMCP generates .mcp.json containing the mcpServers configuration.
// It collects MCP server bindings from plugin objects that have MCP-related
// distribution or capability references. The top-level key is "mcpServers"
// per Codex CLI convention (same as Claude Code).
func renderMCP(
	plugins []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	servers := make(map[string]mcpServerEntry)
	serverOwner := make(map[string]string)
	var sourceObjects []string

	for _, plugin := range plugins {
		entries := extractMCPServers(plugin, objects)
		for name, entry := range entries {
			if _, exists := serverOwner[name]; exists {
				continue
			}
			servers[name] = entry
			serverOwner[name] = plugin.OriginalID
			sourceObjects = append(sourceObjects, plugin.OriginalID)
		}
	}

	if len(servers) == 0 {
		return nil
	}

	mcpConfig := mcpJSON{
		MCPServers: servers,
	}

	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return nil
	}

	data = append(data, '\n')

	sourceObjects = dedup(sourceObjects)
	sort.Strings(sourceObjects)

	return []pipeline.EmittedFile{
		{
			Path:          ".mcp.json",
			Content:       data,
			Layer:         pipeline.LayerExtension,
			SourceObjects: sourceObjects,
		},
	}
}

// mcpJSON is the structure of .mcp.json.
type mcpJSON struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

// mcpServerEntry describes a single MCP server configuration.
type mcpServerEntry struct {
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

// extractMCPServers extracts MCP server entries from a plugin's fields.
func extractMCPServers(
	plugin pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) map[string]mcpServerEntry {
	result := make(map[string]mcpServerEntry)

	if plugin.Fields == nil {
		return result
	}

	if mcpServers, ok := plugin.Fields["mcpServers"].(map[string]any); ok {
		for name, serverRaw := range mcpServers {
			if serverMap, ok := serverRaw.(map[string]any); ok {
				entry := mcpServerEntry{
					Transport: getOrDefault(getString(serverMap, "transport"), "stdio"),
					Command:   getString(serverMap, "command"),
					URL:       getString(serverMap, "url"),
				}
				if args := getStringSlice(serverMap, "args"); len(args) > 0 {
					entry.Args = args
				}
				if env, ok := serverMap["env"].(map[string]any); ok {
					entry.Env = make(map[string]string)
					envKeys := sortedMapKeys(env)
					for _, k := range envKeys {
						if v, ok := env[k].(string); ok {
							entry.Env[k] = v
						}
					}
				}
				result[name] = entry
			}
		}
	}

	if dist, ok := plugin.Fields["distribution"].(map[string]any); ok {
		mode := getString(dist, "mode")
		if mode == "external" {
			ref := getString(dist, "ref")
			if ref != "" {
				serverName := sanitizeID(plugin.OriginalID)
				if _, exists := result[serverName]; !exists {
					result[serverName] = mcpServerEntry{
						Transport: "stdio",
						Command:   ref,
					}
				}
			}
		}
	}

	return result
}

// getOrDefault returns the value if non-empty, otherwise the default.
func getOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// sortedMapKeys returns sorted keys of a map[string]any.
func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dedup removes duplicate strings while preserving order.
func dedup(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
