package plugin

import (
	"encoding/json"
	"testing"
)

func TestDescribeRequest_RoundTrip(t *testing.T) {
	req := DescribeRequest{Type: "describe"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got DescribeRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != "describe" {
		t.Errorf("Type = %q, want %q", got.Type, "describe")
	}
}

func TestDescribeResponse_RoundTrip(t *testing.T) {
	resp := DescribeResponse{
		Name:         "my-plugin",
		Phase:        "render",
		Order:        5,
		TargetFilter: []string{"claude", "cursor"},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got DescribeResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != resp.Name {
		t.Errorf("Name = %q, want %q", got.Name, resp.Name)
	}
	if got.Phase != resp.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, resp.Phase)
	}
	if got.Order != resp.Order {
		t.Errorf("Order = %d, want %d", got.Order, resp.Order)
	}
	if len(got.TargetFilter) != 2 || got.TargetFilter[0] != "claude" || got.TargetFilter[1] != "cursor" {
		t.Errorf("TargetFilter = %v, want [claude cursor]", got.TargetFilter)
	}
}

func TestExecuteRequest_RoundTrip(t *testing.T) {
	input := json.RawMessage(`{"key":"value"}`)
	req := ExecuteRequest{Type: "execute", Input: input}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ExecuteRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != "execute" {
		t.Errorf("Type = %q, want %q", got.Type, "execute")
	}
	if string(got.Input) != `{"key":"value"}` {
		t.Errorf("Input = %s, want %s", got.Input, `{"key":"value"}`)
	}
}

func TestExecuteResponse_Success_RoundTrip(t *testing.T) {
	resp := ExecuteResponse{
		Output: json.RawMessage(`{"result":42}`),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ExecuteResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(got.Output) != `{"result":42}` {
		t.Errorf("Output = %s, want %s", got.Output, `{"result":42}`)
	}
	if got.Error != "" {
		t.Errorf("Error = %q, want empty", got.Error)
	}
}

func TestExecuteResponse_Error_RoundTrip(t *testing.T) {
	resp := ExecuteResponse{
		Error: "something went wrong",
		Diagnostics: []DiagnosticMessage{
			{Severity: "error", Message: "bad input", ObjectID: "obj-1"},
			{Severity: "warning", Message: "deprecated field"},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ExecuteResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error != "something went wrong" {
		t.Errorf("Error = %q, want %q", got.Error, "something went wrong")
	}
	if len(got.Diagnostics) != 2 {
		t.Fatalf("Diagnostics len = %d, want 2", len(got.Diagnostics))
	}
	if got.Diagnostics[0].Severity != "error" {
		t.Errorf("Diagnostics[0].Severity = %q, want %q", got.Diagnostics[0].Severity, "error")
	}
	if got.Diagnostics[0].ObjectID != "obj-1" {
		t.Errorf("Diagnostics[0].ObjectID = %q, want %q", got.Diagnostics[0].ObjectID, "obj-1")
	}
}

func TestDescribeResponse_OmitsEmptyTargetFilter(t *testing.T) {
	resp := DescribeResponse{Name: "p", Phase: "parse", Order: 0}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	m := make(map[string]any)
	_ = json.Unmarshal(data, &m)
	if _, ok := m["targetFilter"]; ok {
		t.Error("expected targetFilter to be omitted when empty")
	}
}

func TestExecuteResponse_OmitsEmptyFields(t *testing.T) {
	resp := ExecuteResponse{}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	m := make(map[string]any)
	_ = json.Unmarshal(data, &m)
	if _, ok := m["output"]; ok {
		t.Error("expected output to be omitted when nil")
	}
	if _, ok := m["diagnostics"]; ok {
		t.Error("expected diagnostics to be omitted when nil")
	}
}
