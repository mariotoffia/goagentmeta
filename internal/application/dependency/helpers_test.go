package dependency_test

import (
	"os"
	"path/filepath"
	"testing"
)

func mustTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "goagentmeta-dep-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
