package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// ---------------------------------------------------------------------------
// Test helper subprocess
// ---------------------------------------------------------------------------
// When this test binary is re-invoked with EXTERNAL_PLUGIN_TEST_MODE set, it
// acts as a mock external process plugin. The mode value selects the behavior.

func TestMain(m *testing.M) {
	mode := os.Getenv("EXTERNAL_PLUGIN_TEST_MODE")
	if mode == "" {
		os.Exit(m.Run())
	}

	// Running as mock subprocess.
	switch mode {
	case "happy":
		mockHappy()
	case "error_response":
		mockErrorResponse()
	case "bad_json":
		mockBadJSON()
	case "exit_nonzero":
		os.Exit(1)
	case "slow":
		time.Sleep(5 * time.Second)
	default:
		fmt.Fprintf(os.Stderr, "unknown test mode: %s\n", mode)
		os.Exit(2)
	}
}

func mockHappy() {
	var raw json.RawMessage
	dec := json.NewDecoder(os.Stdin)
	if err := dec.Decode(&raw); err != nil {
		fmt.Fprintf(os.Stderr, "mock: decode: %v\n", err)
		os.Exit(1)
	}

	// Peek at the type field.
	var envelope struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(raw, &envelope)

	enc := json.NewEncoder(os.Stdout)
	switch envelope.Type {
	case "describe":
		_ = enc.Encode(DescribeResponse{
			Name:         "mock-plugin",
			Phase:        "render",
			Order:        7,
			TargetFilter: []string{"claude"},
		})
	case "execute":
		var req ExecuteRequest
		_ = json.Unmarshal(raw, &req)
		// Echo input back as output, wrapped in {"echoed": ...}.
		out, _ := json.Marshal(map[string]json.RawMessage{"echoed": req.Input})
		_ = enc.Encode(ExecuteResponse{Output: json.RawMessage(out)})
	default:
		fmt.Fprintf(os.Stderr, "mock: unknown type %q\n", envelope.Type)
		os.Exit(1)
	}
}

func mockErrorResponse() {
	// Read and discard the request.
	var raw json.RawMessage
	_ = json.NewDecoder(os.Stdin).Decode(&raw)

	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(ExecuteResponse{
		Error: "something failed",
		Diagnostics: []DiagnosticMessage{
			{Severity: "error", Message: "bad input", ObjectID: "obj-1"},
		},
	})
}

func mockBadJSON() {
	// Read and discard the request.
	var raw json.RawMessage
	_ = json.NewDecoder(os.Stdin).Decode(&raw)

	_, _ = os.Stdout.WriteString("NOT JSON\n")
}

// ---------------------------------------------------------------------------
// Helper to build an ExternalProcessStage that invokes this test binary
// ---------------------------------------------------------------------------

func testStage(t *testing.T, mode string, opts ...ExternalOption) *ExternalProcessStage {
	t.Helper()

	// Use the test binary itself as the external process.
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	entry := dependency.ExternalPluginEntry{
		Name:   "test-plugin",
		Source: "external://test-plugin",
		Phase:  "render",
		Order:  5,
	}

	s, err := NewExternalProcessStage(entry, opts...)
	if err != nil {
		t.Fatalf("NewExternalProcessStage: %v", err)
	}

	// Override lookPathFn to return the test binary.
	s.lookPathFn = func(string) (string, error) { return testBinary, nil }

	// Override cmdFactory to inject the mode env var and use test run args.
	s.cmdFactory = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = append(os.Environ(), "EXTERNAL_PLUGIN_TEST_MODE="+mode)
		return cmd
	}

	return s
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewExternalProcessStage_BadSource(t *testing.T) {
	entry := dependency.ExternalPluginEntry{
		Name:   "bad",
		Source: "http://not-external",
		Phase:  "render",
		Order:  1,
	}
	_, err := NewExternalProcessStage(entry)
	if err == nil {
		t.Fatal("expected error for non-external source")
	}
}

func TestNewExternalProcessStage_EmptyCommand(t *testing.T) {
	entry := dependency.ExternalPluginEntry{
		Name:   "empty",
		Source: "external://",
		Phase:  "render",
		Order:  1,
	}
	_, err := NewExternalProcessStage(entry)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestNewExternalProcessStage_InvalidPhase(t *testing.T) {
	entry := dependency.ExternalPluginEntry{
		Name:   "bad-phase",
		Source: "external://something",
		Phase:  "nonexistent",
		Order:  1,
	}
	_, err := NewExternalProcessStage(entry)
	if err == nil {
		t.Fatal("expected error for invalid phase")
	}
}

func TestNewExternalProcessStage_Descriptor(t *testing.T) {
	entry := dependency.ExternalPluginEntry{
		Name:   "my-renderer",
		Source: "external://my-renderer",
		Phase:  "render",
		Order:  10,
		Target: "claude",
	}
	s, err := NewExternalProcessStage(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := s.Descriptor()
	if d.Name != "my-renderer" {
		t.Errorf("Name = %q, want %q", d.Name, "my-renderer")
	}
	if d.Phase != pipeline.PhaseRender {
		t.Errorf("Phase = %v, want %v", d.Phase, pipeline.PhaseRender)
	}
	if d.Order != 10 {
		t.Errorf("Order = %d, want 10", d.Order)
	}
	if len(d.TargetFilter) != 1 || d.TargetFilter[0] != build.TargetClaude {
		t.Errorf("TargetFilter = %v, want [claude]", d.TargetFilter)
	}
}

func TestDescribe_Happy(t *testing.T) {
	s := testStage(t, "happy")

	if err := s.Describe(context.Background()); err != nil {
		t.Fatalf("Describe: %v", err)
	}

	d := s.Descriptor()
	if d.Name != "mock-plugin" {
		t.Errorf("Name = %q, want %q", d.Name, "mock-plugin")
	}
	if d.Phase != pipeline.PhaseRender {
		t.Errorf("Phase = %v, want render", d.Phase)
	}
	if d.Order != 7 {
		t.Errorf("Order = %d, want 7", d.Order)
	}
	if len(d.TargetFilter) != 1 || d.TargetFilter[0] != build.TargetClaude {
		t.Errorf("TargetFilter = %v, want [claude]", d.TargetFilter)
	}
}

func TestExecute_Happy(t *testing.T) {
	s := testStage(t, "happy")

	input := map[string]string{"hello": "world"}
	out, err := s.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, ok := out.(json.RawMessage)
	if !ok {
		t.Fatalf("output type = %T, want json.RawMessage", out)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	echoed := string(result["echoed"])
	if !strings.Contains(echoed, `"hello"`) {
		t.Errorf("echoed = %s, want to contain hello", echoed)
	}
}

func TestExecute_ErrorResponse(t *testing.T) {
	s := testStage(t, "error_response")

	_, err := s.Execute(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error from error response")
	}
	if !strings.Contains(err.Error(), "something failed") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "something failed")
	}
	if !strings.Contains(err.Error(), "bad input") {
		t.Errorf("error = %q, want to contain diagnostic %q", err.Error(), "bad input")
	}
}

func TestExecute_BadJSON(t *testing.T) {
	s := testStage(t, "bad_json")

	_, err := s.Execute(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error from bad JSON response")
	}
}

func TestExecute_NonZeroExit(t *testing.T) {
	s := testStage(t, "exit_nonzero")

	_, err := s.Execute(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error from non-zero exit")
	}
}

func TestExecute_Timeout(t *testing.T) {
	s := testStage(t, "slow", WithTimeout(200*time.Millisecond))

	_, err := s.Execute(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error from timeout")
	}
}

func TestCommandNotFound(t *testing.T) {
	entry := dependency.ExternalPluginEntry{
		Name:   "missing-cmd",
		Source: "external://nonexistent-binary-xyz-12345",
		Phase:  "render",
		Order:  1,
	}
	s, err := NewExternalProcessStage(entry)
	if err != nil {
		t.Fatalf("NewExternalProcessStage: %v", err)
	}

	_, err = s.Execute(context.Background(), "input")
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "not found")
	}
}

func TestWithArgs(t *testing.T) {
	entry := dependency.ExternalPluginEntry{
		Name:   "with-args",
		Source: "external://something",
		Phase:  "parse",
		Order:  1,
	}
	s, err := NewExternalProcessStage(entry, WithArgs("--verbose", "--flag"))
	if err != nil {
		t.Fatalf("NewExternalProcessStage: %v", err)
	}
	if len(s.args) != 2 || s.args[0] != "--verbose" || s.args[1] != "--flag" {
		t.Errorf("args = %v, want [--verbose --flag]", s.args)
	}
}
