package dependency

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// LockFile records exact versions resolved during a previous build. On
// subsequent builds the resolver reads the lock file to skip re-resolution
// when the locked version still satisfies the declared constraint.
type LockFile struct {
	// SchemaVersion is the lock file format version.
	SchemaVersion int `json:"schemaVersion"`
	// Dependencies holds all locked dependency entries.
	Dependencies []LockEntry `json:"dependencies"`
}

// LockEntry is a single locked dependency.
type LockEntry struct {
	// Name is the fully qualified package name.
	Name string `json:"name"`
	// Version is the exact resolved version.
	Version string `json:"version"`
	// Registry is the name of the registry that provided this version.
	Registry string `json:"registry"`
	// Digest is the integrity hash (e.g., "sha256:a1b2c3...").
	Digest string `json:"digest"`
	// ResolvedAt is when this entry was resolved.
	ResolvedAt time.Time `json:"resolvedAt"`
}

// LockFilePath returns the canonical lock file path for a given root directory.
func LockFilePath(rootDir string) string {
	return filepath.Join(rootDir, ".ai-build", "manifest.lock.json")
}

// ReadLockFile reads and parses the lock file from the given path.
// Returns an empty LockFile (not an error) if the file does not exist.
func ReadLockFile(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LockFile{SchemaVersion: 1}, nil
		}
		return nil, fmt.Errorf("dependency: read lock file %q: %w", path, err)
	}

	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("dependency: parse lock file %q: %w", path, err)
	}

	return &lf, nil
}

// WriteLockFile writes the lock file to the given path with deterministic
// serialization (sorted keys and dependencies).
func WriteLockFile(path string, lf *LockFile) error {
	// Sort dependencies by name for deterministic output.
	sorted := make([]LockEntry, len(lf.Dependencies))
	copy(sorted, lf.Dependencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	output := &LockFile{
		SchemaVersion: lf.SchemaVersion,
		Dependencies:  sorted,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("dependency: serialize lock file: %w", err)
	}

	// Ensure the parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("dependency: create lock file dir: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("dependency: write lock file %q: %w", path, err)
	}

	return nil
}

// FindEntry returns the lock entry for a given package name, or nil if not found.
func (lf *LockFile) FindEntry(name string) *LockEntry {
	for i := range lf.Dependencies {
		if lf.Dependencies[i].Name == name {
			return &lf.Dependencies[i]
		}
	}
	return nil
}

// SetEntry adds or updates a lock entry for the given package.
func (lf *LockFile) SetEntry(entry LockEntry) {
	for i := range lf.Dependencies {
		if lf.Dependencies[i].Name == entry.Name {
			lf.Dependencies[i] = entry
			return
		}
	}
	lf.Dependencies = append(lf.Dependencies, entry)
}
