package model

import "time"

// Hook is deterministic lifecycle automation triggered by defined events.
// Hooks carry explicit effect semantics because not all lowerings preserve
// enforcement. Hooks are highly non-portable across targets.
type Hook struct {
	ObjectMeta

	// Event is the lifecycle event that triggers this hook (e.g., "post-edit",
	// "pre-tool-use", "session-start").
	Event string

	// Action defines what the hook executes when triggered.
	Action HookAction

	// Effect describes the semantic impact of this hook.
	Effect HookEffect

	// Inputs specifies which context data the hook receives.
	Inputs HookInputs

	// Policy controls timeout, retry, and failure behavior.
	Policy HookPolicy
}

// HookAction defines the executable action for a hook.
type HookAction struct {
	// Type is the action kind (e.g., "script", "command", "http", "prompt", "agent").
	Type string
	// Ref is a reference to the action target (e.g., script path, command name).
	Ref string
}

// EffectClass categorizes the semantic impact of a hook.
type EffectClass string

const (
	EffectObserving    EffectClass = "observing"
	EffectValidating   EffectClass = "validating"
	EffectTransforming EffectClass = "transforming"
	EffectSetup        EffectClass = "setup"
	EffectReporting    EffectClass = "reporting"
)

// EnforcementMode controls how strictly a hook's outcome is enforced.
type EnforcementMode string

const (
	EnforcementBlocking EnforcementMode = "blocking"
	EnforcementAdvisory EnforcementMode = "advisory"
	EnforcementBestEffort EnforcementMode = "best-effort"
)

// HookEffect describes the semantic impact and enforcement of a hook.
type HookEffect struct {
	// Class categorizes what the hook does.
	Class EffectClass
	// Enforcement controls how strictly the hook's outcome is applied.
	Enforcement EnforcementMode
}

// HookInputs specifies which context data a hook receives when triggered.
type HookInputs struct {
	// Include lists the input data keys the hook receives
	// (e.g., "changedFiles", "workingDirectory").
	Include []string
}

// HookPolicy controls operational behavior for hook execution.
type HookPolicy struct {
	// Timeout is the maximum duration the hook may run before being killed.
	Timeout time.Duration
	// MaxRetries is the number of retry attempts on failure. Zero means no retries.
	MaxRetries int
	// FailurePolicy controls what happens when the hook fails after all retries.
	// Values: "fail-build", "warn", "ignore".
	FailurePolicy string
}
