package normalizer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// normalizeScope converts scope paths to canonical form (forward-slash separators),
// validates glob syntax, and expands file type aliases. It returns an error if any
// glob pattern is syntactically invalid.
func normalizeScope(scope *model.Scope, objectID, sourcePath string) error {
	for i, p := range scope.Paths {
		scope.Paths[i] = strings.ReplaceAll(p, "\\", "/")
	}

	for _, p := range scope.Paths {
		if _, err := filepath.Match(p, ""); err != nil {
			return pipeline.NewCompilerError(
				pipeline.ErrNormalization,
				fmt.Sprintf("invalid glob pattern %q in scope paths (in %s)", p, sourcePath),
				objectID,
			)
		}
	}

	for i, ft := range scope.FileTypes {
		if !strings.HasPrefix(ft, ".") {
			scope.FileTypes[i] = "." + ft
		}
	}

	return nil
}

// buildScopeIndex populates a scope-path → object-ID index from the set of
// normalized objects. Objects with no scope paths are indexed under the empty
// string key (repository root). The index values are sorted for determinism.
func buildScopeIndex(objects map[string]pipeline.NormalizedObject) map[string][]string {
	index := make(map[string][]string)

	for id, obj := range objects {
		if len(obj.Meta.Scope.Paths) == 0 {
			index[""] = append(index[""], id)
		} else {
			for _, p := range obj.Meta.Scope.Paths {
				index[p] = append(index[p], id)
			}
		}

		for _, label := range obj.Meta.Scope.Labels {
			key := "label:" + label
			index[key] = append(index[key], id)
		}
	}

	for k := range index {
		sort.Strings(index[k])
	}

	return index
}
