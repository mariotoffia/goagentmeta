package validator

import (
	"context"
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time assertion: *Stage satisfies the stage.Stage port interface.
var _ stage.Stage = (*Stage)(nil)

// ValidationError extends CompilerError with the individual diagnostics that
// caused the validation failure. Callers (including the pipeline) can type-
// assert to access the full diagnostic details.
type ValidationError struct {
	*pipeline.CompilerError
	// Diagnostics holds every individual validation diagnostic.
	Diagnostics []pipeline.Diagnostic
}

// Stage implements the PhaseValidate pipeline stage. It combines structural
// and semantic validation, supports FailFast mode, and is non-mutating.
type Stage struct {
	structural *StructuralValidator
	semantic   *SemanticValidator
}

// New creates a new validator Stage with loaded schemas.
func New() (*Stage, error) {
	sv, err := NewStructuralValidator()
	if err != nil {
		return nil, fmt.Errorf("create structural validator: %w", err)
	}
	return &Stage{
		structural: sv,
		semantic:   NewSemanticValidator(),
	}, nil
}

// Descriptor returns the stage metadata for pipeline registration.
func (s *Stage) Descriptor() pipeline.StageDescriptor {
	return pipeline.StageDescriptor{
		Name:  "schema-validator",
		Phase: pipeline.PhaseValidate,
		Order: 10,
	}
}

// Execute validates the SourceTree and returns it unchanged. Diagnostics
// are emitted through the DiagnosticSink (if available via CompilerContext)
// and, if any errors are found, a ValidationError is returned carrying the
// full list. The SourceTree is never mutated.
func (s *Stage) Execute(ctx context.Context, input any) (any, error) {
	tree, ok := input.(pipeline.SourceTree)
	if !ok {
		treePtr, ok := input.(*pipeline.SourceTree)
		if !ok {
			return nil, pipeline.NewCompilerError(
				pipeline.ErrValidation,
				fmt.Sprintf("expected pipeline.SourceTree or *pipeline.SourceTree, got %T", input),
				"schema-validator",
			)
		}
		tree = *treePtr
	}

	cc := compiler.CompilerFromContext(ctx)
	failFast := isFailFast(cc)
	var allDiags []pipeline.Diagnostic

	// Phase 1: Structural validation (per-object).
	for _, obj := range tree.Objects {
		diags := s.structural.Validate(obj)
		s.emitDiagnostics(ctx, cc, diags)
		allDiags = append(allDiags, diags...)

		if failFast && hasErrors(diags) {
			return tree, s.buildError(allDiags)
		}
	}

	// Phase 2: Semantic validation (cross-object).
	semanticDiags := s.semantic.Validate(tree)
	s.emitDiagnostics(ctx, cc, semanticDiags)
	allDiags = append(allDiags, semanticDiags...)

	if failFast && hasErrors(semanticDiags) {
		return tree, s.buildError(allDiags)
	}

	if hasErrors(allDiags) {
		return tree, s.buildError(allDiags)
	}

	return tree, nil
}

// emitDiagnostics sends diagnostics to the DiagnosticSink and BuildReport
// if a CompilerContext is available.
func (s *Stage) emitDiagnostics(ctx context.Context, cc *compiler.CompilerContext, diags []pipeline.Diagnostic) {
	if cc == nil {
		return
	}
	for _, d := range diags {
		if cc.Report != nil {
			cc.Report.Diagnostics = append(cc.Report.Diagnostics, d)
		}
		if cc.Config != nil && cc.Config.DiagnosticSink != nil {
			cc.Config.DiagnosticSink.Emit(ctx, d)
		}
	}
}

// isFailFast reads the FailFast setting from the CompilerContext if available.
func isFailFast(cc *compiler.CompilerContext) bool {
	if cc != nil && cc.Config != nil {
		return cc.Config.FailFast
	}
	// Default to true if context is not available.
	return true
}

// buildError creates a ValidationError carrying all individual diagnostics.
func (s *Stage) buildError(diags []pipeline.Diagnostic) *ValidationError {
	errorCount := 0
	for _, d := range diags {
		if d.Severity == "error" {
			errorCount++
		}
	}

	msg := fmt.Sprintf("validation failed with %d error(s)", errorCount)
	return &ValidationError{
		CompilerError: pipeline.NewCompilerError(pipeline.ErrValidation, msg, "schema-validator"),
		Diagnostics:   diags,
	}
}

// hasErrors returns true if any diagnostic has error severity.
func hasErrors(diags []pipeline.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// Factory returns a StageFactory function for use with pipeline registration.
func Factory() stage.StageFactory {
	return func() (stage.Stage, error) {
		return New()
	}
}

// ValidateTree returns all validation diagnostics for a SourceTree.
// This is a convenience for direct usage outside the pipeline.
func (s *Stage) ValidateTree(tree pipeline.SourceTree) []pipeline.Diagnostic {
	var allDiags []pipeline.Diagnostic

	for _, obj := range tree.Objects {
		allDiags = append(allDiags, s.structural.Validate(obj)...)
	}
	allDiags = append(allDiags, s.semantic.Validate(tree)...)

	return allDiags
}
