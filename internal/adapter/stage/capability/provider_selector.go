package capability

import (
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	domcap "github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

// SelectProvider selects the best provider for a capability surface given
// the target's registry, available plugins, and the build profile. Returns
// the selected candidate (Compatible=true) or a rejection candidate
// (Compatible=false) with a reason.
//
// Provider selection priority (lower number = higher priority):
//  1. native target capability
//  2. configured plugin
//  3. MCP-backed provider
//  4. script-backed provider
//  5. textual documentation fallback
func SelectProvider(
	capSurface string,
	registry *domcap.CapabilityRegistry,
	plugins []domcap.Provider,
	profile build.Profile,
) domcap.ProviderCandidate {
	// Step 1: Check native support in registry.
	if level, ok := registry.Surfaces[capSurface]; ok {
		switch level {
		case domcap.SupportNative:
			return domcap.ProviderCandidate{
				Provider: domcap.Provider{
					ID:           registry.Target + "/native/" + capSurface,
					Type:         "native",
					Capabilities: []string{capSurface},
				},
				Priority:   1,
				Compatible: true,
				Reason:     "native target support",
			}
		case domcap.SupportAdapted:
			return domcap.ProviderCandidate{
				Provider: domcap.Provider{
					ID:           registry.Target + "/adapted/" + capSurface,
					Type:         "native",
					Capabilities: []string{capSurface},
				},
				Priority:   1,
				Compatible: true,
				Reason:     "adapted target support (different syntax/placement)",
			}
		case domcap.SupportLowered:
			return domcap.ProviderCandidate{
				Provider: domcap.Provider{
					ID:           registry.Target + "/lowered/" + capSurface,
					Type:         "native",
					Capabilities: []string{capSurface},
				},
				Priority:   1,
				Compatible: true,
				Reason:     "lowered to alternative primitive",
			}
		case domcap.SupportSkipped:
			// Fall through to plugin/MCP selection.
		case domcap.SupportEmulated:
			// Fall through — emulated is a last-resort native option.
		}
	}

	// Step 2: Check plugins (sorted by type priority, then name for determinism).
	sortedPlugins := make([]domcap.Provider, len(plugins))
	copy(sortedPlugins, plugins)
	sort.Slice(sortedPlugins, func(i, j int) bool {
		pi, pj := priorityForType(sortedPlugins[i].Type), priorityForType(sortedPlugins[j].Type)
		if pi != pj {
			return pi < pj
		}
		return sortedPlugins[i].ID < sortedPlugins[j].ID
	})

	for _, plug := range sortedPlugins {
		if !pluginProvides(plug, capSurface) {
			continue
		}

		// Profile security enforcement: enterprise-locked blocks MCP-backed plugins.
		if profile == build.ProfileEnterpriseLocked && plug.Type == "mcp" {
			continue // skip MCP providers under enterprise-locked
		}

		return domcap.ProviderCandidate{
			Provider:   plug,
			Priority:   priorityForType(plug.Type),
			Compatible: true,
			Reason:     "plugin provider: " + plug.ID,
		}
	}

	// Step 3: Check emulated fallback from registry.
	if level, ok := registry.Surfaces[capSurface]; ok && level == domcap.SupportEmulated {
		return domcap.ProviderCandidate{
			Provider: domcap.Provider{
				ID:           registry.Target + "/emulated/" + capSurface,
				Type:         "native",
				Capabilities: []string{capSurface},
			},
			Priority:   5,
			Compatible: true,
			Reason:     "emulated approximation",
		}
	}

	// Step 4: No provider found.
	return domcap.ProviderCandidate{
		Compatible: false,
		Reason:     "no compatible provider for " + capSurface,
	}
}

// RequiredCapabilities derives the list of capability surface IDs from the
// object's kind. Each object kind maps to known capability surfaces.
func RequiredCapabilities(meta model.ObjectMeta) []string {
	switch meta.Kind {
	case model.KindInstruction:
		return []string{"instructions.layeredFiles", "instructions.scopedSections"}
	case model.KindRule:
		return []string{"rules.scopedRules"}
	case model.KindSkill:
		return []string{"skills.bundles", "skills.supportingFiles"}
	case model.KindAgent:
		return []string{"agents.subagents", "agents.toolPolicies"}
	case model.KindHook:
		return []string{"hooks.lifecycle", "hooks.blockingValidation"}
	case model.KindCommand:
		return []string{"commands.explicitEntryPoints"}
	case model.KindPlugin:
		return []string{"plugins.installablePackages", "plugins.capabilityProviders"}
	case model.KindCapability:
		return nil // meta-capability, no surface needed
	default:
		return nil
	}
}

// pluginProvides checks whether a provider satisfies a specific capability.
func pluginProvides(p domcap.Provider, capSurface string) bool {
	for _, c := range p.Capabilities {
		if c == capSurface {
			return true
		}
	}
	return false
}

// priorityForType returns the selection priority for a provider type.
func priorityForType(typ string) int {
	switch typ {
	case "native":
		return 1
	case "plugin":
		return 2
	case "mcp":
		return 3
	case "script":
		return 4
	default:
		return 5
	}
}
