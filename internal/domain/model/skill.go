package model

// Skill is a reusable, model-facing workflow bundle. Skills describe how work
// should be done (build a Lambda, review IAM, scaffold a bounded context) and
// are portable across targets. Skills follow the AgentSkills.io open standard.
//
// Skills may contain markdown guidance, examples, templates, prompt fragments,
// assets, references, and capability requirements. They are distinct from
// plugins: a skill is model-facing content, while a plugin is a runtime-facing
// integration package.
type Skill struct {
	ObjectMeta

	// Content is the primary markdown skill content (the SKILL.md body).
	Content string

	// Requires lists capability IDs that this skill needs to function.
	Requires []string

	// Resources holds references to supporting files consumed by this skill.
	Resources SkillResources

	// ActivationHints are keywords or phrases that help the AI decide when
	// to load this skill. They correspond to the AgentSkills.io activation model.
	ActivationHints []string

	// UserInvocable controls whether this skill appears as a slash command.
	UserInvocable bool

	// DisableModelInvocation prevents the AI from auto-loading this skill.
	DisableModelInvocation bool

	// AllowedTools lists tool permission expressions that this skill is
	// allowed to use. Entries may be exact tool names (e.g., "Read", "Write")
	// or glob/prefix patterns (e.g., "Bash(go:*)"). This is a flat allowlist
	// matching the AgentSkills.io format — contrast with Agent.ToolPolicy
	// which is a richer allow/deny/ask map.
	AllowedTools []string

	// Compatibility is a free-text statement describing which platforms,
	// runtimes, or AI coding agents this skill is designed for.
	Compatibility string

	// BinaryDeps lists external binary tools that must be present on PATH
	// for this skill to function (e.g., "go", "benchstat", "golangci-lint").
	// These are runtime prerequisites, distinct from Requires (capability IDs).
	BinaryDeps []string

	// InstallSteps describes how to install the skill's binary dependencies.
	// Each step specifies a package manager, package reference, and the
	// binaries it provides.
	InstallSteps []InstallStep

	// Publishing holds optional metadata for published/distributed skills,
	// such as author identity, homepage URL, and display emoji.
	Publishing SkillPublishing
}

// SkillResources groups the supporting files associated with a skill.
type SkillResources struct {
	// References are supplemental knowledge documents loaded on demand.
	References []string
	// Assets are static files (templates, examples, diagrams).
	Assets []string
	// Scripts are executable artifacts used by the skill.
	Scripts []string
}

// InstallStep describes a single installation directive for a skill's
// binary dependencies. It maps to the AgentSkills.io metadata.openclaw.install
// format.
type InstallStep struct {
	// Kind is the package manager or installation method (e.g., "go", "npm",
	// "pip", "brew").
	Kind string
	// Package is the package reference to install (e.g.,
	// "golang.org/x/perf/cmd/benchstat@latest").
	Package string
	// Bins lists the binary names provided by this installation step.
	Bins []string
}

// SkillPublishing holds optional metadata for published or distributed skills.
// These fields carry registry and marketplace presentation data that is not
// required for compilation but is preserved through the pipeline.
type SkillPublishing struct {
	// Author is the original creator or publisher of the skill package.
	// Distinct from ObjectMeta.Owner which is the team/individual responsible
	// for the object in this repository.
	Author string
	// Homepage is the URL of the skill's project page, repository, or
	// documentation site.
	Homepage string
	// Emoji is a display emoji used in marketplace and registry UIs.
	Emoji string
}
