package model

// Agent is a specialized delegate or orchestration wrapper. Agents define a
// role or system prompt, tool and permission policy, allowed delegations,
// linked skills, required capabilities, and optional hooks. An agent is not
// a tool provider — it is a policy and orchestration surface around tools,
// skills, and delegation.
type Agent struct {
	ObjectMeta

	// RolePrompt is the system prompt that defines the agent's specialization.
	RolePrompt string

	// Skills lists skill IDs that this agent has access to.
	Skills []string

	// Requires lists capability IDs needed by this agent.
	Requires []string

	// ToolPolicy maps capability or tool names to access decisions.
	// Values are typically "allow", "deny", or "ask".
	ToolPolicy map[string]string

	// Delegation defines which other agents this agent may call.
	Delegation AgentDelegation

	// Handoffs define guided sequential workflow transitions suggesting the
	// user move to another agent with a pre-filled prompt. Handoffs are
	// currently supported natively by Copilot.
	Handoffs []Handoff

	// Hooks lists hook IDs that are scoped to this agent.
	Hooks []string

	// Model specifies a preferred model for this agent (optional).
	Model string
}

// AgentDelegation controls which other agents an agent may call as subagents.
type AgentDelegation struct {
	// MayCall lists agent IDs that this agent is allowed to delegate to.
	MayCall []string
}

// Handoff describes a guided workflow transition to another agent.
type Handoff struct {
	// Label is the user-facing name for this handoff (e.g., "Start Review").
	Label string
	// Agent is the target agent ID to transition to.
	Agent string
	// Prompt is the pre-filled prompt text for the target agent.
	Prompt string
	// AutoSend controls whether the handoff is executed automatically
	// or requires user confirmation.
	AutoSend bool
}
