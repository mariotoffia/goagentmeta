// Package claude implements the Claude Code target renderer. It transforms
// a LoweredGraph into an EmissionPlan containing Claude-native files:
// CLAUDE.md hierarchy, .claude/rules/, .claude/skills/, .claude/agents/,
// .claude/settings.json (hooks), .mcp.json (MCP servers), and .claude-plugin/.
//
// The renderer is a specialized pipeline Stage registered for PhaseRender
// with TargetFilter: [claude]. It consumes only lowered objects and never
// interprets raw source files directly.
package claude

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

// Renderer implements the Claude Code target renderer. It orchestrates
// layer generators to produce Claude-native files from lowered IR.
type Renderer struct {
	objects map[string]pipeline.NormalizedObject
}

// New creates a new Claude Code renderer.
func New(objects map[string]pipeline.NormalizedObject) *Renderer {
	return &Renderer{objects: objects}
}

// Target returns the target ecosystem this renderer handles.
func (r *Renderer) Target() build.Target {
	return build.TargetClaude
}

// SupportedCapabilities returns the capability registry for Claude Code.
func (r *Renderer) SupportedCapabilities() capability.CapabilityRegistry {
	reg, err := capstage.LoadRegistry("claude")
	if err != nil {
		return capability.CapabilityRegistry{
			Target:   "claude",
			Surfaces: make(map[string]capability.SupportLevel),
		}
	}
	return *reg
}

// Descriptor returns the stage metadata for pipeline registration.
func (r *Renderer) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:         "claude-renderer",
		Phase:        pipeline.PhaseRender,
		Order:        10,
		TargetFilter: []build.Target{build.TargetClaude},
	}
}

// Execute transforms a LoweredGraph into an EmissionPlan with Claude-native files.
func (r *Renderer) Execute(ctx context.Context, input any) (any, error) {
	graph, ok := input.(pipeline.LoweredGraph)
	if !ok {
		graphPtr, ok := input.(*pipeline.LoweredGraph)
		if !ok || graphPtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrRendering,
				fmt.Sprintf("expected pipeline.LoweredGraph or *pipeline.LoweredGraph, got %T", input),
				"claude-renderer",
			)
		}
		graph = *graphPtr
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "RENDER_START",
		Message:  fmt.Sprintf("rendering Claude Code output for %d unit(s)", len(graph.Units)),
		Phase:    pipeline.PhaseRender,
	})

	plan := pipeline.EmissionPlan{
		Units: make(map[string]UnitEmission, len(graph.Units)),
	}

	// Sort unit keys for deterministic output.
	unitKeys := sortedKeys(graph.Units)

	for _, unitKey := range unitKeys {
		unit := graph.Units[unitKey]
		if string(unit.Coordinate.Unit.Target) != "claude" {
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
		Message:  fmt.Sprintf("rendered %d Claude Code unit(s)", len(plan.Units)),
		Phase:    pipeline.PhaseRender,
	})

	return plan, nil
}

// Factory returns a StageFactory for the Claude renderer.
func Factory(objects map[string]pipeline.NormalizedObject) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(objects), nil
	}
}

// renderUnit renders all Claude-native files for a single build unit.
func (r *Renderer) renderUnit(
	ctx context.Context,
	unitKey string,
	unit pipeline.LoweredUnit,
) (pipeline.UnitEmission, error) {
	emission := pipeline.UnitEmission{
		Coordinate: unit.Coordinate,
	}

	// Classify objects by their lowered kind for layer dispatch.
	classified := classifyObjects(unit.Objects)

	// Layer 1: Instructions → CLAUDE.md hierarchy
	files := renderInstructions(classified.instructions, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 2: Rules → .claude/rules/{id}.md
	files = renderRules(classified.rules, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 3: Skills → .claude/skills/{id}/SKILL.md
	skillFiles, skillAssets := renderSkills(classified.skills, r.objects)
	emission.Files = append(emission.Files, skillFiles...)
	emission.Assets = append(emission.Assets, skillAssets...)

	// Layer 4: Agents → .claude/agents/{id}.md
	files = renderAgents(classified.agents, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 5: Hooks → .claude/settings.json
	files = renderHooks(classified.hooks)
	emission.Files = append(emission.Files, files...)

	// Layer 6: MCP config → .mcp.json
	files = renderMCP(classified.plugins, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 7: Plugins → .claude-plugin/
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

	// Collect directories to create.
	emission.Directories = collectDirectories(emission.Files)

	// Record provenance for reporting.
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

	// Sort object keys for deterministic processing.
	keys := make([]string, 0, len(objects))
	for k := range objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		obj := objects[key]

		// Skip objects that were skipped during lowering.
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
// The actual pipeline.EmissionPlan is returned from Execute.
type UnitEmission = pipeline.UnitEmission
