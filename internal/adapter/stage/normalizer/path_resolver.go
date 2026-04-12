package normalizer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portfs "github.com/mariotoffia/goagentmeta/internal/port/filesystem"
)

// resolvePaths resolves relative paths in RawFields to repository-root-relative
// paths using the object's SourcePath as the base directory. If a filesystem
// Reader is provided, it validates that the referenced path exists.
func resolvePaths(
	ctx context.Context,
	obj *pipeline.NormalizedObject,
	rootPath string,
	fs portfs.Reader,
) error {
	sourceDir := filepath.Dir(obj.SourcePath)

	pathFields := []string{
		"reference_path", "asset_path", "script_path",
		"action_ref",
	}

	for _, field := range pathFields {
		raw, ok := obj.ResolvedFields[field]
		if !ok {
			continue
		}
		relPath, ok := raw.(string)
		if !ok || relPath == "" {
			continue
		}

		resolved, err := resolvePath(relPath, sourceDir, rootPath, obj.Meta.ID)
		if err != nil {
			return err
		}
		obj.ResolvedFields[field] = filepath.ToSlash(resolved)

		if fs != nil {
			absPath := filepath.Join(rootPath, resolved)
			if _, err := fs.Stat(ctx, absPath); err != nil {
				return pipeline.NewCompilerError(
					pipeline.ErrNormalization,
					fmt.Sprintf("referenced path %q not found (resolved to %q) in %s", relPath, absPath, obj.SourcePath),
					obj.Meta.ID,
				)
			}
		}
	}

	return nil
}

// resolvePath converts a relative path to a repository-root-relative path.
// Returns an error if the resolved path escapes the repository root.
func resolvePath(relPath, sourceDir, rootPath, objectID string) (string, error) {
	relPath = filepath.ToSlash(relPath)

	var rel string
	if filepath.IsAbs(relPath) {
		r, err := filepath.Rel(rootPath, relPath)
		if err != nil {
			return "", pipeline.NewCompilerError(
				pipeline.ErrNormalization,
				fmt.Sprintf("cannot resolve absolute path %q relative to root", relPath),
				objectID,
			)
		}
		rel = r
	} else {
		abs := filepath.Join(sourceDir, relPath)
		r, err := filepath.Rel(rootPath, abs)
		if err != nil {
			return "", pipeline.NewCompilerError(
				pipeline.ErrNormalization,
				fmt.Sprintf("cannot resolve path %q relative to root", relPath),
				objectID,
			)
		}
		rel = r
	}

	// Prevent path traversal outside the repository root.
	normalized := filepath.ToSlash(rel)
	if strings.HasPrefix(normalized, "..") {
		return "", pipeline.NewCompilerError(
			pipeline.ErrNormalization,
			fmt.Sprintf("path %q escapes repository root (resolved to %q)", relPath, normalized),
			objectID,
		)
	}

	return rel, nil
}
