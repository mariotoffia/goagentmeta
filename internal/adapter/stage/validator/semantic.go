package validator

import (
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/domain/tool"
)

// SemanticValidator performs cross-object relationship validation on a
// SourceTree. It detects duplicate IDs, broken references, circular
// inheritance, and circular delegation. It also validates tool expressions
// when a ToolPluginRegistry is configured.
type SemanticValidator struct {
	// toolRegistry validates tool expressions in allowedTools and toolPolicy.
	// May be nil, in which case tool validation is skipped.
	toolRegistry *tool.Registry
}

// NewSemanticValidator returns a new semantic validator.
func NewSemanticValidator() *SemanticValidator {
	return &SemanticValidator{}
}

// WithToolRegistry sets the tool plugin registry for tool expression validation.
func (v *SemanticValidator) WithToolRegistry(r *tool.Registry) *SemanticValidator {
	v.toolRegistry = r
	return v
}

// Validate checks cross-object semantic constraints and returns diagnostics.
func (v *SemanticValidator) Validate(tree pipeline.SourceTree) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic

	objectsByID := v.buildIndex(tree)

	diags = append(diags, v.checkDuplicateIDs(tree)...)
	diags = append(diags, v.checkReferences(tree, objectsByID)...)
	diags = append(diags, v.checkCircularInheritance(tree, objectsByID)...)
	diags = append(diags, v.checkCircularDelegation(tree, objectsByID)...)
	diags = append(diags, v.checkToolExpressions(tree)...)

	return diags
}

// buildIndex creates a lookup of objects by ID (first occurrence).
func (v *SemanticValidator) buildIndex(tree pipeline.SourceTree) map[string]pipeline.RawObject {
	index := make(map[string]pipeline.RawObject, len(tree.Objects))
	for _, obj := range tree.Objects {
		if obj.Meta.ID == "" {
			continue
		}
		if _, exists := index[obj.Meta.ID]; !exists {
			index[obj.Meta.ID] = obj
		}
	}
	return index
}

// checkDuplicateIDs detects objects sharing the same ID.
func (v *SemanticValidator) checkDuplicateIDs(tree pipeline.SourceTree) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic
	seen := make(map[string]string) // id -> first source path

	for _, obj := range tree.Objects {
		if obj.Meta.ID == "" {
			continue
		}
		if firstPath, exists := seen[obj.Meta.ID]; exists {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("duplicate object id %q (first defined at %s)", obj.Meta.ID, firstPath),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		} else {
			seen[obj.Meta.ID] = obj.SourcePath
		}
	}
	return diags
}

// checkReferences validates that cross-object references point to existing objects.
func (v *SemanticValidator) checkReferences(
	tree pipeline.SourceTree,
	objectsByID map[string]pipeline.RawObject,
) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic

	for _, obj := range tree.Objects {
		if obj.Meta.Kind != model.KindAgent {
			continue
		}

		// Check agent.skills references (from RawFields or Meta-level).
		skillRefs := v.extractStringSlice(obj.RawFields, "skills")
		// Also check links.skills (schema variant).
		if links, ok := obj.RawFields["links"].(map[string]any); ok {
			skillRefs = append(skillRefs, v.extractStringSliceFromAny(links["skills"])...)
		}
		for _, skillID := range skillRefs {
			if target, exists := objectsByID[skillID]; !exists {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q references skill %q which does not exist", obj.Meta.ID, skillID),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			} else if target.Meta.Kind != model.KindSkill {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q references %q as a skill, but it is kind %q", obj.Meta.ID, skillID, target.Meta.Kind),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			}
		}

		// Check agent.delegation.mayCall references.
		delegationRefs := v.extractDelegationRefs(obj.RawFields)
		for _, agentID := range delegationRefs {
			if target, exists := objectsByID[agentID]; !exists {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q delegates to %q which does not exist", obj.Meta.ID, agentID),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			} else if target.Meta.Kind != model.KindAgent {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q delegates to %q, but it is kind %q (not agent)", obj.Meta.ID, agentID, target.Meta.Kind),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			}
		}

		// Check agent.hooks references.
		hookRefs := v.extractStringSlice(obj.RawFields, "hooks")
		for _, hookID := range hookRefs {
			if target, exists := objectsByID[hookID]; !exists {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q references hook %q which does not exist", obj.Meta.ID, hookID),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			} else if target.Meta.Kind != model.KindHook {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q references %q as a hook, but it is kind %q", obj.Meta.ID, hookID, target.Meta.Kind),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			}
		}

		// Check agent.handoffs[].agent references.
		handoffRefs := v.extractHandoffAgentRefs(obj.RawFields)
		for _, agentID := range handoffRefs {
			if target, exists := objectsByID[agentID]; !exists {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q has handoff to %q which does not exist", obj.Meta.ID, agentID),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			} else if target.Meta.Kind != model.KindAgent {
				diags = append(diags, pipeline.Diagnostic{
					Severity:   "error",
					Code:       string(pipeline.ErrValidation),
					Message:    fmt.Sprintf("agent %q has handoff to %q, but it is kind %q (not agent)", obj.Meta.ID, agentID, target.Meta.Kind),
					SourcePath: obj.SourcePath,
					ObjectID:   obj.Meta.ID,
					Phase:      pipeline.PhaseValidate,
				})
			}
		}
	}

	return diags
}

// checkCircularInheritance detects inheritance cycles (A extends B extends A).
func (v *SemanticValidator) checkCircularInheritance(
	tree pipeline.SourceTree,
	objectsByID map[string]pipeline.RawObject,
) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic

	for _, obj := range tree.Objects {
		if len(obj.Meta.Extends) == 0 {
			continue
		}

		visited := map[string]bool{obj.Meta.ID: true}
		if cycle := v.detectCycle(obj.Meta.ID, obj.Meta.Extends, objectsByID, visited); cycle != "" {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("circular inheritance detected: %s", cycle),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}

	return diags
}

// detectCycle walks the extends chain looking for cycles.
// Returns a description of the cycle or empty string if none found.
func (v *SemanticValidator) detectCycle(
	startID string,
	extends []string,
	objectsByID map[string]pipeline.RawObject,
	visited map[string]bool,
) string {
	for _, parentID := range extends {
		if visited[parentID] {
			return fmt.Sprintf("%s -> %s", startID, parentID)
		}
		visited[parentID] = true

		parent, exists := objectsByID[parentID]
		if !exists {
			delete(visited, parentID)
			continue // Missing references are caught by reference checks.
		}
		if cycle := v.detectCycle(parentID, parent.Meta.Extends, objectsByID, visited); cycle != "" {
			return fmt.Sprintf("%s -> %s", startID, cycle)
		}

		delete(visited, parentID)
	}
	return ""
}

// checkCircularDelegation detects delegation cycles (A→B→A).
func (v *SemanticValidator) checkCircularDelegation(
	tree pipeline.SourceTree,
	objectsByID map[string]pipeline.RawObject,
) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic

	for _, obj := range tree.Objects {
		if obj.Meta.Kind != model.KindAgent {
			continue
		}

		delegationRefs := v.extractDelegationRefs(obj.RawFields)
		if len(delegationRefs) == 0 {
			continue
		}

		visited := map[string]bool{obj.Meta.ID: true}
		if cycle := v.detectDelegationCycle(obj.Meta.ID, delegationRefs, objectsByID, visited); cycle != "" {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("circular delegation detected: %s", cycle),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}

	return diags
}

// detectDelegationCycle walks the delegation chain looking for cycles.
func (v *SemanticValidator) detectDelegationCycle(
	startID string,
	delegations []string,
	objectsByID map[string]pipeline.RawObject,
	visited map[string]bool,
) string {
	for _, targetID := range delegations {
		if visited[targetID] {
			return fmt.Sprintf("%s -> %s", startID, targetID)
		}
		visited[targetID] = true

		target, exists := objectsByID[targetID]
		if !exists {
			delete(visited, targetID)
			continue
		}
		if target.Meta.Kind != model.KindAgent {
			delete(visited, targetID)
			continue
		}

		childDelegations := v.extractDelegationRefs(target.RawFields)
		if cycle := v.detectDelegationCycle(targetID, childDelegations, objectsByID, visited); cycle != "" {
			return fmt.Sprintf("%s -> %s", startID, cycle)
		}

		delete(visited, targetID)
	}
	return ""
}

// extractStringSlice extracts a []string from a RawFields key.
func (v *SemanticValidator) extractStringSlice(fields map[string]any, key string) []string {
	val, ok := fields[key]
	if !ok {
		return nil
	}
	return v.extractStringSliceFromAny(val)
}

// extractStringSliceFromAny converts an any value to []string.
func (v *SemanticValidator) extractStringSliceFromAny(val any) []string {
	if val == nil {
		return nil
	}
	arr, ok := val.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// extractDelegationRefs extracts agent IDs from delegation.mayCall.
func (v *SemanticValidator) extractDelegationRefs(fields map[string]any) []string {
	delegation, ok := fields["delegation"]
	if !ok {
		return nil
	}
	delegationMap, ok := delegation.(map[string]any)
	if !ok {
		return nil
	}
	return v.extractStringSliceFromAny(delegationMap["mayCall"])
}

// extractHandoffAgentRefs extracts agent IDs from handoffs[].agent.
func (v *SemanticValidator) extractHandoffAgentRefs(fields map[string]any) []string {
	handoffs, ok := fields["handoffs"]
	if !ok {
		return nil
	}
	arr, ok := handoffs.([]any)
	if !ok {
		return nil
	}
	var refs []string
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if agent, ok := m["agent"].(string); ok {
			refs = append(refs, agent)
		}
	}
	return refs
}

// checkToolExpressions validates tool expressions in allowedTools (skills)
// and toolPolicy keys (agents) against the registered tool plugins.
func (v *SemanticValidator) checkToolExpressions(tree pipeline.SourceTree) []pipeline.Diagnostic {
	if v.toolRegistry == nil {
		return nil
	}

	var diags []pipeline.Diagnostic
	for _, obj := range tree.Objects {
		switch obj.Meta.Kind {
		case model.KindSkill:
			diags = append(diags, v.validateAllowedTools(obj)...)
			diags = append(diags, v.validateBashBinaryDeps(obj)...)
		case model.KindAgent:
			diags = append(diags, v.validateToolPolicy(obj)...)
		}
	}
	return diags
}

// validateAllowedTools checks the allowedTools field of a skill.
func (v *SemanticValidator) validateAllowedTools(obj pipeline.RawObject) []pipeline.Diagnostic {
	tools, ok := obj.RawFields["allowedTools"]
	if !ok {
		return nil
	}
	arr, ok := tools.([]any)
	if !ok {
		return nil
	}

	var diags []pipeline.Diagnostic
	for _, item := range arr {
		expr, ok := item.(string)
		if !ok {
			continue
		}
		if err := v.toolRegistry.ValidateExpression(expr); err != nil {
			severity := "warning"
			if _, isUnknown := err.(*tool.UnknownToolError); !isUnknown {
				severity = "error" // syntax errors are errors; unknown tools are warnings
			}
			diags = append(diags, pipeline.Diagnostic{
				Severity:   severity,
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("allowedTools: tool expression %q: %v", expr, err),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}
	return diags
}

// validateBashBinaryDeps cross-validates Bash(<command>:*) entries in
// allowedTools against the binaryDeps list. If a skill declares
// Bash(go:*) but "go" is not in binaryDeps, emit a warning.
func (v *SemanticValidator) validateBashBinaryDeps(obj pipeline.RawObject) []pipeline.Diagnostic {
	tools, ok := obj.RawFields["allowedTools"]
	if !ok {
		return nil
	}
	arr, ok := tools.([]any)
	if !ok {
		return nil
	}

	// Collect Bash command names from allowedTools.
	var bashCmds []string
	for _, item := range arr {
		expr, ok := item.(string)
		if !ok {
			continue
		}
		parsed := tool.ParseExpression(expr)
		if parsed.Keyword != "Bash" || parsed.Args == "" {
			continue
		}
		parts := splitBashArgs(parsed.Args)
		if parts[0] != "" {
			bashCmds = append(bashCmds, parts[0])
		}
	}

	if len(bashCmds) == 0 {
		return nil
	}

	// Collect binaryDeps set.
	binDeps := extractStringSlice(obj.RawFields, "binaryDeps")
	binSet := make(map[string]bool, len(binDeps))
	for _, b := range binDeps {
		binSet[b] = true
	}

	// If binaryDeps is declared, check that Bash commands are listed.
	if len(binDeps) == 0 {
		return nil // no binaryDeps declared — skip cross-validation
	}

	var diags []pipeline.Diagnostic
	for _, cmd := range bashCmds {
		if !binSet[cmd] {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "warning",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("allowedTools includes Bash(%s:*) but %q is not listed in binaryDeps", cmd, cmd),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}
	return diags
}

// splitBashArgs splits Bash args on ":" returning [command, glob].
func splitBashArgs(args string) [2]string {
	idx := indexByte(args, ':')
	if idx < 0 {
		return [2]string{args, ""}
	}
	return [2]string{args[:idx], args[idx+1:]}
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// extractStringSlice extracts a string slice from a raw fields map.
func extractStringSlice(fields map[string]any, key string) []string {
	val, ok := fields[key]
	if !ok {
		return nil
	}
	arr, ok := val.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// validateToolPolicy checks the toolPolicy keys of an agent.
func (v *SemanticValidator) validateToolPolicy(obj pipeline.RawObject) []pipeline.Diagnostic {
	policy, ok := obj.RawFields["toolPolicy"]
	if !ok {
		return nil
	}
	policyMap, ok := policy.(map[string]any)
	if !ok {
		return nil
	}

	var diags []pipeline.Diagnostic
	for key, val := range policyMap {
		// Validate the tool/capability key.
		if err := v.toolRegistry.ValidateExpression(key); err != nil {
			// Unknown tool that isn't a capability ID is worth warning about.
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "warning",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("toolPolicy: key %q: %v", key, err),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}

		// Validate the decision value.
		decision, ok := val.(string)
		if !ok {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("toolPolicy: value for %q must be a string", key),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
			continue
		}
		switch decision {
		case "allow", "deny", "ask":
			// valid
		default:
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("toolPolicy: invalid decision %q for %q (must be allow, deny, or ask)", decision, key),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}
	return diags
}
