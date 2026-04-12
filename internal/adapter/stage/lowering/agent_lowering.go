package lowering

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// LowerAgent transforms an agent for a target that does not natively support
// agents. For targets without native agent file formats, agents are lowered
// to custom modes (adapted) or flattened into rule sets, depending on the
// target's capability level.
func LowerAgent(
	obj pipeline.NormalizedObject,
	unitCaps pipeline.UnitCapabilities,
) (pipeline.LoweredObject, PreservationResult) {
	subagentsResolved := isCapabilityResolved(unitCaps, "agents.subagents")

	// Native support → pass through unchanged.
	if subagentsResolved {
		return keepObject(obj, model.KindAgent, "target supports agents natively"), PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message:  "agents natively supported",
		}
	}

	preservation := obj.Meta.Preservation
	if preservation == "" {
		preservation = model.PreservationPreferred
	}

	// No native agent support → agent must be lowered or skipped.
	// Lowering an agent is unsafe (delegation, handoffs, tool policies lost).
	decision := pipeline.LoweringDecision{
		Action:       "lowered",
		Reason:       "agent → rule set (delegation/handoff semantics lost)",
		Preservation: preservation,
		Safe:         false,
	}

	result := CheckPreservation(decision, preservation)
	if !result.Allowed {
		action := "skipped"
		if result.Severity == "error" {
			action = "failed"
		}
		return pipeline.LoweredObject{
			OriginalID:   obj.Meta.ID,
			OriginalKind: model.KindAgent,
			LoweredKind:  model.KindAgent,
			Decision: pipeline.LoweringDecision{
				Action:       action,
				Reason:       decision.Reason,
				Preservation: preservation,
				Safe:         false,
			},
			Fields: obj.ResolvedFields,
		}, result
	}

	// Flatten agent into rule content.
	content := buildAgentRuleContent(obj)
	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: model.KindAgent,
		LoweredKind:  model.KindRule,
		Decision:     decision,
		Content:      content,
		Fields:       obj.ResolvedFields,
	}, result
}

// AgentLoweringRecord produces a LoweringRecord for an agent lowering.
func AgentLoweringRecord(lowered pipeline.LoweredObject) pipeline.LoweringRecord {
	return pipeline.LoweringRecord{
		ObjectID:     lowered.OriginalID,
		FromKind:     model.KindAgent,
		ToKind:       lowered.LoweredKind,
		Reason:       lowered.Decision.Reason,
		Preservation: lowered.Decision.Preservation,
		Status:       lowered.Decision.Action,
	}
}

func buildAgentRuleContent(obj pipeline.NormalizedObject) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Agent: %s\n\n", obj.Meta.ID))

	if obj.Meta.Description != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", obj.Meta.Description))
	}

	if rolePrompt, ok := obj.ResolvedFields["rolePrompt"].(string); ok && rolePrompt != "" {
		sb.WriteString("### Role\n\n")
		sb.WriteString(rolePrompt)
		if !strings.HasSuffix(rolePrompt, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
