package registry_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mustTempDir creates a temporary directory that is removed when the test
// completes.
func mustTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "goagentmeta-reg-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// writeTarGz creates a gzip-compressed tar archive containing the given
// files (name → content) and writes it to w.
func writeTarGz(t *testing.T, w io.Writer, files map[string]string) {
	t.Helper()
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

// setupLocalRegistryDir creates a temp directory with two package versions
// for use in local registry tests.
func setupLocalRegistryDir(t *testing.T) string {
	t.Helper()
	root := mustTempDir(t)

	// test-package/1.0.0/
	v1Dir := filepath.Join(root, "test-package", "1.0.0")
	if err := os.MkdirAll(filepath.Join(v1Dir, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "package.yaml"), []byte(
		"name: test-package\nversion: 1.0.0\npublisher: test-publisher\ndescription: v1\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "objects", "test-instruction.yaml"), []byte(
		"kind: instruction\nid: test-instruction\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	// test-package/2.0.0/
	v2Dir := filepath.Join(root, "test-package", "2.0.0")
	if err := os.MkdirAll(filepath.Join(v2Dir, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "package.yaml"), []byte(
		"name: test-package\nversion: 2.0.0\npublisher: test-publisher\ndescription: v2\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v2Dir, "objects", "test-instruction.yaml"), []byte(
		"kind: instruction\nid: test-instruction-v2\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

// setupGitRepoForTest creates a local git repository with two tagged
// versions for use in git registry tests.
func setupGitRepoForTest(t *testing.T) string {
	t.Helper()
	repoDir := mustTempDir(t)

	gitCmd(t, repoDir, "init")
	gitCmd(t, repoDir, "config", "user.email", "test@test.com")
	gitCmd(t, repoDir, "config", "user.name", "Test")

	// v1.0.0
	if err := os.WriteFile(filepath.Join(repoDir, "data.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "package.yaml"),
		[]byte("name: test\nversion: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repoDir, "add", ".")
	gitCmd(t, repoDir, "commit", "-m", "v1.0.0")
	gitCmd(t, repoDir, "tag", "v1.0.0")

	// v2.0.0
	if err := os.WriteFile(filepath.Join(repoDir, "data.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repoDir, "add", ".")
	gitCmd(t, repoDir, "commit", "-m", "v2.0.0")
	gitCmd(t, repoDir, "tag", "v2.0.0")

	return repoDir
}

// gitCmd runs a git command in the given directory.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}
