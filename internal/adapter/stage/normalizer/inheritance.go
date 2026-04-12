package normalizer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// resolveInheritance topologically sorts objects by their Extends chains using
// Kahn's algorithm, then merges parent fields into children (left-to-right,
// child takes precedence). It returns the merge order and the parent→children
// map. Cycles and missing extends targets produce NORMALIZATION errors.
func resolveInheritance(
	objects map[string]*pipeline.NormalizedObject,
) (order []string, chains map[string][]string, err error) {

	chains = make(map[string][]string)

	// Build adjacency graph: parent → children edges.
	inDegree := make(map[string]int)
	adj := make(map[string][]string) // parent → list of children
	allIDs := make([]string, 0, len(objects))

	for id := range objects {
		allIDs = append(allIDs, id)
		inDegree[id] = 0
	}
	sort.Strings(allIDs)

	for _, id := range allIDs {
		obj := objects[id]
		for _, parentID := range obj.Meta.Extends {
			if _, ok := objects[parentID]; !ok {
				return nil, nil, pipeline.NewCompilerError(
					pipeline.ErrNormalization,
					fmt.Sprintf("extends target %q not found (in %s)", parentID, obj.SourcePath),
					id,
				)
			}
			adj[parentID] = append(adj[parentID], id)
			inDegree[id]++
			chains[parentID] = append(chains[parentID], id)
		}
	}

	// Sort adjacency lists for determinism.
	for k := range adj {
		sort.Strings(adj[k])
	}
	for k := range chains {
		sort.Strings(chains[k])
	}

	// Kahn's algorithm: seed with objects that have no parents.
	var queue []string
	for _, id := range allIDs {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	order = make([]string, 0, len(allIDs))
	for len(queue) > 0 {
		sort.Strings(queue)
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, child := range adj[node] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if len(order) != len(allIDs) {
		// Detect cycle participants with source paths.
		var cycleDetails []string
		for _, id := range allIDs {
			if inDegree[id] > 0 {
				cycleDetails = append(cycleDetails, fmt.Sprintf("%s (%s)", id, objects[id].SourcePath))
			}
		}
		return nil, nil, pipeline.NewCompilerError(
			pipeline.ErrNormalization,
			fmt.Sprintf("circular inheritance detected among: %s", strings.Join(cycleDetails, ", ")),
			allIDs[0], // first cycle participant as context
		)
	}

	return order, chains, nil
}

// mergeParentIntoChild merges parent fields into the child. The child takes
// precedence for any field already set. Merge is applied to ResolvedFields
// (map merge), Content (parent content prepended), and ObjectMeta fields.
func mergeParentIntoChild(child, parent *pipeline.NormalizedObject) {
	// Merge ResolvedFields: parent first, child overrides.
	if child.ResolvedFields == nil {
		child.ResolvedFields = make(map[string]any)
	}
	for k, v := range parent.ResolvedFields {
		if _, exists := child.ResolvedFields[k]; !exists {
			child.ResolvedFields[k] = v
		}
	}

	// Content: prepend parent content if child has none.
	if child.Content == "" && parent.Content != "" {
		child.Content = parent.Content
	}

	// Merge scope paths (union, deduplicated).
	child.Meta.Scope.Paths = mergeStringSlices(child.Meta.Scope.Paths, parent.Meta.Scope.Paths)
	child.Meta.Scope.FileTypes = mergeStringSlices(child.Meta.Scope.FileTypes, parent.Meta.Scope.FileTypes)
	child.Meta.Scope.Labels = mergeStringSlices(child.Meta.Scope.Labels, parent.Meta.Scope.Labels)

	// Merge Labels (union).
	child.Meta.Labels = mergeStringSlices(child.Meta.Labels, parent.Meta.Labels)

	// Inherit Description if child has none.
	if child.Meta.Description == "" {
		child.Meta.Description = parent.Meta.Description
	}

	// Inherit Owner if child has none.
	if child.Meta.Owner == "" {
		child.Meta.Owner = parent.Meta.Owner
	}

	// Merge TargetOverrides: parent entries fill gaps.
	if len(parent.Meta.TargetOverrides) > 0 {
		if child.Meta.TargetOverrides == nil {
			child.Meta.TargetOverrides = make(map[string]model.TargetOverride)
		}
		for k, v := range parent.Meta.TargetOverrides {
			if _, exists := child.Meta.TargetOverrides[k]; !exists {
				child.Meta.TargetOverrides[k] = v
			}
		}
	}
}

// mergeStringSlices returns the union of two string slices, preserving the
// order of a then appending new elements from b.
func mergeStringSlices(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]struct{}, len(a))
	for _, s := range a {
		seen[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			a = append(a, s)
			seen[s] = struct{}{}
		}
	}
	return a
}
