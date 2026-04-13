package claude

import (
	"encoding/json"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderSettings generates .claude/settings.json containing hooks and permissions.
// Hooks are grouped by Event → []{type, command, matcher} as per Claude Code's
// hooks convention. Permissions aggregate tool expressions from agents and skills.
func renderSettings(hooks []pipeline.LoweredObject, permissions *permissionsJSON) []pipeline.EmittedFile {
	hooksMap, sourceObjects := buildHooksMap(hooks)

	// If neither hooks nor permissions exist, nothing to emit.
	if len(hooksMap) == 0 && permissions == nil {
		return nil
	}

	settings := settingsJSON{}

	if len(hooksMap) > 0 {
		settings.Hooks = hooksMap
	}
	if permissions != nil {
		settings.Permissions = permissions
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil
	}

	data = append(data, '\n')

	sort.Strings(sourceObjects)

	return []pipeline.EmittedFile{
		{
			Path:          ".claude/settings.json",
			Content:       data,
			Layer:         pipeline.LayerExtension,
			SourceObjects: sourceObjects,
		},
	}
}

// buildHooksMap converts hook objects into a map of event → []hookEntry.
func buildHooksMap(hooks []pipeline.LoweredObject) (map[string][]hookEntry, []string) {
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

		if matcher := hookMatcher(hook); matcher != "" {
			entry.Matcher = matcher
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

	return hooksMap, sourceObjects
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

// settingsJSON is the structure of .claude/settings.json.
type settingsJSON struct {
	Hooks       map[string][]hookEntry `json:"hooks,omitempty"`
	Permissions *permissionsJSON       `json:"permissions,omitempty"`
}

// permissionsJSON holds tool permission entries for settings.json.
type permissionsJSON struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// hookEntry is a single hook entry in settings.json.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Matcher string `json:"matcher,omitempty"`
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

// hookType maps a hook action type to Claude's hook type.
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

// hookMatcher extracts matcher from hook scope paths.
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
