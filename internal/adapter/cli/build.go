package cli

import (
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/spf13/cobra"
)

var (
	buildTargets  []string
	buildProfile  string
	buildOutput   string
	buildFailFast bool
	buildDryRun   bool
	buildSync     string
)

var buildCmd = &cobra.Command{
	Use:   "build [paths...]",
	Short: "Compile .ai/ source tree into target agent configurations",
	Long: `Build compiles .ai/ source files and produces target-specific configuration
files under .ai-build/. By default, all targets are built with the local-dev profile.

Exit codes:
  0  success
  1  build error (parse, validation, or compilation failure)
  2  interrupted (Ctrl+C)`,
	RunE: runBuild,
}

func init() {
	buildCmd.Flags().StringSliceVarP(&buildTargets, "target", "t", nil,
		`targets to build: claude, cursor, copilot, codex, all (default: all)`)
	buildCmd.Flags().StringVarP(&buildProfile, "profile", "p", "local-dev",
		`build profile: local-dev, ci, enterprise-locked, oss-public`)
	buildCmd.Flags().StringVar(&buildOutput, "output-dir", ".ai-build",
		`output directory for generated files`)
	buildCmd.Flags().BoolVar(&buildFailFast, "fail-fast", true,
		`stop on first error`)
	buildCmd.Flags().BoolVar(&buildDryRun, "dry-run", false,
		`print what would be written without writing files`)
	buildCmd.Flags().StringVar(&buildSync, "sync", "build-only",
		`sync mode: build-only, copy, symlink`)
}

func runBuild(cmd *cobra.Command, args []string) error {
	out := newOutputWriter()

	targets, err := resolveTargets(buildTargets)
	if err != nil {
		return err
	}

	rootPaths := args
	if len(rootPaths) == 0 {
		rootPaths = []string{"."}
	}

	cfg := buildConfig{
		targets:    targets,
		profile:    build.Profile(buildProfile),
		outputDir:  buildOutput,
		failFast:   buildFailFast,
		dryRun:     buildDryRun,
		syncMode:   buildSync,
		reportPath: buildOutput + "/report",
	}

	p, err := wirePipeline(cfg)
	if err != nil {
		return fmt.Errorf("wire pipeline: %w", err)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if buildDryRun {
		out.info("Dry-run mode: no files will be written")
	}

	out.debug("Building targets: %v, profile: %s", targets, buildProfile)

	report, err := p.Execute(ctx, rootPaths)
	if err != nil {
		if ctx.Err() != nil {
			out.errorf("interrupted")
			cmd.SilenceErrors = true
			return &exitError{code: 2, err: ctx.Err()}
		}
		return fmt.Errorf("build failed: %w", err)
	}

	filesWritten := 0
	totalLowerings := 0
	totalErrors := 0

	if report != nil {
		for _, d := range report.Diagnostics {
			switch d.Severity {
			case "warning":
				out.warn("%s", d.Message)
			case "error":
				out.errorf("[%s] %s", d.Phase, d.Message)
				totalErrors++
			case "info":
				out.debug("[%s] %s", d.Phase, d.Message)
			}
		}

		for _, u := range report.Units {
			filesWritten += len(u.EmittedFiles)
			totalLowerings += len(u.LoweringRecords)

			for _, w := range u.Warnings {
				out.warn("%s", w)
			}

			out.debug("")
			label := u.Coordinate.OutputDir
			if label == "" && u.Coordinate.Unit.Target != "" {
				label = fmt.Sprintf("target: %s, profile: %s",
					u.Coordinate.Unit.Target, u.Coordinate.Unit.Profile)
			}
			if label == "" && len(u.EmittedFiles) > 0 {
				// Derive from first emitted path (e.g. ".ai-build/claude/local-dev/...")
				parts := strings.SplitN(u.EmittedFiles[0], "/", 4)
				if len(parts) >= 3 {
					label = strings.Join(parts[:3], "/")
				}
			}
			out.debug("%s %s", formatPhaseSuccess("unit"), label)

			if len(u.EmittedFiles) > 0 {
				out.debug("  Emitted files (%d):", len(u.EmittedFiles))
				for _, f := range u.EmittedFiles {
					out.debug("    → %s", f)
				}
			}

			if len(u.LoweringRecords) > 0 {
				out.debug("  Lowering decisions (%d):", len(u.LoweringRecords))
				for _, lr := range u.LoweringRecords {
					out.debug("    %s: %s → %s [%s] %s",
						lr.ObjectID, lr.FromKind, lr.ToKind, lr.Status, lr.Reason)
				}
			}

			if len(u.SkippedObjects) > 0 {
				out.debug("  Skipped objects (%d):", len(u.SkippedObjects))
				for _, s := range u.SkippedObjects {
					out.debug("    ✗ %s", s)
				}
			}

			if len(u.SelectedProviders) > 0 {
				out.debug("  Selected providers:")
				for cap, prov := range u.SelectedProviders {
					out.debug("    %s → %s", cap, prov)
				}
			}
		}
	}

	out.info(formatSummary(len(targets), filesWritten, totalLowerings, totalErrors))
	out.debug("Completed in %s", report.Duration)

	return nil
}

func resolveTargets(raw []string) ([]build.Target, error) {
	if len(raw) == 0 {
		return nil, nil // nil means all targets
	}

	var targets []build.Target
	for _, r := range raw {
		r = strings.TrimSpace(strings.ToLower(r))
		switch r {
		case "all":
			return nil, nil
		case "claude":
			targets = append(targets, build.TargetClaude)
		case "cursor":
			targets = append(targets, build.TargetCursor)
		case "copilot":
			targets = append(targets, build.TargetCopilot)
		case "codex":
			targets = append(targets, build.TargetCodex)
		default:
			return nil, fmt.Errorf("unknown target %q; valid targets: claude, cursor, copilot, codex, all", r)
		}
	}
	return targets, nil
}

// exitError wraps an error with a specific exit code.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
