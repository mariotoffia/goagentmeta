package lowering

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// LowerSkill transforms a skill for a target that does not natively support
// skills. For targets with adapted skill support (e.g., Cursor via plugins),
// skills may be lowered to rule content. For targets with native skill
// support, the skill passes through unchanged.
func LowerSkill(
	obj pipeline.NormalizedObject,
	unitCaps pipeline.UnitCapabilities,
) (pipeline.LoweredObject, PreservationResult) {
	bundlesResolved := isCapabilityResolved(unitCaps, "skills.bundles")

	// Native support → pass through unchanged.
	if bundlesResolved {
		return keepObject(obj, model.KindSkill, "target supports skills natively"), PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message:  "skills natively supported",
		}
	}

	preservation := obj.Meta.Preservation
	if preservation == "" {
		preservation = model.PreservationPreferred
	}

	// No native skill support → skill must be lowered or skipped.
	// Lowering a skill to rule content is unsafe (skill semantics are lost).
	decision := pipeline.LoweringDecision{
		Action:       "lowered",
		Reason:       "skill → rule content (skill activation/invocation semantics lost)",
		Preservation: preservation,
		Safe:         false,
	}

	result := CheckPreservation(decision, preservation)
	if !result.Allowed {
		// Preservation blocks this lowering.
		action := "skipped"
		if result.Severity == "error" {
			action = "failed"
		}
		return pipeline.LoweredObject{
			OriginalID:   obj.Meta.ID,
			OriginalKind: model.KindSkill,
			LoweredKind:  model.KindSkill,
			Decision: pipeline.LoweringDecision{
				Action:       action,
				Reason:       decision.Reason,
				Preservation: preservation,
				Safe:         false,
			},
			Fields: obj.ResolvedFields,
		}, result
	}

	// Preservation allows → lower skill to inline rule content.
	content := buildSkillRuleContent(obj)
	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: model.KindSkill,
		LoweredKind:  model.KindRule,
		Decision:     decision,
		Content:      content,
		Fields:       obj.ResolvedFields,
	}, result
}

// SkillLoweringRecord produces a LoweringRecord for a skill lowering.
func SkillLoweringRecord(lowered pipeline.LoweredObject) pipeline.LoweringRecord {
	return pipeline.LoweringRecord{
		ObjectID:     lowered.OriginalID,
		FromKind:     model.KindSkill,
		ToKind:       lowered.LoweredKind,
		Reason:       lowered.Decision.Reason,
		Preservation: lowered.Decision.Preservation,
		Status:       lowered.Decision.Action,
	}
}

func buildSkillRuleContent(obj pipeline.NormalizedObject) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Skill: %s\n\n", obj.Meta.ID))

	if obj.Meta.Description != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", obj.Meta.Description))
	}

	if content, ok := obj.ResolvedFields["content"].(string); ok && content != "" {
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
