---
id: tool-plugin-dev
kind: skill
version: 1
description: How to create and register new tool plugins for expression validation
preservation: preferred
requires:
  capabilities:
    - filesystem.read
    - filesystem.write
    - terminal.exec
activation:
  hints:
    - tool plugin
    - new tool
    - tool expression
    - tool validation
    - tools
tools:
  - Read
  - Write
  - Edit
  - Bash(go:*)
  - Bash(make:*)
  - Grep
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Creating a New Tool Plugin

Tool plugins validate tool expressions used in `tools` and `disallowedTools` fields.

## Tool Expression Forms

Three expression forms exist:
1. **Bare keyword**: `Read`, `WebSearch` — no parameters
2. **Parameterized**: `Bash(go:*)` — plugin-specific grammar inside parentheses
3. **MCP double-underscore**: `mcp__github__list-repos` — MCP server tools

## Step 1: Implement the Plugin Interface

In `internal/domain/tool/tool.go`, the interface is:

```go
type Plugin interface {
    Keyword() string
    HasParameterizedForm() bool
    ValidateExpression(expr string) error
    ValidateExpressionForTarget(expr string, target build.Target) error
}
```

## Step 2: Create Your Plugin

```go
package tool

type MyToolPlugin struct{}

func (p *MyToolPlugin) Keyword() string { return "MyTool" }
func (p *MyToolPlugin) HasParameterizedForm() bool { return true }

func (p *MyToolPlugin) ValidateExpression(expr string) error {
    // Parse and validate the expression
    // For "MyTool(pattern)" — validate the pattern syntax
    return nil
}

func (p *MyToolPlugin) ValidateExpressionForTarget(expr string, target build.Target) error {
    // Check if this tool is available on the given target
    return nil
}
```

## Step 3: Register the Plugin

Add registration in `internal/adapter/tool/builtins.go`:

```go
func NewDefaultRegistry() *tool.Registry {
    r := tool.NewRegistry()
    // ... existing registrations ...
    r.Register(&MyToolPlugin{})
    return r
}
```

## Step 4: Test

- Unit test the plugin's `ValidateExpression` with valid/invalid inputs
- Test `ValidateExpressionForTarget` for each supported target
- Run `make test` to ensure semantic validation still passes
