// Package tool defines the domain model for tool plugins. A tool plugin
// describes a single tool keyword (e.g., "Bash", "Read", "WebSearch") that
// may appear in Skill.Tools, Skill.DisallowedTools, Agent.Tools, or
// Agent.DisallowedTools. Each plugin knows how to validate expressions
// using its keyword and optionally declares a parameterized syntax grammar.
package tool

import (
	"fmt"
	"strings"
	"sync"
)

// Plugin is the interface that every tool plugin must implement.
// A plugin describes a single tool keyword and provides validation for
// expressions that use that keyword.
type Plugin interface {
	// Keyword returns the canonical tool name (e.g., "Bash", "Read",
	// "WebSearch", "mcp"). Keywords are case-sensitive.
	Keyword() string

	// Description returns a human-readable description of the tool.
	Description() string

	// HasSyntax reports whether this tool supports parameterized syntax
	// beyond just the bare keyword. For example, Bash supports
	// "Bash(go:*)" while Read is just "Read".
	HasSyntax() bool

	// SyntaxHelp returns a human-readable syntax description for tools
	// that support parameterized expressions. Returns empty string if
	// HasSyntax() is false.
	//
	// Example: "Bash(<command>:<glob>)" or "mcp__<server>__<tool>"
	SyntaxHelp() string

	// Targets returns the set of targets that support this tool. An empty
	// slice means the tool is available on all targets. Target strings use
	// the same identifiers as build.Target (e.g., "claude", "copilot").
	Targets() []string

	// RelatedCapabilities returns capability IDs that this tool implements
	// or relates to. For example, the Read tool relates to "filesystem.read"
	// and the Bash tool relates to "terminal.exec". This enables the
	// validator to cross-reference tools/disallowedTools capability entries with tools.
	RelatedCapabilities() []string

	// Validate checks whether the given expression is a valid use of
	// this tool. The expression is the full string as it appears in
	// tools or disallowedTools (e.g., "Bash(go:*)" or "Read").
	//
	// Returns nil if the expression is valid, or an error describing
	// the problem.
	Validate(expr string) error
}

// Expression is a parsed tool reference. It holds the original expression
// string along with the resolved plugin keyword and any arguments.
type Expression struct {
	// Raw is the original expression string (e.g., "Bash(go:*)").
	Raw string

	// Keyword is the resolved tool keyword (e.g., "Bash").
	Keyword string

	// Args is the parameter portion for tools with syntax.
	// Empty for bare keywords like "Read".
	Args string
}

// ParseExpression splits a tool expression into keyword and args.
// It handles three forms:
//   - bare keyword: "Read" → keyword="Read", args=""
//   - parenthesized: "Bash(go:*)" → keyword="Bash", args="go:*"
//   - double-underscore (MCP): "mcp__server__tool" → keyword="mcp", args="server__tool"
func ParseExpression(expr string) Expression {
	// Parenthesized form: Keyword(args)
	if idx := strings.Index(expr, "("); idx > 0 {
		keyword := expr[:idx]
		args := strings.TrimSuffix(expr[idx+1:], ")")
		return Expression{Raw: expr, Keyword: keyword, Args: args}
	}

	// MCP double-underscore form: mcp__server__tool
	if strings.HasPrefix(expr, "mcp__") {
		args := strings.TrimPrefix(expr, "mcp__")
		return Expression{Raw: expr, Keyword: "mcp", Args: args}
	}

	// Bare keyword
	return Expression{Raw: expr, Keyword: expr, Args: ""}
}

// Registry holds all registered tool plugins and provides validation
// services. It is safe for concurrent read access after initial registration.
type Registry struct {
	mu             sync.RWMutex
	plugins        map[string]Plugin
	capabilityIDs  map[string]bool // known capability IDs
	capToTool      map[string][]string // capability ID → tool keywords
}

// NewRegistry creates an empty tool plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:       make(map[string]Plugin),
		capabilityIDs: make(map[string]bool),
		capToTool:     make(map[string][]string),
	}
}

// Register adds a plugin to the registry. Returns an error if a plugin
// with the same keyword is already registered.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	kw := p.Keyword()
	if _, exists := r.plugins[kw]; exists {
		return fmt.Errorf("tool plugin %q already registered", kw)
	}
	r.plugins[kw] = p

	// Index capability relationships.
	for _, cap := range p.RelatedCapabilities() {
		r.capabilityIDs[cap] = true
		r.capToTool[cap] = append(r.capToTool[cap], kw)
	}

	return nil
}

// RegisterCapabilityID adds a known capability ID to the registry without
// associating it with a specific tool. This allows tools/disallowedTools
// entries like "filesystem.write" or "network.http" to pass validation.
func (r *Registry) RegisterCapabilityID(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.capabilityIDs[id] = true
}

// IsCapabilityID reports whether the given string is a recognized capability ID.
func (r *Registry) IsCapabilityID(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.capabilityIDs[id]
}

// ToolsForCapability returns the tool keywords that implement the given
// capability ID. Returns nil if no tools implement it.
func (r *Registry) ToolsForCapability(capID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.capToTool[capID]
}

// MustRegister is like Register but panics on error.
func (r *Registry) MustRegister(p Plugin) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

// Lookup returns the plugin for the given keyword, or nil if not found.
func (r *Registry) Lookup(keyword string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[keyword]
}

// ValidateExpression validates a single tool expression against the
// registered plugins. Returns nil if valid.
//
// If the keyword is not registered and is not a known capability ID,
// returns an UnknownToolError. If the syntax is invalid, returns the
// plugin's validation error.
func (r *Registry) ValidateExpression(expr string) error {
	parsed := ParseExpression(expr)

	r.mu.RLock()
	p, ok := r.plugins[parsed.Keyword]
	isCap := r.capabilityIDs[expr]
	r.mu.RUnlock()

	if !ok {
		if isCap {
			return nil // recognized capability ID (e.g., "filesystem.write")
		}
		return &UnknownToolError{Expression: parsed}
	}
	return p.Validate(expr)
}

// ValidateExpressionForTarget validates a tool expression and additionally
// checks that the tool is available on the specified target. The target
// string uses build.Target identifiers ("claude", "copilot", "codex", "cursor").
func (r *Registry) ValidateExpressionForTarget(expr, target string) error {
	if err := r.ValidateExpression(expr); err != nil {
		return err
	}

	parsed := ParseExpression(expr)
	r.mu.RLock()
	p, ok := r.plugins[parsed.Keyword]
	r.mu.RUnlock()

	if !ok {
		return nil // capability ID — no target restriction
	}

	targets := p.Targets()
	if len(targets) == 0 {
		return nil // available on all targets
	}
	for _, t := range targets {
		if t == target {
			return nil
		}
	}
	return &TargetUnavailableError{
		Expression: parsed,
		Target:     target,
		Available:  targets,
	}
}

// ValidateAll validates a slice of tool expressions. Returns all errors
// found (does not stop at first error).
func (r *Registry) ValidateAll(exprs []string) []error {
	var errs []error
	for _, expr := range exprs {
		if err := r.ValidateExpression(expr); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// All returns all registered plugins in no particular order.
func (r *Registry) All() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

// Keywords returns all registered tool keywords sorted alphabetically.
func (r *Registry) Keywords() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.plugins))
	for k := range r.plugins {
		keys = append(keys, k)
	}
	return keys
}

// UnknownToolError is returned when a tool expression references a
// keyword that has no registered plugin.
type UnknownToolError struct {
	Expression Expression
}

func (e *UnknownToolError) Error() string {
	return fmt.Sprintf("unknown tool %q (from expression %q)", e.Expression.Keyword, e.Expression.Raw)
}

// ValidationError is returned when a tool expression has invalid syntax
// according to its plugin.
type ValidationError struct {
	Expression Expression
	Reason     string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid tool expression %q: %s", e.Expression.Raw, e.Reason)
}

// TargetUnavailableError is returned when a tool is not available on the
// requested target.
type TargetUnavailableError struct {
	Expression Expression
	Target     string
	Available  []string
}

func (e *TargetUnavailableError) Error() string {
	return fmt.Sprintf("tool %q is not available on target %q (available on: %s)",
		e.Expression.Keyword, e.Target, strings.Join(e.Available, ", "))
}
