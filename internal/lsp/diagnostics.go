package lsp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// lineRegexp matches "line N:" in yaml error messages.
var lineRegexp = regexp.MustCompile(`line (\d+):`)

// DiagnoseYAML validates a YAML document and returns LSP diagnostics.
// It checks both YAML syntax and known goagentmeta manifest structure.
func DiagnoseYAML(uri, content string) []Diagnostic {
	var diags []Diagnostic

	// Phase 1: check basic YAML syntax by decoding into a yaml.Node tree.
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		diags = append(diags, yamlErrorToDiagnostics(err)...)
		return diags
	}

	// An empty document is fine — no diagnostics.
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil
	}

	// Phase 2: structural validation on the node tree.
	root := &doc
	// yaml.Unmarshal wraps the real content in a DocumentNode.
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}

	if root.Kind != yaml.MappingNode {
		diags = append(diags, Diagnostic{
			Range:    nodeRange(root),
			Severity: SeverityWarning,
			Source:   diagnosticSource,
			Message:  "expected a YAML mapping at the document root",
		})
		return diags
	}

	diags = append(diags, validateMapping(root)...)
	return diags
}

// validateMapping checks a root mapping node for known goagentmeta structure.
func validateMapping(root *yaml.Node) []Diagnostic {
	var diags []Diagnostic
	keys := mappingKeys(root)

	// Detect if this is a manifest (has "dependencies" or "compiler" keys).
	if containsKey(keys, "dependencies") || containsKey(keys, "compiler") {
		diags = append(diags, validateManifest(root, keys)...)
		return diags
	}

	// Detect if this is an object definition (has "kind" key).
	if containsKey(keys, "kind") {
		diags = append(diags, validateObjectMeta(root, keys)...)
		return diags
	}

	return diags
}

// validateManifest checks a manifest-like mapping for known issues.
func validateManifest(root *yaml.Node, keys []keyEntry) []Diagnostic {
	var diags []Diagnostic
	knownManifestKeys := map[string]bool{
		"dependencies": true,
		"registries":   true,
		"compiler":     true,
	}

	for _, k := range keys {
		if !knownManifestKeys[k.name] {
			diags = append(diags, Diagnostic{
				Range:    nodeRange(k.node),
				Severity: SeverityWarning,
				Source:   diagnosticSource,
				Message:  fmt.Sprintf("unknown manifest key: %q", k.name),
			})
		}
	}

	// Validate compiler section if present.
	if compilerNode := findMappingValue(root, "compiler"); compilerNode != nil {
		diags = append(diags, validateCompilerSection(compilerNode)...)
	}

	return diags
}

// validateCompilerSection validates the "compiler:" block of a manifest.
func validateCompilerSection(node *yaml.Node) []Diagnostic {
	var diags []Diagnostic

	if node.Kind != yaml.MappingNode {
		diags = append(diags, Diagnostic{
			Range:    nodeRange(node),
			Severity: SeverityError,
			Source:   diagnosticSource,
			Message:  "compiler section must be a mapping",
		})
		return diags
	}

	knownCompilerKeys := map[string]bool{
		"name":    true,
		"version": true,
		"phases":  true,
		"source":  true,
	}

	for _, k := range mappingKeys(node) {
		if !knownCompilerKeys[k.name] {
			diags = append(diags, Diagnostic{
				Range:    nodeRange(k.node),
				Severity: SeverityWarning,
				Source:   diagnosticSource,
				Message:  fmt.Sprintf("unknown compiler key: %q", k.name),
			})
		}
	}

	return diags
}

// Known kinds for goagentmeta objects.
var validKinds = map[string]bool{
	"instruction": true,
	"rule":        true,
	"skill":       true,
	"agent":       true,
	"hook":        true,
	"command":     true,
	"capability":  true,
	"plugin":      true,
}

// validateObjectMeta validates object-like YAML documents (kind, id, etc.).
func validateObjectMeta(root *yaml.Node, keys []keyEntry) []Diagnostic {
	var diags []Diagnostic

	// Check "kind" value is valid.
	kindNode := findMappingValue(root, "kind")
	if kindNode != nil && kindNode.Kind == yaml.ScalarNode {
		if !validKinds[kindNode.Value] {
			diags = append(diags, Diagnostic{
				Range:    nodeRange(kindNode),
				Severity: SeverityError,
				Source:   diagnosticSource,
				Message:  fmt.Sprintf("invalid kind: %q; expected one of: instruction, rule, skill, agent, hook, command, capability, plugin", kindNode.Value),
			})
		}
	}

	// "id" is required when "kind" is present.
	if !containsKey(keys, "id") {
		kindKeyNode := findMappingKey(root, "kind")
		r := Range{}
		if kindKeyNode != nil {
			r = nodeRange(kindKeyNode)
		}
		diags = append(diags, Diagnostic{
			Range:    r,
			Severity: SeverityError,
			Source:   diagnosticSource,
			Message:  "object with 'kind' must have an 'id' field",
		})
	}

	return diags
}

// yamlErrorToDiagnostics converts a yaml parse error into LSP diagnostics.
func yamlErrorToDiagnostics(err error) []Diagnostic {
	if err == nil {
		return nil
	}

	// yaml.TypeError contains multiple error strings.
	if te, ok := err.(*yaml.TypeError); ok {
		var diags []Diagnostic
		for _, msg := range te.Errors {
			diags = append(diags, Diagnostic{
				Range:    extractLineRange(msg),
				Severity: SeverityError,
				Source:   diagnosticSource,
				Message:  cleanYAMLError(msg),
			})
		}
		return diags
	}

	// Plain error — extract line if present.
	msg := err.Error()
	return []Diagnostic{{
		Range:    extractLineRange(msg),
		Severity: SeverityError,
		Source:   diagnosticSource,
		Message:  cleanYAMLError(msg),
	}}
}

// extractLineRange tries to extract a line number from a yaml error message.
func extractLineRange(msg string) Range {
	m := lineRegexp.FindStringSubmatch(msg)
	if m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			line := n - 1 // yaml uses 1-based, LSP uses 0-based
			return Range{
				Start: Position{Line: line, Character: 0},
				End:   Position{Line: line, Character: 0},
			}
		}
	}
	return Range{}
}

// cleanYAMLError strips the "yaml: " prefix from error messages.
func cleanYAMLError(msg string) string {
	msg = strings.TrimPrefix(msg, "yaml: ")
	return msg
}

// nodeRange returns an LSP range for a yaml.Node (1-based → 0-based).
func nodeRange(n *yaml.Node) Range {
	line := n.Line - 1
	col := n.Column - 1
	if line < 0 {
		line = 0
	}
	if col < 0 {
		col = 0
	}

	// Estimate end column from the value length.
	endCol := col
	if n.Value != "" {
		endCol = col + len(n.Value)
	}

	return Range{
		Start: Position{Line: line, Character: col},
		End:   Position{Line: line, Character: endCol},
	}
}

// keyEntry holds a mapping key name and its yaml.Node.
type keyEntry struct {
	name string
	node *yaml.Node
}

// mappingKeys returns all keys in a mapping node.
func mappingKeys(node *yaml.Node) []keyEntry {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	var keys []keyEntry
	for i := 0; i+1 < len(node.Content); i += 2 {
		keys = append(keys, keyEntry{
			name: node.Content[i].Value,
			node: node.Content[i],
		})
	}
	return keys
}

// containsKey checks if a key slice contains a given name.
func containsKey(keys []keyEntry, name string) bool {
	for _, k := range keys {
		if k.name == name {
			return true
		}
	}
	return false
}

// findMappingValue returns the value node for a key in a mapping.
func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// findMappingKey returns the key node for a key in a mapping.
func findMappingKey(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i]
		}
	}
	return nil
}

const diagnosticSource = "goagentmeta"
