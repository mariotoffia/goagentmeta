package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portstage "github.com/mariotoffia/goagentmeta/internal/port/stage"
)

// Compile-time check that ExternalProcessStage satisfies the Stage interface.
var _ portstage.Stage = (*ExternalProcessStage)(nil)

const defaultTimeout = 30 * time.Second

// ExternalProcessStage adapts an external subprocess into a pipeline Stage.
// Communication uses a JSON protocol over stdin/stdout. Each Execute call
// spawns a fresh process.
type ExternalProcessStage struct {
	entry      dependency.ExternalPluginEntry
	command    string
	args       []string
	timeout    time.Duration
	descriptor pipeline.StageDescriptor
	described  bool

	// lookPathFn is overridable for testing (defaults to exec.LookPath).
	lookPathFn func(string) (string, error)
	// cmdFactory builds an *exec.Cmd; overridable for testing.
	cmdFactory func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// ExternalOption configures an ExternalProcessStage.
type ExternalOption func(*ExternalProcessStage)

// WithTimeout sets the per-call timeout for subprocess communication.
func WithTimeout(d time.Duration) ExternalOption {
	return func(s *ExternalProcessStage) { s.timeout = d }
}

// WithArgs appends extra arguments passed to the subprocess command.
func WithArgs(args ...string) ExternalOption {
	return func(s *ExternalProcessStage) { s.args = append(s.args, args...) }
}

// NewExternalProcessStage creates a new stage adapter from a manifest entry.
// The command is resolved from the source field by stripping the "external://"
// prefix. Call Describe to perform the handshake before Execute.
func NewExternalProcessStage(entry dependency.ExternalPluginEntry, opts ...ExternalOption) (*ExternalProcessStage, error) {
	const prefix = "external://"
	if !strings.HasPrefix(entry.Source, prefix) {
		return nil, fmt.Errorf("plugin %q: source must start with %q, got %q", entry.Name, prefix, entry.Source)
	}

	cmd := strings.TrimPrefix(entry.Source, prefix)
	if cmd == "" {
		return nil, fmt.Errorf("plugin %q: source has empty command after %q prefix", entry.Name, prefix)
	}

	s := &ExternalProcessStage{
		entry:      entry,
		command:    cmd,
		timeout:    defaultTimeout,
		lookPathFn: exec.LookPath,
		cmdFactory: exec.CommandContext,
	}
	for _, opt := range opts {
		opt(s)
	}

	// Build a descriptor from manifest data so Descriptor() works even
	// before a handshake. Describe() can refine it later.
	phase, ok := pipeline.ParsePhase(entry.Phase)
	if !ok {
		return nil, fmt.Errorf("plugin %q: invalid phase %q", entry.Name, entry.Phase)
	}

	s.descriptor = pipeline.StageDescriptor{
		Name:  entry.Name,
		Phase: phase,
		Order: entry.Order,
	}
	if entry.Target != "" {
		s.descriptor.TargetFilter = []build.Target{build.Target(entry.Target)}
	}

	return s, nil
}

// Describe performs the handshake: starts the subprocess, sends a describe
// request, reads the response, and updates the stage descriptor.
func (s *ExternalProcessStage) Describe(ctx context.Context) error {
	resp, stderr, err := s.roundTrip(ctx, DescribeRequest{Type: "describe"})
	if err != nil {
		return fmt.Errorf("plugin %q describe: %w (stderr: %s)", s.entry.Name, err, stderr)
	}

	var dr DescribeResponse
	if err := json.Unmarshal(resp, &dr); err != nil {
		return fmt.Errorf("plugin %q describe: decode response: %w", s.entry.Name, err)
	}

	phase, ok := pipeline.ParsePhase(dr.Phase)
	if !ok {
		return fmt.Errorf("plugin %q describe: invalid phase %q", s.entry.Name, dr.Phase)
	}

	s.descriptor = pipeline.StageDescriptor{
		Name:  dr.Name,
		Phase: phase,
		Order: dr.Order,
	}
	for _, t := range dr.TargetFilter {
		s.descriptor.TargetFilter = append(s.descriptor.TargetFilter, build.Target(t))
	}

	s.described = true
	return nil
}

// Descriptor returns the stage descriptor populated from the manifest entry
// or from the handshake response if Describe has been called.
func (s *ExternalProcessStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Execute sends the IR input to the subprocess and returns its output.
// The input is JSON-marshalled and sent as an ExecuteRequest. The response
// is returned as json.RawMessage so callers can unmarshal into the
// appropriate IR type.
func (s *ExternalProcessStage) Execute(ctx context.Context, input any) (any, error) {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("plugin %q execute: marshal input: %w", s.entry.Name, err)
	}

	req := ExecuteRequest{
		Type:  "execute",
		Input: json.RawMessage(inputBytes),
	}

	resp, stderr, err := s.roundTrip(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("plugin %q execute: %w (stderr: %s)", s.entry.Name, err, stderr)
	}

	var er ExecuteResponse
	if err := json.Unmarshal(resp, &er); err != nil {
		return nil, fmt.Errorf("plugin %q execute: decode response: %w", s.entry.Name, err)
	}

	if er.Error != "" {
		msg := fmt.Sprintf("plugin %q execute: process error: %s", s.entry.Name, er.Error)
		if len(er.Diagnostics) > 0 {
			var diags []string
			for _, d := range er.Diagnostics {
				diags = append(diags, fmt.Sprintf("[%s] %s", d.Severity, d.Message))
			}
			msg += "; diagnostics: " + strings.Join(diags, ", ")
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return er.Output, nil
}

// roundTrip starts the subprocess, writes a JSON request to stdin, reads
// one JSON object from stdout, and returns the raw bytes plus any stderr.
func (s *ExternalProcessStage) roundTrip(ctx context.Context, req any) ([]byte, string, error) {
	cmdPath, err := s.lookPathFn(s.command)
	if err != nil {
		return nil, "", fmt.Errorf("command %q not found: %w", s.command, err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := s.cmdFactory(ctx, cmdPath, s.args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, "", fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("stdout pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start process: %w", err)
	}

	// Write request and close stdin to signal EOF.
	enc := json.NewEncoder(stdin)
	if err := enc.Encode(req); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return nil, stderrBuf.String(), fmt.Errorf("write request: %w", err)
	}
	_ = stdin.Close()

	// Read exactly one JSON object from stdout.
	var raw json.RawMessage
	dec := json.NewDecoder(stdout)
	if err := dec.Decode(&raw); err != nil {
		_ = cmd.Wait()
		return nil, stderrBuf.String(), fmt.Errorf("read response: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, stderrBuf.String(), fmt.Errorf("process exited with error: %w", err)
	}

	return raw, stderrBuf.String(), nil
}
