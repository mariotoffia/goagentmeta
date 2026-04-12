package packaging

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
)

func (s *PackagingService) packageNPM(
	ctx context.Context,
	config *NPMPackagingConfig,
	filesByTarget map[build.Target][]string,
) (*PackagedArtifact, error) {
	files := collectFiles(config.Targets, filesByTarget)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// package/package.json
	pkgJSON, err := generateNPMPackageJSON(config)
	if err != nil {
		return nil, fmt.Errorf("generate package.json: %w", err)
	}
	if err := writeTarEntry(tw, "package/package.json", pkgJSON); err != nil {
		return nil, fmt.Errorf("add package.json: %w", err)
	}

	// Content files from the materialized output.
	for _, filePath := range files {
		content, err := s.fsReader.ReadFile(ctx, filePath)
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

	filename := npmFilename(config.Scope)
	outputPath := path.Join(s.outputDir, filename)
	if err := s.fsWriter.MkdirAll(ctx, s.outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	if err := s.fsWriter.WriteFile(ctx, outputPath, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("write tarball: %w", err)
	}

	return &PackagedArtifact{
		Type:   "npm",
		Path:   outputPath,
		Target: joinTargets(config.Targets),
		Metadata: map[string]string{
			"scope":   config.Scope,
			"version": versionOrDefault(config.Version),
		},
	}, nil
}

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

// toPackagePath maps .ai-build/{target}/{rest} → package/{rest}.
func toPackagePath(filePath string) string {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.SplitN(normalized, "/", 3) // [".ai-build", target, rest]
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

func generateNPMPackageJSON(config *NPMPackagingConfig) ([]byte, error) {
	version := versionOrDefault(config.Version)
	name := "ai-agent-config"
	if config.Scope != "" {
		name = config.Scope + "/" + name
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
