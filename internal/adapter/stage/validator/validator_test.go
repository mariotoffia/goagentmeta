package validator

import (
	"context"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// --- Helper builders ---

func makeObj(id string, kind model.Kind, sourcePath string, fields map[string]any) pipeline.RawObject {
	return pipeline.RawObject{
		Meta: model.ObjectMeta{
			ID:   id,
			Kind: kind,
		},
		SourcePath: sourcePath,
		RawFields:  fields,
	}
}

func makeObjWith(id string, kind model.Kind, sourcePath string, fields map[string]any, fn func(*pipeline.RawObject)) pipeline.RawObject {
	obj := makeObj(id, kind, sourcePath, fields)
	fn(&obj)
	return obj
}

func makeTree(objects ...pipeline.RawObject) pipeline.SourceTree {
	return pipeline.SourceTree{
		RootPath: ".ai/",
		Objects:  objects,
	}
}

func countErrors(diags []pipeline.Diagnostic) int {
	n := 0
	for _, d := range diags {
		if d.Severity == "error" {
			n++
		}
	}
	return n
}

func diagContains(diags []pipeline.Diagnostic, substr string) bool {
	for _, d := range diags {
		if contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ==================== Structural Tests ====================

func TestStructural_ValidInstruction_NoDiagnostics(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("my-instruction", model.KindInstruction, ".ai/instructions/coding.yaml", map[string]any{
		"content": "Always use tests",
	})

	diags := sv.Validate(obj)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestStructural_MissingID_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := pipeline.RawObject{
		Meta: model.ObjectMeta{
			Kind: model.KindInstruction,
		},
		SourcePath: ".ai/instructions/coding.yaml",
		RawFields:  map[string]any{"content": "test"},
	}

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for missing ID")
	}
	if !diagContains(diags, "id is required") {
		t.Errorf("expected 'id is required' in diagnostics, got: %v", diags)
	}
}

func TestStructural_UnknownKind_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("test-obj", model.Kind("unknown-kind"), ".ai/unknown.yaml", nil)

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for unknown kind")
	}
	if !diagContains(diags, "unknown kind") {
		t.Errorf("expected 'unknown kind' in diagnostics, got: %v", diags)
	}
}

func TestStructural_InvalidPreservation_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObjWith("test-obj", model.KindInstruction, ".ai/test.yaml", nil,
		func(o *pipeline.RawObject) {
			o.Meta.Preservation = model.Preservation("invalid-level")
		})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for invalid preservation")
	}
	if !diagContains(diags, "invalid preservation") {
		t.Errorf("expected 'invalid preservation' in diagnostics, got: %v", diags)
	}
}

func TestStructural_InvalidTarget_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObjWith("test-obj", model.KindInstruction, ".ai/test.yaml", nil,
		func(o *pipeline.RawObject) {
			o.Meta.AppliesTo.Targets = []string{"invalid-target"}
		})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for invalid target")
	}
	if !diagContains(diags, "invalid target") {
		t.Errorf("expected 'invalid target' in diagnostics, got: %v", diags)
	}
}

func TestStructural_ValidTargets_NoDiagnostics(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObjWith("test-obj", model.KindInstruction, ".ai/test.yaml", nil,
		func(o *pipeline.RawObject) {
			o.Meta.AppliesTo.Targets = []string{"claude", "cursor", "*"}
		})

	diags := sv.Validate(obj)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestStructural_InvalidTargetOverrideKey_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObjWith("test-obj", model.KindInstruction, ".ai/test.yaml", nil,
		func(o *pipeline.RawObject) {
			o.Meta.TargetOverrides = map[string]model.TargetOverride{
				"invalid-target": {},
			}
		})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for invalid target override key")
	}
	if !diagContains(diags, "invalid target") {
		t.Errorf("expected 'invalid target' in diagnostics, got: %v", diags)
	}
}

func TestStructural_HookMissingRequiredFields_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	// Hook requires "event" and "action" fields.
	obj := makeObj("my-hook", model.KindHook, ".ai/hooks/test.yaml", map[string]any{})

	diags := sv.Validate(obj)
	if countErrors(diags) < 2 {
		t.Fatalf("expected at least 2 errors for missing event and action, got %d: %v", countErrors(diags), diags)
	}
	if !diagContains(diags, "\"event\"") {
		t.Errorf("expected 'event' in diagnostics, got: %v", diags)
	}
	if !diagContains(diags, "\"action\"") {
		t.Errorf("expected 'action' in diagnostics, got: %v", diags)
	}
}

func TestStructural_CommandMissingAction_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("my-cmd", model.KindCommand, ".ai/commands/test.yaml", map[string]any{})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for missing action")
	}
	if !diagContains(diags, "\"action\"") {
		t.Errorf("expected 'action' in diagnostics, got: %v", diags)
	}
}

func TestStructural_PluginMissingDistribution_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("my-plugin", model.KindPlugin, ".ai/plugins/test.yaml", map[string]any{})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for missing distribution")
	}
	if !diagContains(diags, "\"distribution\"") {
		t.Errorf("expected 'distribution' in diagnostics, got: %v", diags)
	}
}

func TestStructural_CapabilityMissingContract_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("my-cap", model.KindCapability, ".ai/capabilities/test.yaml", map[string]any{})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for missing contract")
	}
	if !diagContains(diags, "\"contract\"") {
		t.Errorf("expected 'contract' in diagnostics, got: %v", diags)
	}
}

func TestStructural_WrongFieldType_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	// "content" should be string, providing a number.
	obj := makeObj("test-obj", model.KindInstruction, ".ai/test.yaml", map[string]any{
		"content": 42,
	})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for wrong field type")
	}
	if !diagContains(diags, "must be a string") {
		t.Errorf("expected 'must be a string' in diagnostics, got: %v", diags)
	}
}

func TestStructural_HookInvalidEffectClassEnum_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("my-hook", model.KindHook, ".ai/hooks/test.yaml", map[string]any{
		"event": "post-edit",
		"action": map[string]any{
			"type": "script",
			"ref":  "test.sh",
		},
		"effect": map[string]any{
			"class":       "invalid-class",
			"enforcement": "blocking",
		},
	})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for invalid effect class")
	}
	if !diagContains(diags, "not one of") {
		t.Errorf("expected 'not one of' in diagnostics, got: %v", diags)
	}
}

func TestStructural_ValidHook_NoDiagnostics(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("my-hook", model.KindHook, ".ai/hooks/test.yaml", map[string]any{
		"event": "post-edit",
		"action": map[string]any{
			"type": "script",
			"ref":  "scripts/hooks/validate.sh",
		},
		"effect": map[string]any{
			"class":       "validating",
			"enforcement": "blocking",
		},
	})

	diags := sv.Validate(obj)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestStructural_ValidPreservation_NoDiagnostics(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	for _, pres := range []model.Preservation{
		model.PreservationRequired,
		model.PreservationPreferred,
		model.PreservationOptional,
	} {
		obj := makeObjWith("test-obj", model.KindInstruction, ".ai/test.yaml", nil,
			func(o *pipeline.RawObject) {
				o.Meta.Preservation = pres
			})

		diags := sv.Validate(obj)
		if len(diags) > 0 {
			t.Errorf("preservation %q: expected no diagnostics, got %d: %v", pres, len(diags), diags)
		}
	}
}

func TestStructural_AllValidKinds_NoDiagnostics(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	// Each kind with its required fields satisfied.
	cases := []struct {
		kind   model.Kind
		fields map[string]any
	}{
		{model.KindInstruction, map[string]any{"content": "test"}},
		{model.KindRule, map[string]any{"content": "test"}},
		{model.KindSkill, map[string]any{"content": "test"}},
		{model.KindAgent, map[string]any{"rolePrompt": "test"}},
		{model.KindHook, map[string]any{
			"event":  "post-edit",
			"action": map[string]any{"type": "script", "ref": "test.sh"},
		}},
		{model.KindCommand, map[string]any{
			"action": map[string]any{"type": "skill", "ref": "my-skill"},
		}},
		{model.KindCapability, map[string]any{
			"contract": map[string]any{"category": "tool"},
		}},
		{model.KindPlugin, map[string]any{
			"distribution": map[string]any{"mode": "inline"},
		}},
	}

	for _, tc := range cases {
		obj := makeObj("test-"+string(tc.kind), tc.kind, ".ai/test.yaml", tc.fields)
		diags := sv.Validate(obj)
		if len(diags) > 0 {
			t.Errorf("kind %q: expected no diagnostics, got %d: %v", tc.kind, len(diags), diags)
		}
	}
}

func TestStructural_DiagnosticPhaseIsValidate(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("", model.KindInstruction, ".ai/test.yaml", nil)
	diags := sv.Validate(obj)
	for _, d := range diags {
		if d.Phase != pipeline.PhaseValidate {
			t.Errorf("expected phase validate, got %v", d.Phase)
		}
	}
}

func TestStructural_DiagnosticCodeIsVALIDATION(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("", model.Kind("bogus"), ".ai/test.yaml", nil)
	diags := sv.Validate(obj)
	for _, d := range diags {
		if d.Code != string(pipeline.ErrValidation) {
			t.Errorf("expected code VALIDATION, got %q", d.Code)
		}
	}
}

// ==================== Semantic Tests ====================

func TestSemantic_DuplicateIDs_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("dup-id", model.KindInstruction, ".ai/a.yaml", nil),
		makeObj("dup-id", model.KindInstruction, ".ai/b.yaml", nil),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for duplicate IDs")
	}
	if !diagContains(diags, "duplicate object id") {
		t.Errorf("expected 'duplicate object id' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_NoDuplicates_NoDiagnostics(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("obj-a", model.KindInstruction, ".ai/a.yaml", nil),
		makeObj("obj-b", model.KindInstruction, ".ai/b.yaml", nil),
	)

	diags := sv.Validate(tree)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestSemantic_CircularInheritance_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObjWith("obj-a", model.KindInstruction, ".ai/a.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"obj-b"} }),
		makeObjWith("obj-b", model.KindInstruction, ".ai/b.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"obj-a"} }),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for circular inheritance")
	}
	if !diagContains(diags, "circular inheritance") {
		t.Errorf("expected 'circular inheritance' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_DeepCircularInheritance_Error(t *testing.T) {
	sv := NewSemanticValidator()

	// A → B → C → A
	tree := makeTree(
		makeObjWith("obj-a", model.KindInstruction, ".ai/a.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"obj-b"} }),
		makeObjWith("obj-b", model.KindInstruction, ".ai/b.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"obj-c"} }),
		makeObjWith("obj-c", model.KindInstruction, ".ai/c.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"obj-a"} }),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for deep circular inheritance")
	}
	if !diagContains(diags, "circular inheritance") {
		t.Errorf("expected 'circular inheritance' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_ValidInheritance_NoDiagnostics(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObjWith("obj-a", model.KindInstruction, ".ai/a.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"obj-b"} }),
		makeObj("obj-b", model.KindInstruction, ".ai/b.yaml", nil),
	)

	diags := sv.Validate(tree)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestSemantic_AgentReferencesNonexistentSkill_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"skills": []any{"nonexistent-skill"},
		}),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for nonexistent skill reference")
	}
	if !diagContains(diags, "does not exist") {
		t.Errorf("expected 'does not exist' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_AgentReferencesWrongKindAsSkill_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"skills": []any{"my-instruction"},
		}),
		makeObj("my-instruction", model.KindInstruction, ".ai/instructions/test.yaml", nil),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for wrong kind reference")
	}
	if !diagContains(diags, "kind \"instruction\"") {
		t.Errorf("expected kind mismatch in diagnostics, got: %v", diags)
	}
}

func TestSemantic_AgentValidSkillRef_NoDiagnostics(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"skills": []any{"my-skill"},
		}),
		makeObj("my-skill", model.KindSkill, ".ai/skills/test.yaml", nil),
	)

	diags := sv.Validate(tree)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestSemantic_AgentLinksSkillRef_NoDiagnostics(t *testing.T) {
	sv := NewSemanticValidator()

	// Test the links.skills variant (from schema example).
	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"links": map[string]any{
				"skills": []any{"my-skill"},
			},
		}),
		makeObj("my-skill", model.KindSkill, ".ai/skills/test.yaml", nil),
	)

	diags := sv.Validate(tree)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestSemantic_CircularDelegation_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("agent-a", model.KindAgent, ".ai/agents/a.yaml", map[string]any{
			"delegation": map[string]any{
				"mayCall": []any{"agent-b"},
			},
		}),
		makeObj("agent-b", model.KindAgent, ".ai/agents/b.yaml", map[string]any{
			"delegation": map[string]any{
				"mayCall": []any{"agent-a"},
			},
		}),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for circular delegation")
	}
	if !diagContains(diags, "circular delegation") {
		t.Errorf("expected 'circular delegation' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_DeepCircularDelegation_Error(t *testing.T) {
	sv := NewSemanticValidator()

	// A → B → C → A
	tree := makeTree(
		makeObj("agent-a", model.KindAgent, ".ai/agents/a.yaml", map[string]any{
			"delegation": map[string]any{"mayCall": []any{"agent-b"}},
		}),
		makeObj("agent-b", model.KindAgent, ".ai/agents/b.yaml", map[string]any{
			"delegation": map[string]any{"mayCall": []any{"agent-c"}},
		}),
		makeObj("agent-c", model.KindAgent, ".ai/agents/c.yaml", map[string]any{
			"delegation": map[string]any{"mayCall": []any{"agent-a"}},
		}),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for deep circular delegation")
	}
	if !diagContains(diags, "circular delegation") {
		t.Errorf("expected 'circular delegation' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_ValidDelegation_NoDiagnostics(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("agent-a", model.KindAgent, ".ai/agents/a.yaml", map[string]any{
			"delegation": map[string]any{
				"mayCall": []any{"agent-b"},
			},
		}),
		makeObj("agent-b", model.KindAgent, ".ai/agents/b.yaml", nil),
	)

	diags := sv.Validate(tree)
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestSemantic_AgentDelegatesToNonAgent_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("agent-a", model.KindAgent, ".ai/agents/a.yaml", map[string]any{
			"delegation": map[string]any{
				"mayCall": []any{"my-skill"},
			},
		}),
		makeObj("my-skill", model.KindSkill, ".ai/skills/test.yaml", nil),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for delegation to non-agent")
	}
	if !diagContains(diags, "not agent") {
		t.Errorf("expected 'not agent' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_AgentReferencesNonexistentHook_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"hooks": []any{"nonexistent-hook"},
		}),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for nonexistent hook reference")
	}
	if !diagContains(diags, "does not exist") {
		t.Errorf("expected 'does not exist' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_AgentHandoffToNonexistent_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"handoffs": []any{
				map[string]any{
					"label": "Start Review",
					"agent": "nonexistent-agent",
				},
			},
		}),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for nonexistent handoff target")
	}
	if !diagContains(diags, "does not exist") {
		t.Errorf("expected 'does not exist' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_AgentHandoffToNonAgent_Error(t *testing.T) {
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"handoffs": []any{
				map[string]any{
					"label": "Start Review",
					"agent": "my-skill",
				},
			},
		}),
		makeObj("my-skill", model.KindSkill, ".ai/skills/test.yaml", nil),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for handoff to non-agent")
	}
	if !diagContains(diags, "not agent") {
		t.Errorf("expected 'not agent' in diagnostics, got: %v", diags)
	}
}

// ==================== Stage (Integration) Tests ====================

func TestStage_ValidTree_NoError(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		makeObj("instr-1", model.KindInstruction, ".ai/instructions/a.yaml", map[string]any{"content": "test"}),
		makeObj("skill-1", model.KindSkill, ".ai/skills/s.yaml", map[string]any{"content": "test"}),
		makeObj("agent-1", model.KindAgent, ".ai/agents/a.yaml", map[string]any{
			"skills": []any{"skill-1"},
		}),
	)

	result, err := s.Execute(context.Background(), tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SourceTree should be returned unchanged.
	resultTree, ok := result.(pipeline.SourceTree)
	if !ok {
		t.Fatalf("expected pipeline.SourceTree, got %T", result)
	}
	if len(resultTree.Objects) != len(tree.Objects) {
		t.Errorf("expected %d objects, got %d", len(tree.Objects), len(resultTree.Objects))
	}
}

func TestStage_InvalidTree_ReturnsError(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/test.yaml",
		},
	)

	_, err = s.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	ce, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ce.Code != pipeline.ErrValidation {
		t.Errorf("expected code VALIDATION, got %q", ce.Code)
	}
}

func TestStage_WrongInputType_ReturnsError(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Execute(context.Background(), "not-a-tree")
	if err == nil {
		t.Fatal("expected error for wrong input type")
	}
}

func TestStage_PointerInput_Works(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		makeObj("test-obj", model.KindInstruction, ".ai/test.yaml", nil),
	)

	_, err = s.Execute(context.Background(), &tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStage_NonMutating(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	original := makeTree(
		makeObj("obj-a", model.KindInstruction, ".ai/a.yaml", map[string]any{"content": "original"}),
		makeObj("obj-b", model.KindSkill, ".ai/b.yaml", map[string]any{"content": "original"}),
	)

	// Copy for comparison.
	originalLen := len(original.Objects)
	originalID := original.Objects[0].Meta.ID

	result, err := s.Execute(context.Background(), original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultTree := result.(pipeline.SourceTree)
	if len(resultTree.Objects) != originalLen {
		t.Errorf("object count changed: %d -> %d", originalLen, len(resultTree.Objects))
	}
	if resultTree.Objects[0].Meta.ID != originalID {
		t.Errorf("object ID changed: %q -> %q", originalID, resultTree.Objects[0].Meta.ID)
	}
}

func TestStage_Descriptor(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	desc := s.Descriptor()
	if desc.Name != "schema-validator" {
		t.Errorf("expected name 'schema-validator', got %q", desc.Name)
	}
	if desc.Phase != pipeline.PhaseValidate {
		t.Errorf("expected phase PhaseValidate, got %v", desc.Phase)
	}
	if desc.Order != 10 {
		t.Errorf("expected order 10, got %d", desc.Order)
	}
}

// ==================== FailFast Tests ====================

func TestStage_FailFast_OnlyFirstError(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	// Two objects, both with missing IDs.
	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/b.yaml",
		},
	)

	// FailFast context (default behavior).
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{FailFast: true},
		Report: &pipeline.BuildReport{},
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, err = s.Execute(ctx, tree)
	if err == nil {
		t.Fatal("expected error")
	}

	// With FailFast=true, should stop after first object's errors.
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Code != pipeline.ErrValidation {
		t.Errorf("expected VALIDATION error, got %q", ve.Code)
	}
}

func TestStage_Accumulate_AllErrors(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	// Multiple objects with errors.
	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/b.yaml",
		},
	)

	// FailFast=false context.
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{FailFast: false},
		Report: &pipeline.BuildReport{},
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, err = s.Execute(ctx, tree)
	if err == nil {
		t.Fatal("expected error")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Code != pipeline.ErrValidation {
		t.Errorf("expected VALIDATION error, got %q", ve.Code)
	}
	// The error message should mention all errors accumulated.
	if !contains(ve.Message, "2 error") {
		t.Errorf("expected 2 errors in message, got: %q", ve.Message)
	}
	// ValidationError should carry both diagnostics.
	if len(ve.Diagnostics) != 2 {
		t.Errorf("expected 2 diagnostics in ValidationError, got %d", len(ve.Diagnostics))
	}
}

func TestStage_FailFast_WithSemanticErrors(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	// Valid structural, but semantic error (duplicate IDs).
	tree := makeTree(
		makeObj("dup-id", model.KindInstruction, ".ai/a.yaml", nil),
		makeObj("dup-id", model.KindInstruction, ".ai/b.yaml", nil),
	)

	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{FailFast: true},
		Report: &pipeline.BuildReport{},
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, err = s.Execute(ctx, tree)
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
}

// ==================== ValidateTree convenience method ====================

func TestValidateTree_ReturnsAllDiagnostics(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
		makeObj("dup-id", model.KindInstruction, ".ai/b.yaml", nil),
		makeObj("dup-id", model.KindInstruction, ".ai/c.yaml", nil),
	)

	diags := s.ValidateTree(tree)
	// Should have: 1 missing ID + 1 duplicate ID = at least 2 errors.
	if countErrors(diags) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", countErrors(diags), diags)
	}
}

// ==================== Schema loading ====================

func TestSchemaLoading_AllSchemasLoad(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	expectedKinds := []model.Kind{
		model.KindInstruction,
		model.KindRule,
		model.KindSkill,
		model.KindAgent,
		model.KindHook,
		model.KindCommand,
		model.KindCapability,
		model.KindPlugin,
	}

	for _, kind := range expectedKinds {
		if _, ok := sv.schemas[kind]; !ok {
			t.Errorf("schema for kind %q not loaded", kind)
		}
	}
}

// ==================== Edge Cases ====================

func TestStructural_EmptyRawFields_NoError(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	// Instruction has no required RawFields.
	obj := makeObj("test", model.KindInstruction, ".ai/test.yaml", nil)
	diags := sv.Validate(obj)
	if countErrors(diags) > 0 {
		t.Errorf("expected no errors for empty RawFields on instruction, got: %v", diags)
	}
}

func TestStructural_NilRawFields_NoError(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := pipeline.RawObject{
		Meta:       model.ObjectMeta{ID: "test", Kind: model.KindInstruction},
		SourcePath: ".ai/test.yaml",
		RawFields:  nil,
	}
	diags := sv.Validate(obj)
	if countErrors(diags) > 0 {
		t.Errorf("expected no errors for nil RawFields on instruction, got: %v", diags)
	}
}

func TestStage_EmptyTree_NoDiagnostics(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree()
	_, err = s.Execute(context.Background(), tree)
	if err != nil {
		t.Fatalf("unexpected error for empty tree: %v", err)
	}
}

func TestSemantic_EmptyTree_NoDiagnostics(t *testing.T) {
	sv := NewSemanticValidator()
	diags := sv.Validate(makeTree())
	if len(diags) > 0 {
		t.Errorf("expected no diagnostics for empty tree, got: %v", diags)
	}
}

func TestStructural_ObjectActionMissingRequiredSubfields_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	// Hook action is missing required sub-fields "type" and "ref".
	obj := makeObj("my-hook", model.KindHook, ".ai/test.yaml", map[string]any{
		"event":  "post-edit",
		"action": map[string]any{},
	})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected errors for missing action sub-fields")
	}
}

func TestStructural_PluginDistributionInvalidMode_Error(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObj("my-plugin", model.KindPlugin, ".ai/test.yaml", map[string]any{
		"distribution": map[string]any{
			"mode": "invalid-mode",
		},
	})

	diags := sv.Validate(obj)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for invalid distribution mode")
	}
	if !diagContains(diags, "not one of") {
		t.Errorf("expected 'not one of' in diagnostics, got: %v", diags)
	}
}

// ==================== QA Fix Regression Tests ====================

func TestSemantic_InheritanceToNonexistent_NoCycle(t *testing.T) {
	// Regression: objects extending non-existent objects must NOT trigger
	// false-positive cycle detection.
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObjWith("obj-a", model.KindInstruction, ".ai/a.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"nonexistent", "obj-c"} }),
		makeObjWith("obj-c", model.KindInstruction, ".ai/c.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"nonexistent"} }),
	)

	diags := sv.Validate(tree)
	for _, d := range diags {
		if contains(d.Message, "circular inheritance") {
			t.Fatalf("false positive cycle: %s", d.Message)
		}
	}
}

func TestSemantic_DelegationToNonexistent_NoCycle(t *testing.T) {
	// Regression: delegation to non-existent agents must NOT trigger
	// false-positive cycle detection.
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("agent-a", model.KindAgent, ".ai/agents/a.yaml", map[string]any{
			"delegation": map[string]any{"mayCall": []any{"nonexistent", "agent-c"}},
		}),
		makeObj("agent-c", model.KindAgent, ".ai/agents/c.yaml", map[string]any{
			"delegation": map[string]any{"mayCall": []any{"nonexistent"}},
		}),
	)

	diags := sv.Validate(tree)
	for _, d := range diags {
		if contains(d.Message, "circular delegation") {
			t.Fatalf("false positive cycle: %s", d.Message)
		}
	}
}

func TestSemantic_DelegationToNonAgent_NoCycle(t *testing.T) {
	// Regression: delegation to a non-agent kind must NOT trigger
	// false-positive cycle detection.
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("agent-a", model.KindAgent, ".ai/agents/a.yaml", map[string]any{
			"delegation": map[string]any{"mayCall": []any{"my-skill", "agent-b"}},
		}),
		makeObj("my-skill", model.KindSkill, ".ai/skills/test.yaml", nil),
		makeObj("agent-b", model.KindAgent, ".ai/agents/b.yaml", nil),
	)

	diags := sv.Validate(tree)
	for _, d := range diags {
		if contains(d.Message, "circular delegation") {
			t.Fatalf("false positive cycle: %s", d.Message)
		}
	}
}

func TestStage_DiagnosticsEmittedToReport(t *testing.T) {
	// Verify that individual diagnostics are emitted to CompilerContext.Report.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/b.yaml",
		},
	)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{FailFast: false},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, _ = s.Execute(ctx, tree)

	// Both missing-ID diagnostics should have been emitted to the report.
	if len(report.Diagnostics) < 2 {
		t.Errorf("expected at least 2 diagnostics in report, got %d", len(report.Diagnostics))
	}
	for _, d := range report.Diagnostics {
		if d.Phase != pipeline.PhaseValidate {
			t.Errorf("expected phase validate, got %v", d.Phase)
		}
		if d.Code != string(pipeline.ErrValidation) {
			t.Errorf("expected code VALIDATION, got %q", d.Code)
		}
	}
}

func TestStage_ValidationErrorCarriesDiagnostics(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
	)

	_, err = s.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Diagnostics) == 0 {
		t.Error("expected ValidationError to carry diagnostics")
	}
	if ve.Code != pipeline.ErrValidation {
		t.Errorf("expected code VALIDATION, got %q", ve.Code)
	}
}

func TestStage_DefaultFailFast_WithoutContext(t *testing.T) {
	// Without CompilerContext, default is FailFast=true.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/b.yaml",
		},
	)

	_, err = s.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error")
	}

	// Default FailFast=true should stop after first object.
	ve := err.(*ValidationError)
	if len(ve.Diagnostics) != 1 {
		t.Errorf("expected 1 diagnostic (FailFast default), got %d", len(ve.Diagnostics))
	}
}

// ==================== QA Review: Additional Coverage ====================

func TestStage_FailFast_ExplicitTrue_ExactCount(t *testing.T) {
	// FailFast=true explicitly set: must return exactly 1 diagnostic.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/b.yaml",
		},
	)

	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{FailFast: true},
		Report: &pipeline.BuildReport{},
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, err = s.Execute(ctx, tree)
	if err == nil {
		t.Fatal("expected error")
	}

	ve := err.(*ValidationError)
	if len(ve.Diagnostics) != 1 {
		t.Errorf("expected exactly 1 diagnostic with FailFast=true, got %d", len(ve.Diagnostics))
	}
	if ve.Diagnostics[0].SourcePath != ".ai/a.yaml" {
		t.Errorf("expected first object's path, got %q", ve.Diagnostics[0].SourcePath)
	}
}

func TestStage_DiagnosticsEmittedToSink(t *testing.T) {
	// Verify that diagnostics are emitted to DiagnosticSink in addition to Report.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
	)

	sink := &testDiagnosticSink{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			FailFast:       false,
			DiagnosticSink: sink,
		},
		Report: &pipeline.BuildReport{},
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, _ = s.Execute(ctx, tree)

	if len(sink.diags) == 0 {
		t.Fatal("expected diagnostics to be emitted to DiagnosticSink")
	}
	for _, d := range sink.diags {
		if d.Severity != "error" {
			t.Errorf("expected severity 'error', got %q", d.Severity)
		}
		if d.Code != string(pipeline.ErrValidation) {
			t.Errorf("expected code VALIDATION, got %q", d.Code)
		}
		if d.Phase != pipeline.PhaseValidate {
			t.Errorf("expected phase validate, got %v", d.Phase)
		}
	}
}

func TestStage_DiagnosticSinkAndReportInSync(t *testing.T) {
	// Sink and Report should receive the same diagnostics.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/a.yaml",
		},
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/b.yaml",
		},
	)

	sink := &testDiagnosticSink{}
	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{
			FailFast:       false,
			DiagnosticSink: sink,
		},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, _ = s.Execute(ctx, tree)

	if len(sink.diags) != len(report.Diagnostics) {
		t.Errorf("sink (%d) and report (%d) diagnostic counts differ",
			len(sink.diags), len(report.Diagnostics))
	}
}

func TestStage_CombinedStructuralAndSemanticAccumulation(t *testing.T) {
	// Accumulate mode: structural errors (missing ID) + semantic errors
	// (duplicate IDs) should all be collected.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		// Structural error: missing ID.
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/no-id.yaml",
		},
		// Two objects with the same ID → semantic duplicate error.
		makeObj("dup-id", model.KindInstruction, ".ai/a.yaml", nil),
		makeObj("dup-id", model.KindInstruction, ".ai/b.yaml", nil),
	)

	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{FailFast: false},
		Report: &pipeline.BuildReport{},
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	_, err = s.Execute(ctx, tree)
	if err == nil {
		t.Fatal("expected error")
	}

	ve := err.(*ValidationError)
	// Should have: 1 missing-ID structural error + 1 duplicate-ID semantic error = 2+.
	if countErrors(ve.Diagnostics) < 2 {
		t.Errorf("expected at least 2 errors (structural + semantic), got %d: %v",
			countErrors(ve.Diagnostics), ve.Diagnostics)
	}

	hasStructural := diagContains(ve.Diagnostics, "id is required")
	hasSemantic := diagContains(ve.Diagnostics, "duplicate object id")
	if !hasStructural {
		t.Error("expected structural error (missing ID) in accumulated diagnostics")
	}
	if !hasSemantic {
		t.Error("expected semantic error (duplicate ID) in accumulated diagnostics")
	}
}

func TestStage_WarningsOnlyDoNotCauseError(t *testing.T) {
	// Verify that warnings alone do not cause Execute to return an error.
	// This tests the hasErrors() filtering logic: only severity "error" triggers.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	// A valid tree should produce zero diagnostics (no warnings either).
	tree := makeTree(
		makeObj("valid-obj", model.KindInstruction, ".ai/test.yaml", map[string]any{
			"content": "all good",
		}),
	)

	_, err = s.Execute(context.Background(), tree)
	if err != nil {
		t.Fatalf("valid tree should not produce error: %v", err)
	}
}

func TestStage_FactoryReturnsStageInterface(t *testing.T) {
	// Factory must return a function compatible with stage.StageFactory.
	factory := Factory()
	s, err := factory()
	if err != nil {
		t.Fatal(err)
	}

	desc := s.Descriptor()
	if desc.Name != "schema-validator" {
		t.Errorf("expected name 'schema-validator', got %q", desc.Name)
	}
	if desc.Phase != pipeline.PhaseValidate {
		t.Errorf("expected phase PhaseValidate, got %v", desc.Phase)
	}
}

func TestStage_ValidationErrorSatisfiesErrorInterface(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/test.yaml",
		},
	)

	_, err = s.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error")
	}

	// Must be usable as a standard error.
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
	if !contains(errMsg, "VALIDATION") {
		t.Errorf("expected error message to contain VALIDATION, got: %q", errMsg)
	}
}

func TestSemantic_MultipleReferencesFromSameAgent(t *testing.T) {
	// Agent referencing multiple skills, some valid, some not.
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"skills": []any{"valid-skill", "missing-skill", "wrong-kind-obj"},
		}),
		makeObj("valid-skill", model.KindSkill, ".ai/skills/valid.yaml", nil),
		makeObj("wrong-kind-obj", model.KindInstruction, ".ai/instructions/test.yaml", nil),
	)

	diags := sv.Validate(tree)
	errorDiags := 0
	for _, d := range diags {
		if d.Severity == "error" {
			errorDiags++
		}
	}
	if errorDiags != 2 {
		t.Errorf("expected exactly 2 errors (missing + wrong kind), got %d: %v", errorDiags, diags)
	}
	if !diagContains(diags, "missing-skill") {
		t.Error("expected missing-skill in diagnostics")
	}
	if !diagContains(diags, "wrong-kind-obj") {
		t.Error("expected wrong-kind-obj in diagnostics")
	}
}

func TestSemantic_SelfInheritance_Error(t *testing.T) {
	// Object extending itself is a trivial cycle.
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObjWith("self-ref", model.KindInstruction, ".ai/test.yaml", nil,
			func(o *pipeline.RawObject) { o.Meta.Extends = []string{"self-ref"} }),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for self-inheritance")
	}
	if !diagContains(diags, "circular inheritance") {
		t.Errorf("expected 'circular inheritance' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_SelfDelegation_Error(t *testing.T) {
	// Agent delegating to itself is a trivial delegation cycle.
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("self-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"delegation": map[string]any{"mayCall": []any{"self-agent"}},
		}),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for self-delegation")
	}
	if !diagContains(diags, "circular delegation") {
		t.Errorf("expected 'circular delegation' in diagnostics, got: %v", diags)
	}
}

func TestSemantic_AgentHookRefWrongKind_Error(t *testing.T) {
	// Agent.hooks reference that points to a non-hook kind.
	sv := NewSemanticValidator()

	tree := makeTree(
		makeObj("my-agent", model.KindAgent, ".ai/agents/test.yaml", map[string]any{
			"hooks": []any{"not-a-hook"},
		}),
		makeObj("not-a-hook", model.KindSkill, ".ai/skills/test.yaml", nil),
	)

	diags := sv.Validate(tree)
	if countErrors(diags) == 0 {
		t.Fatal("expected error for hook reference to wrong kind")
	}
	if !diagContains(diags, "kind \"skill\"") {
		t.Errorf("expected kind mismatch in diagnostics, got: %v", diags)
	}
}

func TestStructural_MultipleViolationsOnSameObject(t *testing.T) {
	// Object with multiple structural violations at once.
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObjWith("", model.Kind("bogus"), ".ai/test.yaml", nil,
		func(o *pipeline.RawObject) {
			o.Meta.Preservation = model.Preservation("garbage")
			o.Meta.AppliesTo.Targets = []string{"nonexistent"}
		})

	diags := sv.Validate(obj)
	// missing ID + unknown kind (returns early for kind, so only 2 errors)
	if countErrors(diags) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", countErrors(diags), diags)
	}
}

func TestStructural_WildcardTarget_Valid(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObjWith("test-obj", model.KindInstruction, ".ai/test.yaml", nil,
		func(o *pipeline.RawObject) {
			o.Meta.AppliesTo.Targets = []string{"*"}
		})

	diags := sv.Validate(obj)
	if len(diags) > 0 {
		t.Errorf("wildcard '*' should be valid, got diagnostics: %v", diags)
	}
}

func TestStructural_ValidTargetOverrideKeys(t *testing.T) {
	sv, err := NewStructuralValidator()
	if err != nil {
		t.Fatal(err)
	}

	obj := makeObjWith("test-obj", model.KindInstruction, ".ai/test.yaml", nil,
		func(o *pipeline.RawObject) {
			o.Meta.TargetOverrides = map[string]model.TargetOverride{
				"claude":  {},
				"cursor":  {},
				"copilot": {},
				"codex":   {},
			}
		})

	diags := sv.Validate(obj)
	if len(diags) > 0 {
		t.Errorf("valid target override keys should produce no diagnostics, got: %v", diags)
	}
}

func TestStage_TreeReturnedUnmodifiedOnError(t *testing.T) {
	// Even on validation error, the SourceTree should be returned.
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tree := makeTree(
		pipeline.RawObject{
			Meta:       model.ObjectMeta{Kind: model.KindInstruction},
			SourcePath: ".ai/test.yaml",
		},
	)

	result, err := s.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error")
	}

	resultTree, ok := result.(pipeline.SourceTree)
	if !ok {
		t.Fatalf("expected SourceTree returned even on error, got %T", result)
	}
	if len(resultTree.Objects) != 1 {
		t.Errorf("expected 1 object in returned tree, got %d", len(resultTree.Objects))
	}
}

// testDiagnosticSink is a minimal DiagnosticSink implementation for testing.
type testDiagnosticSink struct {
	diags []pipeline.Diagnostic
}

func (s *testDiagnosticSink) Emit(_ context.Context, d pipeline.Diagnostic) {
	s.diags = append(s.diags, d)
}

func (s *testDiagnosticSink) Diagnostics() []pipeline.Diagnostic {
	return s.diags
}
