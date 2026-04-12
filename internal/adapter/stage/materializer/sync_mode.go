package materializer

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
)

// SyncMode controls how materialized files in .ai-build/ are synchronized
// to the repository root.
type SyncMode string

const (
	// SyncBuildOnly writes to .ai-build/ only; no files touch the repo root.
	SyncBuildOnly SyncMode = "build-only"

	// SyncCopy copies files from .ai-build/{target}/{profile}/ to repo root.
	SyncCopy SyncMode = "copy"

	// SyncSymlink creates symlinks from repo root pointing to .ai-build/ files.
	SyncSymlink SyncMode = "symlink"

	// SyncAdoptSelected syncs only files matching specific patterns.
	SyncAdoptSelected SyncMode = "adopt-selected"
)

// String implements fmt.Stringer.
func (m SyncMode) String() string { return string(m) }

// applySyncMode applies the chosen sync strategy after the initial
// build-only materialization has completed. It reads files from the
// .ai-build/ directory and copies or symlinks them to the repo root.
//
// The materializer parameter must support both Reader and Writer operations.
// For MemFS-based testing, a single MemFS satisfies all ports.
func applySyncMode(
	ctx context.Context,
	mat portfs.Materializer,
	mode SyncMode,
	patterns []string,
	plan pipeline.EmissionPlan,
) (pipeline.MaterializationResult, error) {
	// We need both Reader and Writer to perform sync. If the materializer
	// also implements these, use it directly. Otherwise, the sync is a no-op
	// because we cannot read the built files.
	type readerWriter interface {
		portfs.Reader
		portfs.Writer
	}

	rw, ok := mat.(readerWriter)
	if !ok {
		return pipeline.MaterializationResult{}, fmt.Errorf(
			"materializer does not implement Reader+Writer; sync mode %q requires both", mode,
		)
	}

	switch mode {
	case SyncBuildOnly:
		return pipeline.MaterializationResult{}, nil
	case SyncCopy:
		return syncCopy(ctx, rw, plan)
	case SyncSymlink:
		return syncSymlink(ctx, rw, plan)
	case SyncAdoptSelected:
		return syncAdoptSelected(ctx, rw, plan, patterns)
	default:
		return pipeline.MaterializationResult{}, fmt.Errorf("unknown sync mode %q", mode)
	}
}

// syncCopy copies all materialized files from each unit's OutputDir to the
// repo root, stripping the .ai-build/{target}/{profile}/ prefix.
func syncCopy(
	ctx context.Context,
	rw interface {
		portfs.Reader
		portfs.Writer
	},
	plan pipeline.EmissionPlan,
) (pipeline.MaterializationResult, error) {
	var result pipeline.MaterializationResult

	unitKeys := sortedKeys(plan.Units)
	for _, outputDir := range unitKeys {
		unit := plan.Units[outputDir]

		for _, f := range unit.Files {
			src := filepath.Join(outputDir, f.Path)
			dest := f.Path

			data, err := rw.ReadFile(ctx, src)
			if err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync copy read: %v", err),
				})
				continue
			}

			if err := rw.MkdirAll(ctx, filepath.Dir(dest), 0o755); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync copy mkdir: %v", err),
				})
				continue
			}

			if err := rw.WriteFile(ctx, dest, data, fs.FileMode(0o644)); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync copy write: %v", err),
				})
				continue
			}
			result.WrittenFiles = append(result.WrittenFiles, dest)
		}
	}

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("sync copy completed with %d error(s)", len(result.Errors))
	}
	return result, nil
}

// syncSymlink creates symlinks from repo root paths to the corresponding
// files inside .ai-build/{target}/{profile}/.
func syncSymlink(
	ctx context.Context,
	rw interface {
		portfs.Reader
		portfs.Writer
	},
	plan pipeline.EmissionPlan,
) (pipeline.MaterializationResult, error) {
	var result pipeline.MaterializationResult

	unitKeys := sortedKeys(plan.Units)
	for _, outputDir := range unitKeys {
		unit := plan.Units[outputDir]

		for _, f := range unit.Files {
			src := filepath.Join(outputDir, f.Path)
			dest := f.Path

			if err := rw.MkdirAll(ctx, filepath.Dir(dest), 0o755); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync symlink mkdir: %v", err),
				})
				continue
			}

			// Remove existing destination to allow clean symlink.
			_ = rw.Remove(ctx, dest)

			if err := rw.Symlink(ctx, src, dest); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync symlink: %v", err),
				})
				continue
			}
			result.SymlinkedFiles = append(result.SymlinkedFiles, dest)
		}
	}

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("sync symlink completed with %d error(s)", len(result.Errors))
	}
	return result, nil
}

// syncAdoptSelected copies only files matching the given patterns from
// .ai-build/ to the repo root.
func syncAdoptSelected(
	ctx context.Context,
	rw interface {
		portfs.Reader
		portfs.Writer
	},
	plan pipeline.EmissionPlan,
	patterns []string,
) (pipeline.MaterializationResult, error) {
	var result pipeline.MaterializationResult

	unitKeys := sortedKeys(plan.Units)
	for _, outputDir := range unitKeys {
		unit := plan.Units[outputDir]

		for _, f := range unit.Files {
			if !matchesAnyPattern(f.Path, patterns) {
				continue
			}

			src := filepath.Join(outputDir, f.Path)
			dest := f.Path

			data, err := rw.ReadFile(ctx, src)
			if err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync adopt read: %v", err),
				})
				continue
			}

			if err := rw.MkdirAll(ctx, filepath.Dir(dest), 0o755); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync adopt mkdir: %v", err),
				})
				continue
			}

			if err := rw.WriteFile(ctx, dest, data, fs.FileMode(0o644)); err != nil {
				result.Errors = append(result.Errors, pipeline.MaterializationError{
					Path: dest, Err: fmt.Sprintf("sync adopt write: %v", err),
				})
				continue
			}
			result.WrittenFiles = append(result.WrittenFiles, dest)
		}
	}

	if len(result.Errors) > 0 {
		return result, fmt.Errorf("sync adopt-selected completed with %d error(s)", len(result.Errors))
	}
	return result, nil
}

// matchesAnyPattern returns true if the path matches any of the given
// glob patterns. An empty pattern list matches nothing.
func matchesAnyPattern(path string, patterns []string) bool {
	for _, p := range patterns {
		// Try matching the full path.
		if ok, _ := filepath.Match(p, path); ok {
			return true
		}
		// Also match against just the base name for simple patterns.
		if ok, _ := filepath.Match(p, filepath.Base(path)); ok {
			return true
		}
	}
	return false
}

// sortedKeys returns sorted keys from the unit map.
func sortedKeys(units map[string]pipeline.UnitEmission) []string {
	keys := make([]string, 0, len(units))
	for k := range units {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
