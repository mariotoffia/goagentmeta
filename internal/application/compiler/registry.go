// Package compiler provides the pipeline orchestrator and stage registry
// for the goagentmeta compiler. It loads, orders, and dispatches compiler
// plugins (stages) through the multi-phase pipeline.
package compiler

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// StageRegistry manages registered pipeline stages and hooks. It provides
// thread-safe registration and ordered retrieval of stages by phase.
//
// The registry resolves ordering constraints (Before/After) within each
// phase using topological sorting. If constraints form a cycle, the
// registry returns an error during resolution.
type StageRegistry struct {
	mu     sync.RWMutex
	stages []stage.Stage
	hooks  []stage.StageHookHandler
}

// NewStageRegistry creates an empty stage registry.
func NewStageRegistry() *StageRegistry {
	return &StageRegistry{}
}

// Register adds a stage to the registry. The stage's descriptor is
// validated before registration. Returns an error if the descriptor
// is invalid or a stage with the same name is already registered.
func (r *StageRegistry) Register(s stage.Stage) error {
	desc := s.Descriptor()
	if err := desc.Validate(); err != nil {
		return fmt.Errorf("register stage: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.stages {
		if existing.Descriptor().Name == desc.Name {
			return pipeline.NewCompilerError(
				pipeline.ErrPipeline,
				fmt.Sprintf("stage %q already registered", desc.Name),
				desc.Name,
			)
		}
	}

	r.stages = append(r.stages, s)
	return nil
}

// RegisterHook adds a stage hook to the registry.
func (r *StageRegistry) RegisterHook(h stage.StageHookHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, h)
}

// StagesForPhase returns registered stages for the given phase, ordered
// by priority and Before/After constraints. Returns an error if ordering
// constraints form a cycle.
func (r *StageRegistry) StagesForPhase(phase pipeline.Phase) ([]stage.Stage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var phaseStages []stage.Stage
	for _, s := range r.stages {
		if s.Descriptor().Phase == phase {
			phaseStages = append(phaseStages, s)
		}
	}

	return r.sortStages(phaseStages)
}

// HooksForPhase returns registered hooks for the given phase and hook point.
func (r *StageRegistry) HooksForPhase(phase pipeline.Phase, point pipeline.HookPoint) []stage.StageHookHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []stage.StageHookHandler
	for _, h := range r.hooks {
		hook := h.Hook()
		if hook.Phase == phase && hook.Point == point {
			result = append(result, h)
		}
	}
	return result
}

// AllStages returns all registered stages (unordered). Useful for validation.
func (r *StageRegistry) AllStages() []stage.Stage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]stage.Stage, len(r.stages))
	copy(result, r.stages)
	return result
}

// sortStages orders stages within a phase by priority and Before/After
// constraints using topological sorting.
func (r *StageRegistry) sortStages(stages []stage.Stage) ([]stage.Stage, error) {
	if len(stages) <= 1 {
		return stages, nil
	}

	// Sort by Order first, then by Name for determinism.
	sort.Slice(stages, func(i, j int) bool {
		di, dj := stages[i].Descriptor(), stages[j].Descriptor()
		if di.Order != dj.Order {
			return di.Order < dj.Order
		}
		return di.Name < dj.Name
	})

	// Build name-to-index map for constraint resolution.
	nameIdx := make(map[string]int, len(stages))
	for i, s := range stages {
		nameIdx[s.Descriptor().Name] = i
	}

	// Build adjacency for topological sort from Before/After constraints.
	n := len(stages)
	inDegree := make([]int, n)
	adj := make([][]int, n)
	for i := range adj {
		adj[i] = nil
	}

	for i, s := range stages {
		desc := s.Descriptor()
		// "Before: [X]" means this stage (i) must run before X.
		// So edge: i -> idx(X), meaning idx(X) depends on i.
		for _, beforeName := range desc.Before {
			if j, ok := nameIdx[beforeName]; ok {
				adj[i] = append(adj[i], j)
				inDegree[j]++
			}
		}
		// "After: [X]" means this stage (i) must run after X.
		// So edge: idx(X) -> i, meaning i depends on idx(X).
		for _, afterName := range desc.After {
			if j, ok := nameIdx[afterName]; ok {
				adj[j] = append(adj[j], i)
				inDegree[i]++
			}
		}
	}

	// Kahn's algorithm for topological sort.
	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	// Sort queue by original order for determinism among equal-degree nodes.
	sort.Ints(queue)

	result := make([]stage.Stage, 0, n)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		result = append(result, stages[curr])

		var nextBatch []int
		for _, neighbor := range adj[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				nextBatch = append(nextBatch, neighbor)
			}
		}
		sort.Ints(nextBatch)
		queue = append(queue, nextBatch...)
	}

	if len(result) != n {
		return nil, pipeline.NewCompilerError(
			pipeline.ErrPipeline,
			"cycle detected in stage ordering constraints",
			"",
		)
	}

	return result, nil
}
