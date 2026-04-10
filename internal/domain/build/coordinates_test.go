package build_test

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
)

func TestAllTargets(t *testing.T) {
	targets := build.AllTargets()
	if len(targets) != 4 {
		t.Fatalf("AllTargets() returned %d targets, want 4", len(targets))
	}

	expected := map[build.Target]bool{
		build.TargetClaude:  false,
		build.TargetCursor:  false,
		build.TargetCopilot: false,
		build.TargetCodex:   false,
	}

	for _, tgt := range targets {
		if _, ok := expected[tgt]; !ok {
			t.Errorf("unexpected target %q", tgt)
		}
		expected[tgt] = true
	}

	for tgt, seen := range expected {
		if !seen {
			t.Errorf("target %q not in AllTargets()", tgt)
		}
	}
}

func TestBuildUnit(t *testing.T) {
	unit := build.BuildUnit{
		Target:  build.TargetClaude,
		Profile: build.ProfileLocalDev,
		Scopes: []build.BuildScope{
			{Paths: []string{"services/**"}},
		},
	}

	if unit.Target != build.TargetClaude {
		t.Errorf("Target = %q, want %q", unit.Target, build.TargetClaude)
	}
	if unit.Profile != build.ProfileLocalDev {
		t.Errorf("Profile = %q, want %q", unit.Profile, build.ProfileLocalDev)
	}
	if len(unit.Scopes) != 1 {
		t.Fatalf("len(Scopes) = %d, want 1", len(unit.Scopes))
	}
	if unit.Scopes[0].Paths[0] != "services/**" {
		t.Errorf("Scopes[0].Paths[0] = %q, want %q", unit.Scopes[0].Paths[0], "services/**")
	}
}

func TestBuildCoordinate(t *testing.T) {
	coord := build.BuildCoordinate{
		Unit: build.BuildUnit{
			Target:  build.TargetCopilot,
			Profile: build.ProfileCI,
		},
		OutputDir: ".ai-build/copilot/ci/",
	}

	if coord.Unit.Target != build.TargetCopilot {
		t.Errorf("Unit.Target = %q, want %q", coord.Unit.Target, build.TargetCopilot)
	}
	if coord.OutputDir != ".ai-build/copilot/ci/" {
		t.Errorf("OutputDir = %q, want %q", coord.OutputDir, ".ai-build/copilot/ci/")
	}
}
