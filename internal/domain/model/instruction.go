package model

// Instruction is an always-on guidance and context object. Instructions are
// unconditional within their scope and provide architecture principles, coding
// standards, testing expectations, review policies, domain vocabulary, and
// workflow guidance.
type Instruction struct {
	ObjectMeta

	// Content is the markdown instruction text injected into the AI context.
	Content string
}
