// Package plugin implements external process stage adapters that communicate
// with subprocess plugins over stdin/stdout using a JSON protocol.
package plugin

import "encoding/json"

// DescribeRequest is sent to the external process to request its stage metadata.
type DescribeRequest struct {
	Type string `json:"type"` // always "describe"
}

// ExecuteRequest is sent to the external process with an IR payload to transform.
type ExecuteRequest struct {
	Type  string          `json:"type"`  // always "execute"
	Input json.RawMessage `json:"input"` // JSON-serialized pipeline IR
}

// DescribeResponse is the external process's reply to a describe request.
type DescribeResponse struct {
	Name         string   `json:"name"`
	Phase        string   `json:"phase"`
	Order        int      `json:"order"`
	TargetFilter []string `json:"targetFilter,omitempty"`
}

// ExecuteResponse is the external process's reply to an execute request.
type ExecuteResponse struct {
	Output      json.RawMessage   `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	Diagnostics []DiagnosticMessage `json:"diagnostics,omitempty"`
}

// DiagnosticMessage is a structured diagnostic emitted by the external process.
type DiagnosticMessage struct {
	Severity string `json:"severity"`          // "error", "warning", "info"
	Message  string `json:"message"`
	ObjectID string `json:"objectId,omitempty"`
}
