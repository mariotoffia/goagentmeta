package cursor

import (
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderInstructions generates AGENTS.md files from instruction objects.
// Instructions are grouped by scope path: root-scope instructions go to
// AGENTS.md, subtree-scope instructions go to {scope_path}/AGENTS.md.
// Within each file, instructions are sorted by object ID for determinism.
func renderInstructions(
	instructions []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) []pipeline.EmittedFile {
	if len(instructions) == 0 {
		return nil
	}

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

		filePath := "AGENTS.md"
		if scopePath != "" {
			filePath = scopePath + "/AGENTS.md"
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
