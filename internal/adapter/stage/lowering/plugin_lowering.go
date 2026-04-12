package lowering

import (
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// LowerPlugin transforms a plugin for a target based on capability support.
//   - If the target supports plugins natively → pass through
//   - If the target supports MCP (e.g., Cursor) → extract MCP server reference
//   - Otherwise → skip with preservation check
func LowerPlugin(
	obj pipeline.NormalizedObject,
	unitCaps pipeline.UnitCapabilities,
	target string,
) (pipeline.LoweredObject, PreservationResult) {
	pluginResolved := isCapabilityResolved(unitCaps, "plugins.installablePackages")

	// Native plugin support → pass through.
	if pluginResolved {
		return keepObject(obj, model.KindPlugin, "target supports plugins natively"), PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message:  "plugins natively supported",
		}
	}

	// Check if target supports MCP for fallback.
	mcpResolved := isCapabilityResolved(unitCaps, "mcp.serverBindings")
	hasMCPProvider := extractMCPProvider(obj)

	if mcpResolved && hasMCPProvider {
		// Plugin has MCP server → extract MCP reference.
		content := fmt.Sprintf(
			"MCP server reference extracted from plugin %s",
			obj.Meta.ID,
		)
		return pipeline.LoweredObject{
			OriginalID:   obj.Meta.ID,
			OriginalKind: model.KindPlugin,
			LoweredKind:  model.KindPlugin,
			Decision: pipeline.LoweringDecision{
				Action:       "lowered",
				Reason:       "plugin → MCP server reference (conditionally safe)",
				Preservation: obj.Meta.Preservation,
				Safe:         true,
			},
			Content: content,
			Fields:  obj.ResolvedFields,
		}, PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message:  "plugin lowered to MCP reference",
		}
	}

	// No plugin or MCP support → skip.
	preservation := obj.Meta.Preservation
	if preservation == "" {
		preservation = model.PreservationPreferred
	}

	decision := pipeline.LoweringDecision{
		Action:       "skipped",
		Reason:       fmt.Sprintf("plugin not supported by target %s (no plugin or MCP fallback)", target),
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
		OriginalKind: model.KindPlugin,
		LoweredKind:  model.KindPlugin,
		Decision: pipeline.LoweringDecision{
			Action:       action,
			Reason:       decision.Reason,
			Preservation: preservation,
			Safe:         false,
		},
		Fields: obj.ResolvedFields,
	}, result
}

// PluginLoweringRecord produces a LoweringRecord for a plugin lowering.
func PluginLoweringRecord(lowered pipeline.LoweredObject) pipeline.LoweringRecord {
	return pipeline.LoweringRecord{
		ObjectID:     lowered.OriginalID,
		FromKind:     model.KindPlugin,
		ToKind:       lowered.LoweredKind,
		Reason:       lowered.Decision.Reason,
		Preservation: lowered.Decision.Preservation,
		Status:       lowered.Decision.Action,
	}
}

// extractMCPProvider checks if the plugin has MCP-related capabilities.
func extractMCPProvider(obj pipeline.NormalizedObject) bool {
	if raw, ok := obj.ResolvedFields["provides"]; ok {
		if provides, ok := raw.([]string); ok {
			for _, p := range provides {
				if p == "mcp.serverBindings" || p == "mcp" {
					return true
				}
			}
		}
	}
	return false
}
