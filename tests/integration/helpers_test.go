package integration_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/claude"
	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/codex"
	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/copilot"
	reporteradapter "github.com/mariotoffia/goagentmeta/internal/adapter/reporter"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/capability"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/lowering"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/materializer"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/normalizer"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/planner"
	reporterstage "github.com/mariotoffia/goagentmeta/internal/adapter/stage/reporter"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/validator"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portstage "github.com/mariotoffia/goagentmeta/internal/port/stage"

	"gopkg.in/yaml.v3"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// ─── Fixture Parser ────────────────────────────────────────────────────────

// parseSourceDir reads a source directory (containing manifest.yaml and known
// subdirectories like instructions/, rules/, etc.) and produces a SourceTree.
// rootDir is the project root for path resolution; sourceDir is the directory
// that contains the objects.
func parseSourceDir(t *testing.T, rootDir, sourceDir string) *pipeline.SourceTree {
	t.Helper()

	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		t.Fatalf("source directory %s does not exist", sourceDir)
	}

	tree := &pipeline.SourceTree{
		RootPath:      rootDir,
		SchemaVersion: 1,
		ManifestPath:  filepath.Join(sourceDir, "manifest.yaml"),
	}

	// Read manifest for schema version.
	if data, err := os.ReadFile(tree.ManifestPath); err == nil {
		var manifest map[string]any
		if err := yaml.Unmarshal(data, &manifest); err == nil {
			if sv, ok := manifest["schemaVersion"].(int); ok {
				tree.SchemaVersion = sv
			}
		}
	}

	// Walk subdirectories for object YAML files.
	subdirs := []string{"instructions", "rules", "skills", "agents", "hooks", "commands", "capabilities", "plugins"}
	for _, sub := range subdirs {
		subPath := filepath.Join(sourceDir, sub)
		entries, err := os.ReadDir(subPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}
			filePath := filepath.Join(subPath, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("reading fixture %s: %v", filePath, err)
			}
			obj := parseYAMLObject(t, data, filePath)
			tree.Objects = append(tree.Objects, obj)
		}
	}

	// Sort objects by ID for deterministic behavior.
	sort.Slice(tree.Objects, func(i, j int) bool {
		return tree.Objects[i].Meta.ID < tree.Objects[j].Meta.ID
	})

	return tree
}

// parseFixtureDir reads a fixture directory containing a .ai/ subdirectory
// and produces a SourceTree suitable for pipeline input. The .ai/ subdirectory
// is the default convention; use parseSourceDir for custom source directories.
func parseFixtureDir(t *testing.T, dir string) *pipeline.SourceTree {
	t.Helper()

	aiDir := filepath.Join(dir, ".ai")
	if _, err := os.Stat(aiDir); os.IsNotExist(err) {
		t.Fatalf("fixture directory %s has no .ai/ subdirectory", dir)
	}

	return parseSourceDir(t, dir, aiDir)
}

// parseYAMLObject parses a YAML file into a pipeline.RawObject.
func parseYAMLObject(t *testing.T, data []byte, sourcePath string) pipeline.RawObject {
	t.Helper()

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing YAML %s: %v", sourcePath, err)
	}

	meta := model.ObjectMeta{}

	if id, ok := raw["id"].(string); ok {
		meta.ID = id
	}
	if kind, ok := raw["kind"].(string); ok {
		meta.Kind = model.Kind(kind)
	}
	if v, ok := raw["version"].(int); ok {
		meta.Version = v
	}
	if desc, ok := raw["description"].(string); ok {
		meta.Description = desc
	}
	if pres, ok := raw["preservation"].(string); ok {
		meta.Preservation = model.Preservation(pres)
	}
	if owner, ok := raw["owner"].(string); ok {
		meta.Owner = owner
	}
	if extends, ok := raw["extends"].([]any); ok {
		meta.Extends = toStringSlice(extends)
	}
	if labels, ok := raw["labels"].([]any); ok {
		meta.Labels = toStringSlice(labels)
	}
	if scopeMap, ok := raw["scope"].(map[string]any); ok {
		meta.Scope = extractScope(scopeMap)
	}
	if atMap, ok := raw["appliesTo"].(map[string]any); ok {
		meta.AppliesTo = extractAppliesTo(atMap)
	}

	// Build RawFields from non-meta fields.
	metaKeys := map[string]bool{
		"id": true, "kind": true, "version": true, "description": true,
		"preservation": true, "scope": true, "appliesTo": true,
		"extends": true, "labels": true, "owner": true, "targetOverrides": true,
		"packageVersion": true, "license": true,
	}
	rawFields := make(map[string]any)
	for k, v := range raw {
		if !metaKeys[k] {
			rawFields[k] = v
		}
	}

	var rawContent string
	if c, ok := raw["content"].(string); ok {
		rawContent = c
	}

	return pipeline.RawObject{
		Meta:       meta,
		SourcePath: sourcePath,
		RawContent: rawContent,
		RawFields:  rawFields,
	}
}

func extractScope(m map[string]any) model.Scope {
	s := model.Scope{}
	if paths, ok := m["paths"].([]any); ok {
		s.Paths = toStringSlice(paths)
	}
	if ft, ok := m["fileTypes"].([]any); ok {
		s.FileTypes = toStringSlice(ft)
	}
	if labels, ok := m["labels"].([]any); ok {
		s.Labels = toStringSlice(labels)
	}
	return s
}

func extractAppliesTo(m map[string]any) model.AppliesTo {
	at := model.AppliesTo{}
	if targets, ok := m["targets"].([]any); ok {
		at.Targets = toStringSlice(targets)
	}
	if profiles, ok := m["profiles"].([]any); ok {
		at.Profiles = toStringSlice(profiles)
	}
	return at
}

func toStringSlice(v []any) []string {
	out := make([]string, 0, len(v))
	for _, item := range v {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ─── Test Parser Stage ─────────────────────────────────────────────────────

// testParserStage wraps a pre-parsed SourceTree as a PhaseParse stage.
type testParserStage struct {
	tree *pipeline.SourceTree
}

func (s *testParserStage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "test-fixture-parser",
		Phase: pipeline.PhaseParse,
		Order: 0,
	}
}

func (s *testParserStage) Execute(_ context.Context, _ any) (any, error) {
	return *s.tree, nil
}

var _ portstage.Stage = (*testParserStage)(nil)

// ─── Multi-Renderer Wrapper ────────────────────────────────────────────────

// multiRendererStage fans out the LoweredGraph to each renderer and merges
// the resulting EmissionPlans. This works around the pipeline's sequential
// stage execution which prevents multiple renderers from receiving the same
// LoweredGraph input.
type multiRendererStage struct {
	renderers []portstage.Stage
}

func (m *multiRendererStage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "multi-renderer",
		Phase: pipeline.PhaseRender,
		Order: 10,
	}
}

func (m *multiRendererStage) Execute(ctx context.Context, input any) (any, error) {
	merged := pipeline.EmissionPlan{Units: make(map[string]pipeline.UnitEmission)}
	for _, r := range m.renderers {
		output, err := r.Execute(ctx, input)
		if err != nil {
			return nil, err
		}
		if plan, ok := output.(pipeline.EmissionPlan); ok {
			for k, v := range plan.Units {
				merged.Units[k] = v
			}
		}
	}
	return merged, nil
}

var _ portstage.Stage = (*multiRendererStage)(nil)

// ─── Normalize Hook Bridge ─────────────────────────────────────────────────

// normalizeHookBridge copies normalized objects from the SemanticGraph into
// a shared map after the normalize phase, enabling downstream stages
// (capability, lowering, renderers) to access object metadata.
type normalizeHookBridge struct {
	objects map[string]pipeline.NormalizedObject
}

func (h *normalizeHookBridge) Hook() pipeline.StageHook {
	return pipeline.StageHook{
		Name:  "normalize-objects-bridge",
		Point: pipeline.HookAfterPhase,
		Phase: pipeline.PhaseNormalize,
		Handler: func(_ context.Context, ir any) (any, error) {
			sg, ok := ir.(pipeline.SemanticGraph)
			if !ok {
				if ptr, ok2 := ir.(*pipeline.SemanticGraph); ok2 {
					sg = *ptr
				} else {
					return ir, nil
				}
			}
			for k, v := range sg.Objects {
				h.objects[k] = v
			}
			return ir, nil
		},
	}
}

var _ portstage.StageHookHandler = (*normalizeHookBridge)(nil)

// ─── Pipeline Builder ──────────────────────────────────────────────────────

type pipelineResult struct {
	Pipeline *compiler.Pipeline
	MemFS    *filesystem.MemFS
	Objects  map[string]pipeline.NormalizedObject
}

// buildTestPipeline constructs a full compiler pipeline using real stages,
// MemFS for materialization, and a test parser stage to inject the fixture.
func buildTestPipeline(
	t *testing.T,
	tree *pipeline.SourceTree,
	targets []build.Target,
	profile build.Profile,
	dryRun bool,
) pipelineResult {
	t.Helper()

	memfs := filesystem.NewMemFS()
	objects := make(map[string]pipeline.NormalizedObject)

	valStage, err := validator.New()
	if err != nil {
		t.Fatalf("validator.New: %v", err)
	}

	matOpts := []materializer.Option{
		materializer.WithSyncMode(materializer.SyncBuildOnly),
		materializer.WithDryRun(dryRun),
	}

	sink := reporteradapter.NewDiagnosticSink()
	rep := reporteradapter.NewReporter()
	prov := reporteradapter.NewProvenanceRecorder()

	opts := []compiler.Option{
		// Infrastructure.
		compiler.WithMaterializer(memfs),
		compiler.WithDiagnosticSink(sink),
		compiler.WithReporter(rep),
		compiler.WithProvenanceRecorder(prov),

		// Config.
		compiler.WithFailFast(true),
		compiler.WithProfile(profile),

		// Stages: parse → validate → normalize → plan → capability → lower → materialize → report.
		compiler.WithStage(&testParserStage{tree: tree}),
		compiler.WithStage(valStage),
		compiler.WithStage(normalizer.New(nil)),
		compiler.WithStage(planner.New()),
		compiler.WithStage(capability.New(objects, nil)),
		compiler.WithStage(lowering.New(objects)),
		compiler.WithStage(materializer.New(memfs, matOpts...)),
		compiler.WithStage(reporterstage.New()),

		// Hook: bridge normalized objects to shared map.
		compiler.WithHook(&normalizeHookBridge{objects: objects}),
	}

	// Register renderers. Use a multi-renderer wrapper to fan out the
	// LoweredGraph to each target renderer and merge EmissionPlans.
	effectiveTargets := targets
	if len(effectiveTargets) == 0 {
		effectiveTargets = []build.Target{build.TargetClaude, build.TargetCopilot, build.TargetCodex}
	}

	var renderers []portstage.Stage
	for _, target := range effectiveTargets {
		switch target {
		case build.TargetClaude:
			renderers = append(renderers, claude.New(objects))
		case build.TargetCopilot:
			renderers = append(renderers, copilot.New(objects))
		case build.TargetCodex:
			renderers = append(renderers, codex.New(objects))
		}
	}
	if len(renderers) == 1 {
		opts = append(opts, compiler.WithStage(renderers[0]))
	} else if len(renderers) > 1 {
		opts = append(opts, compiler.WithStage(&multiRendererStage{renderers: renderers}))
	}

	opts = append(opts, compiler.WithTargets(effectiveTargets...))

	return pipelineResult{
		Pipeline: compiler.NewPipeline(opts...),
		MemFS:    memfs,
		Objects:  objects,
	}
}

// runPipeline executes the pipeline and returns the report.
func runPipeline(t *testing.T, pr pipelineResult) *pipeline.BuildReport {
	t.Helper()
	report, err := pr.Pipeline.Execute(context.Background(), []string{"."})
	if err != nil {
		t.Fatalf("pipeline.Execute: %v", err)
	}
	return report
}

// ─── Golden File Utilities ─────────────────────────────────────────────────

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

func fixturesDir() string {
	return filepath.Join(testdataDir(), "fixtures")
}

func goldenBaseDir() string {
	return filepath.Join(testdataDir(), "golden")
}

// pathToGoldenFilename converts a nested path to a flat golden filename.
// Example: ".claude/rules/go-rule.md" → ".claude__rules__go-rule.md"
func pathToGoldenFilename(path string) string {
	return strings.ReplaceAll(path, "/", "__")
}

// goldenFilenameToPath converts a flat golden filename back to a nested path.
func goldenFilenameToPath(name string) string {
	return strings.ReplaceAll(name, "__", "/")
}

// updateOrCompareGolden either updates or compares a single golden file.
func updateOrCompareGolden(t *testing.T, goldenDir, name string, actual []byte) {
	t.Helper()
	goldenPath := filepath.Join(goldenDir, name)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
			t.Fatalf("writing golden file %s: %v", goldenPath, err)
		}
		t.Logf("updated golden: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\n\n"+
			"hint: run with -update to generate golden files:\n"+
			"  go test ./tests/integration/ -run %s -update", goldenPath, err, t.Name())
	}

	if !bytes.Equal(expected, actual) {
		t.Errorf("golden mismatch: %s\n\nexpected:\n%s\n\nactual:\n%s",
			goldenPath, string(expected), string(actual))
	}
}

// ─── In-Memory SourceTree Builder ──────────────────────────────────────────

// makeRawObject builds a pipeline.RawObject from common fields. Simplifies
// construction of synthetic test fixtures without touching the filesystem.
func makeRawObject(id string, kind model.Kind, content string, opts ...rawObjectOption) pipeline.RawObject {
	obj := pipeline.RawObject{
		Meta: model.ObjectMeta{
			ID:           id,
			Kind:         kind,
			Preservation: model.PreservationPreferred,
		},
		SourcePath: ".ai/" + string(kind) + "s/" + id + ".yaml",
		RawContent: content,
		RawFields:  make(map[string]any),
	}
	if content != "" {
		obj.RawFields["content"] = content
	}
	for _, opt := range opts {
		opt(&obj)
	}
	return obj
}

type rawObjectOption func(*pipeline.RawObject)

func withPreservation(p model.Preservation) rawObjectOption {
	return func(o *pipeline.RawObject) { o.Meta.Preservation = p }
}

func withScopePaths(paths ...string) rawObjectOption {
	return func(o *pipeline.RawObject) { o.Meta.Scope.Paths = paths }
}

func withScopeFileTypes(types ...string) rawObjectOption {
	return func(o *pipeline.RawObject) { o.Meta.Scope.FileTypes = types }
}

func withLabels(labels ...string) rawObjectOption {
	return func(o *pipeline.RawObject) { o.Meta.Labels = labels }
}

func withConditions(conditions []model.RuleCondition) rawObjectOption {
	return func(o *pipeline.RawObject) {
		items := make([]any, len(conditions))
		for i, c := range conditions {
			items[i] = map[string]any{"type": c.Type, "value": c.Value}
		}
		o.RawFields["conditions"] = items
	}
}

func withAppliesTo(targets []string, profiles []string) rawObjectOption {
	return func(o *pipeline.RawObject) {
		o.Meta.AppliesTo.Targets = targets
		o.Meta.AppliesTo.Profiles = profiles
	}
}

func withSkills(skills ...string) rawObjectOption {
	return func(o *pipeline.RawObject) {
		items := make([]any, len(skills))
		for i, s := range skills {
			items[i] = s
		}
		o.RawFields["skills"] = items
	}
}

func withHookFields(event, actionType, actionRef string) rawObjectOption {
	return func(o *pipeline.RawObject) {
		o.RawFields["event"] = event
		o.RawFields["action"] = map[string]any{"type": actionType, "ref": actionRef}
	}
}

func withDistribution(mode string) rawObjectOption {
	return func(o *pipeline.RawObject) {
		o.RawFields["distribution"] = map[string]any{"mode": mode}
	}
}

func withMCPServers(servers map[string]any) rawObjectOption {
	return func(o *pipeline.RawObject) {
		o.RawFields["mcpServers"] = servers
	}
}

func withCommandAction(actionType, ref string) rawObjectOption {
	return func(o *pipeline.RawObject) {
		o.RawFields["action"] = map[string]any{"type": actionType, "ref": ref}
	}
}

func withDescription(desc string) rawObjectOption {
	return func(o *pipeline.RawObject) { o.Meta.Description = desc }
}

func withOwner(owner string) rawObjectOption {
	return func(o *pipeline.RawObject) { o.Meta.Owner = owner }
}

func withExtends(ids ...string) rawObjectOption {
	return func(o *pipeline.RawObject) { o.Meta.Extends = ids }
}

// makeSourceTree creates a SourceTree from a slice of RawObjects.
func makeSourceTree(objects ...pipeline.RawObject) *pipeline.SourceTree {
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Meta.ID < objects[j].Meta.ID
	})
	return &pipeline.SourceTree{
		RootPath:      ".",
		SchemaVersion: 1,
		ManifestPath:  ".ai/manifest.yaml",
		Objects:       objects,
	}
}

// ─── Pipeline Builder Variants ─────────────────────────────────────────────

// buildTestPipelineNoFailFast is like buildTestPipeline but with FailFast disabled.
func buildTestPipelineNoFailFast(
	t *testing.T,
	tree *pipeline.SourceTree,
	targets []build.Target,
	profile build.Profile,
) pipelineResult {
	t.Helper()

	memfs := filesystem.NewMemFS()
	objects := make(map[string]pipeline.NormalizedObject)

	valStage, err := validator.New()
	if err != nil {
		t.Fatalf("validator.New: %v", err)
	}

	matOpts := []materializer.Option{
		materializer.WithSyncMode(materializer.SyncBuildOnly),
	}

	sink := reporteradapter.NewDiagnosticSink()
	rep := reporteradapter.NewReporter()
	prov := reporteradapter.NewProvenanceRecorder()

	opts := []compiler.Option{
		compiler.WithMaterializer(memfs),
		compiler.WithDiagnosticSink(sink),
		compiler.WithReporter(rep),
		compiler.WithProvenanceRecorder(prov),
		compiler.WithFailFast(false),
		compiler.WithProfile(profile),
		compiler.WithStage(&testParserStage{tree: tree}),
		compiler.WithStage(valStage),
		compiler.WithStage(normalizer.New(nil)),
		compiler.WithStage(planner.New()),
		compiler.WithStage(capability.New(objects, nil)),
		compiler.WithStage(lowering.New(objects)),
		compiler.WithStage(materializer.New(memfs, matOpts...)),
		compiler.WithStage(reporterstage.New()),
		compiler.WithHook(&normalizeHookBridge{objects: objects}),
	}

	effectiveTargets := targets
	if len(effectiveTargets) == 0 {
		effectiveTargets = []build.Target{build.TargetClaude, build.TargetCopilot, build.TargetCodex}
	}
	var renderers []portstage.Stage
	for _, target := range effectiveTargets {
		switch target {
		case build.TargetClaude:
			renderers = append(renderers, claude.New(objects))
		case build.TargetCopilot:
			renderers = append(renderers, copilot.New(objects))
		case build.TargetCodex:
			renderers = append(renderers, codex.New(objects))
		}
	}
	if len(renderers) == 1 {
		opts = append(opts, compiler.WithStage(renderers[0]))
	} else if len(renderers) > 1 {
		opts = append(opts, compiler.WithStage(&multiRendererStage{renderers: renderers}))
	}
	opts = append(opts, compiler.WithTargets(effectiveTargets...))

	return pipelineResult{
		Pipeline: compiler.NewPipeline(opts...),
		MemFS:    memfs,
		Objects:  objects,
	}
}

// runPipelineExpectError runs the pipeline and returns report + error.
func runPipelineExpectError(t *testing.T, pr pipelineResult) (*pipeline.BuildReport, error) {
	t.Helper()
	return pr.Pipeline.Execute(context.Background(), []string{"."})
}

// ─── Additional Assertion Helpers ──────────────────────────────────────────

// fileCount returns the number of files in the MemFS.
func fileCount(memfs *filesystem.MemFS) int {
	return len(memfs.Files())
}

// filesForTarget returns all file paths for a given target.
func filesForTarget(memfs *filesystem.MemFS, target string) []string {
	var result []string
	prefix := ".ai-build/" + target + "/"
	for path := range memfs.Files() {
		if hasPrefix(path, prefix) {
			result = append(result, path)
		}
	}
	sort.Strings(result)
	return result
}

// fileExists checks if a specific file path exists in MemFS.
func fileExists(memfs *filesystem.MemFS, path string) bool {
	_, ok := memfs.Files()[path]
	return ok
}

// fileContains checks if a file in MemFS contains a given substring.
func fileContains(memfs *filesystem.MemFS, path, substr string) bool {
	files := memfs.Files()
	content, ok := files[path]
	if !ok {
		return false
	}
	return strings.Contains(string(content), substr)
}

// hasDiagnosticCode checks if the report contains a diagnostic with a given code.
func hasDiagnosticCode(report *pipeline.BuildReport, code string) bool {
	for _, d := range report.Diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

// compareGoldenOutput compares all files in MemFS against golden files
// organized by target/profile subdirectories.
func compareGoldenOutput(t *testing.T, fixture string, memfs *filesystem.MemFS) {
	t.Helper()
	goldenDir := filepath.Join(goldenBaseDir(), fixture)

	files := memfs.Files()

	// Group files by build unit path prefix: .ai-build/{target}/{profile}/
	filesByUnit := make(map[string]map[string][]byte)
	for path, content := range files {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) >= 4 && parts[0] == ".ai-build" {
			unitKey := fmt.Sprintf("%s--%s", parts[1], parts[2])
			relPath := parts[3]
			if filesByUnit[unitKey] == nil {
				filesByUnit[unitKey] = make(map[string][]byte)
			}
			filesByUnit[unitKey][relPath] = content
		}
	}

	// Sort unit keys for deterministic golden output.
	unitKeys := make([]string, 0, len(filesByUnit))
	for k := range filesByUnit {
		unitKeys = append(unitKeys, k)
	}
	sort.Strings(unitKeys)

	for _, unitKey := range unitKeys {
		unitFiles := filesByUnit[unitKey]

		// Sort file paths within the unit.
		filePaths := make([]string, 0, len(unitFiles))
		for p := range unitFiles {
			filePaths = append(filePaths, p)
		}
		sort.Strings(filePaths)

		unitGoldenDir := filepath.Join(goldenDir, unitKey)
		for _, fp := range filePaths {
			goldenName := pathToGoldenFilename(fp)
			updateOrCompareGolden(t, unitGoldenDir, goldenName, unitFiles[fp])
		}

		// When not updating, verify no extra golden files exist.
		if !*updateGolden {
			goldenEntries, err := os.ReadDir(unitGoldenDir)
			if err == nil {
				for _, entry := range goldenEntries {
					if entry.IsDir() {
						continue
					}
					origPath := goldenFilenameToPath(entry.Name())
					if _, found := unitFiles[origPath]; !found {
						t.Errorf("extra golden file not produced by pipeline: %s/%s",
							unitKey, entry.Name())
					}
				}
			}
		}
	}
}
