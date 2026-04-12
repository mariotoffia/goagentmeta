// Package cursor implements the Cursor target renderer. It transforms
// a LoweredGraph into an EmissionPlan containing Cursor-native files:
// AGENTS.md hierarchy, .cursor/rules/*.mdc, .cursor/mcp.json, and
// provenance.json.
//
// Skills are lowered into .cursor/rules/*.mdc files. Agents are lowered
// into rules. Commands are skipped entirely (no Cursor equivalent).
// Only three hook events are supported: beforeMCPExecution,
// afterMCPExecution, preShellCommand.
//
// The renderer is a specialized pipeline Stage registered for PhaseRender
// with TargetFilter: [cursor]. It consumes only lowered objects and never
// interprets raw source files directly.
package cursor

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

// Renderer implements the Cursor target renderer. It orchestrates
// layer generators to produce Cursor-native files from lowered IR.
type Renderer struct {
	objects map[string]pipeline.NormalizedObject
}

// New creates a new Cursor renderer.
func New(objects map[string]pipeline.NormalizedObject) *Renderer {
	return &Renderer{objects: objects}
}

// Target returns the target ecosystem this renderer handles.
func (r *Renderer) Target() build.Target {
	return build.TargetCursor
}

// SupportedCapabilities returns the capability registry for Cursor.
func (r *Renderer) SupportedCapabilities() capability.CapabilityRegistry {
	reg, err := capstage.LoadRegistry("cursor")
	if err != nil {
		return capability.CapabilityRegistry{
			Target:   "cursor",
			Surfaces: make(map[string]capability.SupportLevel),
		}
	}
	return *reg
}

// Descriptor returns the stage metadata for pipeline registration.
func (r *Renderer) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:         "cursor-renderer",
		Phase:        pipeline.PhaseRender,
		Order:        10,
		TargetFilter: []build.Target{build.TargetCursor},
	}
}

// Execute transforms a LoweredGraph into an EmissionPlan with Cursor-native files.
func (r *Renderer) Execute(ctx context.Context, input any) (any, error) {
	graph, ok := input.(pipeline.LoweredGraph)
	if !ok {
		graphPtr, ok := input.(*pipeline.LoweredGraph)
		if !ok || graphPtr == nil {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrRendering,
				fmt.Sprintf("expected pipeline.LoweredGraph or *pipeline.LoweredGraph, got %T", input),
				"cursor-renderer",
			)
		}
		graph = *graphPtr
	}

	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "RENDER_START",
		Message:  fmt.Sprintf("rendering Cursor output for %d unit(s)", len(graph.Units)),
		Phase:    pipeline.PhaseRender,
	})

	plan := pipeline.EmissionPlan{
		Units: make(map[string]UnitEmission, len(graph.Units)),
	}

	unitKeys := sortedKeys(graph.Units)

	for _, unitKey := range unitKeys {
		unit := graph.Units[unitKey]
		if string(unit.Coordinate.Unit.Target) != "cursor" {
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
		Message:  fmt.Sprintf("rendered %d Cursor unit(s)", len(plan.Units)),
		Phase:    pipeline.PhaseRender,
	})

	return plan, nil
}

// Factory returns a StageFactory for the Cursor renderer.
func Factory(objects map[string]pipeline.NormalizedObject) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(objects), nil
	}
}

// renderUnit renders all Cursor-native files for a single build unit.
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

	// Layer 2: Rules → .cursor/rules/{id}.mdc
	files = renderRules(classified.rules, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 3: Skills → lowered into .cursor/rules/{id}.mdc or skipped
	files = renderSkills(ctx, classified.skills, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 4: Agents → lowered into .cursor/rules/{id}.mdc or skipped
	files = renderAgents(ctx, classified.agents, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 5: Hooks → filter to 3 supported events
	files = renderHooks(ctx, classified.hooks)
	emission.Files = append(emission.Files, files...)

	// Layer 6: MCP config → .cursor/mcp.json
	files = renderMCP(classified.plugins, r.objects)
	emission.Files = append(emission.Files, files...)

	// Layer 7: Plugins → Cursor Marketplace references
	pluginFiles, installEntries := renderPlugins(classified.plugins, r.objects)
	emission.Files = append(emission.Files, pluginFiles...)
	emission.InstallMetadata = append(emission.InstallMetadata, installEntries...)

	// Commands are skipped entirely — no native Cursor equivalent.
	if len(classified.commands) > 0 {
		ids := make([]string, len(classified.commands))
		for i, cmd := range classified.commands {
			ids[i] = cmd.OriginalID
		}
		emitDiagnostic(ctx, pipeline.Diagnostic{
			Severity: "info",
			Code:     "RENDER_COMMANDS_SKIPPED",
			Message:  fmt.Sprintf("%d command(s) skipped — no native Cursor equivalent: %v", len(ids), ids),
			Phase:    pipeline.PhaseRender,
		})
	}

	// Layer 8: Provenance headers + provenance.json
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

// sanitizeID converts an object ID to a safe filename component.
func sanitizeID(id string) string {
	id = strings.ReplaceAll(id, "/", "-")
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, ".", "-")
	return id
}

// getString safely extracts a string from a map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getStringSlice safely extracts a string slice from a map.
func getStringSlice(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// getFieldString extracts a string field from Fields.
func getFieldString(obj pipeline.LoweredObject, key string) string {
	if obj.Fields != nil {
		if v, ok := obj.Fields[key].(string); ok {
			return v
		}
	}
	return ""
}

// getFieldStringSlice extracts a string slice field from Fields.
func getFieldStringSlice(obj pipeline.LoweredObject, key string) []string {
	if obj.Fields == nil {
		return nil
	}

	switch v := obj.Fields[key].(type) {
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return nil
}

// getOrDefault returns the value if non-empty, otherwise the default.
func getOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// sortedMapKeys returns sorted keys of a map[string]any.
func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dedup removes duplicate strings while preserving order.
func dedup(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// cleanPath normalizes a path by removing leading/trailing slashes.
func cleanPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return p
}

// UnitEmission is a type alias used only in the renderer's return.
type UnitEmission = pipeline.UnitEmission
