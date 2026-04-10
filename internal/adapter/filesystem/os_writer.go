package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// OSWriter is an os-backed implementation of port/filesystem.Writer.
type OSWriter struct{}

// NewOSWriter creates a new OSWriter.
func NewOSWriter() *OSWriter {
	return &OSWriter{}
}

// WriteFile writes content to path, creating parent directories as needed.
// If the file already exists it is overwritten.
func (w *OSWriter) WriteFile(_ context.Context, path string, content []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("filesystem: makedirs for %q: %w", path, err)
	}
	if err := os.WriteFile(path, content, perm); err != nil {
		return fmt.Errorf("filesystem: write %q: %w", path, err)
	}
	return nil
}

// MkdirAll creates path and all parent directories with the given permissions.
func (w *OSWriter) MkdirAll(_ context.Context, path string, perm fs.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("filesystem: mkdirall %q: %w", path, err)
	}
	return nil
}

// Symlink creates a symbolic link newname pointing to oldname.
// On Windows, if symlinks are not supported, the error is wrapped with a
// clear message instead of a raw os error.
func (w *OSWriter) Symlink(_ context.Context, oldname, newname string) error {
	err := os.Symlink(oldname, newname)
	if err == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return fmt.Errorf("filesystem: symlink not supported on Windows (%q -> %q): %w", newname, oldname, err)
	}
	return fmt.Errorf("filesystem: symlink %q -> %q: %w", newname, oldname, err)
}

// Remove removes the file or empty directory at path.
func (w *OSWriter) Remove(_ context.Context, path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("filesystem: remove %q: %w", path, err)
	}
	return nil
}
