package tool_test

import (
	"testing"

	adaptortool "github.com/mariotoffia/goagentmeta/internal/adapter/tool"
	"github.com/mariotoffia/goagentmeta/internal/domain/tool"
)

func TestBashPlugin_BareKeyword(t *testing.T) {
	p := adaptortool.NewBashPlugin()
	if err := p.Validate("Bash"); err != nil {
		t.Errorf("bare Bash should be valid: %v", err)
	}
}

func TestBashPlugin_ValidScoped(t *testing.T) {
	p := adaptortool.NewBashPlugin()
	cases := []string{
		"Bash(go:*)",
		"Bash(golangci-lint:*)",
		"Bash(git:status)",
		"Bash(aws:s3 ls)",
		"Bash(curl:*)",
	}
	for _, expr := range cases {
		if err := p.Validate(expr); err != nil {
			t.Errorf("expected valid: %q, got error: %v", expr, err)
		}
	}
}

func TestBashPlugin_InvalidSyntax(t *testing.T) {
	p := adaptortool.NewBashPlugin()
	cases := []struct {
		expr   string
		reason string
	}{
		{"Bash()", "requires arguments"},
		{"Bash(go)", "expected <command>:<glob>"},
		{"Bash(:*)", "command name cannot be empty"},
		{"Bash(go:)", "glob pattern cannot be empty"},
	}
	for _, tc := range cases {
		err := p.Validate(tc.expr)
		if err == nil {
			t.Errorf("expected error for %q", tc.expr)
			continue
		}
		ve, ok := err.(*tool.ValidationError)
		if !ok {
			t.Errorf("expected ValidationError for %q, got %T", tc.expr, err)
			continue
		}
		if ve.Reason == "" {
			t.Errorf("expected non-empty reason for %q", tc.expr)
		}
	}
}

func TestMCPPlugin_Valid(t *testing.T) {
	p := adaptortool.NewMCPPlugin()
	cases := []string{
		"mcp__context7__resolve-library-id",
		"mcp__github__list-repos",
		"mcp__postgres__query",
	}
	for _, expr := range cases {
		if err := p.Validate(expr); err != nil {
			t.Errorf("expected valid: %q, got error: %v", expr, err)
		}
	}
}

func TestMCPPlugin_Invalid(t *testing.T) {
	p := adaptortool.NewMCPPlugin()
	cases := []string{
		"mcp__",
		"mcp____",
		"mcp__serveronly",
	}
	for _, expr := range cases {
		if err := p.Validate(expr); err == nil {
			t.Errorf("expected error for %q", expr)
		}
	}
}

func TestSimplePlugin_ExactMatch(t *testing.T) {
	p := adaptortool.NewSimplePlugin("Read", "Read files")
	if err := p.Validate("Read"); err != nil {
		t.Errorf("exact match should be valid: %v", err)
	}
	if err := p.Validate("Read(foo)"); err == nil {
		t.Error("parameterized form should be invalid for simple plugin")
	}
}

func TestDefaultRegistry_AllBuiltins(t *testing.T) {
	r := adaptortool.NewDefaultRegistry()
	keywords := r.Keywords()
	if len(keywords) == 0 {
		t.Fatal("expected non-empty keyword list")
	}

	// Verify key tools are registered
	expected := []string{
		"Read", "Write", "Edit", "Glob", "Grep",
		"Bash", "WebFetch", "WebSearch", "Agent",
		"AskUser", "mcp",
	}
	for _, kw := range expected {
		if r.Lookup(kw) == nil {
			t.Errorf("expected %q to be registered", kw)
		}
	}
}

func TestDefaultRegistry_ValidateAll(t *testing.T) {
	r := adaptortool.NewDefaultRegistry()

	// All valid
	errs := r.ValidateAll([]string{"Read", "Write", "Bash(go:*)", "mcp__github__list"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}

	// One invalid
	errs = r.ValidateAll([]string{"Read", "UnknownTool", "Bash(go:*)"})
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(errs), errs)
	}
}
