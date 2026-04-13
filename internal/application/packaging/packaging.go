package packaging

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	"github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
)

// ErrNotImplemented is returned by packagers that are not yet implemented.
var ErrNotImplemented = errors.New("packaging: not implemented")

// PackagingResult holds the output of the packaging process.
type PackagingResult struct {
	Artifacts []PackagedArtifact
}

// PackagedArtifact describes a single distributable artifact produced by packaging.
type PackagedArtifact struct {
	Type     string            // "vsix", "npm", "oci"
	Path     string            // output path of the artifact
	Target   string            // comma-separated targets included
	Metadata map[string]string // packager-specific metadata
}

// Option configures the PackagingService.
type Option func(*PackagingService)

// WithOutputDir overrides the default output directory for packaged artifacts.
func WithOutputDir(dir string) Option {
	return func(s *PackagingService) {
		s.outputDir = dir
	}
}

// WithRegistry sets a custom packager registry. If not set, the service uses
// the legacy if/else dispatch for backward compatibility.
func WithRegistry(reg packager.PackagerRegistry) Option {
	return func(s *PackagingService) {
		s.registry = reg
	}
}

// PackagingService wraps MaterializationResult output into distributable packages.
// It supports two modes:
//   - Legacy: if/else dispatch to built-in packagers (backward-compatible)
//   - Registry: delegates to registered packager.Packager implementations
type PackagingService struct {
	fsReader  filesystem.Reader
	fsWriter  filesystem.Writer
	outputDir string
	registry  packager.PackagerRegistry
}

// NewPackagingService creates a new PackagingService.
func NewPackagingService(
	fsReader filesystem.Reader,
	fsWriter filesystem.Writer,
	opts ...Option,
) *PackagingService {
	s := &PackagingService{
		fsReader:  fsReader,
		fsWriter:  fsWriter,
		outputDir: ".ai-build/dist",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Package creates distributable artifacts from materialization output.
// Disabled packagers are silently skipped.
func (s *PackagingService) Package(
	ctx context.Context,
	result pipeline.MaterializationResult,
	config PackagingConfig,
) (*PackagingResult, error) {
	filesByTarget := groupFilesByTarget(result.WrittenFiles)
	var artifacts []PackagedArtifact

	if config.VSCodeExtension != nil && config.VSCodeExtension.Enabled {
		artifact, err := s.packageVSCode(ctx, config.VSCodeExtension, filesByTarget)
		if err != nil {
			return nil, fmt.Errorf("vsix packaging: %w", err)
		}
		artifacts = append(artifacts, *artifact)
	}

	if config.NPM != nil && config.NPM.Enabled {
		artifact, err := s.packageNPM(ctx, config.NPM, filesByTarget)
		if err != nil {
			return nil, fmt.Errorf("npm packaging: %w", err)
		}
		artifacts = append(artifacts, *artifact)
	}

	if config.OCI != nil && config.OCI.Enabled {
		if _, err := s.packageOCI(ctx, config.OCI, filesByTarget); err != nil {
			return nil, fmt.Errorf("oci packaging: %w", err)
		}
	}

	return &PackagingResult{Artifacts: artifacts}, nil
}

// PackageWithEmission creates distributable artifacts using the structured
// emission plan and registered packager.Packager implementations. Each entry
// in configs maps a packager.Format to a packager-specific config struct.
// Formats not present in the registry are silently skipped.
func (s *PackagingService) PackageWithEmission(
	ctx context.Context,
	emission pipeline.EmissionPlan,
	result pipeline.MaterializationResult,
	configs map[packager.Format]any,
) (*PackagingResult, error) {
	if s.registry == nil {
		return nil, fmt.Errorf("packaging: no packager registry configured; use WithRegistry option")
	}

	var artifacts []PackagedArtifact

	for format, cfg := range configs {
		p, ok := s.registry.ByFormat(format)
		if !ok {
			continue
		}

		output, err := p.Package(ctx, packager.PackagerInput{
			EmissionPlan:          emission,
			MaterializationResult: result,
			Config:                cfg,
			OutputDir:             s.outputDir,
		})
		if err != nil {
			return nil, fmt.Errorf("%s packaging: %w", format, err)
		}

		for _, a := range output.Artifacts {
			artifacts = append(artifacts, PackagedArtifact{
				Type:     string(a.Format),
				Path:     a.Path,
				Target:   joinTargets(a.Targets),
				Metadata: a.Metadata,
			})
		}
	}

	return &PackagingResult{Artifacts: artifacts}, nil
}

// Registry returns the packager registry, or nil if none is configured.
func (s *PackagingService) Registry() packager.PackagerRegistry {
	return s.registry
}

// groupFilesByTarget partitions materialized file paths by their build target,
// derived from the .ai-build/{target}/... path convention.
func groupFilesByTarget(files []string) map[build.Target][]string {
	result := make(map[build.Target][]string)
	for _, f := range files {
		if target, ok := extractTarget(f); ok {
			result[target] = append(result[target], f)
		}
	}
	return result
}

// extractTarget parses a build target from a file path of the form
// .ai-build/{target}/...
func extractTarget(filePath string) (build.Target, bool) {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.Split(normalized, "/")
	if len(parts) >= 2 && parts[0] == ".ai-build" {
		return build.Target(parts[1]), true
	}
	return "", false
}

// joinTargets produces a sorted, comma-separated string of target names.
func joinTargets(targets []build.Target) string {
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = string(t)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// collectFiles gathers and deterministically sorts files for the given targets.
func collectFiles(targets []build.Target, filesByTarget map[build.Target][]string) []string {
	var result []string
	for _, target := range targets {
		result = append(result, filesByTarget[target]...)
	}
	sort.Strings(result)
	return result
}
