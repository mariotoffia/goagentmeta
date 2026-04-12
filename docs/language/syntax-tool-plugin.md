# Tool Plugin System

The tool plugin system validates tool expressions that appear in `Skill.allowedTools` and `Agent.toolPolicy`. Each tool plugin describes a single tool keyword, its optional parameterized syntax, and validation logic.

## Concepts

| Concept | Description |
|---|---|
| **Tool keyword** | The canonical name of a tool: `Read`, `Bash`, `WebSearch`, `mcp` |
| **Tool expression** | A full tool reference as written in `allowedTools` or `toolPolicy`. May be a bare keyword (`Read`) or parameterized (`Bash(go:*)`, `mcp__github__list-repos`) |
| **Tool plugin** | An implementation of the `tool.Plugin` interface that validates expressions for one keyword |
| **Tool registry** | A `tool.Registry` holding all registered plugins |

## Expression Syntax

Three expression forms are supported:

### 1. Bare keyword

```
Read
```

Matches exactly the tool name. Used for tools that take no parameters.

### 2. Parenthesized parameters

```
Bash(<command>:<glob>)
```

The keyword is followed by arguments in parentheses. The specific grammar is defined by the plugin.

### 3. Double-underscore (MCP tools)

```
mcp__<server-name>__<tool-name>
```

MCP tool references use `__` as a separator. The `mcp` keyword is implicit from the prefix.

## Built-in Tool Plugins

### Filesystem Tools

| Keyword | Syntax | Description |
|---|---|---|
| `Read` | Bare | Read file contents from the repository |
| `Write` | Bare | Write or create files in the repository |
| `Edit` | Bare | Make targeted edits to existing files |
| `MultiEdit` | Bare | Apply multiple edits to a file atomically |

### Search Tools

| Keyword | Syntax | Description |
|---|---|---|
| `Glob` | Bare | Find files by glob pattern |
| `Grep` | Bare | Search file contents with regex patterns |
| `Search` | Bare | Semantic or full-text code search |

### Terminal Tools

| Keyword | Syntax | Description |
|---|---|---|
| `Bash` | `Bash` or `Bash(<command>:<glob>)` | Execute shell commands; supports scoped restrictions |
| `Terminal` | Bare | Interactive terminal session |

**Bash syntax:** The scoped form restricts which commands and arguments the tool may execute:

```yaml
allowedTools:
  - Bash                      # Unrestricted shell access
  - "Bash(go:*)"              # Only `go` commands, any arguments
  - "Bash(golangci-lint:*)"   # Only `golangci-lint` commands
  - "Bash(git:status)"        # Only `git status`
```

The format is `Bash(<command>:<glob>)` where:
- `<command>` is the binary name (required, non-empty)
- `<glob>` is an argument glob pattern (required, non-empty; use `*` for any)

### Web Tools

| Keyword | Syntax | Description |
|---|---|---|
| `WebFetch` | Bare | Fetch a URL and return its content |
| `WebSearch` | Bare | Search the web for information |

### Agent Interaction Tools

| Keyword | Syntax | Description |
|---|---|---|
| `Agent` | Bare | Delegate work to another agent |
| `AskUser` | Bare | Ask the user a question and wait for a response |
| `AskUserQuestion` | Bare | Alias for AskUser |

### Task Management Tools

| Keyword | Syntax | Description |
|---|---|---|
| `Task` | Bare | Launch a sub-agent for a specific task |
| `TodoRead` | Bare | Read items from the TODO/task list |
| `TodoWrite` | Bare | Write items to the TODO/task list |

### Version Control Tools

| Keyword | Syntax | Description |
|---|---|---|
| `GitCommit` | Bare | Create a git commit |
| `GitDiff` | Bare | Show git diff output |
| `GitLog` | Bare | Show git commit log |
| `GitStatus` | Bare | Show git working tree status |

### Code Intelligence Tools

| Keyword | Syntax | Description |
|---|---|---|
| `LSP` | Bare | Language Server Protocol operations |
| `Symbols` | Bare | Find symbols in the codebase |
| `References` | Bare | Find references to a symbol |
| `Definition` | Bare | Go to symbol definition |

### Notebook / REPL Tools

| Keyword | Syntax | Description |
|---|---|---|
| `Notebook` | Bare | Execute code in a notebook cell |
| `REPL` | Bare | Execute code in an interactive REPL |

### MCP Tools

| Keyword | Syntax | Description |
|---|---|---|
| `mcp` | `mcp__<server>__<tool>` | Model Context Protocol server tool reference |

**MCP syntax:** References follow the `mcp__<server-name>__<tool-name>` convention:

```yaml
allowedTools:
  - mcp__context7__resolve-library-id
  - mcp__context7__query-docs
  - mcp__github__list-repos
  - mcp__postgres__query
```

Both `<server-name>` and `<tool-name>` are required and non-empty.

## Validation Behavior

Tool expressions are validated during the `PhaseValidate` stage across several dimensions:

### Expression Syntax Validation

| Condition | Severity | Example |
|---|---|---|
| Unknown tool keyword | **warning** | `"FooTool"` — not registered |
| Invalid syntax for known tool | **error** | `"Bash()"` — empty args |
| Invalid toolPolicy decision | **error** | `"Bash": "maybe"` — not allow/deny/ask |
| Recognized capability ID | **pass** | `"filesystem.write"` in toolPolicy |

Unknown tools produce warnings (not errors) because provider-specific tools may not be in the built-in registry.

### Capability ID Recognition

`toolPolicy` keys may be capability IDs rather than tool names. The registry recognizes all standard capability IDs:

```yaml
toolPolicy:
  filesystem.write: allow    # ✓ recognized capability ID — no warning
  terminal.exec: deny        # ✓ recognized capability ID
  Read: allow                # ✓ recognized tool keyword
  FooTool: deny              # ⚠ warning: unknown tool
```

Standard capability IDs: `filesystem.read`, `filesystem.write`, `filesystem.read-system`, `terminal.exec`, `terminal.exec-restricted`, `repo.search`, `repo.graph.query`, `repo.history`, `mcp.github`, `mcp.slack`, `mcp.jira`, `mcp.postgres`, `mcp.memory`, `network.http`, `network.http.outbound`, `secrets.read`.

### Capability Cross-Referencing

Each tool plugin declares which capabilities it relates to:

| Tool | Related Capabilities |
|---|---|
| `Read`, `Glob` | `filesystem.read` |
| `Write`, `Edit`, `MultiEdit` | `filesystem.write` |
| `Grep`, `Search` | `repo.search` |
| `Bash` | `terminal.exec`, `terminal.exec-restricted` |
| `Terminal`, `Notebook`, `REPL` | `terminal.exec` |
| `WebFetch`, `WebSearch` | `network.http.outbound` |
| `GitCommit`, `GitDiff`, `GitLog`, `GitStatus` | `repo.history` |
| `LSP`, `Symbols`, `References`, `Definition` | `repo.graph.query` |

### Bash ↔ binaryDeps Cross-Validation

When a skill declares `Bash(<command>:*)` in `allowedTools` and also declares `binaryDeps`, the validator checks that each Bash command name appears in the binary deps list:

```yaml
---
id: my-skill
kind: skill
allowedTools:
  - "Bash(go:*)"            # ← uses "go"
  - "Bash(golangci-lint:*)"  # ← uses "golangci-lint"
binaryDeps:
  - go                       # ✓ matches Bash(go:*)
  # golangci-lint missing    # ⚠ warning: Bash(golangci-lint:*) but not in binaryDeps
---
```

### Target Availability

Each tool plugin declares which targets support it. The `ValidateExpressionForTarget()` API checks availability:

```go
err := registry.ValidateExpressionForTarget("Handoff", "claude")
// → TargetUnavailableError: tool "Handoff" not available on "claude"
```

Currently all built-in tools are available on all targets (empty target list = universal). Target restrictions apply when custom tool plugins are registered for provider-specific features.

## Authoring a Custom Tool Plugin

Implement the `tool.Plugin` interface:

```go
package mytool

import (
    "github.com/mariotoffia/goagentmeta/internal/domain/tool"
)

type MyToolPlugin struct{}

func (p *MyToolPlugin) Keyword() string     { return "MyTool" }
func (p *MyToolPlugin) Description() string { return "Does something special." }
func (p *MyToolPlugin) HasSyntax() bool     { return true }
func (p *MyToolPlugin) SyntaxHelp() string  { return "MyTool(<arg>)" }

// Targets returns which targets support this tool.
// Return nil for all targets, or specific target IDs.
func (p *MyToolPlugin) Targets() []string { return []string{"claude", "copilot"} }

// RelatedCapabilities returns capability IDs this tool implements.
func (p *MyToolPlugin) RelatedCapabilities() []string { return []string{"terminal.exec"} }

func (p *MyToolPlugin) Validate(expr string) error {
    parsed := tool.ParseExpression(expr)
    if parsed.Args == "" {
        return &tool.ValidationError{
            Expression: parsed,
            Reason:     "MyTool requires an argument",
        }
    }
    return nil
}
```

Register it in the pipeline:

```go
registry := adaptortool.NewDefaultRegistry()
registry.MustRegister(&mytool.MyToolPlugin{})

valStage, _ := validator.New(validator.WithToolRegistry(registry))
```

## Relationship to Capabilities

Tool plugins and capabilities are complementary:

| Concept | Purpose | Example |
|---|---|---|
| **Tool plugin** | Validates tool expression syntax | `Bash(go:*)` is syntactically correct |
| **Capability** | Declares abstract requirements | `terminal.exec` is needed |
| **Provider** | Satisfies capabilities at runtime | Plugin X provides `terminal.exec` |

`toolPolicy` keys may be either tool keywords or capability IDs. The validator warns (does not error) for unknown keys in `toolPolicy` to accommodate both.
