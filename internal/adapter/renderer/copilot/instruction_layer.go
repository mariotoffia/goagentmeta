package copilot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderInstructions generates .github/copilot-instructions.md and AGENTS.md
// files from instruction objects. Root-scope instructions go to both
// .github/copilot-instructions.md and AGENTS.md. Subtree-scope instructions
// go to {scope_path}/AGENTS.md (Copilot reads nearest AGENTS.md).
func renderInstructions(
	instructions []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(instructions) == 0 {
		return nil
	}

	// Group instructions by their scope path.
	groups := make(map[string][]pipeline.LoweredObject)

	for _, inst := range instructions {
		scopePath := instructionScopePath(inst, objects)
		groups[scopePath] = append(groups[scopePath], inst)
	}

	groupKeys := sortedKeys(groups)

	var files []pipeline.EmittedFile
	for _, scopePath := range groupKeys {
		objs := groups[scopePath]

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

		if scopePath == "" {
			// Root scope: emit both .github/copilot-instructions.md and AGENTS.md.
			files = append(files, pipeline.EmittedFile{
				Path:          ".github/copilot-instructions.md",
				Content:       []byte(content.String()),
				Layer:         pipeline.LayerInstruction,
				SourceObjects: sourceObjects,
			})
			files = append(files, pipeline.EmittedFile{
				Path:          "AGENTS.md",
				Content:       []byte(content.String()),
				Layer:         pipeline.LayerInstruction,
				SourceObjects: sourceObjects,
			})
		} else {
			// Subtree scope: emit {path}/AGENTS.md.
			files = append(files, pipeline.EmittedFile{
				Path:          scopePath + "/AGENTS.md",
				Content:       []byte(content.String()),
				Layer:         pipeline.LayerInstruction,
				SourceObjects: sourceObjects,
			})
		}
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
	if obj.Fields != nil {
		if scope, ok := obj.Fields["scope"].(map[string]any); ok {
			if paths, ok := scope["paths"].([]any); ok && len(paths) > 0 {
				if p, ok := paths[0].(string); ok && p != "" && p != "." && p != "/" {
					return cleanPath(p)
				}
			}
		}
	}

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

// renderScopedInstructions generates .github/instructions/*.instructions.md
// files from rule objects. Each rule gets its own file with YAML frontmatter
// containing applyTo: as a comma-separated string (Copilot format).
func renderScopedInstructions(
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
		if len(paths) > 0 {
			content.WriteString("---\n")
			// Copilot uses applyTo: as a comma-separated string (not array).
			content.WriteString(fmt.Sprintf("applyTo: %s\n", yamlScalar(strings.Join(paths, ", "))))

			// excludeAgent: frontmatter if applicable.
			if excludeAgent := getFieldStringSlice(rule, "excludeAgent"); len(excludeAgent) > 0 {
				content.WriteString(fmt.Sprintf("excludeAgent: %s\n", yamlScalar(strings.Join(excludeAgent, ", "))))
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

		content.WriteString(rule.Content)

		filePath := fmt.Sprintf(".github/instructions/%s.instructions.md", sanitizeID(rule.OriginalID))

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

// ruleCondition represents a parsed condition from a rule.
type ruleCondition struct {
	Type  string
	Value string
}

// ruleConditions extracts conditions from a rule object.
func ruleConditions(
	obj pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []ruleCondition {
	if obj.Fields != nil {
		if conds, ok := obj.Fields["conditions"].([]any); ok {
			var result []ruleCondition
			for _, c := range conds {
				if m, ok := c.(map[string]any); ok {
					result = append(result, ruleCondition{
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

// cleanPath normalizes a path by removing leading/trailing slashes.
func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return p
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

// getFieldString extracts a string field from Fields.
func getFieldString(obj pipeline.LoweredObject, key string) string {
	if obj.Fields != nil {
		if v, ok := obj.Fields[key].(string); ok {
			return v
		}
	}
	return ""
}

// getFieldStringSlice extracts a string slice field from Fields.
func getFieldStringSlice(obj pipeline.LoweredObject, key string) []string {
	if obj.Fields == nil {
		return nil
	}

	switch v := obj.Fields[key].(type) {
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

// yamlScalar formats a string as a safe YAML scalar value.
func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	needsQuoting := strings.ContainsAny(s, "*?#:{}\n\r[]|>&!%@`,") ||
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
