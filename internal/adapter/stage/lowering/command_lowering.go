package lowering

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// LowerCommand transforms a command for a target based on its capability support.
//   - Copilot: commands are native (.prompt.md files) → pass through
//   - Claude/Codex: commands.explicitEntryPoints = lowered → merge into skill content
//   - Cursor: commands have no native equivalent → skip with record
func LowerCommand(
	obj pipeline.NormalizedObject,
	unitCaps pipeline.UnitCapabilities,
	target string,
) (pipeline.LoweredObject, PreservationResult) {
	cmdResolved := isCapabilityResolved(unitCaps, "commands.explicitEntryPoints")

	// Check the provider type for lowered support.
	if cmdResolved {
		provider := unitCaps.Resolved["commands.explicitEntryPoints"]

		// Lowered support (Claude/Codex) → merge into skill content.
		if strings.Contains(provider.ID, "/lowered/") {
			content := buildCommandSkillContent(obj)
			return pipeline.LoweredObject{
					OriginalID:   obj.Meta.ID,
					OriginalKind: model.KindCommand,
					LoweredKind:  model.KindSkill,
					Decision: pipeline.LoweringDecision{
						Action:       "lowered",
						Reason:       "command → skill content (explicitEntryPoints: lowered)",
						Preservation: obj.Meta.Preservation,
						Safe:         true,
					},
					Content: content,
					Fields:  obj.ResolvedFields,
				}, PreservationResult{
					Allowed:  true,
					Severity: "info",
					Message:  "command lowered to skill content",
				}
		}

		// Native or plugin/MCP-provided command support → pass through.
		return keepObject(obj, model.KindCommand, "target supports commands ("+provider.Type+")"), PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message:  "commands supported via " + provider.Type,
		}
	}

	// Not resolved → target doesn't support commands. Skip.
	preservation := obj.Meta.Preservation
	if preservation == "" {
		preservation = model.PreservationPreferred
	}

	decision := pipeline.LoweringDecision{
		Action:       "skipped",
		Reason:       fmt.Sprintf("command not supported by target %s", target),
		Preservation: preservation,
		Safe:         false,
	}
	result := CheckPreservation(decision, preservation)

	action := "skipped"
	if !result.Allowed && result.Severity == "error" {
		action = "failed"
	}

	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: model.KindCommand,
		LoweredKind:  model.KindCommand,
		Decision: pipeline.LoweringDecision{
			Action:       action,
			Reason:       decision.Reason,
			Preservation: preservation,
			Safe:         false,
		},
		Fields: obj.ResolvedFields,
	}, result
}

// CommandLoweringRecord produces a LoweringRecord for a command lowering.
func CommandLoweringRecord(lowered pipeline.LoweredObject) pipeline.LoweringRecord {
	return pipeline.LoweringRecord{
		ObjectID:     lowered.OriginalID,
		FromKind:     model.KindCommand,
		ToKind:       lowered.LoweredKind,
		Reason:       lowered.Decision.Reason,
		Preservation: lowered.Decision.Preservation,
		Status:       lowered.Decision.Action,
	}
}

func buildCommandSkillContent(obj pipeline.NormalizedObject) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Command: %s\n\n", obj.Meta.ID))

	if obj.Meta.Description != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", obj.Meta.Description))
	}

	if ref, ok := obj.ResolvedFields["actionRef"].(string); ok && ref != "" {
		sb.WriteString(fmt.Sprintf("Execute: `%s`\n", ref))
	}

	return sb.String()
}
