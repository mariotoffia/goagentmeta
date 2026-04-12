package normalizer

import (
	"context"
	"fmt"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion: *Stage satisfies the stage.Stage port interface.
var _ stage.Stage = (*Stage)(nil)

// Stage implements the PhaseNormalize pipeline stage. It transforms a SourceTree
// into a SemanticGraph by resolving inheritance, applying defaults, normalizing
// scopes, and resolving relative paths.
type Stage struct {
	fs portfs.Reader
}

// New creates a new normalizer Stage. The filesystem reader is optional; when
// provided it validates that referenced paths exist.
func New(fs portfs.Reader) *Stage {
	return &Stage{fs: fs}
}

// Descriptor returns the stage metadata for pipeline registration.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "normalizer",
		Phase: pipeline.PhaseNormalize,
		Order: 10,
	}
}

// Execute transforms a SourceTree into a SemanticGraph.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	tree, ok := input.(pipeline.SourceTree)
	if !ok {
		treePtr, ok := input.(*pipeline.SourceTree)
		if !ok || treePtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrNormalization,
				fmt.Sprintf("expected pipeline.SourceTree or *pipeline.SourceTree, got %T", input),
				"normalizer",
			)
		}
		tree = *treePtr
	}

	// Empty source tree → empty graph.
	if len(tree.Objects) == 0 {
		return pipeline.SemanticGraph{
			Objects:           make(map[string]pipeline.NormalizedObject),
			InheritanceChains: make(map[string][]string),
			ScopeIndex:        make(map[string][]string),
		}, nil
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "NORMALIZE_START",
		Message:  fmt.Sprintf("normalizing %d objects", len(tree.Objects)),
		Phase:    pipeline.PhaseNormalize,
	})

	// Step 1: Build mutable working set indexed by ID.
	// Copy each Meta to avoid mutating the input SourceTree.
	work := make(map[string]*pipeline.NormalizedObject, len(tree.Objects))
	for i := range tree.Objects {
		raw := &tree.Objects[i]
		meta := raw.Meta // value copy — preserves purity, no mutation of input
		content := applyDefaults(&meta, raw.RawContent)

		fields := make(map[string]any, len(raw.RawFields))
		for k, v := range raw.RawFields {
			fields[k] = v
		}

		work[raw.Meta.ID] = &pipeline.NormalizedObject{
			Meta:           meta,
			SourcePath:     raw.SourcePath,
			Content:        content,
			ResolvedFields: fields,
		}
	}

	// Step 2: Normalize scopes.
	ids := sortedKeys(work)
	for _, id := range ids {
		obj := work[id]
		if err := normalizeScope(&obj.Meta.Scope, id, obj.SourcePath); err != nil {
			return nil, err
		}
	}

	// Step 3: Resolve inheritance (topological sort + merge).
	order, chains, err := resolveInheritance(work)
	if err != nil {
		return nil, err
	}

	// Merge in topological order: parents are processed before children.
	for _, id := range order {
		obj := work[id]
		for _, parentID := range obj.Meta.Extends {
			parent := work[parentID]
			mergeParentIntoChild(obj, parent)
		}
	}

	// Step 4: Resolve paths.
	for _, id := range ids {
		obj := work[id]
		if err := resolvePaths(ctx, obj, tree.RootPath, s.fs); err != nil {
			return nil, err
		}
	}

	// Step 5: Build output SemanticGraph.
	objects := make(map[string]pipeline.NormalizedObject, len(work))
	for id, obj := range work {
		objects[id] = *obj
	}

	scopeIndex := buildScopeIndex(objects)

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "NORMALIZE_COMPLETE",
		Message:  fmt.Sprintf("normalized %d objects, %d inheritance chains", len(objects), len(chains)),
		Phase:    pipeline.PhaseNormalize,
	})

	return pipeline.SemanticGraph{
		Objects:           objects,
		InheritanceChains: chains,
		ScopeIndex:        scopeIndex,
	}, nil
}

// Factory returns a StageFactory function for use with pipeline registration.
func Factory(fs portfs.Reader) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(fs), nil
	}
}

// sortedKeys returns the map keys in sorted order for deterministic iteration.
func sortedKeys(m map[string]*pipeline.NormalizedObject) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// emitDiagnostic sends a diagnostic through the CompilerContext if available.
func emitDiagnostic(ctx context.Context, d pipeline.Diagnostic) {
	cc := compiler.CompilerFromContext(ctx)
	if cc == nil {
		return
	}
	if cc.Report != nil {
		cc.Report.Diagnostics = append(cc.Report.Diagnostics, d)
	}
	if cc.Config != nil && cc.Config.DiagnosticSink != nil {
		cc.Config.DiagnosticSink.Emit(ctx, d)
	}
}
