// Package build defines build coordinates that the compiler uses to determine
// what to compile and for which environments. A build unit is the fundamental
// compilation target: (target, profile, selected scopes).
package build

// Target identifies the vendor or ecosystem being emitted for.
type Target string

const (
	// TargetClaude is the Claude Code target.
	TargetClaude Target = "claude"
	// TargetCursor is the Cursor target.
	TargetCursor Target = "cursor"
	// TargetCopilot is the GitHub Copilot target.
	TargetCopilot Target = "copilot"
	// TargetCodex is the Codex CLI target.
	TargetCodex Target = "codex"
)

// AllTargets returns all well-known targets.
func AllTargets() []Target {
	return []Target{TargetClaude, TargetCursor, TargetCopilot, TargetCodex}
}

// Profile identifies the runtime environment or policy shape.
// Profiles control security-sensitive behavior such as hook enablement,
// script emission, plugin auto-install, and network capability access.
type Profile string

const (
	// ProfileLocalDev is the default local development profile.
	ProfileLocalDev Profile = "local-dev"
	// ProfileCI is the continuous integration profile.
	ProfileCI Profile = "ci"
	// ProfileEnterpriseLocked is the enterprise-locked security profile.
	ProfileEnterpriseLocked Profile = "enterprise-locked"
	// ProfileOSSPublic is the open-source public profile.
	ProfileOSSPublic Profile = "oss-public"
)

// BuildScope identifies where canonical objects apply within a build.
// It combines path patterns, file types, and labels.
type BuildScope struct {
	// Paths are filesystem paths or globs.
	Paths []string
	// FileTypes restricts to specific file extensions.
	FileTypes []string
	// Labels are semantic tags for domain scoping.
	Labels []string
}

// BuildUnit is the fundamental compilation target. The compiler may emit
// many build units in a single run, supporting "compile to one or more
// environments" without duplicating source content.
type BuildUnit struct {
	// Target is the vendor/ecosystem to emit for.
	Target Target
	// Profile is the runtime environment or policy shape.
	Profile Profile
	// Scopes are the selected scopes for this build unit.
	Scopes []BuildScope
}

// BuildCoordinate is a fully resolved reference to a specific build unit
// with its output directory.
type BuildCoordinate struct {
	// Unit is the build unit this coordinate refers to.
	Unit BuildUnit
	// OutputDir is the resolved output directory for this build unit
	// (e.g., ".ai-build/claude/local-dev/").
	OutputDir string
}
