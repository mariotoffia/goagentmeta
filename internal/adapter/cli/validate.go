package cli

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/mariotoffia/goagentmeta/internal/adapter/filesystem"
	reporteradapter "github.com/mariotoffia/goagentmeta/internal/adapter/reporter"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/normalizer"
	"github.com/mariotoffia/goagentmeta/internal/adapter/stage/validator"
	"github.com/mariotoffia/goagentmeta/internal/application/compiler"
	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [paths...]",
	Short: "Validate .ai/ source files without emitting output",
	Long: `Validate runs the parse and validate phases only. No files are written.

Exit codes:
  0  all files are valid
  1  validation errors found
  2  interrupted`,
	RunE: runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	out := newOutputWriter()

	rootPaths := args
	if len(rootPaths) == 0 {
		rootPaths = []string{"."}
	}

	fsReader := filesystem.NewOSReader()
	sink := reporteradapter.NewDiagnosticSink()

	valStage, err := validator.New()
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	p := compiler.NewPipeline(
		compiler.WithFSReader(fsReader),
		compiler.WithDiagnosticSink(sink),
		compiler.WithFailFast(true),
		compiler.WithProfile(build.ProfileLocalDev),
		compiler.WithStage(valStage),
		compiler.WithStage(normalizer.New(fsReader)),
	)

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	_, err = p.Execute(ctx, rootPaths)
	if err != nil {
		if ctx.Err() != nil {
			out.errorf("interrupted")
			return &exitError{code: 2, err: ctx.Err()}
		}

		diags := sink.Diagnostics()
		for _, d := range diags {
			out.errorf("%s: %s (%s)", d.Phase, d.Message, d.SourcePath)
		}
		if len(diags) == 0 {
			out.errorf("%v", err)
		}
		return &exitError{code: 1, err: err}
	}

	out.info(colorize(colorGreen, "✓") + " validation passed")
	return nil
}
