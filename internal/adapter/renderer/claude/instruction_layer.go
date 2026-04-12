package claude

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderInstructions generates CLAUDE.md files from instruction objects.
// Instructions are grouped by scope path: root-scope instructions go to
// CLAUDE.md, subtree-scope instructions go to {scope_path}/CLAUDE.md.
// Within each file, instructions are sorted by object ID for determinism.
func renderInstructions(
	instructions []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(instructions) == 0 {
		return nil
	}

	// Group instructions by their scope path.
	// Empty/root scope → "CLAUDE.md"
	// Subtree scope → "{path}/CLAUDE.md"
	groups := make(map[string][]pipeline.LoweredObject)

	for _, inst := range instructions {
		scopePath := instructionScopePath(inst, objects)
		groups[scopePath] = append(groups[scopePath], inst)
	}

	// Sort group keys for deterministic output.
	groupKeys := sortedKeys(groups)

	var files []pipeline.EmittedFile
	for _, scopePath := range groupKeys {
		objs := groups[scopePath]

		// Sort instructions within each group by original ID.
		sort.Slice(objs, func(i, j int) bool {
			return objs[i].OriginalID < objs[j].OriginalID
		})

		var content strings.Builder
		var sourceObjects []string

		for i, inst := range objs {
			if i > 0 {
				content.WriteString("\n\n")
			}
			content.WriteString(inst.Content)
			sourceObjects = append(sourceObjects, inst.OriginalID)
		}

		filePath := "CLAUDE.md"
		if scopePath != "" {
			filePath = scopePath + "/CLAUDE.md"
		}

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: sourceObjects,
		})
	}

	return files
}

// instructionScopePath determines the output path for an instruction based on
// its scope. Root-scope instructions return "", subtree-scope return the first
// scope path.
func instructionScopePath(
	obj pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) string {
	// Check Fields for scope information.
	if obj.Fields != nil {
		if scope, ok := obj.Fields["scope"].(map[string]any); ok {
			if paths, ok := scope["paths"].([]any); ok && len(paths) > 0 {
				if p, ok := paths[0].(string); ok && p != "" && p != "." && p != "/" {
					return cleanPath(p)
				}
			}
		}
	}

	// Fall back to normalized object metadata.
	if norm, ok := objects[obj.OriginalID]; ok {
		if len(norm.Meta.Scope.Paths) > 0 {
			p := norm.Meta.Scope.Paths[0]
			if p != "" && p != "." && p != "/" {
				return cleanPath(p)
			}
		}
	}

	return ""
}

// cleanPath normalizes a path by removing leading/trailing slashes.
func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return p
}

// renderRules generates .claude/rules/{id}.md files from rule objects.
// Each rule gets its own file with YAML frontmatter containing paths: scope
// and the rule content as the Markdown body.
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

		// Generate YAML frontmatter.
		paths := rulePaths(rule, objects)
		if len(paths) > 0 {
			content.WriteString("---\n")
			content.WriteString("paths:\n")
			for _, p := range paths {
				content.WriteString(fmt.Sprintf("  - %s\n", p))
			}
			content.WriteString("---\n\n")
		}

		// Add conditions as conditional prose if present.
		conditions := ruleConditions(rule, objects)
		if len(conditions) > 0 {
			content.WriteString("## Conditions\n\n")
			for _, cond := range conditions {
				content.WriteString(fmt.Sprintf("- **%s**: %s\n", cond.Type, cond.Value))
			}
			content.WriteString("\n")
		}

		// Add rule content.
		content.WriteString(rule.Content)

		filePath := fmt.Sprintf(".claude/rules/%s.md", sanitizeID(rule.OriginalID))

		files = append(files, pipeline.EmittedFile{
			Path:          filePath,
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{rule.OriginalID},
		})
	}

	return files
}

// rulePaths extracts the paths: scope from a rule object.
func rulePaths(
	obj pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []string {
	// Check Fields for scope paths.
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

	// Fall back to normalized object metadata.
	if norm, ok := objects[obj.OriginalID]; ok {
		return norm.Meta.Scope.Paths
	}

	return nil
}

// ruleConditions extracts conditions from a rule object.
func ruleConditions(
	obj pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []model.RuleCondition {
	if obj.Fields != nil {
		if conds, ok := obj.Fields["conditions"].([]any); ok {
			var result []model.RuleCondition
			for _, c := range conds {
				if m, ok := c.(map[string]any); ok {
					result = append(result, model.RuleCondition{
						Type:  getString(m, "type"),
						Value: getString(m, "value"),
					})
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}

	return nil
}

// sanitizeID converts an object ID to a safe filename component.
func sanitizeID(id string) string {
	id = strings.ReplaceAll(id, "/", "-")
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, ".", "-")
	return id
}

// getString safely extracts a string from a map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getStringSlice safely extracts a string slice from a map.
func getStringSlice(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// yamlScalar formats a string as a safe YAML scalar value. Values containing
// YAML-special characters (#, :, {, }, [, ], newlines, leading/trailing
// whitespace) are double-quoted with internal quotes escaped.
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
