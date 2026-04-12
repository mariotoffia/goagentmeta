package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new .ai/ directory structure",
	Long: `Init creates a .ai/ directory with a default manifest and example
objects to get started quickly.`,
	RunE: runInit,
}

func runInit(_ *cobra.Command, _ []string) error {
	out := newOutputWriter()

	aiDir := ".ai"
	if _, err := os.Stat(aiDir); err == nil {
		return fmt.Errorf(".ai/ directory already exists; remove it first or use a different directory")
	}

	dirs := []string{
		aiDir,
		filepath.Join(aiDir, "instructions"),
		filepath.Join(aiDir, "rules"),
		filepath.Join(aiDir, "skills"),
		filepath.Join(aiDir, "agents"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
		out.debug("Created %s/", d)
	}

	manifest := `# .ai Manifest — goagentmeta project configuration
schema_version: 1
project:
  name: my-project
  description: "AI agent metadata project"

targets:
  - claude
  - copilot
  - codex

profile: local-dev
`
	if err := os.WriteFile(filepath.Join(aiDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write manifest.yaml: %w", err)
	}

	exampleInstruction := `kind: instruction
id: code-style
version: 1
description: "Code style guidelines"
scope:
  file_types:
    - "*.go"
    - "*.ts"
content: |
  Follow consistent naming conventions and write clear comments.
`
	if err := os.WriteFile(
		filepath.Join(aiDir, "instructions", "code-style.yaml"),
		[]byte(exampleInstruction), 0o644,
	); err != nil {
		return fmt.Errorf("write example instruction: %w", err)
	}

	exampleRule := `kind: rule
id: no-secrets
version: 1
description: "Prevent secrets in code"
preservation: required
content: |
  Never commit API keys, tokens, or passwords to source code.
`
	if err := os.WriteFile(
		filepath.Join(aiDir, "rules", "no-secrets.yaml"),
		[]byte(exampleRule), 0o644,
	); err != nil {
		return fmt.Errorf("write example rule: %w", err)
	}

	out.info(colorize(colorGreen, "✓") + " Initialized .ai/ directory")
	out.info("  manifest:    .ai/manifest.yaml")
	out.info("  instruction: .ai/instructions/code-style.yaml")
	out.info("  rule:        .ai/rules/no-secrets.yaml")
	out.info("")
	out.info("Next: edit these files and run " + colorize(colorBold, "goagentmeta build"))

	return nil
}
