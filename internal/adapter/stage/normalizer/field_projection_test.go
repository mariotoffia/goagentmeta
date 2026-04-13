package normalizer

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

// ---------------------------------------------------------------------------
// projectMetaIntoFields
// ---------------------------------------------------------------------------

func TestProjectMetaIntoFields_Description(t *testing.T) {
	meta := &model.ObjectMeta{Description: "My skill description"}
	fields := map[string]any{}

	projectMetaIntoFields(meta, fields)

	got, ok := fields["description"].(string)
	if !ok || got != "My skill description" {
		t.Errorf("description = %q, want %q", got, "My skill description")
	}
}

func TestProjectMetaIntoFields_EmptyDescription(t *testing.T) {
	meta := &model.ObjectMeta{Description: ""}
	fields := map[string]any{}

	projectMetaIntoFields(meta, fields)

	if _, exists := fields["description"]; exists {
		t.Error("empty description should not be projected into fields")
	}
}

func TestProjectMetaIntoFields_DoesNotOverrideExisting(t *testing.T) {
	meta := &model.ObjectMeta{Description: "meta version"}
	fields := map[string]any{"description": "field version"}

	projectMetaIntoFields(meta, fields)

	got := fields["description"].(string)
	if got != "field version" {
		t.Errorf("description = %q, want %q (should not override existing)", got, "field version")
	}
}

func TestProjectMetaIntoFields_NilFields(t *testing.T) {
	meta := &model.ObjectMeta{Description: "desc"}

	defer func() {
		if r := recover(); r == nil {
			t.Error("projectMetaIntoFields should panic on nil fields map")
		}
	}()

	// nil map assignment panics — caller must always provide a non-nil map.
	projectMetaIntoFields(meta, nil)
}

func TestProjectMetaIntoFields_UnicodeDescription(t *testing.T) {
	meta := &model.ObjectMeta{Description: "Überprüfe architektur — ñoño 日本語"}
	fields := map[string]any{}

	projectMetaIntoFields(meta, fields)

	got := fields["description"].(string)
	if got != meta.Description {
		t.Errorf("description = %q, want %q", got, meta.Description)
	}
}

func TestProjectMetaIntoFields_MultilineDescription(t *testing.T) {
	meta := &model.ObjectMeta{Description: "Line one\nLine two\nLine three"}
	fields := map[string]any{}

	projectMetaIntoFields(meta, fields)

	got := fields["description"].(string)
	if got != meta.Description {
		t.Errorf("description = %q, want %q", got, meta.Description)
	}
}

func TestProjectMetaIntoFields_WhitespaceOnlyDescription(t *testing.T) {
	// Whitespace-only is NOT empty string — it gets projected.
	meta := &model.ObjectMeta{Description: "   "}
	fields := map[string]any{}

	projectMetaIntoFields(meta, fields)

	if _, exists := fields["description"]; !exists {
		t.Error("whitespace-only description should be projected (non-empty string)")
	}
}

func TestProjectMetaIntoFields_PreservesOtherFields(t *testing.T) {
	meta := &model.ObjectMeta{Description: "desc"}
	fields := map[string]any{
		"tools":    []string{"Read", "Write"},
		"model":    "claude-4",
		"numField": 42,
	}

	projectMetaIntoFields(meta, fields)

	// Verify existing fields are untouched.
	if tools, ok := fields["tools"].([]string); !ok || len(tools) != 2 {
		t.Error("tools field was modified")
	}
	if fields["model"] != "claude-4" {
		t.Error("model field was modified")
	}
	if fields["numField"] != 42 {
		t.Error("numField was modified")
	}
	if fields["description"] != "desc" {
		t.Error("description was not added")
	}
}

// ---------------------------------------------------------------------------
// flattenNestedFields
// ---------------------------------------------------------------------------

func TestFlattenNestedFields_ActivationHints(t *testing.T) {
	fields := map[string]any{
		"activation": map[string]any{
			"hints": []any{"hint one", "hint two"},
		},
	}

	flattenNestedFields(fields)

	hints, ok := fields["activationHints"].([]any)
	if !ok {
		t.Fatalf("activationHints type = %T, want []any", fields["activationHints"])
	}
	if len(hints) != 2 {
		t.Fatalf("activationHints len = %d, want 2", len(hints))
	}
	if hints[0] != "hint one" || hints[1] != "hint two" {
		t.Errorf("activationHints = %v, want [hint one, hint two]", hints)
	}
}

func TestFlattenNestedFields_ActivationHintsStringSlice(t *testing.T) {
	// Some YAML parsers produce []string directly.
	fields := map[string]any{
		"activation": map[string]any{
			"hints": []string{"alpha", "beta"},
		},
	}

	flattenNestedFields(fields)

	// The function copies the value as-is, so it can be []string.
	hints, ok := fields["activationHints"].([]string)
	if !ok {
		t.Fatalf("activationHints type = %T, want []string", fields["activationHints"])
	}
	if len(hints) != 2 || hints[0] != "alpha" || hints[1] != "beta" {
		t.Errorf("activationHints = %v, want [alpha, beta]", hints)
	}
}

func TestFlattenNestedFields_NoActivation(t *testing.T) {
	fields := map[string]any{"tools": []string{"Read"}}

	flattenNestedFields(fields)

	if _, exists := fields["activationHints"]; exists {
		t.Error("activationHints should not exist when activation is absent")
	}
}

func TestFlattenNestedFields_ActivationWithoutHints(t *testing.T) {
	fields := map[string]any{
		"activation": map[string]any{
			"triggers": []string{"some-trigger"},
		},
	}

	flattenNestedFields(fields)

	if _, exists := fields["activationHints"]; exists {
		t.Error("activationHints should not exist when activation.hints is absent")
	}
}

func TestFlattenNestedFields_ActivationNotMap(t *testing.T) {
	fields := map[string]any{
		"activation": "not-a-map",
	}

	flattenNestedFields(fields)

	if _, exists := fields["activationHints"]; exists {
		t.Error("activationHints should not exist when activation is not a map")
	}
}

func TestFlattenNestedFields_DoesNotOverrideExisting(t *testing.T) {
	fields := map[string]any{
		"activation": map[string]any{
			"hints": []any{"nested-hint"},
		},
		"activationHints": []any{"explicit-hint"},
	}

	flattenNestedFields(fields)

	hints := fields["activationHints"].([]any)
	if len(hints) != 1 || hints[0] != "explicit-hint" {
		t.Errorf("activationHints = %v, want [explicit-hint] (should not override)", hints)
	}
}

func TestFlattenNestedFields_EmptyHintsSlice(t *testing.T) {
	fields := map[string]any{
		"activation": map[string]any{
			"hints": []any{},
		},
	}

	flattenNestedFields(fields)

	// Even empty slice is flattened (renderers will see len()==0 and skip).
	hints, ok := fields["activationHints"].([]any)
	if !ok {
		t.Fatalf("activationHints type = %T, want []any", fields["activationHints"])
	}
	if len(hints) != 0 {
		t.Errorf("activationHints len = %d, want 0", len(hints))
	}
}

func TestFlattenNestedFields_NilHintsValue(t *testing.T) {
	fields := map[string]any{
		"activation": map[string]any{
			"hints": nil,
		},
	}

	flattenNestedFields(fields)

	// nil is a valid value for the key — it's copied through.
	if _, exists := fields["activationHints"]; !exists {
		t.Error("activationHints should exist (nil value is still a value)")
	}
}

func TestFlattenNestedFields_PreservesActivationField(t *testing.T) {
	fields := map[string]any{
		"activation": map[string]any{
			"hints":    []any{"h1"},
			"triggers": []string{"t1"},
		},
	}

	flattenNestedFields(fields)

	// Original activation map must be preserved.
	act := fields["activation"].(map[string]any)
	if act["triggers"] == nil {
		t.Error("activation.triggers should still exist")
	}
}

func TestFlattenNestedFields_PreservesOtherFields(t *testing.T) {
	fields := map[string]any{
		"activation": map[string]any{"hints": []any{"h"}},
		"tools":      []string{"Read"},
		"delegation": map[string]any{"mayCall": []string{"agent-a"}},
	}

	flattenNestedFields(fields)

	if fields["tools"] == nil || fields["delegation"] == nil {
		t.Error("other fields should not be modified")
	}
}

// ---------------------------------------------------------------------------
// Combined: both functions together
// ---------------------------------------------------------------------------

func TestProjectAndFlatten_FullSkillFields(t *testing.T) {
	meta := &model.ObjectMeta{
		Description: "A useful skill",
	}
	fields := map[string]any{
		"tools": []any{"Read", "Write", "Bash(go:*)"},
		"activation": map[string]any{
			"hints": []any{"do something", "help me"},
		},
		"publishing": map[string]any{
			"author":   "testuser",
			"homepage": "https://example.com",
		},
	}

	projectMetaIntoFields(meta, fields)
	flattenNestedFields(fields)

	// description projected
	if fields["description"] != "A useful skill" {
		t.Errorf("description = %v, want %q", fields["description"], "A useful skill")
	}

	// activationHints flattened
	hints, ok := fields["activationHints"].([]any)
	if !ok || len(hints) != 2 {
		t.Errorf("activationHints = %v, want 2 hints", fields["activationHints"])
	}

	// tools preserved
	tools, ok := fields["tools"].([]any)
	if !ok || len(tools) != 3 {
		t.Error("tools should be preserved with 3 entries")
	}

	// publishing preserved
	pub := fields["publishing"].(map[string]any)
	if pub["author"] != "testuser" {
		t.Error("publishing.author should be preserved")
	}
}

func TestProjectAndFlatten_FullAgentFields(t *testing.T) {
	meta := &model.ObjectMeta{
		Description: "An agent that reviews",
	}
	fields := map[string]any{
		"tools":          []any{"Read", "Grep"},
		"disallowedTools": []any{"Write"},
		"skills":         []any{"skill-a", "skill-b"},
		"delegation":     map[string]any{"mayCall": []any{"other-agent"}},
	}

	projectMetaIntoFields(meta, fields)
	flattenNestedFields(fields)

	if fields["description"] != "An agent that reviews" {
		t.Errorf("description = %v", fields["description"])
	}
	// No activation → no activationHints
	if _, exists := fields["activationHints"]; exists {
		t.Error("activationHints should not exist for agents without activation")
	}
	// delegation preserved
	del := fields["delegation"].(map[string]any)
	if del["mayCall"] == nil {
		t.Error("delegation.mayCall should be preserved")
	}
}

func TestProjectAndFlatten_CommandFields(t *testing.T) {
	meta := &model.ObjectMeta{
		Description: "Build check command",
	}
	fields := map[string]any{
		"action": map[string]any{
			"type": "script",
			"ref":  "scripts/commands/build-check.sh",
		},
	}

	projectMetaIntoFields(meta, fields)
	flattenNestedFields(fields)

	if fields["description"] != "Build check command" {
		t.Errorf("description = %v", fields["description"])
	}
	action := fields["action"].(map[string]any)
	if action["type"] != "script" {
		t.Error("action.type should be preserved")
	}
}

func TestProjectAndFlatten_InstructionFields(t *testing.T) {
	meta := &model.ObjectMeta{
		Description: "Project context instruction",
	}
	fields := map[string]any{}

	projectMetaIntoFields(meta, fields)
	flattenNestedFields(fields)

	if fields["description"] != "Project context instruction" {
		t.Errorf("description = %v", fields["description"])
	}
	// Instructions typically have no extra fields.
	if len(fields) != 1 {
		t.Errorf("fields len = %d, want 1 (just description)", len(fields))
	}
}

func TestProjectAndFlatten_RuleFields(t *testing.T) {
	meta := &model.ObjectMeta{
		Description: "Adapter isolation rule",
	}
	fields := map[string]any{
		"severity":   "error",
		"conditions": map[string]any{"language": "go"},
	}

	projectMetaIntoFields(meta, fields)
	flattenNestedFields(fields)

	if fields["description"] != "Adapter isolation rule" {
		t.Errorf("description = %v", fields["description"])
	}
	if fields["severity"] != "error" {
		t.Error("severity should be preserved")
	}
}

func TestProjectAndFlatten_Idempotent(t *testing.T) {
	meta := &model.ObjectMeta{Description: "desc"}
	fields := map[string]any{
		"activation": map[string]any{"hints": []any{"h1"}},
	}

	// Run twice.
	projectMetaIntoFields(meta, fields)
	flattenNestedFields(fields)
	projectMetaIntoFields(meta, fields)
	flattenNestedFields(fields)

	if fields["description"] != "desc" {
		t.Error("description should remain stable after second pass")
	}
	hints := fields["activationHints"].([]any)
	if len(hints) != 1 || hints[0] != "h1" {
		t.Error("activationHints should remain stable after second pass")
	}
}
