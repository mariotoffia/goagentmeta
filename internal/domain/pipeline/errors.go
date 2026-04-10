package pipeline

import "fmt"

// ErrorCode classifies compiler errors by pipeline phase or category.
type ErrorCode string

const (
	// ErrParse indicates a parse-phase failure.
	ErrParse ErrorCode = "PARSE"
	// ErrValidation indicates a schema or semantic validation failure.
	ErrValidation ErrorCode = "VALIDATION"
	// ErrResolution indicates a dependency resolution failure.
	ErrResolution ErrorCode = "RESOLUTION"
	// ErrNormalization indicates a normalization failure.
	ErrNormalization ErrorCode = "NORMALIZATION"
	// ErrPlanning indicates a build planning failure.
	ErrPlanning ErrorCode = "PLANNING"
	// ErrCapability indicates a capability resolution failure.
	ErrCapability ErrorCode = "CAPABILITY"
	// ErrLowering indicates a lowering failure.
	ErrLowering ErrorCode = "LOWERING"
	// ErrRendering indicates a rendering failure.
	ErrRendering ErrorCode = "RENDERING"
	// ErrMaterialization indicates a materialization (file writing) failure.
	ErrMaterialization ErrorCode = "MATERIALIZATION"
	// ErrReporting indicates a report generation failure.
	ErrReporting ErrorCode = "REPORTING"
	// ErrPipeline indicates a pipeline orchestration failure.
	ErrPipeline ErrorCode = "PIPELINE"
)

// CompilerError is a structured error for compiler operations.
// It carries an error code, a message, and an optional context identifier
// (e.g., stage name, object ID, or file path).
type CompilerError struct {
	// Code classifies the error by pipeline phase or category.
	Code ErrorCode
	// Message is the human-readable error description.
	Message string
	// Context is an optional identifier for the error source
	// (stage name, object ID, file path).
	Context string
	// Wrapped is an optional underlying error.
	Wrapped error
}

// NewCompilerError creates a new CompilerError.
func NewCompilerError(code ErrorCode, message string, context string) *CompilerError {
	return &CompilerError{
		Code:    code,
		Message: message,
		Context: context,
	}
}

// Wrap creates a new CompilerError wrapping an underlying error.
func Wrap(code ErrorCode, message string, context string, err error) *CompilerError {
	return &CompilerError{
		Code:    code,
		Message: message,
		Context: context,
		Wrapped: err,
	}
}

// Error implements the error interface.
func (e *CompilerError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Context, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *CompilerError) Unwrap() error {
	return e.Wrapped
}
