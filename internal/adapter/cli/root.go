package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information set by the build system via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Global flags.
var (
	verbose  bool
	noColor  bool
	cfgFile  string
)

var rootCmd = &cobra.Command{
	Use:   "goagentmeta",
	Short: "AI agent metadata compiler",
	Long: `goagentmeta compiles .ai/ source trees into target-specific agent
configuration files for Claude Code, Cursor, GitHub Copilot, and Codex CLI.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: .ai/manifest.yaml)")

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(targetsCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err.Error()))
		return err
	}
	return nil
}
