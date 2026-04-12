// Package normalizer implements the PhaseNormalize pipeline stage. It transforms
// a flat list of RawObjects (SourceTree) into a SemanticGraph — the first
// semantically meaningful IR. This phase resolves inheritance chains, fills in
// default values, normalizes scope selectors, and resolves relative paths.
package normalizer

import (
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

// applyDefaults fills in missing default values on the ObjectMeta and content
// fields. It mutates the provided meta and returns the trimmed content.
func applyDefaults(meta *model.ObjectMeta, content string) string {
	if meta.Preservation == "" {
		meta.Preservation = model.PreservationOptional
	}

	if meta.Version == 0 {
		meta.Version = 1
	}

	if len(meta.AppliesTo.Targets) == 0 {
		targets := build.AllTargets()
		meta.AppliesTo.Targets = make([]string, len(targets))
		for i, t := range targets {
			meta.AppliesTo.Targets[i] = string(t)
		}
	}

	return strings.TrimSpace(content)
}
