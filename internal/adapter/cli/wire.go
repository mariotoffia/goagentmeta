package cli

import (
	"context"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	registryadapter "github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/claude"
	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/codex"
	"github.com/mariotoffia/goagentmeta/internal/adapter/renderer/copilot"
	cursorrenderer "github.com/mariotoffia/goagentmeta/internal/adapter/renderer/cursor"
	reporteradapter "github.com/mariotoffia/goagentmeta/internal/adapter/reporter"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/capability"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/lowering"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/materializer"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/normalizer"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/parser"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/planner"
	reporterstage "github.com/mariotoffia/goagentmeta/internal/adapter/stage/reporter"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/resolver"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/validator"
	adaptortool "github.com/mariotoffia/goagentmeta/internal/adapter/tool"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portstage "github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// buildConfig holds the resolved CLI options for pipeline construction.
type buildConfig struct {
	targets    []build.Target
	profile    build.Profile
	outputDir  string
	failFast   bool
	dryRun     bool
	syncMode   string
	reportPath string
}

// wirePipeline constructs the full compiler pipeline with all stages
// and adapters wired together. It solves the late-binding problem by
// creating a shared normalized-objects map that is populated via a
// hook after the normalize phase.
func wirePipeline(cfg buildConfig) (*compiler.Pipeline, error) {
	// Infrastructure adapters
	fsReader := filesystem.NewOSReader()
	fsWriter := filesystem.NewOSWriter()
	mat := filesystem.NewMaterializer(fsReader, fsWriter)

	// Reporter adapters
	rep := reporteradapter.NewReporter()
	sink := reporteradapter.NewDiagnosticSink()
	prov := reporteradapter.NewProvenanceRecorder()

	// Report writers
	var reportOpts []reporterstage.Option
	if cfg.reportPath != "" {
		jsonWriter := reporteradapter.NewJSONReportWriter(fsWriter, cfg.reportPath+".json")
		mdWriter := reporteradapter.NewMarkdownReportWriter(fsWriter, cfg.reportPath+".md")
		reportOpts = append(reportOpts,
			reporterstage.WithJSONWriter(jsonWriter),
			reporterstage.WithMarkdownWriter(mdWriter),
		)
	}

	// Registry adapters
	localReg := registryadapter.NewLocalRegistry(".")
	verifier := registryadapter.NewSHA256Verifier()
	depResolver := dependency.NewDependencyResolver(localReg, localReg, verifier)

	// Shared mutable map for late-binding: populated after normalize phase,
	// consumed by capability, lowering, and renderer stages.
	objects := make(map[string]pipeline.NormalizedObject)

	// Materializer stage options
	var matOpts []materializer.Option
	matOpts = append(matOpts, materializer.WithDryRun(cfg.dryRun))
	switch cfg.syncMode {
	case "copy":
		matOpts = append(matOpts, materializer.WithSyncMode(materializer.SyncCopy))
	case "symlink":
		matOpts = append(matOpts, materializer.WithSyncMode(materializer.SyncSymlink))
	default:
		matOpts = append(matOpts, materializer.WithSyncMode(materializer.SyncBuildOnly))
	}

	// Build stages
	toolReg := adaptortool.NewDefaultRegistry()

	valStage, err := validator.New(validator.WithToolRegistry(toolReg))
	if err != nil {
		return nil, err
	}

	opts := []compiler.Option{
		// Infrastructure
		compiler.WithFSReader(fsReader),
		compiler.WithFSWriter(fsWriter),
		compiler.WithMaterializer(mat),
		compiler.WithReporter(rep),
		compiler.WithDiagnosticSink(sink),
		compiler.WithProvenanceRecorder(prov),

		// Config
		compiler.WithFailFast(cfg.failFast),
		compiler.WithProfile(cfg.profile),

		// Stages: parse → validate → resolve → normalize → plan → capability → lower → render → materialize → report
		compiler.WithStage(parser.New()),
		compiler.WithStage(valStage),
		compiler.WithStage(resolver.New(depResolver)),
		compiler.WithStage(normalizer.New(fsReader)),
		compiler.WithStage(planner.New()),
		compiler.WithStage(capability.New(objects, nil)),
		compiler.WithStage(lowering.New(objects)),
		compiler.WithStage(materializer.New(mat, matOpts...)),
		compiler.WithStage(reporterstage.New(reportOpts...)),

		// Late-binding hook: populate shared objects map after normalize phase
		compiler.WithHook(&normalizeHook{objects: objects}),
	}

	// Renderers — only add requested targets
	targets := cfg.targets
	if len(targets) == 0 {
		targets = build.AllTargets()
	}
	for _, t := range targets {
		switch t {
		case build.TargetClaude:
			opts = append(opts, compiler.WithStage(claude.New(objects)))
		case build.TargetCodex:
			opts = append(opts, compiler.WithStage(codex.New(objects)))
		case build.TargetCopilot:
			opts = append(opts, compiler.WithStage(copilot.New(objects)))
		case build.TargetCursor:
			opts = append(opts, compiler.WithStage(cursorrenderer.New(objects)))
		}
	}
	opts = append(opts, compiler.WithTargets(targets...))

	return compiler.NewPipeline(opts...), nil
}

// normalizeHook is a StageHookHandler that copies normalized objects from
// the SemanticGraph into a shared map after the normalize phase completes.
// This solves the late-binding problem: stages in later phases (capability,
// lowering, renderers) receive the shared map at construction time and see
// it populated once the normalizer produces the SemanticGraph.
type normalizeHook struct {
	objects map[string]pipeline.NormalizedObject
}

func (h *normalizeHook) Hook() pipeline.StageHook {
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

// Compile-time assertion.
var _ portstage.StageHookHandler = (*normalizeHook)(nil)
