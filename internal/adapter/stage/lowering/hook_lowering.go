package lowering

import (
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// LowerHook transforms a hook for a target. Hook lowering depends on:
//   - The target's support for hooks.lifecycle and hooks.blockingValidation
//   - The hook's effect class and enforcement mode
//   - The hook's preservation level
//
// Returns the lowered object and a preservation result indicating whether
// the lowering is allowed.
func LowerHook(
	obj pipeline.NormalizedObject,
	unitCaps pipeline.UnitCapabilities,
) (pipeline.LoweredObject, PreservationResult) {
	enforcement := extractEnforcement(obj)
	effectClass := extractEffectClass(obj)

	// Check if the target natively supports hooks.
	lifecycleResolved := isCapabilityResolved(unitCaps, "hooks.lifecycle")
	blockingResolved := isCapabilityResolved(unitCaps, "hooks.blockingValidation")

	// If both hook surfaces are natively resolved → pass through unchanged.
	if lifecycleResolved && blockingResolved {
		return keepObject(obj, model.KindHook, "target supports hooks natively"), PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message:  "hooks natively supported",
		}
	}

	// Target does not support hooks — determine safety.
	preservation := obj.Meta.Preservation
	if preservation == "" {
		preservation = model.PreservationPreferred
	}

	// Blocking enforcement + no hook support = unsafe lowering.
	if enforcement == model.EnforcementBlocking {
		decision := pipeline.LoweringDecision{
			Action:       "skipped",
			Reason:       "blocking hook cannot be safely lowered (enforcement semantics lost)",
			Preservation: preservation,
			Safe:         false,
		}
		result := CheckPreservation(decision, preservation)

		return pipeline.LoweredObject{
			OriginalID:   obj.Meta.ID,
			OriginalKind: model.KindHook,
			LoweredKind:  model.KindHook,
			Decision:     decision,
			Content:      "",
			Fields:       obj.ResolvedFields,
		}, result
	}

	// Advisory / best-effort hooks → lower to command, documented step, or skip.
	if enforcement == model.EnforcementAdvisory || enforcement == model.EnforcementBestEffort {
		// Check if target supports commands for fallback.
		cmdResolved := isCapabilityResolved(unitCaps, "commands.explicitEntryPoints")
		if cmdResolved {
			return lowerHookToCommand(obj, effectClass), PreservationResult{
				Allowed:  true,
				Severity: "info",
				Message:  "advisory hook lowered to command",
			}
		}

		// Setup/reporting effect classes without command support → document as manual step.
		// Per spec §5.3: effect_class: setup/reporting → lower to documented manual command.
		if effectClass == model.EffectSetup || effectClass == model.EffectReporting {
			return lowerHookToDocumented(obj, effectClass), PreservationResult{
				Allowed:  true,
				Severity: "info",
				Message:  fmt.Sprintf("%s hook documented as manual step", effectClass),
			}
		}

		// No command support and not a documentable effect → skip with record.
		decision := pipeline.LoweringDecision{
			Action:       "skipped",
			Reason:       fmt.Sprintf("%s hook skipped (no hook or command support)", enforcement),
			Preservation: preservation,
			Safe:         enforcement == model.EnforcementBestEffort,
		}
		result := CheckPreservation(decision, preservation)

		return pipeline.LoweredObject{
			OriginalID:   obj.Meta.ID,
			OriginalKind: model.KindHook,
			LoweredKind:  model.KindHook,
			Decision:     decision,
			Content:      "",
			Fields:       obj.ResolvedFields,
		}, result
	}

	// Default: skip unknown enforcement modes.
	decision := pipeline.LoweringDecision{
		Action:       "skipped",
		Reason:       "hook type not lowerable for this target",
		Preservation: preservation,
		Safe:         false,
	}
	result := CheckPreservation(decision, preservation)
	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: model.KindHook,
		LoweredKind:  model.KindHook,
		Decision:     decision,
		Content:      "",
		Fields:       obj.ResolvedFields,
	}, result
}

// HookLoweringRecord produces a LoweringRecord for a hook lowering.
func HookLoweringRecord(lowered pipeline.LoweredObject) pipeline.LoweringRecord {
	return pipeline.LoweringRecord{
		ObjectID:     lowered.OriginalID,
		FromKind:     model.KindHook,
		ToKind:       lowered.LoweredKind,
		Reason:       lowered.Decision.Reason,
		Preservation: lowered.Decision.Preservation,
		Status:       lowered.Decision.Action,
	}
}

func lowerHookToCommand(obj pipeline.NormalizedObject, effectClass model.EffectClass) pipeline.LoweredObject {
	content := fmt.Sprintf(
		"# Hook: %s\n\n> Effect: %s (advisory)\n\nRun manually: `%s`\n",
		obj.Meta.ID, effectClass, extractHookRef(obj),
	)
	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: model.KindHook,
		LoweredKind:  model.KindCommand,
		Decision: pipeline.LoweringDecision{
			Action:       "lowered",
			Reason:       fmt.Sprintf("advisory %s hook → command", effectClass),
			Preservation: obj.Meta.Preservation,
			Safe:         true,
		},
		Content: content,
		Fields:  obj.ResolvedFields,
	}
}

func lowerHookToDocumented(obj pipeline.NormalizedObject, effectClass model.EffectClass) pipeline.LoweredObject {
	content := fmt.Sprintf(
		"# Manual Step: %s\n\n> Originally: %s hook\n\nExecute: `%s`\n",
		obj.Meta.ID, effectClass, extractHookRef(obj),
	)
	return pipeline.LoweredObject{
		OriginalID:   obj.Meta.ID,
		OriginalKind: model.KindHook,
		LoweredKind:  model.KindInstruction,
		Decision: pipeline.LoweringDecision{
			Action:       "lowered",
			Reason:       fmt.Sprintf("%s hook → documented manual command", effectClass),
			Preservation: obj.Meta.Preservation,
			Safe:         true,
		},
		Content: content,
		Fields:  obj.ResolvedFields,
	}
}

func extractEnforcement(obj pipeline.NormalizedObject) model.EnforcementMode {
	if raw, ok := obj.ResolvedFields["enforcement"]; ok {
		if s, ok := raw.(string); ok {
			return model.EnforcementMode(s)
		}
		if e, ok := raw.(model.EnforcementMode); ok {
			return e
		}
	}
	return model.EnforcementAdvisory
}

func extractEffectClass(obj pipeline.NormalizedObject) model.EffectClass {
	if raw, ok := obj.ResolvedFields["effectClass"]; ok {
		if s, ok := raw.(string); ok {
			return model.EffectClass(s)
		}
		if e, ok := raw.(model.EffectClass); ok {
			return e
		}
	}
	return model.EffectObserving
}

func extractHookRef(obj pipeline.NormalizedObject) string {
	if raw, ok := obj.ResolvedFields["actionRef"]; ok {
		if s, ok := raw.(string); ok {
			return s
		}
	}
	return obj.Meta.ID
}

func isCapabilityResolved(uc pipeline.UnitCapabilities, surface string) bool {
	_, ok := uc.Resolved[surface]
	return ok
}
