package tool_test

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/tool"
)

func TestParseExpression_BareKeyword(t *testing.T) {
	expr := tool.ParseExpression("Read")
	if expr.Keyword != "Read" {
		t.Errorf("expected keyword Read, got %q", expr.Keyword)
	}
	if expr.Args != "" {
		t.Errorf("expected empty args, got %q", expr.Args)
	}
}

func TestParseExpression_Parenthesized(t *testing.T) {
	expr := tool.ParseExpression("Bash(go:*)")
	if expr.Keyword != "Bash" {
		t.Errorf("expected keyword Bash, got %q", expr.Keyword)
	}
	if expr.Args != "go:*" {
		t.Errorf("expected args go:*, got %q", expr.Args)
	}
}

func TestParseExpression_MCP(t *testing.T) {
	expr := tool.ParseExpression("mcp__context7__resolve-library-id")
	if expr.Keyword != "mcp" {
		t.Errorf("expected keyword mcp, got %q", expr.Keyword)
	}
	if expr.Args != "context7__resolve-library-id" {
		t.Errorf("expected args context7__resolve-library-id, got %q", expr.Args)
	}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	reg := tool.NewRegistry()

	p := &mockPlugin{keyword: "Test"}
	if err := reg.Register(p); err != nil {
		t.Fatal(err)
	}

	got := reg.Lookup("Test")
	if got == nil {
		t.Fatal("expected plugin, got nil")
	}
	if got.Keyword() != "Test" {
		t.Errorf("expected Test, got %q", got.Keyword())
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	reg := tool.NewRegistry()

	p := &mockPlugin{keyword: "Test"}
	if err := reg.Register(p); err != nil {
		t.Fatal(err)
	}

	err := reg.Register(p)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_ValidateExpression_Unknown(t *testing.T) {
	reg := tool.NewRegistry()

	err := reg.ValidateExpression("NoSuchTool")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}

	ute, ok := err.(*tool.UnknownToolError)
	if !ok {
		t.Fatalf("expected UnknownToolError, got %T", err)
	}
	if ute.Expression.Keyword != "NoSuchTool" {
		t.Errorf("expected keyword NoSuchTool, got %q", ute.Expression.Keyword)
	}
}

func TestRegistry_ValidateAll(t *testing.T) {
	reg := tool.NewRegistry()
	reg.MustRegister(&mockPlugin{keyword: "Read"})
	reg.MustRegister(&mockPlugin{keyword: "Write"})

	errs := reg.ValidateAll([]string{"Read", "Write", "Unknown"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestRegistry_CapabilityID(t *testing.T) {
	reg := tool.NewRegistry()
	reg.RegisterCapabilityID("filesystem.read")
	reg.RegisterCapabilityID("terminal.exec")

	// Capability IDs should pass validation even without a tool plugin.
	if err := reg.ValidateExpression("filesystem.read"); err != nil {
		t.Errorf("capability ID should be valid: %v", err)
	}

	// Unknown non-capability should still fail.
	if err := reg.ValidateExpression("no.such.cap"); err == nil {
		t.Error("unknown ID should fail")
	}
}

func TestRegistry_CapabilityFromPlugin(t *testing.T) {
	reg := tool.NewRegistry()
	reg.MustRegister(&mockPluginWithCaps{
		keyword: "Read",
		caps:    []string{"filesystem.read"},
	})

	if !reg.IsCapabilityID("filesystem.read") {
		t.Error("expected filesystem.read from plugin registration")
	}

	tools := reg.ToolsForCapability("filesystem.read")
	if len(tools) != 1 || tools[0] != "Read" {
		t.Errorf("expected [Read], got %v", tools)
	}

	// "filesystem.read" should also pass as a direct expression now.
	if err := reg.ValidateExpression("filesystem.read"); err != nil {
		t.Errorf("capability ID should be valid: %v", err)
	}
}

func TestRegistry_ValidateForTarget(t *testing.T) {
	reg := tool.NewRegistry()
	reg.MustRegister(&mockPluginWithTargets{
		keyword: "Handoff",
		targets: []string{"copilot"},
	})

	// Available on copilot.
	if err := reg.ValidateExpressionForTarget("Handoff", "copilot"); err != nil {
		t.Errorf("should be valid on copilot: %v", err)
	}

	// Not available on claude.
	err := reg.ValidateExpressionForTarget("Handoff", "claude")
	if err == nil {
		t.Fatal("expected error for claude")
	}
	tae, ok := err.(*tool.TargetUnavailableError)
	if !ok {
		t.Fatalf("expected TargetUnavailableError, got %T", err)
	}
	if tae.Target != "claude" {
		t.Errorf("expected target claude, got %q", tae.Target)
	}
}

type mockPluginWithCaps struct {
	keyword string
	caps    []string
}

func (m *mockPluginWithCaps) Keyword() string              { return m.keyword }
func (m *mockPluginWithCaps) Description() string           { return "mock" }
func (m *mockPluginWithCaps) HasSyntax() bool               { return false }
func (m *mockPluginWithCaps) SyntaxHelp() string            { return "" }
func (m *mockPluginWithCaps) Targets() []string             { return nil }
func (m *mockPluginWithCaps) RelatedCapabilities() []string { return m.caps }
func (m *mockPluginWithCaps) Validate(_ string) error       { return nil }

type mockPluginWithTargets struct {
	keyword string
	targets []string
}

func (m *mockPluginWithTargets) Keyword() string              { return m.keyword }
func (m *mockPluginWithTargets) Description() string           { return "mock" }
func (m *mockPluginWithTargets) HasSyntax() bool               { return false }
func (m *mockPluginWithTargets) SyntaxHelp() string            { return "" }
func (m *mockPluginWithTargets) Targets() []string             { return m.targets }
func (m *mockPluginWithTargets) RelatedCapabilities() []string { return nil }
func (m *mockPluginWithTargets) Validate(expr string) error {
	if expr != m.keyword {
		return &tool.ValidationError{Expression: tool.ParseExpression(expr), Reason: "bad"}
	}
	return nil
}

type mockPlugin struct {
	keyword string
}

func (m *mockPlugin) Keyword() string              { return m.keyword }
func (m *mockPlugin) Description() string           { return "mock" }
func (m *mockPlugin) HasSyntax() bool               { return false }
func (m *mockPlugin) SyntaxHelp() string            { return "" }
func (m *mockPlugin) Targets() []string             { return nil }
func (m *mockPlugin) RelatedCapabilities() []string { return nil }
func (m *mockPlugin) Validate(_ string) error {
	return nil
}
