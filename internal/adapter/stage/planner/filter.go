package planner

import (
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

// TargetFilter returns true when the object should be included for the given
// target. An object matches when:
//   - AppliesTo.Targets is empty (applies to all), or
//   - AppliesTo.Targets contains "*" (wildcard), or
//   - AppliesTo.Targets contains the target string.
//
// Additionally, if a TargetOverride exists for the target with Enabled
// explicitly set to false, the object is excluded even if the target matches.
func TargetFilter(meta model.ObjectMeta, target build.Target) bool {
	// Check explicit override: Enabled=false excludes the object.
	if ov, ok := meta.TargetOverrides[string(target)]; ok && ov.Enabled != nil && !*ov.Enabled {
		return false
	}

	if len(meta.AppliesTo.Targets) == 0 {
		return true
	}

	ts := string(target)
	for _, t := range meta.AppliesTo.Targets {
		if t == "*" || t == ts {
			return true
		}
	}
	return false
}

// ProfileFilter returns true when the object should be included for the given
// profile. An object matches when:
//   - AppliesTo.Profiles is empty (applies to all), or
//   - AppliesTo.Profiles contains "*" (wildcard), or
//   - AppliesTo.Profiles contains the profile string.
func ProfileFilter(meta model.ObjectMeta, profile build.Profile) bool {
	if len(meta.AppliesTo.Profiles) == 0 {
		return true
	}

	ps := string(profile)
	for _, p := range meta.AppliesTo.Profiles {
		if p == "*" || p == ps {
			return true
		}
	}
	return false
}
