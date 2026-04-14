package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	claudegen "github.com/mariotoffia/goagentmeta/internal/adapter/marketplace/claude"
	claudepkg "github.com/mariotoffia/goagentmeta/internal/adapter/packager/claude"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	domain "github.com/mariotoffia/goagentmeta/internal/domain/marketplace"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
	portmp "github.com/mariotoffia/goagentmeta/internal/port/marketplace"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
	"github.com/spf13/cobra"
)

var (
	pkgTargets          []string
	pkgProfile          string
	pkgOutputDir        string
	pkgFormat           string
	pkgPluginName       string
	pkgVersion          string
	pkgAuthor           string
	pkgDesc             string
	pkgLicense          string
	pkgKeywords         []string
	pkgCategory         string
	pkgTags             []string
	pkgMarketplaceName  string
	pkgMarketplaceOwner string
	pkgSource           string
	pkgDryRun           bool
)

var packageCmd = &cobra.Command{
	Use:   "package [paths...]",
	Short: "Build and package .ai/ source into distributable Claude Code plugins or marketplace catalogs",
	Long: `Package compiles .ai/ source files and then packages the compiled output
into distributable formats for Claude Code's plugin/marketplace system.

Modes:
  plugin       Package as a distributable Claude Code plugin directory
  marketplace  Generate a marketplace.json catalog

Examples:
  # Build & package as a Claude Code plugin
  goagentmeta package --format plugin --name my-plugin --version 1.0.0

  # Build for Claude only & package with author info
  goagentmeta package -t claude --format plugin --name my-plugin \
    --version 1.0.0 --author "My Team" --license MIT

  # Generate a marketplace catalog referencing a local plugin
  goagentmeta package --format marketplace --marketplace-name my-tools \
    --marketplace-owner "DevTools Team" --name my-plugin --source ./plugins/my-plugin

  # Dry-run to see what would be produced
  goagentmeta package --format plugin --name my-plugin --dry-run`,
	RunE: runPackage,
}

func init() {
	packageCmd.Flags().StringSliceVarP(&pkgTargets, "target", "t", []string{"claude"},
		`targets to build: claude, cursor, copilot, codex, all`)
	packageCmd.Flags().StringVarP(&pkgProfile, "profile", "p", "local-dev",
		`build profile`)
	packageCmd.Flags().StringVar(&pkgOutputDir, "output-dir", ".ai-build/dist",
		`output directory for packaged artifacts`)
	packageCmd.Flags().StringVar(&pkgFormat, "format", "plugin",
		`packaging format: plugin, marketplace`)
	packageCmd.Flags().StringVar(&pkgPluginName, "name", "",
		`plugin name (kebab-case, required)`)
	packageCmd.Flags().StringVar(&pkgVersion, "version", "0.0.1",
		`plugin version (semver)`)
	packageCmd.Flags().StringVar(&pkgAuthor, "author", "",
		`plugin author name`)
	packageCmd.Flags().StringVar(&pkgDesc, "description", "",
		`plugin description`)
	packageCmd.Flags().StringVar(&pkgLicense, "license", "",
		`SPDX license identifier (e.g., MIT, Apache-2.0)`)
	packageCmd.Flags().StringSliceVar(&pkgKeywords, "keyword", nil,
		`plugin keywords for discovery (repeatable)`)
	packageCmd.Flags().StringVar(&pkgCategory, "category", "",
		`plugin category (e.g., code-intelligence, external-integrations)`)
	packageCmd.Flags().StringSliceVar(&pkgTags, "tag", nil,
		`plugin tags for searchability (repeatable)`)
	packageCmd.Flags().StringVar(&pkgMarketplaceName, "marketplace-name", "",
		`marketplace catalog name (for --format marketplace)`)
	packageCmd.Flags().StringVar(&pkgMarketplaceOwner, "marketplace-owner", "",
		`marketplace owner name (for --format marketplace)`)
	packageCmd.Flags().StringVar(&pkgSource, "source", "",
		`plugin source for marketplace entry (e.g., ./plugins/my-plugin, github:owner/repo)`)
	packageCmd.Flags().BoolVar(&pkgDryRun, "dry-run", false,
		`print what would be produced without writing files`)
}

func runPackage(cmd *cobra.Command, args []string) error {
	out := newOutputWriter()

	if pkgPluginName == "" {
		return fmt.Errorf("--name is required")
	}

	targets, err := resolveTargets(pkgTargets)
	if err != nil {
		return err
	}

	rootPaths := args
	if len(rootPaths) == 0 {
		rootPaths = []string{"."}
	}

	// Step 1: Compile .ai/ source tree.
	out.info("Compiling .ai/ source...")

	buildCfg := buildConfig{
		targets:    targets,
		profile:    build.Profile(pkgProfile),
		outputDir:  ".ai-build",
		failFast:   true,
		dryRun:     pkgDryRun,
		syncMode:   "build-only",
		reportPath: ".ai-build/report",
	}

	p, err := wirePipeline(buildCfg)
	if err != nil {
		return fmt.Errorf("wire pipeline: %w", err)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	report, err := p.Execute(ctx, rootPaths)
	if err != nil {
		if ctx.Err() != nil {
			return &exitError{code: 2, err: ctx.Err()}
		}
		return fmt.Errorf("build failed: %w", err)
	}

	// Collect materialized files from report.
	var allFiles []string
	filesWritten := 0
	if report != nil {
		for _, u := range report.Units {
			allFiles = append(allFiles, u.EmittedFiles...)
			filesWritten += len(u.EmittedFiles)
		}
	}
	out.info("  %s %d files compiled", colorize(colorGreen, "✓"), filesWritten)

	// Step 2: Package based on format.
	switch pkgFormat {
	case "plugin":
		return runPackagePlugin(ctx, out, allFiles)
	case "marketplace":
		return runPackageMarketplace(ctx, out)
	default:
		return fmt.Errorf("unknown format %q; valid formats: plugin, marketplace", pkgFormat)
	}
}

func runPackagePlugin(ctx context.Context, out *outputWriter, compiledFiles []string) error {
	out.info("Packaging as Claude Code plugin '%s'...", pkgPluginName)

	if pkgDryRun {
		out.info("  (dry-run mode)")
		printPluginSummary(out)
		return nil
	}

	fsReader := filesystem.NewOSReader()
	fsWriter := filesystem.NewOSWriter()
	pkger := claudepkg.New(fsReader, fsWriter)

	config := &claudepkg.Config{
		Name:        pkgPluginName,
		Version:     pkgVersion,
		Description: pkgDesc,
		Author:      domain.Author{Name: pkgAuthor},
		License:     pkgLicense,
		Keywords:    pkgKeywords,
	}

	input := packager.PackagerInput{
		EmissionPlan:          pipeline.EmissionPlan{Units: map[string]pipeline.UnitEmission{}},
		MaterializationResult: pipeline.MaterializationResult{WrittenFiles: compiledFiles},
		Config:                config,
		OutputDir:             pkgOutputDir,
	}

	output, err := pkger.Package(ctx, input)
	if err != nil {
		return fmt.Errorf("packaging failed: %w", err)
	}

	for _, a := range output.Artifacts {
		out.info("  %s %s → %s", colorize(colorGreen, "✓"), string(a.Format), a.Path)
		if a.Metadata["files"] != "" {
			out.debug("    %s files", a.Metadata["files"])
		}
	}

	out.info("")
	out.info(colorize(colorBold, "Plugin ready at: ") + pkgOutputDir + "/" + pkgPluginName)
	out.info("")
	out.info("To test locally:")
	out.info("  claude --plugin-dir %s/%s", pkgOutputDir, pkgPluginName)
	out.info("")
	out.info("To distribute via marketplace, run:")
	out.info("  goagentmeta package --format marketplace --name %s \\", pkgPluginName)
	out.info("    --marketplace-name <name> --marketplace-owner <owner> \\")
	out.info("    --source ./%s/%s", pkgOutputDir, pkgPluginName)

	return nil
}

func runPackageMarketplace(ctx context.Context, out *outputWriter) error {
	if pkgMarketplaceName == "" {
		return fmt.Errorf("--marketplace-name is required for marketplace format")
	}
	if pkgMarketplaceOwner == "" {
		return fmt.Errorf("--marketplace-owner is required for marketplace format")
	}

	out.info("Generating marketplace catalog '%s'...", pkgMarketplaceName)

	source := parseSource(pkgSource)

	mp := domain.Marketplace{
		SchemaVersion: 1,
		Name:          pkgMarketplaceName,
		Owner:         domain.Owner{Name: pkgMarketplaceOwner},
		Plugins: []domain.PluginEntry{
			{
				Name:        pkgPluginName,
				Source:      source,
				Description: pkgDesc,
				Version:     pkgVersion,
				Author:      domain.Author{Name: pkgAuthor},
				License:     pkgLicense,
				Keywords:    pkgKeywords,
				Category:    pkgCategory,
				Tags:        pkgTags,
			},
		},
	}

	if pkgDryRun {
		out.info("  (dry-run mode)")
		catalogJSON, _ := json.MarshalIndent(buildDryCatalog(mp), "", "  ")
		fmt.Fprintln(os.Stdout, string(catalogJSON))
		return nil
	}

	fsWriter := filesystem.NewOSWriter()
	gen := claudegen.New(fsWriter)

	output, err := gen.Generate(ctx, portmp.GeneratorInput{
		Marketplace: mp,
		OutputDir:   pkgOutputDir,
	})
	if err != nil {
		return fmt.Errorf("marketplace generation failed: %w", err)
	}

	out.info("  %s marketplace.json → %s", colorize(colorGreen, "✓"), output.CatalogPath)
	out.info("")
	out.info(colorize(colorBold, "Marketplace ready at: ") + output.CatalogPath)
	out.info("")
	out.info("To use with Claude Code:")
	out.info("  /plugin marketplace add <path-or-repo>")
	out.info("  /plugin install %s@%s", pkgPluginName, pkgMarketplaceName)

	return nil
}

func parseSource(raw string) domain.Source {
	if raw == "" {
		return domain.Source{
			Type:     domain.SourceRelativePath,
			Location: "./" + pkgPluginName,
		}
	}

	if strings.HasPrefix(raw, "github:") {
		return domain.Source{
			Type:     domain.SourceGitHub,
			Location: strings.TrimPrefix(raw, "github:"),
		}
	}
	if strings.HasPrefix(raw, "npm:") {
		return domain.Source{
			Type:    domain.SourceNPM,
			Package: strings.TrimPrefix(raw, "npm:"),
		}
	}
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "git@") {
		return domain.Source{
			Type:     domain.SourceGitURL,
			Location: raw,
		}
	}

	return domain.Source{
		Type:     domain.SourceRelativePath,
		Location: raw,
	}
}

func printPluginSummary(out *outputWriter) {
	out.info("  Plugin: %s v%s", pkgPluginName, pkgVersion)
	if pkgAuthor != "" {
		out.info("  Author: %s", pkgAuthor)
	}
	if pkgDesc != "" {
		out.info("  Description: %s", pkgDesc)
	}
	if pkgLicense != "" {
		out.info("  License: %s", pkgLicense)
	}
	if len(pkgKeywords) > 0 {
		out.info("  Keywords: %s", strings.Join(pkgKeywords, ", "))
	}
	out.info("  Output: %s/%s/", pkgOutputDir, pkgPluginName)
}

func buildDryCatalog(mp domain.Marketplace) map[string]any {
	result := map[string]any{
		"name":  mp.Name,
		"owner": map[string]string{"name": mp.Owner.Name},
	}
	var plugins []map[string]any
	for _, p := range mp.Plugins {
		entry := map[string]any{"name": p.Name}
		if p.Description != "" {
			entry["description"] = p.Description
		}
		if p.Version != "" {
			entry["version"] = p.Version
		}
		if p.Category != "" {
			entry["category"] = p.Category
		}
		if len(p.Tags) > 0 {
			entry["tags"] = p.Tags
		}
		plugins = append(plugins, entry)
	}
	result["plugins"] = plugins
	return result
}
