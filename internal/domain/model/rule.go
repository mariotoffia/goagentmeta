package model

// Rule is a scoped or conditional policy object. Rules are semantically
// distinct from instructions: they apply conditionally based on language,
// file type, path, or other criteria. Some targets may require lowering
// rules into instructions.
type Rule struct {
	ObjectMeta

	// Content is the markdown rule text.
	Content string

	// Conditions describe when this rule is active beyond its scope.
	// For example, a rule may apply only to generated code or security-sensitive paths.
	Conditions []RuleCondition
}

// RuleCondition describes a single activation condition for a rule.
type RuleCondition struct {
	// Type identifies the condition kind (e.g., "language", "generated", "path-pattern").
	Type string
	// Value is the condition value (e.g., "go", "true", "services/**").
	Value string
}
