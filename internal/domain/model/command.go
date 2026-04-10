package model

// Command is an explicit user-invoked entry point (e.g., /review-iam,
// /build-lambda). Commands map to different target surfaces: prompt files
// in Copilot, skills in Claude Code, and have no native equivalent in Cursor.
type Command struct {
	ObjectMeta

	// Description is a human-readable summary of what this command does.
	Description string

	// Action defines what the command executes.
	Action CommandAction
}

// CommandAction defines the executable action for a command.
type CommandAction struct {
	// Type is the action kind (e.g., "script", "skill", "prompt").
	Type string
	// Ref is a reference to the action target.
	Ref string
}
