package cursor

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderRules generates .cursor/rules/{id}.mdc files from rule objects.
// Each rule gets its own file with YAML frontmatter containing globs,
// alwaysApply, and description fields per Cursor's .mdc format.
//
// Application modes:
//  1. alwaysApply: true — Always Apply
//  2. description-based — Apply Intelligently (uses description)
//  3. globs: [...] — Apply to Specific Files
//  4. @mention only — Apply Manually (skipped or noted in diagnostic)
//
// Mapping logic: If a rule has AppliesTo scope paths, convert to globs.
// If no paths but a scope description, use alwaysApply: true (over-apply strategy).
func renderRules(
	rules []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(rules) == 0 {
		return nil
	}

	var files []pipeline.EmittedFile

	for _, rule := range rules {
		var content strings.Builder

		paths := rulePaths(rule, objects)
		desc := ruleDescription(rule, objects)

		// Determine application mode and generate frontmatter.
		content.WriteString("---\n")

		if len(paths) > 0 {
			// Mode 3: Apply to Specific Files — use globs.
			content.WriteString("globs:\n")
			for _, p := range paths {
				content.WriteString(fmt.Sprintf("  - %s\n", p))
			}
			content.WriteString("alwaysApply: false\n")
		} else if desc != "" {
			// Mode 2: Apply Intelligently — use description.
			content.WriteString("alwaysApply: false\n")
		} else {
			// Mode 1: Always Apply (over-apply strategy when no scope info).
			content.WriteString("alwaysApply: true\n")
		}

		if desc != "" {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(desc)))
		}

		content.WriteString("---\n\n")

		// Add rule content.
		content.WriteString(rule.Content)

		filePath := fmt.Sprintf(".cursor/rules/%s.mdc", sanitizeID(rule.OriginalID))

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{rule.OriginalID},
		})
	}

	return files
}

// rulePaths extracts the paths scope from a rule object.
func rulePaths(
	obj pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []string {
	if obj.Fields != nil {
		if scope, ok := obj.Fields["scope"].(map[string]any); ok {
			if paths, ok := scope["paths"].([]any); ok {
				var result []string
				for _, p := range paths {
					if s, ok := p.(string); ok {
						result = append(result, s)
					}
				}
				if len(result) > 0 {
					return result
				}
			}
		}
	}

	if norm, ok := objects[obj.OriginalID]; ok {
		return norm.Meta.Scope.Paths
	}

	return nil
}

// ruleDescription extracts the description from a rule object.
func ruleDescription(
	obj pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) string {
	if desc := getFieldString(obj, "description"); desc != "" {
		return desc
	}

	if norm, ok := objects[obj.OriginalID]; ok {
		return norm.Meta.Description
	}

	return ""
}

// yamlScalar formats a string as a safe YAML scalar value.
func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	needsQuoting := strings.ContainsAny(s, "#:{}\n\r[]|>&!%@`,") ||
		strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") ||
		strings.HasPrefix(s, "'") || strings.HasPrefix(s, `"`)

	if !needsQuoting {
		return s
	}

	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	escaped = strings.ReplaceAll(escaped, "\r", `\r`)
	return `"` + escaped + `"`
}
