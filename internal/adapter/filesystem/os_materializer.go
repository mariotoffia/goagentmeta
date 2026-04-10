package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// OSMaterializer is an os-backed implementation of port/filesystem.Materializer.
// It writes an EmissionPlan to disk, creating directories, writing files, and
// copying or symlinking assets and scripts.
//
// Materialization is idempotent: files with identical content are not
// rewritten (preserving modification times for build tools).
type OSMaterializer struct {
	writer *OSWriter
	reader *OSReader
}

// NewOSMaterializer creates a new OSMaterializer.
func NewOSMaterializer() *OSMaterializer {
	return &OSMaterializer{
		writer: NewOSWriter(),
		reader: NewOSReader(),
	}
}

// Materialize writes all files described in plan to disk.
// The plan's UnitEmission.Coordinate is used to derive the output root path
// from EmissionPlan's perspective; callers should set EmittedFile.Path
// relative to the intended output directory.
//
// Each UnitEmission key in plan.Units is the output directory for that unit.
func (m *OSMaterializer) Materialize(ctx context.Context, plan pipeline.EmissionPlan) (pipeline.MaterializationResult, error) {
	var result pipeline.MaterializationResult

	for outputDir, unit := range plan.Units {
		// Ensure the unit output directory exists.
		if err := m.writer.MkdirAll(ctx, outputDir, 0o755); err != nil {
			result.Errors = append(result.Errors, pipeline.MaterializationError{
				Path: outputDir,
				Err:  err.Error(),
			})
			continue
		}
		result.CreatedDirs = append(result.CreatedDirs, outputDir)

		// Create explicit sub-directories.
		for _, dir := range unit.Directories {
			abs := filepath.Join(outputDir, dir)
			if err := m.writer.MkdirAll(ctx, abs, 0o755); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: abs,
					Err:  err.Error(),
				})
				continue
			}
			result.CreatedDirs = append(result.CreatedDirs, abs)
		}

		// Write emitted files (idempotent).
		for _, f := range unit.Files {
			abs := filepath.Join(outputDir, f.Path)
			if m.contentUnchanged(ctx, abs, f.Content) {
				continue
			}
			if err := m.writer.WriteFile(ctx, abs, f.Content, 0o644); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: abs,
					Err:  err.Error(),
				})
				continue
			}
			result.WrittenFiles = append(result.WrittenFiles, abs)
		}

		// Copy/symlink assets.
		for _, a := range unit.Assets {
			dest := filepath.Join(outputDir, a.DestPath)
			if err := m.copyOrSymlink(ctx, a.SourcePath, dest); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest,
					Err:  err.Error(),
				})
				continue
			}
			result.SymlinkedFiles = append(result.SymlinkedFiles, dest)
		}

		// Copy/symlink scripts.
		for _, s := range unit.Scripts {
			dest := filepath.Join(outputDir, s.DestPath)
			if err := m.copyOrSymlink(ctx, s.SourcePath, dest); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest,
					Err:  err.Error(),
				})
				continue
			}
			result.SymlinkedFiles = append(result.SymlinkedFiles, dest)
		}

		// Write plugin bundle files.
		for _, pb := range unit.PluginBundles {
			bundleDir := filepath.Join(outputDir, pb.DestDir)
			if err := m.writer.MkdirAll(ctx, bundleDir, 0o755); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: bundleDir,
					Err:  err.Error(),
				})
				continue
			}
			result.CreatedDirs = append(result.CreatedDirs, bundleDir)

			for _, f := range pb.Files {
				abs := filepath.Join(bundleDir, f.Path)
				if m.contentUnchanged(ctx, abs, f.Content) {
					continue
				}
				if err := m.writer.WriteFile(ctx, abs, f.Content, 0o644); err != nil {
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

// contentUnchanged returns true if the file at path already has exactly the
// given content, allowing idempotent materializations.
func (m *OSMaterializer) contentUnchanged(ctx context.Context, path string, content []byte) bool {
	existing, err := m.reader.ReadFile(ctx, path)
	if err != nil {
		return false
	}
	return bytes.Equal(existing, content)
}

// copyOrSymlink attempts to create a symlink from src to dest.
// If symlinking fails (e.g. on Windows) it falls back to copying the file.
func (m *OSMaterializer) copyOrSymlink(ctx context.Context, src, dest string) error {
	if err := m.writer.MkdirAll(ctx, filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// Remove an existing destination so the symlink/copy is clean.
	_ = os.Remove(dest)

	if err := m.writer.Symlink(ctx, src, dest); err == nil {
		return nil
	}
	// Fallback: copy the file content.
	data, err := m.reader.ReadFile(ctx, src)
	if err != nil {
		return fmt.Errorf("filesystem: copy fallback read %q: %w", src, err)
	}
	return m.writer.WriteFile(ctx, dest, data, 0o644)
}
