// Package filesystem defines port interfaces for filesystem I/O.
// These ports decouple the domain and application layers from the os package,
// enabling testing with in-memory filesystems and ensuring the domain has no
// infrastructure dependencies.
package filesystem

import (
	"context"
	"io/fs"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// Reader provides read-only filesystem access. The compiler's parse phase
// and other stages use this port to read source files and discover the
// .ai/ directory structure.
type Reader interface {
	// ReadFile reads the contents of the file at the given path.
	ReadFile(ctx context.Context, path string) ([]byte, error)

	// ReadDir returns the directory entries at the given path.
	ReadDir(ctx context.Context, path string) ([]fs.DirEntry, error)

	// Stat returns file info for the given path.
	Stat(ctx context.Context, path string) (fs.FileInfo, error)

	// Glob returns paths matching the given pattern.
	Glob(ctx context.Context, pattern string) ([]string, error)
}

// Writer provides write filesystem access. The materialize phase uses
// this port to write compiled output to the .ai-build/ directory.
type Writer interface {
	// WriteFile writes content to the file at the given path.
	// It creates parent directories as needed. If the file exists,
	// it is overwritten.
	WriteFile(ctx context.Context, path string, content []byte, perm fs.FileMode) error

	// MkdirAll creates a directory and all parent directories.
	MkdirAll(ctx context.Context, path string, perm fs.FileMode) error

	// Symlink creates a symbolic link from newname pointing to oldname.
	Symlink(ctx context.Context, oldname, newname string) error

	// Remove removes the file or empty directory at the given path.
	Remove(ctx context.Context, path string) error
}

// Materializer writes an EmissionPlan to disk. This is a higher-level port
// that combines Reader and Writer operations to materialize the full build
// output for a build unit.
type Materializer interface {
	// Materialize writes all files, directories, symlinks, and plugin bundles
	// described in the emission plan. It returns a result recording what was
	// written and any errors encountered.
	Materialize(ctx context.Context, plan pipeline.EmissionPlan) (pipeline.MaterializationResult, error)
}
