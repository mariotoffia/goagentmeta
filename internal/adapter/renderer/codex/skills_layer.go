package codex

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// renderSkills generates .codex/skills/{id}/SKILL.md files in AgentSkills.io format.
// Each skill gets its own directory with a SKILL.md and optional sidecar files
// for references, assets, and scripts.
func renderSkills(
	skills []pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
) ([]pipeline.EmittedFile, []pipeline.EmittedAsset) {
	if len(skills) == 0 {
		return nil, nil
	}

	var files []pipeline.EmittedFile
	var assets []pipeline.EmittedAsset

	for _, skill := range skills {
		skillID := sanitizeID(skill.OriginalID)
		skillDir := fmt.Sprintf(".codex/skills/%s", skillID)

		var content strings.Builder

		content.WriteString("---\n")
		content.WriteString(fmt.Sprintf("name: %s\n", yamlScalar(skill.OriginalID)))

		if desc := getFieldString(skill, "description"); desc != "" {
			content.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(desc)))
		}

		if hints := getFieldStringSlice(skill, "activationHints"); len(hints) > 0 {
			content.WriteString("activation_hints:\n")
			for _, h := range hints {
				content.WriteString(fmt.Sprintf("  - %s\n", yamlScalar(h)))
			}
		}

		if userInvocable, ok := skill.Fields["userInvocable"].(bool); ok && userInvocable {
			content.WriteString("user_invocable: true\n")
		}

		if tools := getFieldStringSlice(skill, "allowedTools"); len(tools) > 0 {
			content.WriteString("allowed-tools:\n")
			for _, t := range tools {
				content.WriteString(fmt.Sprintf("  - %s\n", yamlScalar(t)))
			}
		}

		if m := getFieldString(skill, "model"); m != "" {
			content.WriteString(fmt.Sprintf("model: %s\n", yamlScalar(m)))
		}

		if deps := getFieldStringSlice(skill, "binaryDeps"); len(deps) > 0 {
			content.WriteString("binary_deps:\n")
			for _, d := range deps {
				content.WriteString(fmt.Sprintf("  - %s\n", yamlScalar(d)))
			}
		}

		if steps, ok := skill.Fields["installSteps"].([]any); ok && len(steps) > 0 {
			content.WriteString("install:\n")
			for _, step := range steps {
				if m, ok := step.(map[string]any); ok {
					content.WriteString(fmt.Sprintf("  - kind: %s\n", yamlScalar(getString(m, "kind"))))
					content.WriteString(fmt.Sprintf("    package: %s\n", yamlScalar(getString(m, "package"))))
					if bins := getStringSlice(m, "bins"); len(bins) > 0 {
						content.WriteString("    bins:\n")
						for _, b := range bins {
							content.WriteString(fmt.Sprintf("      - %s\n", yamlScalar(b)))
						}
					}
				}
			}
		}

		if pub, ok := skill.Fields["publishing"].(map[string]any); ok {
			if author := getString(pub, "author"); author != "" {
				content.WriteString(fmt.Sprintf("author: %s\n", yamlScalar(author)))
			}
			if homepage := getString(pub, "homepage"); homepage != "" {
				content.WriteString(fmt.Sprintf("homepage: %s\n", yamlScalar(homepage)))
			}
			if emoji := getString(pub, "emoji"); emoji != "" {
				content.WriteString(fmt.Sprintf("emoji: %s\n", yamlScalar(emoji)))
			}
		}

		content.WriteString("---\n\n")

		content.WriteString(skill.Content)

		files = append(files, pipeline.EmittedFile{
			Path:          skillDir + "/SKILL.md",
			Content:       []byte(content.String()),
			Layer:         pipeline.LayerInstruction,
			SourceObjects: []string{skill.OriginalID},
		})

		sidecarAssets := skillSidecarAssets(skill, objects, skillDir)
		assets = append(assets, sidecarAssets...)
	}

	return files, assets
}

// skillSidecarAssets collects asset references from a skill's resources.
func skillSidecarAssets(
	skill pipeline.LoweredObject,
	objects map[string]pipeline.NormalizedObject,
	skillDir string,
) []pipeline.EmittedAsset {
	var assets []pipeline.EmittedAsset

	if skill.Fields == nil {
		return assets
	}

	if resources, ok := skill.Fields["resources"].(map[string]any); ok {
		if refs := getStringSlice(resources, "references"); len(refs) > 0 {
			for _, ref := range refs {
				assets = append(assets, pipeline.EmittedAsset{
					SourcePath: ref,
					DestPath:   skillDir + "/" + lastSegment(ref),
				})
			}
		}

		if assetList := getStringSlice(resources, "assets"); len(assetList) > 0 {
			for _, a := range assetList {
				assets = append(assets, pipeline.EmittedAsset{
					SourcePath: a,
					DestPath:   skillDir + "/" + lastSegment(a),
				})
			}
		}

		if scripts := getStringSlice(resources, "scripts"); len(scripts) > 0 {
			for _, s := range scripts {
				assets = append(assets, pipeline.EmittedAsset{
					SourcePath: s,
					DestPath:   skillDir + "/" + lastSegment(s),
				})
			}
		}
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].DestPath < assets[j].DestPath
	})

	return assets
}

// lastSegment returns the last path segment (filename).
func lastSegment(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
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
