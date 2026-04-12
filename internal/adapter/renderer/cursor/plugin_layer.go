package cursor

import (
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderPlugins generates Cursor Marketplace references from plugin objects.
// Inline plugins are not supported in Cursor's plugin model, so only
// registry plugins emit install metadata entries. External plugins are
// handled by the MCP layer.
func renderPlugins(
	plugins []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) ([]pipeline.EmittedFile, []pipeline.InstallEntry) {
	if len(plugins) == 0 {
		return nil, nil
	}

	var files []pipeline.EmittedFile
	var installEntries []pipeline.InstallEntry

	for _, plugin := range plugins {
		distMode := pluginDistMode(plugin)

		switch distMode {
		case "registry":
			entry := renderRegistryPluginEntry(plugin)
			if entry.PluginID != "" {
				installEntries = append(installEntries, entry)
			}

		case "inline", "external":
			// Inline: not natively supported by Cursor; handled via MCP if applicable.
			// External: handled by the MCP layer.
		}
	}

	return files, installEntries
}

// pluginDistMode extracts the distribution mode from a plugin.
func pluginDistMode(obj pipeline.LoweredObject) string {
	if obj.Fields != nil {
		if dist, ok := obj.Fields["distribution"].(map[string]any); ok {
			return getString(dist, "mode")
		}
	}
	return ""
}

// renderRegistryPluginEntry creates an install entry for a Cursor Marketplace plugin.
func renderRegistryPluginEntry(plugin pipeline.LoweredObject) pipeline.InstallEntry {
	var ref, version string

	if plugin.Fields != nil {
		if dist, ok := plugin.Fields["distribution"].(map[string]any); ok {
			ref = getString(dist, "ref")
			version = getString(dist, "version")
		}
	}

	return pipeline.InstallEntry{
		PluginID: plugin.OriginalID,
		Format:   "cursor-marketplace",
		Config: map[string]any{
			"ref":     ref,
			"version": version,
		},
	}
}
