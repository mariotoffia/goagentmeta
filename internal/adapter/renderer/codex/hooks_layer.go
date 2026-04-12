package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderHooks generates .codex/settings.json containing the hooks section.
// Hooks are grouped by Event → []{type, command, matcher} as per Codex CLI's
// hooks convention. Only events in codexSupportedHookEvents are emitted;
// hooks for unsupported events are filtered with a diagnostic warning.
func renderHooks(ctx context.Context, hooks []pipeline.LoweredObject) []pipeline.EmittedFile {
	if len(hooks) == 0 {
		return nil
	}

	hooksMap := make(map[string][]hookEntry)
	var sourceObjects []string
	var unsupportedEvents []string

	for _, hook := range hooks {
		event := hookEvent(hook)
		if event == "" {
			continue
		}

		// Filter to Codex-supported events only.
		if !codexSupportedHookEvents[event] {
			unsupportedEvents = append(unsupportedEvents, fmt.Sprintf("%s(%s)", hook.OriginalID, event))
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

	// Emit diagnostic for unsupported hook events.
	if len(unsupportedEvents) > 0 {
		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "warning",
			Code:     "RENDER_UNSUPPORTED_HOOK_EVENT",
			Message:  fmt.Sprintf("codex does not support hook event(s): %v", unsupportedEvents),
			Phase:    pipeline.PhaseRender,
		})
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

	settings := settingsJSON{
		Hooks: hooksMap,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil
	}

	data = append(data, '\n')

	sort.Strings(sourceObjects)

	return []pipeline.EmittedFile{
		{
			Path:          ".codex/settings.json",
			Content:       data,
			Layer:         pipeline.LayerExtension,
			SourceObjects: sourceObjects,
		},
	}
}

// settingsJSON is the structure of .codex/settings.json.
type settingsJSON struct {
	Hooks map[string][]hookEntry `json:"hooks"`
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

// hookType maps a hook action type to Codex's hook type.
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
