package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// memFileInfo is a minimal fs.FileInfo implementation for MemFS.
type memFileInfo struct {
	name  string
	size  int64
	isDir bool
	mode  fs.FileMode
}

func (f *memFileInfo) Name() string       { return f.name }
func (f *memFileInfo) Size() int64        { return f.size }
func (f *memFileInfo) Mode() fs.FileMode  { return f.mode }
func (f *memFileInfo) ModTime() time.Time { return time.Time{} }
func (f *memFileInfo) IsDir() bool        { return f.isDir }
func (f *memFileInfo) Sys() any           { return nil }

// memDirEntry is a minimal fs.DirEntry for MemFS directory listings.
type memDirEntry struct {
	name  string
	isDir bool
	mode  fs.FileMode
	size  int64
}

func (d *memDirEntry) Name() string      { return d.name }
func (d *memDirEntry) IsDir() bool       { return d.isDir }
func (d *memDirEntry) Type() fs.FileMode { return d.mode.Type() }
func (d *memDirEntry) Info() (fs.FileInfo, error) {
	return &memFileInfo{
		name:  d.name,
		size:  d.size,
		isDir: d.isDir,
		mode:  d.mode,
	}, nil
}

// MemFS is a thread-safe in-memory filesystem that implements the
// port/filesystem Reader, Writer, and Materializer interfaces.
//
// It is designed for use in tests: no disk I/O is performed, and the full
// in-memory state can be inspected via Files().
type MemFS struct {
	mu       sync.RWMutex
	files    map[string][]byte   // path → content
	dirs     map[string]struct{} // path → exists
	symlinks map[string]string   // newname → oldname
}

// NewMemFS creates an empty MemFS.
func NewMemFS() *MemFS {
	m := &MemFS{}
	m.Reset()
	return m
}

// Reset clears all state — files, directories, and symlinks.
func (m *MemFS) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files = make(map[string][]byte)
	m.dirs = make(map[string]struct{})
	m.symlinks = make(map[string]string)
}

// Files returns a shallow copy of all files in the filesystem.
// Useful for test assertions without exposing internal state.
func (m *MemFS) Files() map[string][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]byte, len(m.files))
	for k, v := range m.files {
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// Dirs returns all directories currently registered in the filesystem.
func (m *MemFS) Dirs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dirs := make([]string, 0, len(m.dirs))
	for d := range m.dirs {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// Symlinks returns all symlinks as a map of newname → oldname.
func (m *MemFS) Symlinks() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.symlinks))
	for k, v := range m.symlinks {
		out[k] = v
	}
	return out
}

// --- port/filesystem.Reader ---

// ReadFile returns the content of the in-memory file at path.
func (m *MemFS) ReadFile(_ context.Context, path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("filesystem: read %q: file not found", path)
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

// ReadDir returns the direct children of the given directory path.
// Only immediate children are returned (non-recursive).
func (m *MemFS) ReadDir(_ context.Context, path string) ([]fs.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Normalize the path to end without slash.
	path = strings.TrimRight(path, "/")

	// Ensure the directory exists.
	if _, ok := m.dirs[path]; !ok {
		// A path that has files directly under it still counts as a valid dir.
		hasChildren := false
		prefix := path + "/"
		for f := range m.files {
			if strings.HasPrefix(f, prefix) {
				hasChildren = true
				break
			}
		}
		if !hasChildren {
			for d := range m.dirs {
				if strings.HasPrefix(d, prefix) {
					hasChildren = true
					break
				}
			}
		}
		if !hasChildren {
			return nil, fmt.Errorf("filesystem: readdir %q: directory not found", path)
		}
	}

	seen := make(map[string]struct{})
	var entries []fs.DirEntry
	prefix := path + "/"

	// Direct file children.
	for f, data := range m.files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rel := strings.TrimPrefix(f, prefix)
		if strings.Contains(rel, "/") {
			continue // skip descendants beyond immediate children
		}
		if _, dup := seen[rel]; dup {
			continue
		}
		seen[rel] = struct{}{}
		entries = append(entries, &memDirEntry{
			name: rel,
			mode: 0o644,
			size: int64(len(data)),
		})
	}

	// Direct directory children.
	for d := range m.dirs {
		if !strings.HasPrefix(d, prefix) {
			continue
		}
		rel := strings.TrimPrefix(d, prefix)
		if strings.Contains(rel, "/") {
			continue
		}
		if _, dup := seen[rel]; dup {
			continue
		}
		seen[rel] = struct{}{}
		entries = append(entries, &memDirEntry{
			name:  rel,
			isDir: true,
			mode:  fs.ModeDir | 0o755,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

// Stat returns FileInfo for the given path.
func (m *MemFS) Stat(_ context.Context, path string) (fs.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if data, ok := m.files[path]; ok {
		return &memFileInfo{
			name:  filepath.Base(path),
			size:  int64(len(data)),
			mode:  0o644,
			isDir: false,
		}, nil
	}
	if _, ok := m.dirs[path]; ok {
		return &memFileInfo{
			name:  filepath.Base(path),
			mode:  fs.ModeDir | 0o755,
			isDir: true,
		}, nil
	}
	return nil, fmt.Errorf("filesystem: stat %q: not found", path)
}

// Glob returns paths matching the given pattern using filepath.Match semantics.
// Only file paths (not directories) are matched.
func (m *MemFS) Glob(_ context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []string
	for f := range m.files {
		ok, err := filepath.Match(pattern, f)
		if err != nil {
			return nil, fmt.Errorf("filesystem: glob %q: %w", pattern, err)
		}
		if ok {
			matches = append(matches, f)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

// --- port/filesystem.Writer ---

// WriteFile writes content to the in-memory file at path.
// Parent directories are implicitly created.
func (m *MemFS) WriteFile(_ context.Context, path string, content []byte, _ fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(content))
	copy(cp, content)
	m.files[path] = cp
	// Register implicit parent directories.
	m.registerParents(path)
	return nil
}

// MkdirAll creates a directory and all parent directories.
func (m *MemFS) MkdirAll(_ context.Context, path string, _ fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerDirAndParents(path)
	return nil
}

// Symlink records a symbolic link from newname pointing to oldname.
// No actual filesystem symlink is created.
func (m *MemFS) Symlink(_ context.Context, oldname, newname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.symlinks[newname] = oldname
	m.registerParents(newname)
	return nil
}

// Remove removes the file or symlink at path. It returns an error if
// the path does not exist.
func (m *MemFS) Remove(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[path]; ok {
		delete(m.files, path)
		return nil
	}
	if _, ok := m.symlinks[path]; ok {
		delete(m.symlinks, path)
		return nil
	}
	if _, ok := m.dirs[path]; ok {
		// Only remove if the directory is empty.
		prefix := path + "/"
		for f := range m.files {
			if strings.HasPrefix(f, prefix) {
				return fmt.Errorf("filesystem: remove %q: directory not empty", path)
			}
		}
		for d := range m.dirs {
			if strings.HasPrefix(d, prefix) {
				return fmt.Errorf("filesystem: remove %q: directory not empty", path)
			}
		}
		delete(m.dirs, path)
		return nil
	}
	return fmt.Errorf("filesystem: remove %q: not found", path)
}

// --- port/filesystem.Materializer ---

// Materialize writes all files described in plan to the in-memory filesystem.
// The key of each entry in plan.Units is used as the output directory.
func (m *MemFS) Materialize(ctx context.Context, plan pipeline.EmissionPlan) (pipeline.MaterializationResult, error) {
	var result pipeline.MaterializationResult

	// Sort unit keys for deterministic output ordering.
	unitKeys := make([]string, 0, len(plan.Units))
	for k := range plan.Units {
		unitKeys = append(unitKeys, k)
	}
	sort.Strings(unitKeys)

	for _, outputDir := range unitKeys {
		unit := plan.Units[outputDir]
		if err := m.MkdirAll(ctx, outputDir, 0o755); err != nil {
			result.Errors = append(result.Errors, pipeline.MaterializationError{
				Path: outputDir,
				Err:  err.Error(),
			})
			continue
		}
		result.CreatedDirs = append(result.CreatedDirs, outputDir)

		for _, dir := range unit.Directories {
			abs := filepath.Join(outputDir, dir)
			if err := m.MkdirAll(ctx, abs, 0o755); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: abs,
					Err:  err.Error(),
				})
				continue
			}
			result.CreatedDirs = append(result.CreatedDirs, abs)
		}

		for _, f := range unit.Files {
			abs := filepath.Join(outputDir, f.Path)
			// Idempotent: skip only if the file already exists with identical content.
			// If ReadFile errors the file is absent; we must write regardless of content length.
			if existing, err := m.ReadFile(ctx, abs); err == nil && bytes.Equal(existing, f.Content) {
				continue
			}
			if err := m.WriteFile(ctx, abs, f.Content, 0o644); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: abs,
					Err:  err.Error(),
				})
				continue
			}
			result.WrittenFiles = append(result.WrittenFiles, abs)
		}

		for _, a := range unit.Assets {
			dest := filepath.Join(outputDir, a.DestPath)
			if err := m.Symlink(ctx, a.SourcePath, dest); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest,
					Err:  err.Error(),
				})
				continue
			}
			result.SymlinkedFiles = append(result.SymlinkedFiles, dest)
		}

		for _, s := range unit.Scripts {
			dest := filepath.Join(outputDir, s.DestPath)
			if err := m.Symlink(ctx, s.SourcePath, dest); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest,
					Err:  err.Error(),
				})
				continue
			}
			result.SymlinkedFiles = append(result.SymlinkedFiles, dest)
		}

		for _, pb := range unit.PluginBundles {
			bundleDir := filepath.Join(outputDir, pb.DestDir)
			if err := m.MkdirAll(ctx, bundleDir, 0o755); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: bundleDir,
					Err:  err.Error(),
				})
				continue
			}
			result.CreatedDirs = append(result.CreatedDirs, bundleDir)

			for _, f := range pb.Files {
				abs := filepath.Join(bundleDir, f.Path)
				// Idempotent: skip only if already present with identical content.
				if existing, err := m.ReadFile(ctx, abs); err == nil && bytes.Equal(existing, f.Content) {
					continue
				}
				if err := m.WriteFile(ctx, abs, f.Content, 0o644); err != nil {
					result.Errors = append(result.Errors, pipeline.MaterializationError{
						Path: abs,
						Err:  err.Error(),
					})
					continue
				}
				result.WrittenFiles = append(result.WrittenFiles, abs)
			}
		}
	}

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("filesystem: materialization completed with %d error(s)", len(result.Errors))
	}
	return result, nil
}

// --- internal helpers ---

// registerParents registers all parent directories of path.
func (m *MemFS) registerParents(path string) {
	dir := filepath.Dir(path)
	for dir != "." && dir != "/" && dir != "" {
		m.dirs[dir] = struct{}{}
		dir = filepath.Dir(dir)
	}
}

// registerDirAndParents registers path and all its parents.
func (m *MemFS) registerDirAndParents(path string) {
	m.dirs[path] = struct{}{}
	m.registerParents(path)
}
