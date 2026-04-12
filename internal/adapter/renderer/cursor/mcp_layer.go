package cursor

import (
	"encoding/json"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderMCP generates .cursor/mcp.json containing the mcpServers configuration.
// The top-level key is "mcpServers" per Cursor convention.
//
// If two plugins define the same server name, the first one wins
// (deterministic: plugins are pre-sorted by ID).
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
			Path:          ".cursor/mcp.json",
			Content:       data,
			Layer:         pipeline.LayerExtension,
			SourceObjects: sourceObjects,
		},
	}
}

// mcpJSON is the structure of .cursor/mcp.json.
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
