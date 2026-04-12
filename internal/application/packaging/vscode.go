package packaging

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
)

func (s *PackagingService) packageVSCode(
	ctx context.Context,
	config *VSCodePackagingConfig,
	filesByTarget map[build.Target][]string,
) (*PackagedArtifact, error) {
	files := collectFiles(config.Targets, filesByTarget)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// [Content_Types].xml — required by the VSIX format.
	if err := writeZipEntry(zw, "[Content_Types].xml", []byte(generateContentTypesXML())); err != nil {
		return nil, fmt.Errorf("write content types: %w", err)
	}

	// extension.vsixmanifest
	if err := writeZipEntry(zw, "extension.vsixmanifest", []byte(generateVSIXManifest(config))); err != nil {
		return nil, fmt.Errorf("write vsix manifest: %w", err)
	}

	// extension/package.json
	pkgJSON, err := generateVSCodePackageJSON(config)
	if err != nil {
		return nil, fmt.Errorf("generate package.json: %w", err)
	}
	if err := writeZipEntry(zw, "extension/package.json", pkgJSON); err != nil {
		return nil, fmt.Errorf("write package.json: %w", err)
	}

	// Content files from the materialized output.
	for _, filePath := range files {
		content, err := s.fsReader.ReadFile(ctx, filePath)
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

	outputPath := path.Join(s.outputDir, "extension.vsix")
	if err := s.fsWriter.MkdirAll(ctx, s.outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	if err := s.fsWriter.WriteFile(ctx, outputPath, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("write vsix: %w", err)
	}

	return &PackagedArtifact{
		Type:   "vsix",
		Path:   outputPath,
		Target: joinTargets(config.Targets),
		Metadata: map[string]string{
			"publisher": config.Publisher,
			"version":   versionOrDefault(config.Version),
		},
	}, nil
}

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// toExtensionPath maps .ai-build/{target}/{rest} → extension/{rest}.
func toExtensionPath(filePath string) string {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.SplitN(normalized, "/", 3) // [".ai-build", target, rest]
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

func generateVSIXManifest(config *VSCodePackagingConfig) string {
	id := config.Publisher + "." + sanitizeID(config.DisplayName)
	version := versionOrDefault(config.Version)
	displayName := config.DisplayName
	if displayName == "" {
		displayName = "AI Agent Configuration"
	}
	description := config.Description
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
</PackageManifest>`, id, version, config.Publisher, displayName, description)
}

func sanitizeID(name string) string {
	if name == "" {
		return "ai-agent-config"
	}
	r := strings.NewReplacer(" ", "-", "_", "-")
	return strings.ToLower(r.Replace(name))
}

func generateVSCodePackageJSON(config *VSCodePackagingConfig) ([]byte, error) {
	version := versionOrDefault(config.Version)
	displayName := config.DisplayName
	if displayName == "" {
		displayName = "AI Agent Configuration"
	}
	description := config.Description
	if description == "" {
		description = "AI agent configuration extension"
	}

	pkg := map[string]any{
		"name":        config.Publisher + "-ai-agent-config",
		"displayName": displayName,
		"description": description,
		"version":     version,
		"publisher":   config.Publisher,
		"engines": map[string]string{
			"vscode": "^1.80.0",
		},
		"categories": []string{"Other"},
	}

	return json.MarshalIndent(pkg, "", "  ")
}

func versionOrDefault(v string) string {
	if v == "" {
		return "0.0.1"
	}
	return v
}
