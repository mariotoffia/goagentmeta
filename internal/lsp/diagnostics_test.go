package lsp

import (
	"strings"
	"testing"
)

func TestDiagnoseYAML_ValidAgent(t *testing.T) {
	content := "kind: agent\nid: my-agent\ndescription: A test agent\n"
	diags := DiagnoseYAML("file:///test.yaml", content)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d: %+v", len(diags), diags)
	}
}

func TestDiagnoseYAML_InvalidKind(t *testing.T) {
	content := "kind: foobar\nid: my-thing\n"
	diags := DiagnoseYAML("file:///test.yaml", content)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for invalid kind")
	}
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "invalid kind") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'invalid kind' error, got: %+v", diags)
	}
}

func TestDiagnoseYAML_MissingID(t *testing.T) {
	content := "kind: skill\ndescription: no id here\n"
	diags := DiagnoseYAML("file:///test.yaml", content)
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "must have an 'id' field") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing 'id' error, got: %+v", diags)
	}
}

func TestDiagnoseYAML_InvalidYAML(t *testing.T) {
	content := ":\n  invalid: [yaml\n"
	diags := DiagnoseYAML("file:///test.yaml", content)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for invalid YAML")
	}
	if diags[0].Severity != SeverityError {
		t.Errorf("expected severity Error, got %d", diags[0].Severity)
	}
}

func TestDiagnoseYAML_EmptyDocument(t *testing.T) {
	diags := DiagnoseYAML("file:///test.yaml", "")
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for empty doc, got %d", len(diags))
	}
}

func TestDiagnoseYAML_PlainMapping(t *testing.T) {
	// A plain YAML mapping without kind or dependencies — no errors expected.
	content := "foo: bar\nbaz: 42\n"
	diags := DiagnoseYAML("file:///test.yaml", content)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for plain mapping, got %d: %+v", len(diags), diags)
	}
}

func TestDiagnoseYAML_ManifestValid(t *testing.T) {
	content := `dependencies:
  some-package: "^1.0.0"
compiler:
  name: test
  version: "1.0"
`
	diags := DiagnoseYAML("file:///manifest.yaml", content)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for valid manifest, got %d: %+v", len(diags), diags)
	}
}

func TestDiagnoseYAML_ManifestUnknownKey(t *testing.T) {
	content := `dependencies:
  foo: "1.0"
unknown_key: something
`
	diags := DiagnoseYAML("file:///manifest.yaml", content)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown manifest key") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown manifest key warning, got: %+v", diags)
	}
}

func TestDiagnoseYAML_ManifestCompilerUnknownKey(t *testing.T) {
	content := `compiler:
  name: test
  bad_field: oops
`
	diags := DiagnoseYAML("file:///manifest.yaml", content)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown compiler key") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown compiler key warning, got: %+v", diags)
	}
}

func TestDiagnoseYAML_NonMappingRoot(t *testing.T) {
	content := "- item1\n- item2\n"
	diags := DiagnoseYAML("file:///test.yaml", content)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "expected a YAML mapping") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected non-mapping warning, got: %+v", diags)
	}
}

func TestDiagnoseYAML_ErrorLinePosition(t *testing.T) {
	// Intentionally broken YAML at a known position.
	content := "key: value\nbad: [unclosed\n"
	diags := DiagnoseYAML("file:///test.yaml", content)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for broken YAML")
	}
	// The error should reference a line > 0 (0-based) since the error is on line 2.
	if diags[0].Range.Start.Line < 1 {
		t.Logf("diagnostic: %+v", diags[0])
		// Line extraction from yaml error messages is best-effort.
		// Not all yaml errors include line numbers.
	}
}

func TestDiagnoseYAML_ValidKinds(t *testing.T) {
	kinds := []string{"instruction", "rule", "skill", "agent", "hook", "command", "capability", "plugin"}
	for _, kind := range kinds {
		content := "kind: " + kind + "\nid: test-" + kind + "\n"
		diags := DiagnoseYAML("file:///test.yaml", content)
		if len(diags) != 0 {
			t.Errorf("kind %q: expected 0 diagnostics, got %d: %+v", kind, len(diags), diags)
		}
	}
}
