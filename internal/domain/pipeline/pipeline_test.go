package pipeline_test

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase pipeline.Phase
		want  string
	}{
		{pipeline.PhaseParse, "parse"},
		{pipeline.PhaseValidate, "validate"},
		{pipeline.PhaseResolve, "resolve"},
		{pipeline.PhaseNormalize, "normalize"},
		{pipeline.PhasePlan, "plan"},
		{pipeline.PhaseCapability, "capability"},
		{pipeline.PhaseLower, "lower"},
		{pipeline.PhaseRender, "render"},
		{pipeline.PhaseMaterialize, "materialize"},
		{pipeline.PhaseReport, "report"},
		{pipeline.Phase(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.phase.String(); got != tt.want {
				t.Errorf("Phase(%d).String() = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestAllPhasesOrder(t *testing.T) {
	phases := pipeline.AllPhases()
	if len(phases) != pipeline.PhaseCount {
		t.Fatalf("AllPhases() returned %d phases, want %d", len(phases), pipeline.PhaseCount)
	}

	for i := 1; i < len(phases); i++ {
		if phases[i] <= phases[i-1] {
			t.Errorf("AllPhases()[%d] (%s) <= AllPhases()[%d] (%s): not strictly ordered",
				i, phases[i], i-1, phases[i-1])
		}
	}
}

func TestStageDescriptorValidate(t *testing.T) {
	tests := []struct {
		name    string
		desc    pipeline.StageDescriptor
		wantErr bool
	}{
		{
			name:    "valid descriptor",
			desc:    pipeline.StageDescriptor{Name: "parser", Phase: pipeline.PhaseParse},
			wantErr: false,
		},
		{
			name:    "empty name",
			desc:    pipeline.StageDescriptor{Name: "", Phase: pipeline.PhaseParse},
			wantErr: true,
		},
		{
			name:    "invalid phase",
			desc:    pipeline.StageDescriptor{Name: "bad", Phase: pipeline.Phase(99)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.desc.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompilerError(t *testing.T) {
	t.Run("with context", func(t *testing.T) {
		err := pipeline.NewCompilerError(pipeline.ErrParse, "bad syntax", "file.yaml")
		want := "[PARSE] file.yaml: bad syntax"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("without context", func(t *testing.T) {
		err := pipeline.NewCompilerError(pipeline.ErrValidation, "missing field", "")
		want := "[VALIDATION] missing field"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("wrap unwrap", func(t *testing.T) {
		inner := pipeline.NewCompilerError(pipeline.ErrParse, "inner", "")
		outer := pipeline.Wrap(pipeline.ErrPipeline, "outer", "stage", inner)
		if outer.Unwrap() != inner {
			t.Error("Unwrap() did not return the wrapped error")
		}
	})
}
