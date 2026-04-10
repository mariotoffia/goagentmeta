// Package filesystem provides os-backed and in-memory implementations of the
// port/filesystem interfaces.
package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// OSReader is an os-backed implementation of port/filesystem.Reader.
// All errors include the path for debugging context.
type OSReader struct{}

// NewOSReader creates a new OSReader.
func NewOSReader() *OSReader {
	return &OSReader{}
}

// ReadFile reads the contents of the file at path.
func (r *OSReader) ReadFile(_ context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: read %q: %w", path, err)
	}
	return data, nil
}

// ReadDir returns the directory entries for the given path.
func (r *OSReader) ReadDir(_ context.Context, path string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: readdir %q: %w", path, err)
	}
	return entries, nil
}

// Stat returns FileInfo for the given path.
func (r *OSReader) Stat(_ context.Context, path string) (fs.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: stat %q: %w", path, err)
	}
	return info, nil
}

// Glob returns paths matching the given pattern using filepath.Glob.
func (r *OSReader) Glob(_ context.Context, pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("filesystem: glob %q: %w", pattern, err)
	}
	return matches, nil
}
