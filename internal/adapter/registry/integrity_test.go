package registry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

func mustSetupFixtureDir(t *testing.T) string {
	t.Helper()
	dir := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "file2.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSHA256Verifier_CorrectDigest(t *testing.T) {
	dir := mustSetupFixtureDir(t)
	hash, err := registry.ComputeIntegrityHash(dir)
	if err != nil {
		t.Fatal(err)
	}

	verifier := registry.NewSHA256Verifier()
	pkg := portregistry.ResolvedPackage{
		Name:          "test-pkg",
		IntegrityHash: hash,
	}
	contents := portregistry.PackageContents{
		Package: pkg,
		RootDir: dir,
	}

	if err := verifier.Verify(pkg, contents); err != nil {
		t.Errorf("Verify with correct digest: unexpected error: %v", err)
	}
}

func TestSHA256Verifier_WrongDigest(t *testing.T) {
	dir := mustSetupFixtureDir(t)
	verifier := registry.NewSHA256Verifier()
	pkg := portregistry.ResolvedPackage{
		Name:          "test-pkg",
		IntegrityHash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	contents := portregistry.PackageContents{
		Package: pkg,
		RootDir: dir,
	}

	err := verifier.Verify(pkg, contents)
	if err == nil {
		t.Fatal("Verify with wrong digest: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "expected=") || !strings.Contains(err.Error(), "actual=") {
		t.Errorf("error should contain expected and actual hashes, got: %v", err)
	}
}

func TestSHA256Verifier_EmptyHash(t *testing.T) {
	verifier := registry.NewSHA256Verifier()
	pkg := portregistry.ResolvedPackage{
		Name:          "test-pkg",
		IntegrityHash: "",
	}

	if err := verifier.Verify(pkg, portregistry.PackageContents{}); err != nil {
		t.Errorf("Verify with empty hash: unexpected error: %v", err)
	}
}

func TestComputeIntegrityHash_Deterministic(t *testing.T) {
	dir := mustSetupFixtureDir(t)

	hash1, err := registry.ComputeIntegrityHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := registry.ComputeIntegrityHash(dir)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 != hash2 {
		t.Errorf("hash not deterministic: %s != %s", hash1, hash2)
	}
	if !strings.HasPrefix(hash1, "sha256:") {
		t.Errorf("hash should start with sha256:, got %s", hash1)
	}
}

func TestComputeIntegrityHash_DifferentContent(t *testing.T) {
	dir1 := mustSetupFixtureDir(t)
	dir2 := mustTempDir(t)
	if err := os.WriteFile(filepath.Join(dir2, "different.txt"), []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1, err := registry.ComputeIntegrityHash(dir1)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := registry.ComputeIntegrityHash(dir2)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("different content should produce different hashes")
	}
}

func TestComputeIntegrityHash_EmptyDir(t *testing.T) {
	dir := mustTempDir(t)
	hash, err := registry.ComputeIntegrityHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("hash should start with sha256:, got %s", hash)
	}
}

func TestComputeIntegrityHash_NonexistentDir(t *testing.T) {
	_, err := registry.ComputeIntegrityHash("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent dir, got nil")
	}
}

// Compile-time interface check.
var _ portregistry.IntegrityVerifier = (*registry.SHA256Verifier)(nil)
