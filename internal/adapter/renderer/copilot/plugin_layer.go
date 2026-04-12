package copilot

import (
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderPlugins generates .claude-plugin/ packaging for inline plugins
// and emits MCP reference entries for external plugins. Reuses Claude plugin
// format since Copilot reads the shared .claude-plugin/ structure.
func renderPlugins(
	plugins []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) ([]pipeline.EmittedFile, []pipeline.EmittedPlugin, []pipeline.InstallEntry) {
	if len(plugins) == 0 {
		return nil, nil, nil
	}

	var files []pipeline.EmittedFile
	var bundles []pipeline.EmittedPlugin
	var installEntries []pipeline.InstallEntry

	for _, plugin := range plugins {
		distMode := pluginDistMode(plugin)

		switch distMode {
		case "inline":
			bundle := renderInlinePlugin(plugin, objects)
			if len(bundle.Files) > 0 {
				bundles = append(bundles, bundle)
			}

		case "external":
			// External plugins are handled by the MCP layer.

		case "registry":
			entry := renderRegistryPluginEntry(plugin)
			if entry.PluginID != "" {
				installEntries = append(installEntries, entry)
			}
		}
	}

	return files, bundles, installEntries
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

// renderInlinePlugin packages an inline plugin's artifacts into .claude-plugin/.
func renderInlinePlugin(
	plugin pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) pipeline.EmittedPlugin {
	pluginID := sanitizeID(plugin.OriginalID)
	destDir := ".claude-plugin/" + pluginID

	var pluginFiles []pipeline.EmittedFile

	if plugin.Fields != nil {
		if artifacts, ok := plugin.Fields["artifacts"].(map[string]any); ok {
			for _, script := range getStringSlice(artifacts, "scripts") {
				pluginFiles = append(pluginFiles, pipeline.EmittedFile{
					Path:          destDir + "/" + lastSegment(script),
					Content:       []byte(""),
					Layer:         pipeline.LayerExtension,
					SourceObjects: []string{plugin.OriginalID},
				})
			}

			for _, config := range getStringSlice(artifacts, "configs") {
				pluginFiles = append(pluginFiles, pipeline.EmittedFile{
					Path:          destDir + "/" + lastSegment(config),
					Content:       []byte(""),
					Layer:         pipeline.LayerExtension,
					SourceObjects: []string{plugin.OriginalID},
				})
			}

			for _, manifest := range getStringSlice(artifacts, "manifests") {
				pluginFiles = append(pluginFiles, pipeline.EmittedFile{
					Path:          destDir + "/" + lastSegment(manifest),
					Content:       []byte(""),
					Layer:         pipeline.LayerExtension,
					SourceObjects: []string{plugin.OriginalID},
				})
			}
		}
	}

	return pipeline.EmittedPlugin{
		PluginID: plugin.OriginalID,
		DestDir:  destDir,
		Files:    pluginFiles,
	}
}

// renderRegistryPluginEntry creates an install entry for a registry plugin.
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
		Format:   "registry-ref",
		Config: map[string]any{
			"ref":     ref,
			"version": version,
		},
	}
}
