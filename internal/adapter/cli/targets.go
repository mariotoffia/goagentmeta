package cli

import (
	"encoding/json"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/spf13/cobra"
)

var (
	targetsJSON bool
)

var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "List supported compilation targets",
	Long:  `Targets lists all supported AI agent targets and their implementation status.`,
	RunE:  runTargets,
}

func init() {
	targetsCmd.Flags().BoolVar(&targetsJSON, "json", false, "output as JSON")
}

type targetInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func runTargets(_ *cobra.Command, _ []string) error {
	targets := []targetInfo{
		{Name: string(build.TargetClaude), Status: "available"},
		{Name: string(build.TargetCursor), Status: "not implemented"},
		{Name: string(build.TargetCopilot), Status: "available"},
		{Name: string(build.TargetCodex), Status: "available"},
	}

	if targetsJSON {
		enc := json.NewEncoder(rootCmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(targets)
	}

	out := newOutputWriter()
	out.info(colorize(colorBold, "Supported Targets:"))
	out.info("")
	out.info("  %-12s %s", "TARGET", "STATUS")
	out.info("  %-12s %s", "------", "------")
	for _, t := range targets {
		status := colorize(colorGreen, t.Status)
		if t.Status == "not implemented" {
			status = colorize(colorYellow, t.Status)
		}
		out.info("  %-12s %s", t.Name, status)
	}

	return nil
}
