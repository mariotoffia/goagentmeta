package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/parser"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

func TestParser_MarkdownFrontmatter(t *testing.T) {
	dir := t.TempDir()
	aiDir := filepath.Join(dir, ".ai")
	setupDir(t, aiDir, "skills")

	writeFile(t, filepath.Join(aiDir, "manifest.yaml"), `schemaVersion: 1`)
	writeFile(t, filepath.Join(aiDir, "skills", "my-skill.md"), `---
id: my-skill
kind: skill
description: A test skill
allowedTools:
  - Read
  - "Bash(go:*)"
---

This is the skill body with **markdown** content.

It can have multiple paragraphs.
`)

	stage := parser.New()
	result, err := stage.Execute(nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	tree := assertSourceTree(t, result)
	if len(tree.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(tree.Objects))
	}

	obj := tree.Objects[0]
	assertEqual(t, "ID", obj.Meta.ID, "my-skill")
	assertEqual(t, "Kind", string(obj.Meta.Kind), "skill")
	assertEqual(t, "Description", obj.Meta.Description, "A test skill")

	if obj.RawContent == "" {
		t.Fatal("expected non-empty RawContent (markdown body)")
	}
	if !contains(obj.RawContent, "**markdown** content") {
		t.Errorf("RawContent missing markdown: %q", obj.RawContent)
	}

	// Check raw fields
	tools, ok := obj.RawFields["allowedTools"].([]any)
	if !ok {
		t.Fatal("expected allowedTools in RawFields")
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 allowedTools, got %d", len(tools))
	}
}

func TestParser_YAMLFile(t *testing.T) {
	dir := t.TempDir()
	aiDir := filepath.Join(dir, ".ai")
	setupDir(t, aiDir, "hooks")

	writeFile(t, filepath.Join(aiDir, "manifest.yaml"), `schemaVersion: 1`)
	writeFile(t, filepath.Join(aiDir, "hooks", "post-edit.yaml"), `
id: post-edit-lint
kind: hook
description: Run linter after edits
event: post-edit
action:
  type: script
  ref: scripts/hooks/lint.sh
`)

	stage := parser.New()
	result, err := stage.Execute(nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	tree := assertSourceTree(t, result)
	if len(tree.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(tree.Objects))
	}

	obj := tree.Objects[0]
	assertEqual(t, "ID", obj.Meta.ID, "post-edit-lint")
	assertEqual(t, "Kind", string(obj.Meta.Kind), "hook")

	action, ok := obj.RawFields["action"].(map[string]any)
	if !ok {
		t.Fatal("expected action in RawFields")
	}
	if action["type"] != "script" {
		t.Errorf("expected action.type=script, got %v", action["type"])
	}
}

func TestParser_MixedFormats(t *testing.T) {
	dir := t.TempDir()
	aiDir := filepath.Join(dir, ".ai")
	setupDir(t, aiDir, "instructions", "agents")

	writeFile(t, filepath.Join(aiDir, "manifest.yaml"), `schemaVersion: 1`)

	writeFile(t, filepath.Join(aiDir, "instructions", "code-style.md"), `---
id: code-style
kind: instruction
description: Code style guide
---

Always use gofmt. Prefer short variable names.
`)

	writeFile(t, filepath.Join(aiDir, "agents", "reviewer.md"), `---
id: reviewer
kind: agent
description: Code reviewer agent
toolPolicy:
  filesystem.read: allow
  terminal.exec: deny
---

You are a meticulous code reviewer. Focus on correctness and security.
`)

	stage := parser.New()
	result, err := stage.Execute(nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	tree := assertSourceTree(t, result)
	if len(tree.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(tree.Objects))
	}

	// Objects are sorted by ID
	assertEqual(t, "first ID", tree.Objects[0].Meta.ID, "code-style")
	assertEqual(t, "second ID", tree.Objects[1].Meta.ID, "reviewer")

	if tree.Objects[0].RawContent == "" {
		t.Error("instruction should have body content")
	}
	if tree.Objects[1].RawContent == "" {
		t.Error("agent should have body content (rolePrompt)")
	}
}

func TestParser_AgentFromMarkdown(t *testing.T) {
	dir := t.TempDir()
	aiDir := filepath.Join(dir, ".ai")
	setupDir(t, aiDir, "agents")

	writeFile(t, filepath.Join(aiDir, "manifest.yaml"), `schemaVersion: 1`)
	writeFile(t, filepath.Join(aiDir, "agents", "implementer.md"), `---
id: implementer
kind: agent
description: Go implementation agent
skills:
  - go-testing
  - go-perf
delegation:
  mayCall:
    - reviewer
---

You are a senior Go developer. Implement features following clean architecture.
`)

	stage := parser.New()
	result, err := stage.Execute(nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	tree := assertSourceTree(t, result)
	obj := tree.Objects[0]
	assertEqual(t, "ID", obj.Meta.ID, "implementer")
	assertEqual(t, "Kind", string(obj.Meta.Kind), "agent")

	if !contains(obj.RawContent, "senior Go developer") {
		t.Errorf("agent body should contain rolePrompt text: %q", obj.RawContent)
	}

	// Skills should be in RawFields, not body
	skills, ok := obj.RawFields["skills"].([]any)
	if !ok {
		t.Fatal("expected skills in RawFields")
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestParser_NoFrontmatter_Error(t *testing.T) {
	dir := t.TempDir()
	aiDir := filepath.Join(dir, ".ai")
	setupDir(t, aiDir, "instructions")

	writeFile(t, filepath.Join(aiDir, "manifest.yaml"), `schemaVersion: 1`)
	writeFile(t, filepath.Join(aiDir, "instructions", "bad.md"), `
This file has no frontmatter at all.
Just plain markdown.
`)

	stage := parser.New()
	_, err := stage.Execute(nil, dir)
	if err == nil {
		t.Fatal("expected error for .md file without frontmatter")
	}
}

func TestParser_EmptySourceDir(t *testing.T) {
	dir := t.TempDir()
	aiDir := filepath.Join(dir, ".ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(aiDir, "manifest.yaml"), `schemaVersion: 1`)

	stage := parser.New()
	result, err := stage.Execute(nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	tree := assertSourceTree(t, result)
	if len(tree.Objects) != 0 {
		t.Errorf("expected 0 objects, got %d", len(tree.Objects))
	}
}

func TestParser_CustomSourceDir(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, ".custom-ai")
	setupDir(t, customDir, "instructions")

	writeFile(t, filepath.Join(customDir, "manifest.yaml"), `schemaVersion: 2`)
	writeFile(t, filepath.Join(customDir, "instructions", "hello.md"), `---
id: hello
kind: instruction
---

Hello world.
`)

	stage := parser.New(parser.WithSourceDir(".custom-ai"))
	result, err := stage.Execute(nil, dir)
	if err != nil {
		t.Fatal(err)
	}

	tree := assertSourceTree(t, result)
	if tree.SchemaVersion != 2 {
		t.Errorf("expected schema version 2, got %d", tree.SchemaVersion)
	}
	if len(tree.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(tree.Objects))
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func assertSourceTree(t *testing.T, result any) pipeline.SourceTree {
	t.Helper()
	tree, ok := result.(pipeline.SourceTree)
	if !ok {
		t.Fatalf("expected pipeline.SourceTree, got %T", result)
	}
	return tree
}

func assertEqual(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", label, got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func setupDir(t *testing.T, base string, subdirs ...string) {
	t.Helper()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(base, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
