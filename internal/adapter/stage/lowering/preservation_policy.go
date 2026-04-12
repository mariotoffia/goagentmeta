package lowering

import (
	"fmt"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// PreservationResult describes the outcome of a preservation policy check.
type PreservationResult struct {
	// Allowed is true if the lowering may proceed.
	Allowed bool
	// Severity is "error", "warning", or "info" depending on the preservation level.
	Severity string
	// Message describes the outcome.
	Message string
}

// CheckPreservation evaluates whether a lowering decision is permissible given
// the object's preservation level. Unsafe lowerings with required preservation
// produce errors; preferred produces warnings; optional always allows.
func CheckPreservation(decision pipeline.LoweringDecision, preservation model.Preservation) PreservationResult {
	if preservation == "" {
		preservation = model.PreservationPreferred
	}

	// Safe lowerings are always permitted regardless of preservation level.
	if decision.Safe {
		return PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message:  fmt.Sprintf("safe lowering (%s): %s", decision.Action, decision.Reason),
		}
	}

	// Unsafe lowering paths.
	switch preservation {
	case model.PreservationRequired:
		return PreservationResult{
			Allowed:  false,
			Severity: "error",
			Message: fmt.Sprintf(
				"unsafe lowering blocked: %s (%s) with preservation=required",
				decision.Action, decision.Reason,
			),
		}
	case model.PreservationPreferred:
		return PreservationResult{
			Allowed:  false,
			Severity: "warning",
			Message: fmt.Sprintf(
				"unsafe lowering skipped: %s (%s) with preservation=preferred",
				decision.Action, decision.Reason,
			),
		}
	case model.PreservationOptional:
		return PreservationResult{
			Allowed:  true,
			Severity: "info",
			Message: fmt.Sprintf(
				"unsafe lowering permitted: %s (%s) with preservation=optional",
				decision.Action, decision.Reason,
			),
		}
	default:
		// Unknown preservation level — treat as preferred (warn + skip).
		return PreservationResult{
			Allowed:  false,
			Severity: "warning",
			Message: fmt.Sprintf(
				"unsafe lowering skipped: unknown preservation %q", preservation,
			),
		}
	}
}
