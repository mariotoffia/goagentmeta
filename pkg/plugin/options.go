// Package plugin provides the public plugin registry for statically compiled
// pipeline extensions. Plugin authors import this package to register custom
// stages and renderers during application bootstrap.
//
// Typical usage:
//
//	reg := plugin.NewRegistry()
//	reg.MustRegisterStage(&myStage{})
//	reg.MustRegisterRenderer(&myRenderer{})
//
// The registry is safe for concurrent registration but is typically populated
// once at startup before the pipeline runs.
package plugin

import (
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Option configures a DefaultRegistry during construction.
type Option func(*DefaultRegistry)

// WithStageValidator adds custom validation logic that is invoked for every
// stage at registration time. If the validator returns a non-nil error the
// registration is rejected.
func WithStageValidator(fn func(stage.Stage) error) Option {
	return func(r *DefaultRegistry) {
		r.stageValidators = append(r.stageValidators, fn)
	}
}
