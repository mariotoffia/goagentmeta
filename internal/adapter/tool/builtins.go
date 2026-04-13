// Package tool provides built-in tool plugin implementations and a
// factory for creating a pre-populated registry with all standard tools.
//
// Each tool plugin validates expressions for a single tool keyword.
// Plugins with parameterized syntax (like Bash and MCP) validate the
// internal grammar of their arguments.
//
// Each plugin also declares:
//   - Which targets support it (empty = all targets)
//   - Which capabilities it relates to (for tools/disallowedTools cross-referencing)
package tool

import (
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/tool"
)

// NewDefaultRegistry creates a Registry pre-populated with all built-in
// tool plugins and all standard capability IDs. This is the standard
// registry used by the compiler pipeline.
func NewDefaultRegistry() *tool.Registry {
	r := tool.NewRegistry()
	for _, p := range AllBuiltins() {
		r.MustRegister(p)
	}

	// Register standard capability IDs that can appear in tools or
	// disallowedTools but are not direct tool keywords.
	for _, cap := range standardCapabilityIDs() {
		r.RegisterCapabilityID(cap)
	}

	return r
}

// standardCapabilityIDs returns all known capability IDs from the
// capability domain. These may appear in tools or disallowedTools lists.
func standardCapabilityIDs() []string {
	return []string{
		// Filesystem
		"filesystem.read",
		"filesystem.write",
		"filesystem.read-system",
		// Terminal
		"terminal.exec",
		"terminal.exec-restricted",
		// Repository
		"repo.search",
		"repo.graph.query",
		"repo.history",
		// MCP servers
		"mcp.github",
		"mcp.slack",
		"mcp.jira",
		"mcp.postgres",
		"mcp.memory",
		// Network
		"network.http",
		"network.http.outbound",
		// Secrets
		"secrets.read",
	}
}

// AllBuiltins returns all built-in tool plugin instances.
func AllBuiltins() []tool.Plugin {
	return []tool.Plugin{
		// Filesystem tools — available on all targets
		newTool("Read", "Read file contents from the repository.",
			nil, []string{"filesystem.read"}),
		newTool("Write", "Write or create files in the repository.",
			nil, []string{"filesystem.write"}),
		newTool("Edit", "Make targeted edits to existing files.",
			nil, []string{"filesystem.write"}),
		newTool("MultiEdit", "Apply multiple edits to a file atomically.",
			nil, []string{"filesystem.write"}),

		// Search tools — available on all targets
		newTool("Glob", "Find files by glob pattern.",
			nil, []string{"filesystem.read"}),
		newTool("Grep", "Search file contents with regex patterns.",
			nil, []string{"repo.search"}),
		newTool("Search", "Semantic or full-text code search.",
			nil, []string{"repo.search"}),

		// Terminal tools — available on all targets
		NewBashPlugin(),
		newTool("Terminal", "Interactive terminal session.",
			nil, []string{"terminal.exec"}),

		// Web tools — available on all targets
		newTool("WebFetch", "Fetch a URL and return its content.",
			nil, []string{"network.http.outbound"}),
		newTool("WebSearch", "Search the web for information.",
			nil, []string{"network.http.outbound"}),

		// Agent interaction tools — available on all targets
		newTool("Agent", "Delegate work to another agent.",
			nil, nil),
		newTool("AskUser", "Ask the user a question and wait for a response.",
			nil, nil),
		newTool("AskUserQuestion", "Ask the user a question (alias for AskUser).",
			nil, nil),

		// Task management tools — available on all targets
		newTool("Task", "Launch a sub-agent for a specific task.",
			nil, nil),
		newTool("TodoRead", "Read items from the TODO/task list.",
			nil, nil),
		newTool("TodoWrite", "Write items to the TODO/task list.",
			nil, nil),

		// Version control tools — available on all targets
		newTool("GitCommit", "Create a git commit.",
			nil, []string{"repo.history"}),
		newTool("GitDiff", "Show git diff output.",
			nil, []string{"repo.history"}),
		newTool("GitLog", "Show git commit log.",
			nil, []string{"repo.history"}),
		newTool("GitStatus", "Show git working tree status.",
			nil, []string{"repo.history"}),

		// Code intelligence tools — available on all targets
		newTool("LSP", "Language Server Protocol operations.",
			nil, []string{"repo.graph.query"}),
		newTool("Symbols", "Find symbols in the codebase.",
			nil, []string{"repo.graph.query"}),
		newTool("References", "Find references to a symbol.",
			nil, []string{"repo.graph.query"}),
		newTool("Definition", "Go to symbol definition.",
			nil, []string{"repo.graph.query"}),

		// Notebook / REPL tools — available on all targets
		newTool("Notebook", "Execute code in a notebook cell.",
			nil, []string{"terminal.exec"}),
		newTool("REPL", "Execute code in an interactive REPL.",
			nil, []string{"terminal.exec"}),

		// MCP tools — available on all targets
		NewMCPPlugin(),
	}
}

// simplePlugin is a tool plugin for tools that have no parameterized
// syntax — only a bare keyword like "Read" or "WebSearch".
type simplePlugin struct {
	keyword      string
	description  string
	targets      []string
	capabilities []string
}

// newTool creates a tool plugin with target and capability metadata.
// Pass nil for targets to indicate availability on all targets.
func newTool(keyword, description string, targets, capabilities []string) tool.Plugin {
	return &simplePlugin{
		keyword:      keyword,
		description:  description,
		targets:      targets,
		capabilities: capabilities,
	}
}

// NewSimplePlugin creates a tool plugin that only accepts its bare keyword.
// For backward compatibility — prefer newTool for new plugins.
func NewSimplePlugin(keyword, description string) tool.Plugin {
	return &simplePlugin{keyword: keyword, description: description}
}

func (p *simplePlugin) Keyword() string              { return p.keyword }
func (p *simplePlugin) Description() string           { return p.description }
func (p *simplePlugin) HasSyntax() bool               { return false }
func (p *simplePlugin) SyntaxHelp() string            { return "" }
func (p *simplePlugin) Targets() []string             { return p.targets }
func (p *simplePlugin) RelatedCapabilities() []string { return p.capabilities }

func (p *simplePlugin) Validate(expr string) error {
	if expr != p.keyword {
		return &tool.ValidationError{
			Expression: tool.ParseExpression(expr),
			Reason:     fmt.Sprintf("expected exact keyword %q, got %q", p.keyword, expr),
		}
	}
	return nil
}

// bashPlugin validates Bash tool expressions. Bash supports two forms:
//   - Bare: "Bash" (unrestricted shell access)
//   - Scoped: "Bash(<command>:<glob>)" where <command> is a binary name
//     and <glob> is an argument glob pattern (e.g., "go:*", "git:status")
type bashPlugin struct{}

// NewBashPlugin creates a Bash tool plugin.
func NewBashPlugin() tool.Plugin {
	return &bashPlugin{}
}

func (p *bashPlugin) Keyword() string     { return "Bash" }
func (p *bashPlugin) Description() string { return "Execute shell commands. Supports scoped restrictions." }
func (p *bashPlugin) HasSyntax() bool     { return true }
func (p *bashPlugin) SyntaxHelp() string  { return "Bash(<command>:<glob>)" }
func (p *bashPlugin) Targets() []string   { return nil }
func (p *bashPlugin) RelatedCapabilities() []string {
	return []string{"terminal.exec", "terminal.exec-restricted"}
}

func (p *bashPlugin) Validate(expr string) error {
	if expr == "Bash" {
		return nil // bare keyword = unrestricted
	}

	parsed := tool.ParseExpression(expr)
	if parsed.Keyword != "Bash" {
		return &tool.ValidationError{
			Expression: parsed,
			Reason:     fmt.Sprintf("expected keyword Bash, got %q", parsed.Keyword),
		}
	}

	if parsed.Args == "" {
		return &tool.ValidationError{
			Expression: parsed,
			Reason:     "Bash(...) requires arguments in the form <command>:<glob>",
		}
	}

	// Validate args format: <command>:<glob>
	parts := strings.SplitN(parsed.Args, ":", 2)
	if len(parts) != 2 {
		return &tool.ValidationError{
			Expression: parsed,
			Reason:     fmt.Sprintf("expected <command>:<glob> format, got %q", parsed.Args),
		}
	}

	command := parts[0]
	if command == "" {
		return &tool.ValidationError{
			Expression: parsed,
			Reason:     "command name cannot be empty in Bash(<command>:<glob>)",
		}
	}

	glob := parts[1]
	if glob == "" {
		return &tool.ValidationError{
			Expression: parsed,
			Reason:     "glob pattern cannot be empty in Bash(<command>:<glob>)",
		}
	}

	return nil
}

// mcpPlugin validates MCP (Model Context Protocol) tool references.
// MCP tools use the double-underscore convention: mcp__<server>__<tool>
type mcpPlugin struct{}

// NewMCPPlugin creates an MCP tool plugin.
func NewMCPPlugin() tool.Plugin {
	return &mcpPlugin{}
}

func (p *mcpPlugin) Keyword() string              { return "mcp" }
func (p *mcpPlugin) Description() string           { return "Model Context Protocol server tool reference." }
func (p *mcpPlugin) HasSyntax() bool               { return true }
func (p *mcpPlugin) SyntaxHelp() string            { return "mcp__<server-name>__<tool-name>" }
func (p *mcpPlugin) Targets() []string             { return nil }
func (p *mcpPlugin) RelatedCapabilities() []string { return nil }

func (p *mcpPlugin) Validate(expr string) error {
	parsed := tool.ParseExpression(expr)
	if parsed.Keyword != "mcp" {
		return &tool.ValidationError{
			Expression: parsed,
			Reason:     fmt.Sprintf("expected keyword mcp, got %q", parsed.Keyword),
		}
	}

	if parsed.Args == "" {
		return &tool.ValidationError{
			Expression: parsed,
			Reason:     "mcp tool requires server and tool name: mcp__<server>__<tool>",
		}
	}

	// Args should be "server__tool" (at least one __ separator)
	parts := strings.SplitN(parsed.Args, "__", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return &tool.ValidationError{
			Expression: parsed,
			Reason: fmt.Sprintf(
				"expected mcp__<server>__<tool> format, got %q", expr,
			),
		}
	}

	return nil
}
