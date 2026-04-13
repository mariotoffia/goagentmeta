package copilot

import (
	"encoding/json"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderSettings generates both .github/hooks/{event}.json per event type (Copilot
// native) AND .claude/settings.json (compatibility layer) with hooks and permissions.
// Copilot only supports "command" hook types. Matcher values are OMITTED in the
// Copilot hook output since Copilot ignores them.
func renderSettings(hooks []pipeline.LoweredObject, permissions *permissionsJSON) []pipeline.EmittedFile {
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

	// If neither hooks nor permissions exist, nothing to emit.
	if len(hooksMap) == 0 && permissions == nil {
		return nil
	}

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

	compatSettings := settingsJSON{}
	if len(compatMap) > 0 {
		compatSettings.Hooks = compatMap
	}
	if permissions != nil {
		compatSettings.Permissions = permissions
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

// collectPermissions aggregates tool expressions from agents and skills into
// a permissions struct. Returns nil if no tool expressions are present.
func collectPermissions(agents, skills []pipeline.LoweredObject) *permissionsJSON {
	allowSet := make(map[string]bool)
	denySet := make(map[string]bool)

	for _, obj := range append(agents, skills...) {
		if tools := getFieldStringSlice(obj, "tools"); len(tools) > 0 {
			for _, t := range tools {
				allowSet[t] = true
			}
		}
		if denied := getFieldStringSlice(obj, "disallowedTools"); len(denied) > 0 {
			for _, t := range denied {
				denySet[t] = true
			}
		}
	}

	if len(allowSet) == 0 && len(denySet) == 0 {
		return nil
	}

	perms := &permissionsJSON{}
	for t := range allowSet {
		perms.Allow = append(perms.Allow, t)
	}
	for t := range denySet {
		perms.Deny = append(perms.Deny, t)
	}
	sort.Strings(perms.Allow)
	sort.Strings(perms.Deny)
	return perms
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
	Hooks       map[string][]compatHookEntry `json:"hooks,omitempty"`
	Permissions *permissionsJSON             `json:"permissions,omitempty"`
}

// permissionsJSON holds tool permission entries for settings.json.
type permissionsJSON struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
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
