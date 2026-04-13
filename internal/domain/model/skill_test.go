package model_test

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

func TestSkillTools(t *testing.T) {
	skill := model.Skill{
		Tools: []string{
			"Read", "Edit", "Write", "Glob", "Grep",
			"Bash(go:*)", "Bash(golangci-lint:*)", "Bash(git:*)",
			"Agent", "WebFetch",
		},
	}

	if len(skill.Tools) != 10 {
		t.Fatalf("len(Tools) = %d, want 10", len(skill.Tools))
	}
	if skill.Tools[5] != "Bash(go:*)" {
		t.Errorf("Tools[5] = %q, want %q", skill.Tools[5], "Bash(go:*)")
	}
}

func TestSkillCompatibility(t *testing.T) {
	skill := model.Skill{
		Compatibility: "Designed for Claude Code or similar AI coding agents, and for projects using Golang.",
	}

	if skill.Compatibility == "" {
		t.Error("Compatibility should not be empty")
	}
}

func TestSkillBinaryDeps(t *testing.T) {
	skill := model.Skill{
		BinaryDeps: []string{"go", "benchstat"},
	}

	if len(skill.BinaryDeps) != 2 {
		t.Fatalf("len(BinaryDeps) = %d, want 2", len(skill.BinaryDeps))
	}
	if skill.BinaryDeps[0] != "go" {
		t.Errorf("BinaryDeps[0] = %q, want %q", skill.BinaryDeps[0], "go")
	}
	if skill.BinaryDeps[1] != "benchstat" {
		t.Errorf("BinaryDeps[1] = %q, want %q", skill.BinaryDeps[1], "benchstat")
	}
}

func TestSkillInstallSteps(t *testing.T) {
	skill := model.Skill{
		InstallSteps: []model.InstallStep{
			{
				Kind:    "go",
				Package: "golang.org/x/perf/cmd/benchstat@latest",
				Bins:    []string{"benchstat"},
			},
		},
	}

	if len(skill.InstallSteps) != 1 {
		t.Fatalf("len(InstallSteps) = %d, want 1", len(skill.InstallSteps))
	}

	step := skill.InstallSteps[0]
	if step.Kind != "go" {
		t.Errorf("Kind = %q, want %q", step.Kind, "go")
	}
	if step.Package != "golang.org/x/perf/cmd/benchstat@latest" {
		t.Errorf("Package = %q, want %q", step.Package, "golang.org/x/perf/cmd/benchstat@latest")
	}
	if len(step.Bins) != 1 || step.Bins[0] != "benchstat" {
		t.Errorf("Bins = %v, want [benchstat]", step.Bins)
	}
}

func TestSkillPublishing(t *testing.T) {
	skill := model.Skill{
		Publishing: model.SkillPublishing{
			Author:   "samber",
			Homepage: "https://github.com/samber/cc-skills-golang",
			Emoji:    "📊",
		},
	}

	if skill.Publishing.Author != "samber" {
		t.Errorf("Author = %q, want %q", skill.Publishing.Author, "samber")
	}
	if skill.Publishing.Homepage != "https://github.com/samber/cc-skills-golang" {
		t.Errorf("Homepage = %q, want %q",
			skill.Publishing.Homepage, "https://github.com/samber/cc-skills-golang")
	}
	if skill.Publishing.Emoji != "📊" {
		t.Errorf("Emoji = %q, want %q", skill.Publishing.Emoji, "📊")
	}
}

// TestSkillFullAgentSkillsIoMapping validates that all fields from the
// AgentSkills.io SKILL.md frontmatter have a corresponding home in the
// canonical Skill model.
func TestSkillFullAgentSkillsIoMapping(t *testing.T) {
	skill := model.Skill{
		ObjectMeta: model.ObjectMeta{
			ID:             "golang-benchmark",
			Kind:           model.KindSkill,
			Description:    "Golang benchmarking, profiling, and performance measurement.",
			PackageVersion: "1.1.3",
			License:        "MIT",
		},
		UserInvocable: true,
		Compatibility: "Designed for Claude Code or similar AI coding agents, and for projects using Golang.",
		Tools: []string{
			"Read", "Edit", "Write", "Glob", "Grep",
			"Bash(go:*)", "Bash(golangci-lint:*)", "Bash(git:*)",
			"Agent", "WebFetch",
			"Bash(benchstat:*)", "Bash(benchdiff:*)", "Bash(cob:*)",
			"Bash(gobenchdata:*)", "Bash(curl:*)",
			"mcp__context7__resolve-library-id",
			"mcp__context7__query-docs",
			"WebSearch", "AskUserQuestion",
		},
		BinaryDeps: []string{"go", "benchstat"},
		InstallSteps: []model.InstallStep{
			{
				Kind:    "go",
				Package: "golang.org/x/perf/cmd/benchstat@latest",
				Bins:    []string{"benchstat"},
			},
		},
		Publishing: model.SkillPublishing{
			Author:   "samber",
			Homepage: "https://github.com/samber/cc-skills-golang",
			Emoji:    "📊",
		},
	}

	// Verify all frontmatter fields have non-zero values.
	checks := []struct {
		name string
		ok   bool
	}{
		{"ID", skill.ID != ""},
		{"Kind", skill.Kind == model.KindSkill},
		{"Description", skill.Description != ""},
		{"PackageVersion", skill.PackageVersion != ""},
		{"License", skill.License != ""},
		{"UserInvocable", skill.UserInvocable},
		{"Compatibility", skill.Compatibility != ""},
		{"Tools", len(skill.Tools) > 0},
		{"BinaryDeps", len(skill.BinaryDeps) > 0},
		{"InstallSteps", len(skill.InstallSteps) > 0},
		{"Publishing.Author", skill.Publishing.Author != ""},
		{"Publishing.Homepage", skill.Publishing.Homepage != ""},
		{"Publishing.Emoji", skill.Publishing.Emoji != ""},
	}

	for _, c := range checks {
		if !c.ok {
			t.Errorf("field %s is zero-valued", c.name)
		}
	}
}

// TestSkillRequiresVsBinaryDeps verifies that Requires (capability IDs) and
// BinaryDeps (external binaries on PATH) are orthogonal concepts.
func TestSkillRequiresVsBinaryDeps(t *testing.T) {
	skill := model.Skill{
		Requires:   []string{"terminal.exec", "repo.search"},
		BinaryDeps: []string{"go", "benchstat"},
	}

	if len(skill.Requires) != 2 {
		t.Fatalf("len(Requires) = %d, want 2", len(skill.Requires))
	}
	if len(skill.BinaryDeps) != 2 {
		t.Fatalf("len(BinaryDeps) = %d, want 2", len(skill.BinaryDeps))
	}

	// They must not overlap — capability IDs use dotted names, binaries are
	// bare command names.
	for _, cap := range skill.Requires {
		for _, bin := range skill.BinaryDeps {
			if cap == bin {
				t.Errorf("Requires %q should not equal BinaryDeps %q", cap, bin)
			}
		}
	}
}
