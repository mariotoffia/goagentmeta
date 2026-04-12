package normalizer_test

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/normalizer"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// --- mock filesystem ---

type mockReader struct {
	statFunc func(ctx context.Context, path string) (fs.FileInfo, error)
}

func (m *mockReader) ReadFile(_ context.Context, _ string) ([]byte, error) { return nil, nil }
func (m *mockReader) ReadDir(_ context.Context, _ string) ([]fs.DirEntry, error) {
	return nil, nil
}
func (m *mockReader) Stat(ctx context.Context, path string) (fs.FileInfo, error) {
	if m.statFunc != nil {
		return m.statFunc(ctx, path)
	}
	return nil, nil
}
func (m *mockReader) Glob(_ context.Context, _ string) ([]string, error) { return nil, nil }

// --- helpers ---

func rawObj(id string, kind model.Kind, opts ...func(*pipeline.RawObject)) pipeline.RawObject {
	obj := pipeline.RawObject{
		Meta: model.ObjectMeta{
			ID:   id,
			Kind: kind,
		},
		SourcePath: "/repo/.ai/" + id + ".yaml",
		RawFields:  make(map[string]any),
	}
	for _, o := range opts {
		o(&obj)
	}
	return obj
}

func withExtends(parents ...string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.Meta.Extends = parents }
}

func withContent(c string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.RawContent = c }
}

func withPreservation(p model.Preservation) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.Meta.Preservation = p }
}

func withVersion(v int) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.Meta.Version = v }
}

func withTargets(targets ...string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.Meta.AppliesTo.Targets = targets }
}

func withScope(paths []string, fileTypes []string, labels []string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) {
		o.Meta.Scope = model.Scope{
			Paths:     paths,
			FileTypes: fileTypes,
			Labels:    labels,
		}
	}
}

func withDescription(d string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.Meta.Description = d }
}

func withOwner(owner string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.Meta.Owner = owner }
}

func withLabels(labels ...string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.Meta.Labels = labels }
}

func withRawField(key string, val any) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) {
		if o.RawFields == nil {
			o.RawFields = make(map[string]any)
		}
		o.RawFields[key] = val
	}
}

func withSourcePath(p string) func(*pipeline.RawObject) {
	return func(o *pipeline.RawObject) { o.SourcePath = p }
}

func sourceTree(objects ...pipeline.RawObject) pipeline.SourceTree {
	return pipeline.SourceTree{
		RootPath:      "/repo/.ai",
		Objects:       objects,
		SchemaVersion: 1,
		ManifestPath:  "/repo/.ai/manifest.yaml",
	}
}

func execute(t *testing.T, tree pipeline.SourceTree, fsReader ...func(*mockReader)) pipeline.SemanticGraph {
	t.Helper()
	var stage *normalizer.Stage
	if len(fsReader) > 0 {
		reader := &mockReader{}
		for _, f := range fsReader {
			f(reader)
		}
		stage = normalizer.New(reader)
	} else {
		stage = normalizer.New(nil)
	}
	result, err := stage.Execute(context.Background(), tree)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	graph, ok := result.(pipeline.SemanticGraph)
	if !ok {
		t.Fatalf("result type = %T, want pipeline.SemanticGraph", result)
	}
	return graph
}

func executeErr(t *testing.T, tree pipeline.SourceTree) *pipeline.CompilerError {
	t.Helper()
	stage := normalizer.New(nil)
	_, err := stage.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T: %v", err, err)
	}
	return compErr
}

// --- Descriptor ---

func TestStage_Descriptor(t *testing.T) {
	stage := normalizer.New(nil)
	desc := stage.Descriptor()

	if desc.Name != "normalizer" {
		t.Errorf("Name = %q, want normalizer", desc.Name)
	}
	if desc.Phase != pipeline.PhaseNormalize {
		t.Errorf("Phase = %v, want PhaseNormalize", desc.Phase)
	}
	if desc.Order != 10 {
		t.Errorf("Order = %d, want 10", desc.Order)
	}
}

// --- Input type handling ---

func TestStage_InvalidInput(t *testing.T) {
	stage := normalizer.New(nil)
	_, err := stage.Execute(context.Background(), "not a source tree")
	if err == nil {
		t.Fatal("expected error for invalid input type")
	}
	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T", err)
	}
	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
}

func TestStage_PointerInput(t *testing.T) {
	tree := &pipeline.SourceTree{
		RootPath: "/repo/.ai",
		Objects: []pipeline.RawObject{
			rawObj("obj1", model.KindInstruction),
		},
	}
	stage := normalizer.New(nil)
	result, err := stage.Execute(context.Background(), tree)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, ok := result.(pipeline.SemanticGraph); !ok {
		t.Fatalf("result type = %T, want pipeline.SemanticGraph", result)
	}
}

// --- Empty source tree ---

func TestStage_EmptySourceTree(t *testing.T) {
	graph := execute(t, sourceTree())

	if len(graph.Objects) != 0 {
		t.Errorf("Objects len = %d, want 0", len(graph.Objects))
	}
	if graph.InheritanceChains == nil {
		t.Error("InheritanceChains is nil, want empty map")
	}
	if graph.ScopeIndex == nil {
		t.Error("ScopeIndex is nil, want empty map")
	}
}

// --- Default values ---

func TestDefaults_PreservationOptionalWhenOmitted(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction),
	))

	obj := graph.Objects["obj1"]
	if obj.Meta.Preservation != model.PreservationOptional {
		t.Errorf("Preservation = %q, want %q", obj.Meta.Preservation, model.PreservationOptional)
	}
}

func TestDefaults_PreservationPreservedWhenSet(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction, withPreservation(model.PreservationRequired)),
	))

	obj := graph.Objects["obj1"]
	if obj.Meta.Preservation != model.PreservationRequired {
		t.Errorf("Preservation = %q, want %q", obj.Meta.Preservation, model.PreservationRequired)
	}
}

func TestDefaults_VersionDefaultsTo1(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction),
	))

	obj := graph.Objects["obj1"]
	if obj.Meta.Version != 1 {
		t.Errorf("Version = %d, want 1", obj.Meta.Version)
	}
}

func TestDefaults_VersionPreservedWhenSet(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction, withVersion(3)),
	))

	obj := graph.Objects["obj1"]
	if obj.Meta.Version != 3 {
		t.Errorf("Version = %d, want 3", obj.Meta.Version)
	}
}

func TestDefaults_TargetsDefaultToAll(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction),
	))

	obj := graph.Objects["obj1"]
	allTargets := build.AllTargets()
	if len(obj.Meta.AppliesTo.Targets) != len(allTargets) {
		t.Fatalf("Targets len = %d, want %d", len(obj.Meta.AppliesTo.Targets), len(allTargets))
	}
	for i, tgt := range allTargets {
		if obj.Meta.AppliesTo.Targets[i] != string(tgt) {
			t.Errorf("Targets[%d] = %q, want %q", i, obj.Meta.AppliesTo.Targets[i], tgt)
		}
	}
}

func TestDefaults_TargetsPreservedWhenSet(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction, withTargets("claude", "cursor")),
	))

	obj := graph.Objects["obj1"]
	if len(obj.Meta.AppliesTo.Targets) != 2 {
		t.Fatalf("Targets len = %d, want 2", len(obj.Meta.AppliesTo.Targets))
	}
}

func TestDefaults_ContentTrimmed(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction, withContent("  hello world  \n\n")),
	))

	obj := graph.Objects["obj1"]
	if obj.Content != "hello world" {
		t.Errorf("Content = %q, want %q", obj.Content, "hello world")
	}
}

// --- Inheritance ---

func TestInheritance_NoExtends(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction, withContent("hello")),
	))

	obj := graph.Objects["obj1"]
	if obj.Content != "hello" {
		t.Errorf("Content = %q, want %q", obj.Content, "hello")
	}
	if len(graph.InheritanceChains) != 0 {
		t.Errorf("InheritanceChains len = %d, want 0", len(graph.InheritanceChains))
	}
}

func TestInheritance_SingleParent(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("parent", model.KindInstruction,
			withContent("parent content"),
			withDescription("parent desc"),
			withOwner("team-a"),
			withRawField("inherited_key", "inherited_val"),
		),
		rawObj("child", model.KindInstruction,
			withExtends("parent"),
			withRawField("child_key", "child_val"),
		),
	))

	child := graph.Objects["child"]

	// Child inherits parent content when it has none.
	if child.Content != "parent content" {
		t.Errorf("Content = %q, want %q", child.Content, "parent content")
	}

	// Child inherits parent description.
	if child.Meta.Description != "parent desc" {
		t.Errorf("Description = %q, want %q", child.Meta.Description, "parent desc")
	}

	// Child inherits parent owner.
	if child.Meta.Owner != "team-a" {
		t.Errorf("Owner = %q, want %q", child.Meta.Owner, "team-a")
	}

	// ResolvedFields merges parent + child.
	if child.ResolvedFields["inherited_key"] != "inherited_val" {
		t.Errorf("ResolvedFields[inherited_key] = %v, want inherited_val", child.ResolvedFields["inherited_key"])
	}
	if child.ResolvedFields["child_key"] != "child_val" {
		t.Errorf("ResolvedFields[child_key] = %v, want child_val", child.ResolvedFields["child_key"])
	}

	// InheritanceChains: parent → [child].
	if children, ok := graph.InheritanceChains["parent"]; !ok {
		t.Error("InheritanceChains missing parent key")
	} else if len(children) != 1 || children[0] != "child" {
		t.Errorf("InheritanceChains[parent] = %v, want [child]", children)
	}
}

func TestInheritance_ChildOverridesParent(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("parent", model.KindInstruction,
			withContent("parent content"),
			withDescription("parent desc"),
			withRawField("shared_key", "parent_val"),
		),
		rawObj("child", model.KindInstruction,
			withExtends("parent"),
			withContent("child content"),
			withDescription("child desc"),
			withRawField("shared_key", "child_val"),
		),
	))

	child := graph.Objects["child"]
	if child.Content != "child content" {
		t.Errorf("Content = %q, want %q", child.Content, "child content")
	}
	if child.Meta.Description != "child desc" {
		t.Errorf("Description = %q, want %q", child.Meta.Description, "child desc")
	}
	if child.ResolvedFields["shared_key"] != "child_val" {
		t.Errorf("ResolvedFields[shared_key] = %v, want child_val", child.ResolvedFields["shared_key"])
	}
}

func TestInheritance_MultipleParents_LeftToRight(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("p1", model.KindInstruction,
			withRawField("from_p1", "p1"),
			withRawField("shared", "from_p1"),
			withDescription("p1 desc"),
		),
		rawObj("p2", model.KindInstruction,
			withRawField("from_p2", "p2"),
			withRawField("shared", "from_p2"),
			withDescription("p2 desc"),
		),
		rawObj("child", model.KindInstruction,
			withExtends("p1", "p2"),
		),
	))

	child := graph.Objects["child"]

	// Both parents' fields inherited.
	if child.ResolvedFields["from_p1"] != "p1" {
		t.Errorf("from_p1 = %v, want p1", child.ResolvedFields["from_p1"])
	}
	if child.ResolvedFields["from_p2"] != "p2" {
		t.Errorf("from_p2 = %v, want p2", child.ResolvedFields["from_p2"])
	}

	// Shared key: first parent (p1) wins because it's merged first and child has no value.
	if child.ResolvedFields["shared"] != "from_p1" {
		t.Errorf("shared = %v, want from_p1 (left-to-right merge order)", child.ResolvedFields["shared"])
	}

	// Description: first parent wins.
	if child.Meta.Description != "p1 desc" {
		t.Errorf("Description = %q, want %q", child.Meta.Description, "p1 desc")
	}
}

func TestInheritance_CircularDetected(t *testing.T) {
	compErr := executeErr(t, sourceTree(
		rawObj("a", model.KindInstruction, withExtends("b")),
		rawObj("b", model.KindInstruction, withExtends("a")),
	))

	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
	if compErr.Message == "" {
		t.Error("expected non-empty error message for cycle detection")
	}
}

func TestInheritance_MissingTarget(t *testing.T) {
	compErr := executeErr(t, sourceTree(
		rawObj("child", model.KindInstruction, withExtends("nonexistent")),
	))

	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
	if compErr.Context != "child" {
		t.Errorf("Context = %q, want child", compErr.Context)
	}
}

func TestInheritance_LabelsMerged(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("parent", model.KindInstruction, withLabels("a", "b")),
		rawObj("child", model.KindInstruction, withExtends("parent"), withLabels("b", "c")),
	))

	child := graph.Objects["child"]
	labels := child.Meta.Labels
	sort.Strings(labels)
	expected := []string{"a", "b", "c"}
	if len(labels) != len(expected) {
		t.Fatalf("Labels = %v, want %v", labels, expected)
	}
	for i, l := range expected {
		if labels[i] != l {
			t.Errorf("Labels[%d] = %q, want %q", i, labels[i], l)
		}
	}
}

func TestInheritance_ThreeLevelChain(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("grandparent", model.KindInstruction,
			withRawField("gp_key", "gp_val"),
			withOwner("gp-owner"),
		),
		rawObj("parent", model.KindInstruction,
			withExtends("grandparent"),
			withRawField("p_key", "p_val"),
		),
		rawObj("child", model.KindInstruction,
			withExtends("parent"),
		),
	))

	child := graph.Objects["child"]

	// Three-level inheritance: grandparent fields propagate through parent.
	if child.ResolvedFields["gp_key"] != "gp_val" {
		t.Errorf("gp_key = %v, want gp_val", child.ResolvedFields["gp_key"])
	}
	if child.ResolvedFields["p_key"] != "p_val" {
		t.Errorf("p_key = %v, want p_val", child.ResolvedFields["p_key"])
	}
	if child.Meta.Owner != "gp-owner" {
		t.Errorf("Owner = %q, want gp-owner", child.Meta.Owner)
	}
}

// --- Scope normalization ---

func TestScope_PathsNormalizedToSlash(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindRule,
			withScope([]string{"src\\main\\java", "test/fixtures"}, nil, nil),
		),
	))

	obj := graph.Objects["obj1"]
	expected := []string{"src/main/java", "test/fixtures"}
	if len(obj.Meta.Scope.Paths) != len(expected) {
		t.Fatalf("Scope.Paths = %v, want %v", obj.Meta.Scope.Paths, expected)
	}
	for i, p := range expected {
		if obj.Meta.Scope.Paths[i] != p {
			t.Errorf("Scope.Paths[%d] = %q, want %q", i, obj.Meta.Scope.Paths[i], p)
		}
	}
}

func TestScope_FileTypesNormalized(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindRule,
			withScope(nil, []string{"go", ".ts", "py"}, nil),
		),
	))

	obj := graph.Objects["obj1"]
	expected := []string{".go", ".ts", ".py"}
	if len(obj.Meta.Scope.FileTypes) != len(expected) {
		t.Fatalf("FileTypes = %v, want %v", obj.Meta.Scope.FileTypes, expected)
	}
	for i, ft := range expected {
		if obj.Meta.Scope.FileTypes[i] != ft {
			t.Errorf("FileTypes[%d] = %q, want %q", i, obj.Meta.Scope.FileTypes[i], ft)
		}
	}
}

func TestScope_InvalidGlob(t *testing.T) {
	compErr := executeErr(t, sourceTree(
		rawObj("obj1", model.KindRule,
			withScope([]string{"[invalid"}, nil, nil),
		),
	))

	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
}

// --- ScopeIndex ---

func TestScopeIndex_MapsPathToObjects(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindRule,
			withScope([]string{"src/**"}, nil, nil),
		),
		rawObj("obj2", model.KindRule,
			withScope([]string{"src/**", "test/**"}, nil, nil),
		),
	))

	srcObjs := graph.ScopeIndex["src/**"]
	sort.Strings(srcObjs)
	if len(srcObjs) != 2 || srcObjs[0] != "obj1" || srcObjs[1] != "obj2" {
		t.Errorf("ScopeIndex[src/**] = %v, want [obj1 obj2]", srcObjs)
	}

	testObjs := graph.ScopeIndex["test/**"]
	if len(testObjs) != 1 || testObjs[0] != "obj2" {
		t.Errorf("ScopeIndex[test/**] = %v, want [obj2]", testObjs)
	}
}

func TestScopeIndex_RootScopeUsesEmptyKey(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction),
	))

	rootObjs := graph.ScopeIndex[""]
	if len(rootObjs) != 1 || rootObjs[0] != "obj1" {
		t.Errorf("ScopeIndex[\"\"] = %v, want [obj1]", rootObjs)
	}
}

func TestScopeIndex_LabelIndex(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindRule,
			withScope(nil, nil, []string{"domain:auth"}),
		),
	))

	objs := graph.ScopeIndex["label:domain:auth"]
	if len(objs) != 1 || objs[0] != "obj1" {
		t.Errorf("ScopeIndex[label:domain:auth] = %v, want [obj1]", objs)
	}
}

// --- InheritanceChains structure ---

func TestInheritanceChains_ParentToChildren(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("base", model.KindInstruction),
		rawObj("child1", model.KindInstruction, withExtends("base")),
		rawObj("child2", model.KindInstruction, withExtends("base")),
	))

	children := graph.InheritanceChains["base"]
	sort.Strings(children)
	if len(children) != 2 || children[0] != "child1" || children[1] != "child2" {
		t.Errorf("InheritanceChains[base] = %v, want [child1 child2]", children)
	}
}

// --- Path resolution ---

func TestPathResolver_RelativePathResolved(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction,
			withSourcePath("/repo/.ai/rules/my-rule.yaml"),
			withRawField("asset_path", "../assets/diagram.png"),
		),
	))

	obj := graph.Objects["obj1"]
	resolved := obj.ResolvedFields["asset_path"]
	if resolved != "assets/diagram.png" {
		t.Errorf("asset_path = %v, want assets/diagram.png", resolved)
	}
}

func TestPathResolver_ValidationFailure(t *testing.T) {
	tree := sourceTree(
		rawObj("obj1", model.KindInstruction,
			withSourcePath("/repo/.ai/rules/my-rule.yaml"),
			withRawField("asset_path", "../assets/missing.png"),
		),
	)

	fsReader := &mockReader{
		statFunc: func(_ context.Context, _ string) (fs.FileInfo, error) {
			return nil, fs.ErrNotExist
		},
	}

	stage := normalizer.New(fsReader)
	_, err := stage.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error for missing path")
	}

	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T", err)
	}
	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
}

func TestPathResolver_NoFSReader_SkipsValidation(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction,
			withSourcePath("/repo/.ai/rules/my-rule.yaml"),
			withRawField("asset_path", "../assets/anything.png"),
		),
	))

	obj := graph.Objects["obj1"]
	if obj.ResolvedFields["asset_path"] != "assets/anything.png" {
		t.Errorf("asset_path = %v, want assets/anything.png", obj.ResolvedFields["asset_path"])
	}
}

// --- SourcePath provenance ---

func TestSourcePath_Preserved(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("obj1", model.KindInstruction,
			withSourcePath("/repo/.ai/instructions/coding.yaml"),
		),
	))

	obj := graph.Objects["obj1"]
	if obj.SourcePath != "/repo/.ai/instructions/coding.yaml" {
		t.Errorf("SourcePath = %q, want /repo/.ai/instructions/coding.yaml", obj.SourcePath)
	}
}

// --- External objects ---

func TestExternalObjects_IncludedInGraph(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("local-obj", model.KindInstruction),
		rawObj("ext-obj", model.KindInstruction,
			withRawField("_external", true),
		),
	))

	if len(graph.Objects) != 2 {
		t.Fatalf("Objects len = %d, want 2", len(graph.Objects))
	}
	if _, ok := graph.Objects["ext-obj"]; !ok {
		t.Error("external object not found in graph")
	}
	if graph.Objects["ext-obj"].ResolvedFields["_external"] != true {
		t.Error("_external flag not preserved")
	}
}

// --- Diagnostics ---

func TestDiagnostics_EmittedOnNormalize(t *testing.T) {
	tree := sourceTree(
		rawObj("obj1", model.KindInstruction),
	)

	report := &pipeline.BuildReport{}
	cc := &compiler.CompilerContext{
		Config: &compiler.PipelineConfig{},
		Report: report,
	}
	ctx := compiler.ContextWithCompiler(context.Background(), cc)

	stage := normalizer.New(nil)
	_, err := stage.Execute(ctx, tree)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(report.Diagnostics) < 2 {
		t.Fatalf("Diagnostics len = %d, want >= 2", len(report.Diagnostics))
	}

	var foundStart, foundComplete bool
	for _, d := range report.Diagnostics {
		if d.Code == "NORMALIZE_START" {
			foundStart = true
		}
		if d.Code == "NORMALIZE_COMPLETE" {
			foundComplete = true
		}
	}
	if !foundStart {
		t.Error("missing NORMALIZE_START diagnostic")
	}
	if !foundComplete {
		t.Error("missing NORMALIZE_COMPLETE diagnostic")
	}
}

// --- Factory ---

func TestFactory(t *testing.T) {
	factory := normalizer.Factory(nil)
	stage, err := factory()
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if stage == nil {
		t.Fatal("Factory returned nil stage")
	}
	if stage.Descriptor().Name != "normalizer" {
		t.Errorf("Name = %q, want normalizer", stage.Descriptor().Name)
	}
}

// --- Determinism ---

func TestDeterminism_SameInputSameOutput(t *testing.T) {
	tree := sourceTree(
		rawObj("z-obj", model.KindInstruction, withScope([]string{"z/**"}, nil, nil)),
		rawObj("a-obj", model.KindInstruction, withScope([]string{"a/**"}, nil, nil)),
		rawObj("m-obj", model.KindInstruction,
			withExtends("a-obj"),
			withScope([]string{"m/**"}, nil, nil),
		),
	)

	graph1 := execute(t, tree)
	graph2 := execute(t, tree)

	// Check objects.
	for id, obj1 := range graph1.Objects {
		obj2, ok := graph2.Objects[id]
		if !ok {
			t.Errorf("graph2 missing object %q", id)
			continue
		}
		if obj1.Content != obj2.Content {
			t.Errorf("Content mismatch for %q: %q vs %q", id, obj1.Content, obj2.Content)
		}
		if obj1.SourcePath != obj2.SourcePath {
			t.Errorf("SourcePath mismatch for %q", id)
		}
	}

	// Check ScopeIndex.
	for key, ids1 := range graph1.ScopeIndex {
		ids2 := graph2.ScopeIndex[key]
		if len(ids1) != len(ids2) {
			t.Errorf("ScopeIndex[%q] len mismatch: %d vs %d", key, len(ids1), len(ids2))
			continue
		}
		for i := range ids1 {
			if ids1[i] != ids2[i] {
				t.Errorf("ScopeIndex[%q][%d] mismatch: %q vs %q", key, i, ids1[i], ids2[i])
			}
		}
	}

	// Check InheritanceChains.
	for key, chain1 := range graph1.InheritanceChains {
		chain2 := graph2.InheritanceChains[key]
		if len(chain1) != len(chain2) {
			t.Errorf("InheritanceChains[%q] len mismatch", key)
			continue
		}
		for i := range chain1 {
			if chain1[i] != chain2[i] {
				t.Errorf("InheritanceChains[%q][%d] mismatch", key, i)
			}
		}
	}
}

// --- Multiple object kinds ---

func TestMultipleKinds_AllNormalized(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("instr1", model.KindInstruction, withContent("instruction")),
		rawObj("rule1", model.KindRule, withContent("rule")),
		rawObj("skill1", model.KindSkill, withContent("skill")),
		rawObj("agent1", model.KindAgent),
		rawObj("hook1", model.KindHook),
		rawObj("cmd1", model.KindCommand),
	))

	if len(graph.Objects) != 6 {
		t.Fatalf("Objects len = %d, want 6", len(graph.Objects))
	}

	for _, id := range []string{"instr1", "rule1", "skill1", "agent1", "hook1", "cmd1"} {
		obj, ok := graph.Objects[id]
		if !ok {
			t.Errorf("missing object %q", id)
			continue
		}
		if obj.Meta.Preservation != model.PreservationOptional {
			t.Errorf("%q: Preservation = %q, want optional", id, obj.Meta.Preservation)
		}
		if obj.Meta.Version != 1 {
			t.Errorf("%q: Version = %d, want 1", id, obj.Meta.Version)
		}
		if len(obj.Meta.AppliesTo.Targets) != len(build.AllTargets()) {
			t.Errorf("%q: Targets len = %d, want %d", id, len(obj.Meta.AppliesTo.Targets), len(build.AllTargets()))
		}
	}
}

// --- Scope inheritance merges ---

func TestScopeInheritance_PathsMerged(t *testing.T) {
	graph := execute(t, sourceTree(
		rawObj("parent", model.KindRule,
			withScope([]string{"src/**"}, []string{".go"}, []string{"backend"}),
		),
		rawObj("child", model.KindRule,
			withExtends("parent"),
			withScope([]string{"test/**"}, []string{".ts"}, []string{"testing"}),
		),
	))

	child := graph.Objects["child"]

	paths := child.Meta.Scope.Paths
	sort.Strings(paths)
	if len(paths) != 2 {
		t.Fatalf("Scope.Paths = %v, want [src/** test/**]", paths)
	}

	fileTypes := child.Meta.Scope.FileTypes
	sort.Strings(fileTypes)
	if len(fileTypes) != 2 {
		t.Fatalf("Scope.FileTypes = %v, want [.go .ts]", fileTypes)
	}

	labels := child.Meta.Scope.Labels
	sort.Strings(labels)
	if len(labels) != 2 {
		t.Fatalf("Scope.Labels = %v, want [backend testing]", labels)
	}
}

// Ensure we haven't accidentally imported `time` — suppress unused import lint.
var _ = time.Now

// --- QA review fixes ---

func TestStage_NilPointerInput(t *testing.T) {
	stage := normalizer.New(nil)
	_, err := stage.Execute(context.Background(), (*pipeline.SourceTree)(nil))
	if err == nil {
		t.Fatal("expected error for nil *SourceTree")
	}
	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T", err)
	}
	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
}

func TestPurity_InputNotMutated(t *testing.T) {
	tree := sourceTree(
		rawObj("obj1", model.KindInstruction),
	)

	// Preservation should be empty before execution.
	if tree.Objects[0].Meta.Preservation != "" {
		t.Fatal("setup: Preservation should be empty before test")
	}

	_ = execute(t, tree)

	// After execution, the original tree must NOT be mutated.
	if tree.Objects[0].Meta.Preservation != "" {
		t.Error("input SourceTree was mutated: Preservation set to non-empty")
	}
	if tree.Objects[0].Meta.Version != 0 {
		t.Error("input SourceTree was mutated: Version changed from 0")
	}
	if len(tree.Objects[0].Meta.AppliesTo.Targets) != 0 {
		t.Error("input SourceTree was mutated: AppliesTo.Targets populated")
	}
}

func TestPathTraversal_Blocked(t *testing.T) {
	tree := sourceTree(
		rawObj("obj1", model.KindInstruction,
			withSourcePath("/repo/.ai/rules/my-rule.yaml"),
			withRawField("asset_path", "../../../../etc/passwd"),
		),
	)

	stage := normalizer.New(nil)
	_, err := stage.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
	var compErr *pipeline.CompilerError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected CompilerError, got %T", err)
	}
	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
}

func TestErrorMessages_IncludeSourcePath(t *testing.T) {
	// Missing extends target should include source path.
	tree := sourceTree(
		rawObj("child", model.KindInstruction,
			withExtends("nonexistent"),
			withSourcePath("/repo/.ai/child.yaml"),
		),
	)
	stage := normalizer.New(nil)
	_, err := stage.Execute(context.Background(), tree)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "/repo/.ai/child.yaml") {
		t.Errorf("error message should contain source path, got: %s", err.Error())
	}
}

// --- Diamond inheritance ---

func TestInheritance_Diamond(t *testing.T) {
	// Diamond: D extends B and C, both B and C extend A.
	graph := execute(t, sourceTree(
		rawObj("a", model.KindInstruction,
			withRawField("from_a", "a_val"),
			withOwner("team-a"),
		),
		rawObj("b", model.KindInstruction,
			withExtends("a"),
			withRawField("from_b", "b_val"),
		),
		rawObj("c", model.KindInstruction,
			withExtends("a"),
			withRawField("from_c", "c_val"),
		),
		rawObj("d", model.KindInstruction,
			withExtends("b", "c"),
		),
	))

	d := graph.Objects["d"]

	// D should have fields from all ancestors.
	if d.ResolvedFields["from_a"] != "a_val" {
		t.Errorf("from_a = %v, want a_val", d.ResolvedFields["from_a"])
	}
	if d.ResolvedFields["from_b"] != "b_val" {
		t.Errorf("from_b = %v, want b_val", d.ResolvedFields["from_b"])
	}
	if d.ResolvedFields["from_c"] != "c_val" {
		t.Errorf("from_c = %v, want c_val", d.ResolvedFields["from_c"])
	}
	if d.Meta.Owner != "team-a" {
		t.Errorf("Owner = %q, want team-a (inherited through chain)", d.Meta.Owner)
	}
}

// --- Self-referencing circular inheritance ---

func TestInheritance_SelfReference(t *testing.T) {
	compErr := executeErr(t, sourceTree(
		rawObj("self", model.KindInstruction, withExtends("self")),
	))

	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
}

// --- Three-way circular inheritance ---

func TestInheritance_ThreeWayCircle(t *testing.T) {
	compErr := executeErr(t, sourceTree(
		rawObj("a", model.KindInstruction, withExtends("c")),
		rawObj("b", model.KindInstruction, withExtends("a")),
		rawObj("c", model.KindInstruction, withExtends("b")),
	))

	if compErr.Code != pipeline.ErrNormalization {
		t.Errorf("Code = %q, want NORMALIZATION", compErr.Code)
	}
}

// --- TargetOverrides inheritance ---

func TestInheritance_TargetOverridesMerged(t *testing.T) {
	tree := sourceTree(
		rawObj("parent", model.KindInstruction),
		rawObj("child", model.KindInstruction, withExtends("parent")),
	)
	// Set parent target overrides directly.
	tree.Objects[0].Meta.TargetOverrides = map[string]model.TargetOverride{
		"claude": {Syntax: map[string]string{"format": "xml"}},
	}

	graph := execute(t, tree)

	child := graph.Objects["child"]
	if child.Meta.TargetOverrides == nil {
		t.Fatal("TargetOverrides is nil, expected inherited overrides")
	}
	override, ok := child.Meta.TargetOverrides["claude"]
	if !ok {
		t.Fatal("missing claude override")
	}
	if override.Syntax["format"] != "xml" {
		t.Errorf("Syntax[format] = %q, want xml", override.Syntax["format"])
	}
}
