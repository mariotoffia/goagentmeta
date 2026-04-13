package normalizer

import "github.com/mariotoffia/goagentmeta/internal/domain/model"

// projectMetaIntoFields re-injects Meta fields that the parser strips from
// RawFields. Renderers read all output data from Fields, so any Meta field
// that should appear in the rendered frontmatter must be present here.
func projectMetaIntoFields(meta *model.ObjectMeta, fields map[string]any) {
	if meta.Description != "" {
		if _, exists := fields["description"]; !exists {
			fields["description"] = meta.Description
		}
	}
}

// flattenNestedFields rewrites known nested YAML structures into the flat keys
// that renderers expect.
//
// Mappings:
//
//	activation.hints → activationHints
func flattenNestedFields(fields map[string]any) {
	if act, ok := fields["activation"].(map[string]any); ok {
		if hints, ok := act["hints"]; ok {
			if _, exists := fields["activationHints"]; !exists {
				fields["activationHints"] = hints
			}
		}
	}
}
