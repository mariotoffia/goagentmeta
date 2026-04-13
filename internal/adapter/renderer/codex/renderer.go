// Package codex implements the Codex CLI target renderer. It transforms
// a LoweredGraph into an EmissionPlan containing Codex-native files:
// AGENTS.md hierarchy, .codex/rules/, .codex/skills/, .codex/agents/,
// .codex/settings.json (hooks), .mcp.json (MCP servers), and .codex-plugin/.
//
// Codex is similar to Claude Code in most capability surfaces. Key differences:
//   - Instruction files are named AGENTS.md (not CLAUDE.md)
//   - Skills live in .codex/skills/ (not .claude/skills/)
//   - Commands are lowered to skill invocations (same as Claude)
//   - Hooks support a subset of events: SessionStart, UserPromptSubmit,
//     PreToolUse, PostToolUse, Stop
//   - agents.handoffs is skipped
//
// The renderer is a specialized pipeline Stage registered for PhaseRender
// with TargetFilter: [codex]. It consumes only lowered objects and never
// interprets raw source files directly.
package codex

import (
	"context"
	"fmt"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/capability"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/renderer"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"

	capstage "github.com/mariotoffia/goagentmeta/internal/adapter/stage/capability"
)

// Compile-time assertions.
var (
	_ stage.Stage       = (*Renderer)(nil)
	_ renderer.Renderer = (*Renderer)(nil)
)

// codexSupportedHookEvents lists the hook events that Codex CLI natively
// supports. Hooks with events not in this set are filtered out with a
// diagnostic warning.
var codexSupportedHookEvents = map[string]bool{
	"SessionStart":     true,
	"UserPromptSubmit": true,
	"PreToolUse":       true,
	"PostToolUse":      true,
	"Stop":             true,
}

// Renderer implements the Codex CLI target renderer. It orchestrates
// layer generators to produce Codex-native files from lowered IR.
type Renderer struct {
	objects map[string]pipeline.NormalizedObject
}

// New creates a new Codex CLI renderer.
func New(objects map[string]pipeline.NormalizedObject) *Renderer {
	return &Renderer{objects: objects}
}

// Target returns the target ecosystem this renderer handles.
func (r *Renderer) Target() build.Target {
	return build.TargetCodex
}

// SupportedCapabilities returns the capability registry for Codex CLI.
func (r *Renderer) SupportedCapabilities() capability.CapabilityRegistry {
	reg, err := capstage.LoadRegistry("codex")
	if err != nil {
		return capability.CapabilityRegistry{
			Target:   "codex",
			Surfaces: make(map[string]capability.SupportLevel),
		}
	}
	return *reg
}

// Descriptor returns the stage metadata for pipeline registration.
func (r *Renderer) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:         "codex-renderer",
		Phase:        pipeline.PhaseRender,
		Order:        10,
		TargetFilter: []build.Target{build.TargetCodex},
	}
}

// Execute transforms a LoweredGraph into an EmissionPlan with Codex-native files.
func (r *Renderer) Execute(ctx context.Context, input any) (any, error) {
	graph, ok := input.(pipeline.LoweredGraph)
	if !ok {
		graphPtr, ok := input.(*pipeline.LoweredGraph)
		if !ok || graphPtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrRendering,
				fmt.Sprintf("expected pipeline.LoweredGraph or *pipeline.LoweredGraph, got %T", input),
				"codex-renderer",
			)
		}
		graph = *graphPtr
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "RENDER_START",
		Message:  fmt.Sprintf("rendering Codex CLI output for %d unit(s)", len(graph.Units)),
		Phase:    pipeline.PhaseRender,
	})

	plan := pipeline.EmissionPlan{
		Units: make(map[string]UnitEmission, len(graph.Units)),
	}

	unitKeys := sortedKeys(graph.Units)

	for _, unitKey := range unitKeys {
		unit := graph.Units[unitKey]
		if string(unit.Coordinate.Unit.Target) != "codex" {
			continue
		}

		emission, err := r.renderUnit(ctx, unitKey, unit)
		if err != nil {
			return plan, err
		}
		plan.Units[unitKey] = emission
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "RENDER_COMPLETE",
		Message:  fmt.Sprintf("rendered %d Codex CLI unit(s)", len(plan.Units)),
		Phase:    pipeline.PhaseRender,
	})

	return plan, nil
}

// Factory returns a StageFactory for the Codex renderer.
func Factory(objects map[string]pipeline.NormalizedObject) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(objects), nil
	}
}

// renderUnit renders all Codex-native files for a single build unit.
func (r *Renderer) renderUnit(
	ctx context.Context,
	unitKey string,
	unit pipeline.LoweredUnit,
) (pipeline.UnitEmission, error) {
	emission := pipeline.UnitEmission{
		Coordinate: unit.Coordinate,
	}

	classified := classifyObjects(unit.Objects)

	// Layer 1: Instructions → AGENTS.md hierarchy
	files := renderInstructions(classified.instructions, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 2: Rules → .codex/rules/{id}.md
	files = renderRules(classified.rules, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 3: Skills → .codex/skills/{id}/SKILL.md
	skillFiles, skillAssets := renderSkills(classified.skills, r.objects)
	emission.Files = append(emission.Files, skillFiles...)
	emission.Assets = append(emission.Assets, skillAssets...)

	// Layer 4: Agents → .codex/agents/{id}.md
	files = renderAgents(classified.agents, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 5: Settings → .codex/settings.json (hooks + permissions)
	perms := collectPermissions(classified.agents, classified.skills)
	files = renderSettings(ctx, classified.hooks, perms)
	emission.Files = append(emission.Files, files...)

	// Layer 6: MCP config → .mcp.json
	files = renderMCP(classified.plugins, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 7: Plugins → .codex-plugin/
	pluginFiles, pluginBundles, installEntries := renderPlugins(classified.plugins, r.objects)
	emission.Files = append(emission.Files, pluginFiles...)
	emission.PluginBundles = append(emission.PluginBundles, pluginBundles...)
	emission.InstallMetadata = append(emission.InstallMetadata, installEntries...)

	// Warn about un-rendered command objects (should be lowered to skills before rendering).
	if len(classified.commands) > 0 {
		ids := make([]string, len(classified.commands))
		for i, cmd := range classified.commands {
			ids[i] = cmd.OriginalID
		}
		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "warning",
			Code:     "RENDER_UNLOWERED_COMMANDS",
			Message:  fmt.Sprintf("%d command(s) reached renderer without lowering: %v", len(ids), ids),
			Phase:    pipeline.PhaseRender,
		})
	}

	// Layer 8: Provenance headers (applied to all files) + provenance.json
	emission.Files = injectProvenanceHeaders(emission.Files)
	provenanceFile := renderProvenance(unitKey, emission.Files)
	emission.Files = append(emission.Files, provenanceFile)

	emission.Directories = collectDirectories(emission.Files)

	recordProvenance(ctx, emission.Files)

	return emission, nil
}

// classifiedObjects groups lowered objects by their effective kind for layer dispatch.
type classifiedObjects struct {
	instructions []pipeline.LoweredObject
	rules        []pipeline.LoweredObject
	skills       []pipeline.LoweredObject
	agents       []pipeline.LoweredObject
	hooks        []pipeline.LoweredObject
	commands     []pipeline.LoweredObject
	plugins      []pipeline.LoweredObject
}

// classifyObjects groups lowered objects by kind, sorting each group by ID
// for deterministic output.
func classifyObjects(objects map[string]pipeline.LoweredObject) classifiedObjects {
	var c classifiedObjects

	keys := make([]string, 0, len(objects))
	for k := range objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		obj := objects[key]

		if obj.Decision.Action == "skipped" {
			continue
		}

		switch obj.LoweredKind {
		case model.KindInstruction:
			c.instructions = append(c.instructions, obj)
		case model.KindRule:
			c.rules = append(c.rules, obj)
		case model.KindSkill:
			c.skills = append(c.skills, obj)
		case model.KindAgent:
			c.agents = append(c.agents, obj)
		case model.KindHook:
			c.hooks = append(c.hooks, obj)
		case model.KindCommand:
			c.commands = append(c.commands, obj)
		case model.KindPlugin:
			c.plugins = append(c.plugins, obj)
		}
	}

	return c
}

// sortedKeys returns the sorted keys of a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// collectDirectories extracts unique directory paths from emitted files.
func collectDirectories(files []pipeline.EmittedFile) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, f := range files {
		dir := dirOf(f.Path)
		if dir != "" && dir != "." && !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// dirOf returns the directory component of a path.
func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
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

// recordProvenance records provenance through the CompilerContext if available.
func recordProvenance(ctx context.Context, files []pipeline.EmittedFile) {
	cc := compiler.CompilerFromContext(ctx)
	if cc == nil || cc.Config == nil || cc.Config.ProvenanceRecorder == nil {
		return
	}
	for _, f := range files {
		for _, src := range f.SourceObjects {
			cc.Config.ProvenanceRecorder.Record(ctx, src, f.Path, []string{"rendered"})
		}
	}
}

// UnitEmission is a type alias used only in the renderer's return.
type UnitEmission = pipeline.UnitEmission
