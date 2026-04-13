// Package npm implements a Packager that produces npm-compatible tarballs
// (.tgz) from compiled emission plans. The tarball includes a package.json
// and content files from the materialized build output.
package npm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
)

// Config controls generation of an npm-compatible tarball.
type Config struct {
	Scope       string         `yaml:"scope"`
	Targets     []build.Target `yaml:"targets"`
	IncludeOnly string         `yaml:"includeOnly,omitempty"`
	Version     string         `yaml:"version,omitempty"`
}

// Packager produces npm-compatible tarballs (.tgz).
type Packager struct {
	fsReader filesystem.Reader
	fsWriter filesystem.Writer
}

// New creates an npm packager.
func New(fsReader filesystem.Reader, fsWriter filesystem.Writer) *Packager {
	return &Packager{fsReader: fsReader, fsWriter: fsWriter}
}

// Format returns FormatNPM.
func (p *Packager) Format() packager.Format { return packager.FormatNPM }

// Targets returns nil (target-agnostic — targets are selected via config).
func (p *Packager) Targets() []build.Target { return nil }

// Package produces an npm tarball.
func (p *Packager) Package(ctx context.Context, input packager.PackagerInput) (*packager.PackagerOutput, error) {
	cfg, ok := input.Config.(*Config)
	if !ok {
		return nil, fmt.Errorf("npm packager: expected *npm.Config, got %T", input.Config)
	}

	filesByTarget := groupFilesByTarget(input.MaterializationResult.WrittenFiles)
	files := collectFiles(cfg.Targets, filesByTarget)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	pkgJSON, err := generatePackageJSON(cfg)
	if err != nil {
		return nil, fmt.Errorf("generate package.json: %w", err)
	}
	if err := writeTarEntry(tw, "package/package.json", pkgJSON); err != nil {
		return nil, fmt.Errorf("add package.json: %w", err)
	}

	for _, filePath := range files {
		content, err := p.fsReader.ReadFile(ctx, filePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filePath, err)
		}
		archivePath := toPackagePath(filePath)
		if err := writeTarEntry(tw, archivePath, content); err != nil {
			return nil, fmt.Errorf("add tar entry %s: %w", archivePath, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}

	filename := npmFilename(cfg.Scope)
	outputPath := path.Join(input.OutputDir, filename)
	if err := p.fsWriter.MkdirAll(ctx, input.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	if err := p.fsWriter.WriteFile(ctx, outputPath, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("write tarball: %w", err)
	}

	return &packager.PackagerOutput{
		Artifacts: []packager.PackagedArtifact{{
			Format:  packager.FormatNPM,
			Path:    outputPath,
			Targets: cfg.Targets,
			Metadata: map[string]string{
				"scope":   cfg.Scope,
				"version": versionOrDefault(cfg.Version),
			},
		}},
	}, nil
}

// Compile-time assertion.
var _ packager.Packager = (*Packager)(nil)

// --- helpers ---

func writeTarEntry(tw *tar.Writer, name string, content []byte) error {
	header := &tar.Header{
		Name: name,
		Size: int64(len(content)),
		Mode: 0o644,
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

func toPackagePath(filePath string) string {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.SplitN(normalized, "/", 3)
	if len(parts) >= 3 {
		return "package/" + parts[2]
	}
	return "package/" + path.Base(normalized)
}

func npmFilename(scope string) string {
	if scope != "" {
		name := strings.TrimPrefix(scope, "@")
		name = strings.ReplaceAll(name, "/", "-")
		return name + "-ai-agent-config.tgz"
	}
	return "ai-agent-config.tgz"
}

func generatePackageJSON(cfg *Config) ([]byte, error) {
	version := versionOrDefault(cfg.Version)
	name := "ai-agent-config"
	if cfg.Scope != "" {
		name = cfg.Scope + "/" + name
	}

	pkg := map[string]any{
		"name":        name,
		"version":     version,
		"description": "AI agent configuration package",
		"license":     "UNLICENSED",
		"files":       []string{"*"},
	}

	return json.MarshalIndent(pkg, "", "  ")
}

func versionOrDefault(v string) string {
	if v == "" {
		return "0.0.1"
	}
	return v
}

func groupFilesByTarget(files []string) map[build.Target][]string {
	result := make(map[build.Target][]string)
	for _, f := range files {
		normalized := path.Clean(strings.ReplaceAll(f, "\\", "/"))
		parts := strings.Split(normalized, "/")
		if len(parts) >= 2 && parts[0] == ".ai-build" {
			target := build.Target(parts[1])
			result[target] = append(result[target], f)
		}
	}
	return result
}

func collectFiles(targets []build.Target, filesByTarget map[build.Target][]string) []string {
	var result []string
	for _, target := range targets {
		result = append(result, filesByTarget[target]...)
	}
	sortStrings(result)
	return result
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
