// Package resolver implements the PhaseResolve pipeline stage. It wraps
// the DependencyResolver use case as a stage.Stage, enforces profile trust
// policies, and reads dependency declarations from the manifest.
package resolver

import (
	"context"
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion: *Stage satisfies the stage.Stage port interface.
var _ stage.Stage = (*Stage)(nil)

// Stage implements the PhaseResolve pipeline stage. It resolves external
// dependencies from registries and merges them into the SourceTree.
type Stage struct {
	resolver *dependency.DependencyResolver
}

// New creates a new resolver Stage with the given DependencyResolver.
func New(resolver *dependency.DependencyResolver) *Stage {
	return &Stage{resolver: resolver}
}

// Descriptor returns the stage metadata for pipeline registration.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "dependency-resolver",
		Phase: pipeline.PhaseResolve,
		Order: 10,
	}
}

// Execute resolves external dependencies and merges them into the SourceTree.
// It enforces profile trust policies before performing any resolution.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	tree, ok := input.(pipeline.SourceTree)
	if !ok {
		treePtr, ok := input.(*pipeline.SourceTree)
		if !ok {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrResolution,
				fmt.Sprintf("expected pipeline.SourceTree or *pipeline.SourceTree, got %T", input),
				"dependency-resolver",
			)
		}
		tree = *treePtr
	}

	// Read dependencies from manifest.
	deps, err := dependency.ParseManifestDependencies(tree.ManifestPath)
	if err != nil {
		return tree, pipeline.Wrap(
			pipeline.ErrResolution,
			fmt.Sprintf("failed to parse manifest dependencies: %v", err),
			"dependency-resolver",
			err,
		)
	}

	// No dependencies declared — pass through unchanged.
	if len(deps) == 0 {
		return tree, nil
	}

	// Read registry configs to check for external registries.
	registries, err := dependency.ParseManifestRegistries(tree.ManifestPath)
	if err != nil {
		return tree, pipeline.Wrap(
			pipeline.ErrResolution,
			fmt.Sprintf("failed to parse manifest registries: %v", err),
			"dependency-resolver",
			err,
		)
	}

	// Enforce profile trust policy.
	profile := profileFromContext(ctx)
	if profile == build.ProfileEnterpriseLocked {
		if len(registries) == 0 {
			// No registries declared — the resolver would use its default
			// registry configuration which may include external sources.
			return tree, pipeline.NewCompilerError(
				pipeline.ErrResolution,
				"profile enterprise-locked requires explicit registry declarations when dependencies are present",
				"dependency-resolver",
			)
		}
		if hasExternalRegistry(registries) {
			return tree, pipeline.NewCompilerError(
				pipeline.ErrResolution,
				"profile enterprise-locked blocks external registry access",
				"dependency-resolver",
			)
		}
	}

	// Emit diagnostic for resolution start.
	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "RESOLVE_START",
		Message:  fmt.Sprintf("resolving %d dependencies", len(deps)),
		Phase:    pipeline.PhaseResolve,
	})

	// Resolve dependencies.
	result, err := s.resolver.Resolve(ctx, tree, deps)
	if err != nil {
		return tree, err
	}

	// Emit diagnostic for resolution complete.
	externalCount := countExternalObjects(result)
	emitDiagnostic(ctx, pipeline.Diagnostic{
		Severity: "info",
		Code:     "RESOLVE_COMPLETE",
		Message:  fmt.Sprintf("resolved %d dependencies, merged %d external objects", len(deps), externalCount),
		Phase:    pipeline.PhaseResolve,
	})

	return result, nil
}

// Factory returns a StageFactory function for use with pipeline registration.
func Factory(resolver *dependency.DependencyResolver) stage.StageFactory {
	return func() (stage.Stage, error) {
		return New(resolver), nil
	}
}

// hasExternalRegistry returns true if any configured registry uses a non-local
// type (http, git).
func hasExternalRegistry(registries []dependency.RegistryConfig) bool {
	for _, r := range registries {
		if r.Type != "local" {
			return true
		}
	}
	return false
}

// profileFromContext extracts the build profile from the CompilerContext.
// Returns empty string if no profile is configured.
func profileFromContext(ctx context.Context) build.Profile {
	cc := compiler.CompilerFromContext(ctx)
	if cc == nil || cc.Config == nil {
		return ""
	}
	return cc.Config.Profile
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

// countExternalObjects counts objects marked as external in the tree.
func countExternalObjects(tree pipeline.SourceTree) int {
	count := 0
	for _, obj := range tree.Objects {
		if obj.RawFields != nil {
			if ext, ok := obj.RawFields["_external"].(bool); ok && ext {
				count++
			}
		}
	}
	return count
}
