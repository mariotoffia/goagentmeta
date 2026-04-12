// Package parser implements the PhaseParse stage. It reads the .ai/ source
// directory and produces a [pipeline.SourceTree] containing [pipeline.RawObject]
// entries for every canonical object found.
//
// The parser supports two file formats:
//   - YAML files (.yaml): Pure YAML documents for kinds without a body
//     (hook, command, capability, plugin) or as an alternative for any kind.
//   - Markdown files (.md): YAML frontmatter delimited by "---" lines,
//     followed by a markdown body. The body becomes RawContent. This is the
//     primary authoring format for instruction, rule, skill, and agent kinds.
//
// The parser does NOT validate schemas — that is the validator's job.
// It simply splits frontmatter from body and extracts ObjectMeta fields.
package parser

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portstage "github.com/mariotoffia/goagentmeta/internal/port/stage"

	"gopkg.in/yaml.v3"
)

// Stage implements the PhaseParse pipeline stage.
type Stage struct {
	// sourceDirName is the name of the source directory within the project
	// root. Defaults to ".ai".
	sourceDirName string
}

// Option configures the parser stage.
type Option func(*Stage)

// WithSourceDir overrides the default ".ai" source directory name.
func WithSourceDir(name string) Option {
	return func(s *Stage) { s.sourceDirName = name }
}

// New creates a parser stage with the given options.
func New(opts ...Option) *Stage {
	s := &Stage{sourceDirName: ".ai"}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Compile-time assertions.
var _ portstage.Stage = (*Stage)(nil)

func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "source-parser",
		Phase: pipeline.PhaseParse,
		Order: 0,
	}
}

// Execute reads the source directory and returns a [pipeline.SourceTree].
// The input is expected to be a string (project root path) or a []string
// of root paths (first is used).
func (s *Stage) Execute(_ context.Context, input any) (any, error) {
	rootPath, err := resolveRootPath(input)
	if err != nil {
		return nil, err
	}

	sourceDir := filepath.Join(rootPath, s.sourceDirName)
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return nil, pipeline.NewCompilerError(
			pipeline.ErrParse,
			fmt.Sprintf("source directory %s does not exist", sourceDir),
			"source-parser",
		)
	}

	tree := &pipeline.SourceTree{
		RootPath:      rootPath,
		SchemaVersion: 1,
		ManifestPath:  filepath.Join(sourceDir, "manifest.yaml"),
	}

	// Read manifest for schema version.
	if data, err := os.ReadFile(tree.ManifestPath); err == nil {
		var manifest map[string]any
		if err := yaml.Unmarshal(data, &manifest); err == nil {
			if sv, ok := manifest["schemaVersion"].(int); ok {
				tree.SchemaVersion = sv
			}
		}
	}

	// Walk known subdirectories for object files.
	subdirs := []string{
		"instructions", "rules", "skills", "agents",
		"hooks", "commands", "capabilities", "plugins",
	}

	for _, sub := range subdirs {
		subPath := filepath.Join(sourceDir, sub)
		entries, err := os.ReadDir(subPath)
		if err != nil {
			continue // directory doesn't exist — that's fine
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			filePath := filepath.Join(subPath, name)

			switch {
			case strings.HasSuffix(name, ".yaml"), strings.HasSuffix(name, ".yml"):
				obj, err := parseYAMLFile(filePath)
				if err != nil {
					return nil, err
				}
				tree.Objects = append(tree.Objects, obj)

			case strings.HasSuffix(name, ".md"):
				obj, err := parseMarkdownFile(filePath)
				if err != nil {
					return nil, err
				}
				tree.Objects = append(tree.Objects, obj)
			}
		}
	}

	// Sort objects by ID for deterministic pipeline behavior.
	sort.Slice(tree.Objects, func(i, j int) bool {
		return tree.Objects[i].Meta.ID < tree.Objects[j].Meta.ID
	})

	return *tree, nil
}

// parseYAMLFile parses a pure YAML file into a RawObject.
func parseYAMLFile(path string) (pipeline.RawObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pipeline.RawObject{}, pipeline.NewCompilerError(
			pipeline.ErrParse,
			fmt.Sprintf("reading %s: %v", path, err),
			"source-parser",
		)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return pipeline.RawObject{}, pipeline.NewCompilerError(
			pipeline.ErrParse,
			fmt.Sprintf("parsing YAML %s: %v", path, err),
			"source-parser",
		)
	}

	meta := extractMeta(raw)
	rawFields := extractRawFields(raw)

	var rawContent string
	if c, ok := raw["content"].(string); ok {
		rawContent = c
	}
	// Agent rolePrompt also acts as content body.
	if rp, ok := raw["rolePrompt"].(string); ok && rawContent == "" {
		rawContent = rp
	}

	return pipeline.RawObject{
		Meta:       meta,
		SourcePath: path,
		RawContent: rawContent,
		RawFields:  rawFields,
	}, nil
}

// parseMarkdownFile parses a markdown file with YAML frontmatter.
// The frontmatter is everything between the first "---" line and the
// next "---" line. Everything after the second "---" is the body.
func parseMarkdownFile(path string) (pipeline.RawObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pipeline.RawObject{}, pipeline.NewCompilerError(
			pipeline.ErrParse,
			fmt.Sprintf("reading %s: %v", path, err),
			"source-parser",
		)
	}

	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return pipeline.RawObject{}, pipeline.NewCompilerError(
			pipeline.ErrParse,
			fmt.Sprintf("parsing frontmatter in %s: %v", path, err),
			"source-parser",
		)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(frontmatter, &raw); err != nil {
		return pipeline.RawObject{}, pipeline.NewCompilerError(
			pipeline.ErrParse,
			fmt.Sprintf("parsing YAML frontmatter in %s: %v", path, err),
			"source-parser",
		)
	}

	meta := extractMeta(raw)
	rawFields := extractRawFields(raw)

	return pipeline.RawObject{
		Meta:       meta,
		SourcePath: path,
		RawContent: strings.TrimSpace(body),
		RawFields:  rawFields,
	}, nil
}

// splitFrontmatter extracts YAML frontmatter and body from a markdown file.
// Returns (frontmatter bytes, body string, error).
func splitFrontmatter(data []byte) ([]byte, string, error) {
	const delimiter = "---"

	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(delimiter)) {
		return nil, "", fmt.Errorf("file does not start with --- frontmatter delimiter")
	}

	// Skip the first "---" line.
	rest := trimmed[len(delimiter):]
	rest = bytes.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	// Find the closing "---" line.
	idx := bytes.Index(rest, []byte("\n"+delimiter))
	if idx < 0 {
		return nil, "", fmt.Errorf("no closing --- delimiter found")
	}

	frontmatter := rest[:idx]
	afterDelimiter := rest[idx+1+len(delimiter):]

	// Skip the rest of the closing delimiter line.
	if nl := bytes.IndexByte(afterDelimiter, '\n'); nl >= 0 {
		afterDelimiter = afterDelimiter[nl+1:]
	} else {
		afterDelimiter = nil
	}

	return frontmatter, string(afterDelimiter), nil
}

// resolveRootPath extracts the project root path from the stage input.
func resolveRootPath(input any) (string, error) {
	switch v := input.(type) {
	case string:
		return v, nil
	case []string:
		if len(v) == 0 {
			return "", pipeline.NewCompilerError(
				pipeline.ErrParse, "no root paths provided", "source-parser",
			)
		}
		return v[0], nil
	default:
		return "", pipeline.NewCompilerError(
			pipeline.ErrParse,
			fmt.Sprintf("expected string or []string input, got %T", input),
			"source-parser",
		)
	}
}

// extractMeta extracts ObjectMeta fields from a raw YAML map.
func extractMeta(raw map[string]any) model.ObjectMeta {
	meta := model.ObjectMeta{}

	if id, ok := raw["id"].(string); ok {
		meta.ID = id
	}
	if kind, ok := raw["kind"].(string); ok {
		meta.Kind = model.Kind(kind)
	}
	if v, ok := raw["version"].(int); ok {
		meta.Version = v
	}
	if desc, ok := raw["description"].(string); ok {
		meta.Description = desc
	}
	if pres, ok := raw["preservation"].(string); ok {
		meta.Preservation = model.Preservation(pres)
	}
	if owner, ok := raw["owner"].(string); ok {
		meta.Owner = owner
	}
	if pv, ok := raw["packageVersion"].(string); ok {
		meta.PackageVersion = pv
	}
	if lic, ok := raw["license"].(string); ok {
		meta.License = lic
	}
	if extends, ok := raw["extends"].([]any); ok {
		meta.Extends = toStringSlice(extends)
	}
	if labels, ok := raw["labels"].([]any); ok {
		meta.Labels = toStringSlice(labels)
	}
	if scopeMap, ok := raw["scope"].(map[string]any); ok {
		meta.Scope = extractScope(scopeMap)
	}
	if atMap, ok := raw["appliesTo"].(map[string]any); ok {
		meta.AppliesTo = extractAppliesTo(atMap)
	}
	if toMap, ok := raw["targetOverrides"].(map[string]any); ok {
		meta.TargetOverrides = extractTargetOverrides(toMap)
	}

	return meta
}

func extractScope(m map[string]any) model.Scope {
	s := model.Scope{}
	if paths, ok := m["paths"].([]any); ok {
		s.Paths = toStringSlice(paths)
	}
	if ft, ok := m["fileTypes"].([]any); ok {
		s.FileTypes = toStringSlice(ft)
	}
	if labels, ok := m["labels"].([]any); ok {
		s.Labels = toStringSlice(labels)
	}
	return s
}

func extractAppliesTo(m map[string]any) model.AppliesTo {
	at := model.AppliesTo{}
	if targets, ok := m["targets"].([]any); ok {
		at.Targets = toStringSlice(targets)
	}
	if profiles, ok := m["profiles"].([]any); ok {
		at.Profiles = toStringSlice(profiles)
	}
	return at
}

func extractTargetOverrides(m map[string]any) map[string]model.TargetOverride {
	overrides := make(map[string]model.TargetOverride)
	for target, v := range m {
		om, ok := v.(map[string]any)
		if !ok {
			continue
		}
		to := model.TargetOverride{}
		if enabled, ok := om["enabled"].(bool); ok {
			to.Enabled = &enabled
		}
		if syntax, ok := om["syntax"].(map[string]any); ok {
			to.Syntax = toStringMap(syntax)
		}
		if placement, ok := om["placement"].(map[string]any); ok {
			to.Placement = toStringMap(placement)
		}
		if extra, ok := om["extra"].(map[string]any); ok {
			to.Extra = toStringMap(extra)
		}
		overrides[target] = to
	}
	return overrides
}

// extractRawFields returns all non-meta fields from a raw YAML map.
func extractRawFields(raw map[string]any) map[string]any {
	metaKeys := map[string]bool{
		"id": true, "kind": true, "version": true, "description": true,
		"preservation": true, "scope": true, "appliesTo": true,
		"extends": true, "labels": true, "owner": true, "targetOverrides": true,
		"packageVersion": true, "license": true,
	}

	fields := make(map[string]any)
	for k, v := range raw {
		if !metaKeys[k] {
			fields[k] = v
		}
	}
	return fields
}

func toStringSlice(s []any) []string {
	result := make([]string, 0, len(s))
	for _, v := range s {
		if str, ok := v.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func toStringMap(m map[string]any) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}
