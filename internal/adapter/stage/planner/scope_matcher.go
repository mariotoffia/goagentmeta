package planner

import (
	"path"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

// ScopeMatch returns true when the object's scope is compatible with the given
// build scope paths. An object matches when:
//   - Scope.Paths is empty (repository-wide), or
//   - at least one scope path glob-matches any of the buildPaths.
//
// When buildPaths is empty, only objects with empty scope (repo-wide) match.
func ScopeMatch(scope model.Scope, buildPaths []string) bool {
	if len(scope.Paths) == 0 {
		return true // repository-wide
	}

	if len(buildPaths) == 0 {
		return false // scope requires paths but build has none
	}

	for _, pattern := range scope.Paths {
		for _, bp := range buildPaths {
			if matchGlob(pattern, bp) {
				return true
			}
		}
	}
	return false
}

// FileTypeMatch returns true when the object's file type scope is compatible
// with the given build file types. An object matches when:
//   - Scope.FileTypes is empty (all file types), or
//   - at least one file type intersects.
func FileTypeMatch(scope model.Scope, buildFileTypes []string) bool {
	if len(scope.FileTypes) == 0 {
		return true
	}

	if len(buildFileTypes) == 0 {
		return false
	}

	for _, ft := range scope.FileTypes {
		ft = normalizeFileType(ft)
		for _, bft := range buildFileTypes {
			bft = normalizeFileType(bft)
			if ft == bft {
				return true
			}
		}
	}
	return false
}

// LabelMatch returns true when the object's label scope is compatible with
// the given build labels. An object matches when:
//   - Scope.Labels is empty (all labels), or
//   - at least one label intersects.
func LabelMatch(scope model.Scope, buildLabels []string) bool {
	if len(scope.Labels) == 0 {
		return true
	}

	if len(buildLabels) == 0 {
		return false
	}

	set := make(map[string]struct{}, len(buildLabels))
	for _, l := range buildLabels {
		set[l] = struct{}{}
	}

	for _, l := range scope.Labels {
		if _, ok := set[l]; ok {
			return true
		}
	}
	return false
}

// matchGlob matches a pattern against a candidate path. It uses path.Match
// for single-segment patterns and supports ** as a multi-segment wildcard.
func matchGlob(pattern, candidate string) bool {
	// Normalize separators.
	pattern = path.Clean(pattern)
	candidate = path.Clean(candidate)

	// Fast path: exact match.
	if pattern == candidate {
		return true
	}

	// Handle "**" prefix (e.g., "services/**").
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(candidate, prefix+"/") || candidate == prefix {
			return true
		}
	}

	// Handle "**/" prefix (e.g., "**/test.go").
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if strings.HasSuffix(candidate, "/"+suffix) || candidate == suffix {
			return true
		}
		// Also try path.Match on just the basename.
		if ok, _ := path.Match(suffix, path.Base(candidate)); ok {
			return true
		}
	}

	// Standard single-segment glob match.
	if ok, _ := path.Match(pattern, candidate); ok {
		return true
	}

	return false
}

// normalizeFileType ensures a leading dot for file type comparison.
func normalizeFileType(ft string) string {
	if ft != "" && !strings.HasPrefix(ft, ".") {
		return "." + ft
	}
	return ft
}
