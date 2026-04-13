// Package vscode implements a Packager that produces VS Code extension archives
// (.vsix) from compiled emission plans. The archive includes a VSIX manifest,
// package.json, and content files from the materialized build output.
package vscode

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/port/filesystem"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
)

// Config controls generation of a .vsix extension archive.
type Config struct {
	Publisher        string         `yaml:"publisher"`
	Targets          []build.Target `yaml:"targets"`
	IncludeSkills    bool           `yaml:"includeSkills"`
	IncludeMcpConfig bool           `yaml:"includeMcpConfig"`
	Version          string         `yaml:"version,omitempty"`
	DisplayName      string         `yaml:"displayName,omitempty"`
	Description      string         `yaml:"description,omitempty"`
}

// Packager produces VS Code extension archives (.vsix).
type Packager struct {
	fsReader filesystem.Reader
	fsWriter filesystem.Writer
}

// New creates a VS Code packager.
func New(fsReader filesystem.Reader, fsWriter filesystem.Writer) *Packager {
	return &Packager{fsReader: fsReader, fsWriter: fsWriter}
}

// Format returns FormatVSIX.
func (p *Packager) Format() packager.Format { return packager.FormatVSIX }

// Targets returns nil (target-agnostic — targets are selected via config).
func (p *Packager) Targets() []build.Target { return nil }

// Package produces a .vsix archive.
func (p *Packager) Package(ctx context.Context, input packager.PackagerInput) (*packager.PackagerOutput, error) {
	cfg, ok := input.Config.(*Config)
	if !ok {
		return nil, fmt.Errorf("vscode packager: expected *vscode.Config, got %T", input.Config)
	}

	filesByTarget := groupFilesByTarget(input.MaterializationResult.WrittenFiles)
	files := collectFiles(cfg.Targets, filesByTarget)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := writeZipEntry(zw, "[Content_Types].xml", []byte(generateContentTypesXML())); err != nil {
		return nil, fmt.Errorf("write content types: %w", err)
	}

	if err := writeZipEntry(zw, "extension.vsixmanifest", []byte(generateVSIXManifest(cfg))); err != nil {
		return nil, fmt.Errorf("write vsix manifest: %w", err)
	}

	pkgJSON, err := generatePackageJSON(cfg)
	if err != nil {
		return nil, fmt.Errorf("generate package.json: %w", err)
	}
	if err := writeZipEntry(zw, "extension/package.json", pkgJSON); err != nil {
		return nil, fmt.Errorf("write package.json: %w", err)
	}

	for _, filePath := range files {
		content, err := p.fsReader.ReadFile(ctx, filePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filePath, err)
		}
		archivePath := toExtensionPath(filePath)
		if err := writeZipEntry(zw, archivePath, content); err != nil {
			return nil, fmt.Errorf("write archive entry %s: %w", archivePath, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	outputDir := input.OutputDir
	outputPath := path.Join(outputDir, "extension.vsix")
	if err := p.fsWriter.MkdirAll(ctx, outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	if err := p.fsWriter.WriteFile(ctx, outputPath, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("write vsix: %w", err)
	}

	return &packager.PackagerOutput{
		Artifacts: []packager.PackagedArtifact{{
			Format:  packager.FormatVSIX,
			Path:    outputPath,
			Targets: cfg.Targets,
			Metadata: map[string]string{
				"publisher": cfg.Publisher,
				"version":   versionOrDefault(cfg.Version),
			},
		}},
	}, nil
}

// Compile-time assertion.
var _ packager.Packager = (*Packager)(nil)

// --- helpers ---

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func toExtensionPath(filePath string) string {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.SplitN(normalized, "/", 3)
	if len(parts) >= 3 {
		return "extension/" + parts[2]
	}
	return "extension/" + path.Base(normalized)
}

func generateContentTypesXML() string {
	return `<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension=".json" ContentType="application/json" />
  <Default Extension=".md" ContentType="text/markdown" />
  <Default Extension=".txt" ContentType="text/plain" />
  <Default Extension=".yaml" ContentType="text/yaml" />
  <Default Extension=".yml" ContentType="text/yaml" />
  <Default Extension=".xml" ContentType="application/xml" />
  <Default Extension=".vsixmanifest" ContentType="text/xml" />
</Types>`
}

func generateVSIXManifest(cfg *Config) string {
	id := cfg.Publisher + "." + sanitizeID(cfg.DisplayName)
	version := versionOrDefault(cfg.Version)
	displayName := cfg.DisplayName
	if displayName == "" {
		displayName = "AI Agent Configuration"
	}
	description := cfg.Description
	if description == "" {
		description = "AI agent configuration extension"
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://microsoft.com/VisualStudio/ConnectedServices/Manifest">
  <Metadata>
    <Identity Id="%s" Version="%s" Publisher="%s" />
    <DisplayName>%s</DisplayName>
    <Description>%s</Description>
  </Metadata>
  <Installation>
    <InstallationTarget Id="Microsoft.VisualStudio.Code" />
  </Installation>
  <Assets>
    <Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" />
  </Assets>
</PackageManifest>`, id, version, cfg.Publisher, displayName, description)
}

func sanitizeID(name string) string {
	if name == "" {
		return "ai-agent-config"
	}
	r := strings.NewReplacer(" ", "-", "_", "-")
	return strings.ToLower(r.Replace(name))
}

func generatePackageJSON(cfg *Config) ([]byte, error) {
	version := versionOrDefault(cfg.Version)
	displayName := cfg.DisplayName
	if displayName == "" {
		displayName = "AI Agent Configuration"
	}
	description := cfg.Description
	if description == "" {
		description = "AI agent configuration extension"
	}

	pkg := map[string]any{
		"name":        cfg.Publisher + "-ai-agent-config",
		"displayName": displayName,
		"description": description,
		"version":     version,
		"publisher":   cfg.Publisher,
		"engines":     map[string]string{"vscode": "^1.80.0"},
		"categories":  []string{"Other"},
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
	// Deterministic order.
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
