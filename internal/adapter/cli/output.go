package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func colorize(color, text string) string {
	if noColor {
		return text
	}
	return color + text + colorReset
}

func formatPhaseStart(phase string) string {
	return colorize(colorCyan, "● ") + colorize(colorBold, phase)
}

func formatPhaseSuccess(phase string) string {
	return colorize(colorGreen, "✓ ") + phase
}

func formatPhaseFailure(phase, message string) string {
	return colorize(colorRed, "✗ ") + phase + ": " + message
}

func formatError(message string) string {
	return colorize(colorRed, "[ERROR] ") + message
}

func formatWarning(message string) string {
	return colorize(colorYellow, "[WARN] ") + message
}

func formatSummary(targets, files, lowerings, errors int) string {
	parts := []string{
		fmt.Sprintf("%d targets", targets),
		fmt.Sprintf("%d files written", files),
		fmt.Sprintf("%d lowerings", lowerings),
	}

	errPart := fmt.Sprintf("%d errors", errors)
	if errors > 0 {
		errPart = colorize(colorRed, errPart)
	}
	parts = append(parts, errPart)

	return colorize(colorBold, "Built ") + strings.Join(parts, ", ")
}

type outputWriter struct {
	w       io.Writer
	verbose bool
}

func newOutputWriter() *outputWriter {
	return &outputWriter{w: os.Stdout, verbose: verbose}
}

func (o *outputWriter) info(format string, args ...any) {
	fmt.Fprintf(o.w, format+"\n", args...)
}

func (o *outputWriter) debug(format string, args ...any) {
	if o.verbose {
		fmt.Fprintf(o.w, format+"\n", args...)
	}
}

func (o *outputWriter) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, formatWarning(msg))
}

func (o *outputWriter) errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, formatError(msg))
}
