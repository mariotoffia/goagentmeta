package copilot

import (
	"encoding/json"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderHooks generates both .github/hooks/{event}.json per event type (Copilot
// native) AND .claude/settings.json (compatibility layer). Copilot only supports
// "command" hook types. Matcher values are OMITTED in the Copilot hook output
// since Copilot ignores them.
func renderHooks(hooks []pipeline.LoweredObject) []pipeline.EmittedFile {
	if len(hooks) == 0 {
		return nil
	}

	// Build hooks map: event → []hookEntry.
	hooksMap := make(map[string][]hookEntry)
	var sourceObjects []string

	for _, hook := range hooks {
		event := hookEvent(hook)
		if event == "" {
			continue
		}

		entry := hookEntry{
			Type:    hookType(hook),
			Command: hookCommand(hook),
		}

		hooksMap[event] = append(hooksMap[event], entry)
		sourceObjects = append(sourceObjects, hook.OriginalID)
	}

	if len(hooksMap) == 0 {
		return nil
	}

	// Sort entries within each event group for determinism.
	for event := range hooksMap {
		entries := hooksMap[event]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Type != entries[j].Type {
				return entries[i].Type < entries[j].Type
			}
			return entries[i].Command < entries[j].Command
		})
	}

	sort.Strings(sourceObjects)
	sourceObjects = dedup(sourceObjects)

	var files []pipeline.EmittedFile

	// Emit .github/hooks/{event}.json for each event (Copilot native format).
	// Matcher values are omitted — Copilot runs hooks on all matching events.
	events := sortedMapKeys(hooksMap)
	for _, event := range events {
		entries := hooksMap[event]

		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			continue
		}
		data = append(data, '\n')

		files = append(files, pipeline.EmittedFile{
			Path:          ".github/hooks/" + event + ".json",
			Content:       data,
			Layer:         pipeline.LayerExtension,
			SourceObjects: sourceObjects,
		})
	}

	// Also emit .claude/settings.json for compatibility (Copilot reads Claude format).
	// This includes matcher values for Claude compat.
	compatMap := make(map[string][]compatHookEntry)
	for _, hook := range hooks {
		event := hookEvent(hook)
		if event == "" {
			continue
		}
		entry := compatHookEntry{
			Type:    hookType(hook),
			Command: hookCommand(hook),
		}
		if matcher := hookMatcher(hook); matcher != "" {
			entry.Matcher = matcher
		}
		compatMap[event] = append(compatMap[event], entry)
	}

	// Sort compat entries for determinism.
	for event := range compatMap {
		entries := compatMap[event]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Type != entries[j].Type {
				return entries[i].Type < entries[j].Type
			}
			return entries[i].Command < entries[j].Command
		})
	}

	compatSettings := settingsJSON{
		Hooks: compatMap,
	}

	compatData, err := json.MarshalIndent(compatSettings, "", "  ")
	if err == nil {
		compatData = append(compatData, '\n')
		files = append(files, pipeline.EmittedFile{
			Path:          ".claude/settings.json",
			Content:       compatData,
			Layer:         pipeline.LayerExtension,
			SourceObjects: sourceObjects,
		})
	}

	return files
}

// hookEntry is a single hook entry in .github/hooks/{event}.json.
// No Matcher field — Copilot ignores matcher values.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// compatHookEntry is used for .claude/settings.json compatibility.
type compatHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Matcher string `json:"matcher,omitempty"`
}

// settingsJSON is the structure of .claude/settings.json.
type settingsJSON struct {
	Hooks map[string][]compatHookEntry `json:"hooks"`
}

// hookEvent extracts the event name from a hook's Fields.
func hookEvent(obj pipeline.LoweredObject) string {
	if obj.Fields != nil {
		if event, ok := obj.Fields["event"].(string); ok {
			return event
		}
	}
	return ""
}

// hookType maps a hook action type. Only "command" is supported by Copilot.
func hookType(obj pipeline.LoweredObject) string {
	if obj.Fields != nil {
		if action, ok := obj.Fields["action"].(map[string]any); ok {
			if t, ok := action["type"].(string); ok {
				return t
			}
		}
	}
	return "command"
}

// hookCommand extracts the command reference from a hook.
func hookCommand(obj pipeline.LoweredObject) string {
	if obj.Fields != nil {
		if action, ok := obj.Fields["action"].(map[string]any); ok {
			if ref, ok := action["ref"].(string); ok {
				return ref
			}
		}
	}
	return ""
}

// hookMatcher extracts matcher from hook scope paths (for Claude compat only).
func hookMatcher(obj pipeline.LoweredObject) string {
	if obj.Fields != nil {
		if scope, ok := obj.Fields["scope"].(map[string]any); ok {
			if paths, ok := scope["paths"].([]any); ok && len(paths) > 0 {
				if p, ok := paths[0].(string); ok {
					return p
				}
			}
		}
	}
	return ""
}

// sortedMapKeys returns sorted keys of a map with any value type.
func sortedMapKeys[V any](m map[string]V) []string {
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
