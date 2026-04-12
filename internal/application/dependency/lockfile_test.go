package dependency_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/application/dependency"
)

func TestLockFile_ReadWrite_RoundTrip(t *testing.T) {
	dir := mustTempDir(t)
	lockPath := filepath.Join(dir, ".ai-build", "manifest.lock.json")

	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	original := &dependency.LockFile{
		SchemaVersion: 1,
		Dependencies: []dependency.LockEntry{
			{
				Name:       "z-package",
				Version:    "2.0.0",
				Registry:   "http",
				Digest:     "sha256:abcdef",
				ResolvedAt: now,
			},
			{
				Name:       "a-package",
				Version:    "1.0.0",
				Registry:   "local",
				Digest:     "sha256:123456",
				ResolvedAt: now,
			},
		},
	}

	if err := dependency.WriteLockFile(lockPath, original); err != nil {
		t.Fatalf("WriteLockFile: %v", err)
	}

	loaded, err := dependency.ReadLockFile(lockPath)
	if err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}

	if loaded.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", loaded.SchemaVersion)
	}
	if len(loaded.Dependencies) != 2 {
		t.Fatalf("Dependencies len = %d, want 2", len(loaded.Dependencies))
	}

	// Verify sorted order (a-package before z-package).
	if loaded.Dependencies[0].Name != "a-package" {
		t.Errorf("Dependencies[0].Name = %q, want %q", loaded.Dependencies[0].Name, "a-package")
	}
	if loaded.Dependencies[1].Name != "z-package" {
		t.Errorf("Dependencies[1].Name = %q, want %q", loaded.Dependencies[1].Name, "z-package")
	}
}

func TestLockFile_DeterministicOutput(t *testing.T) {
	dir := mustTempDir(t)
	lockPath := filepath.Join(dir, ".ai-build", "manifest.lock.json")

	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	lf := &dependency.LockFile{
		SchemaVersion: 1,
		Dependencies: []dependency.LockEntry{
			{Name: "c-pkg", Version: "3.0.0", Registry: "local", Digest: "sha256:ccc", ResolvedAt: now},
			{Name: "a-pkg", Version: "1.0.0", Registry: "http", Digest: "sha256:aaa", ResolvedAt: now},
			{Name: "b-pkg", Version: "2.0.0", Registry: "git", Digest: "sha256:bbb", ResolvedAt: now},
		},
	}

	// Write twice and compare.
	if err := dependency.WriteLockFile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockFile 1: %v", err)
	}
	data1, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := dependency.WriteLockFile(lockPath, lf); err != nil {
		t.Fatalf("WriteLockFile 2: %v", err)
	}
	data2, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data1) != string(data2) {
		t.Error("lock file output is not deterministic")
	}

	// Verify it's valid JSON.
	var parsed dependency.LockFile
	if err := json.Unmarshal(data1, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify sorted order.
	if parsed.Dependencies[0].Name != "a-pkg" {
		t.Errorf("first entry = %q, want a-pkg", parsed.Dependencies[0].Name)
	}
}

func TestLockFile_MissingFile_ReturnsEmpty(t *testing.T) {
	lf, err := dependency.ReadLockFile("/nonexistent/manifest.lock.json")
	if err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}

	if lf.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", lf.SchemaVersion)
	}
	if len(lf.Dependencies) != 0 {
		t.Errorf("Dependencies len = %d, want 0", len(lf.Dependencies))
	}
}

func TestLockFile_FindEntry(t *testing.T) {
	now := time.Now()
	lf := &dependency.LockFile{
		SchemaVersion: 1,
		Dependencies: []dependency.LockEntry{
			{Name: "pkg-a", Version: "1.0.0", Digest: "sha256:aaa", ResolvedAt: now},
			{Name: "pkg-b", Version: "2.0.0", Digest: "sha256:bbb", ResolvedAt: now},
		},
	}

	entry := lf.FindEntry("pkg-a")
	if entry == nil {
		t.Fatal("FindEntry returned nil for pkg-a")
	}
	if entry.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", entry.Version)
	}

	if lf.FindEntry("nonexistent") != nil {
		t.Error("FindEntry should return nil for nonexistent package")
	}
}

func TestLockFile_SetEntry_Update(t *testing.T) {
	now := time.Now()
	lf := &dependency.LockFile{
		SchemaVersion: 1,
		Dependencies: []dependency.LockEntry{
			{Name: "pkg-a", Version: "1.0.0", Digest: "sha256:old", ResolvedAt: now},
		},
	}

	lf.SetEntry(dependency.LockEntry{
		Name:       "pkg-a",
		Version:    "1.1.0",
		Digest:     "sha256:new",
		ResolvedAt: now,
	})

	if len(lf.Dependencies) != 1 {
		t.Fatalf("Dependencies len = %d, want 1", len(lf.Dependencies))
	}
	if lf.Dependencies[0].Version != "1.1.0" {
		t.Errorf("Version = %q, want 1.1.0", lf.Dependencies[0].Version)
	}
}

func TestLockFile_SetEntry_Add(t *testing.T) {
	lf := &dependency.LockFile{SchemaVersion: 1}

	lf.SetEntry(dependency.LockEntry{
		Name:    "new-pkg",
		Version: "1.0.0",
		Digest:  "sha256:new",
	})

	if len(lf.Dependencies) != 1 {
		t.Fatalf("Dependencies len = %d, want 1", len(lf.Dependencies))
	}
	if lf.Dependencies[0].Name != "new-pkg" {
		t.Errorf("Name = %q, want new-pkg", lf.Dependencies[0].Name)
	}
}

func TestLockFilePath(t *testing.T) {
	path := dependency.LockFilePath("/project/root")
	want := filepath.Join("/project/root", ".ai-build", "manifest.lock.json")
	if path != want {
		t.Errorf("LockFilePath = %q, want %q", path, want)
	}
}
