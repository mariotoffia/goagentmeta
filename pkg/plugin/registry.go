package plugin

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/port/renderer"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Registry is the central registration point for statically compiled plugins.
// Plugin packages call Register methods during application bootstrap to
// contribute stages, renderers, or other extensions to the pipeline.
type Registry interface {
	// RegisterStage registers a pipeline stage. Returns error if a stage with
	// the same name is already registered.
	RegisterStage(s stage.Stage) error

	// MustRegisterStage registers a pipeline stage, panicking on error.
	// Suitable for init-time registration where failure is a programming bug.
	MustRegisterStage(s stage.Stage)

	// RegisterRenderer registers a target renderer. Returns error if a renderer
	// for the same target is already registered.
	RegisterRenderer(r renderer.Renderer) error

	// MustRegisterRenderer registers a target renderer, panicking on error.
	MustRegisterRenderer(r renderer.Renderer)

	// Stages returns all registered stages, ordered by phase (ascending) then
	// by order (ascending).
	Stages() []stage.Stage

	// Renderers returns all registered renderers.
	Renderers() []renderer.Renderer

	// StageByName returns a stage by its descriptor name, or nil if not found.
	StageByName(name string) stage.Stage

	// RendererByTarget returns a renderer for the given target, or nil if not
	// found.
	RendererByTarget(target build.Target) renderer.Renderer
}

// DefaultRegistry is the standard in-memory implementation of Registry.
// It is safe for concurrent use during registration but is typically
// populated once at startup before the pipeline runs.
type DefaultRegistry struct {
	mu              sync.RWMutex
	stages          []stage.Stage
	renderers       []renderer.Renderer
	stageIdx        map[string]stage.Stage
	rendIdx         map[build.Target]renderer.Renderer
	stageValidators []func(stage.Stage) error
}

// NewRegistry creates a new DefaultRegistry with the given options.
func NewRegistry(opts ...Option) *DefaultRegistry {
	r := &DefaultRegistry{
		stageIdx: make(map[string]stage.Stage),
		rendIdx:  make(map[build.Target]renderer.Renderer),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RegisterStage registers a pipeline stage. Returns error if a stage with the
// same descriptor name is already registered or if a custom validator rejects
// it.
func (r *DefaultRegistry) RegisterStage(s stage.Stage) error {
	name := s.Descriptor().Name

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.stageIdx[name]; exists {
		return fmt.Errorf("plugin: stage %q already registered", name)
	}

	for _, v := range r.stageValidators {
		if err := v(s); err != nil {
			return fmt.Errorf("plugin: stage %q rejected by validator: %w", name, err)
		}
	}

	r.stages = append(r.stages, s)
	r.stageIdx[name] = s
	return nil
}

// MustRegisterStage registers a pipeline stage, panicking on error.
func (r *DefaultRegistry) MustRegisterStage(s stage.Stage) {
	if err := r.RegisterStage(s); err != nil {
		panic(err)
	}
}

// RegisterRenderer registers a target renderer. Returns error if a renderer
// for the same target is already registered.
func (r *DefaultRegistry) RegisterRenderer(rend renderer.Renderer) error {
	target := rend.Target()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rendIdx[target]; exists {
		return fmt.Errorf("plugin: renderer for target %q already registered", target)
	}

	r.renderers = append(r.renderers, rend)
	r.rendIdx[target] = rend
	return nil
}

// MustRegisterRenderer registers a target renderer, panicking on error.
func (r *DefaultRegistry) MustRegisterRenderer(rend renderer.Renderer) {
	if err := r.RegisterRenderer(rend); err != nil {
		panic(err)
	}
}

// Stages returns all registered stages sorted by phase (ascending) then by
// order (ascending). The returned slice is a copy.
func (r *DefaultRegistry) Stages() []stage.Stage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]stage.Stage, len(r.stages))
	copy(out, r.stages)

	sort.SliceStable(out, func(i, j int) bool {
		di, dj := out[i].Descriptor(), out[j].Descriptor()
		if di.Phase != dj.Phase {
			return di.Phase < dj.Phase
		}
		return di.Order < dj.Order
	})

	return out
}

// Renderers returns all registered renderers. The returned slice is a copy.
func (r *DefaultRegistry) Renderers() []renderer.Renderer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]renderer.Renderer, len(r.renderers))
	copy(out, r.renderers)
	return out
}

// StageByName returns a stage by its descriptor name, or nil if not found.
func (r *DefaultRegistry) StageByName(name string) stage.Stage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.stageIdx[name]
}

// RendererByTarget returns a renderer for the given target, or nil if not
// found.
func (r *DefaultRegistry) RendererByTarget(target build.Target) renderer.Renderer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.rendIdx[target]
}
