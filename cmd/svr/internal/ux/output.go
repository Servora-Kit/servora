package ux

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Style tokens using AdaptiveColor for light/dark theme support.
var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#CC3333", Dark: "#FF7F7F"})

	Info = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#AAAAAA"})

	Success = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#2D7A2D", Dark: "#B8E6B8"})

	Warn = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#CC7700", Dark: "#FFAA33"})

	Error = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#CC2222", Dark: "#FF6B6B"})

	Path = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#66AAFF"})

	Counter = lipgloss.NewStyle().
		Bold(true)

	Summary = lipgloss.NewStyle().
		Bold(true)
)

// PrintSuccess prints a success message with a green checkmark.
func PrintSuccess(title, msg string) {
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", Success.Render("✓"), Success.Render(title+" "+msg))
}

// PrintError prints an error message with a red cross to stderr.
func PrintError(title, msg string) {
	_, _ = fmt.Fprintf(os.Stderr, "%s %s\n", Error.Render("✗"), Error.Render(title+" "+msg))
}

// PrintInfo prints an informational message.
func PrintInfo(msg string) {
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", Info.Render(msg))
}

// PrintProgress prints a progress indicator in the format [current/total] msg.
func PrintProgress(current, total int, msg string) {
	counter := Counter.Render(fmt.Sprintf("[%d/%d]", current, total))
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", counter, msg)
}

// PrintSummary prints a parseable summary line.
func PrintSummary(success, failed int) {
	line := fmt.Sprintf("Summary: success=%d failed=%d", success, failed)
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", Summary.Render(line))
}

// PrintFailureDetail prints a single failure detail line.
func PrintFailureDetail(service, errorType, message string) {
	_, _ = fmt.Fprintf(os.Stderr, "- %s [%s] %s\n", service, errorType, message)
}

// PrintDryRun prints dry-run output showing planned paths.
func PrintDryRun(daoPath, poPath string) {
	_, _ = fmt.Fprintf(os.Stdout, "[DRY-RUN] Would generate to:\n")
	_, _ = fmt.Fprintf(os.Stdout, "  DAO: %s\n", Path.Render(daoPath))
	_, _ = fmt.Fprintf(os.Stdout, "  PO:  %s\n", Path.Render(poPath))
}

// PrintDBConnected prints a database connection success message.
func PrintDBConnected(driver string) {
	PrintSuccess("DB connected", fmt.Sprintf("(driver: %s)", driver))
}

// PrintGenerated prints a generation success message.
func PrintGenerated(serviceName, daoPath, poPath string) {
	PrintSuccess(serviceName, "generated")
	_, _ = fmt.Fprintf(os.Stdout, "  DAO: %s\n", Path.Render(daoPath))
	_, _ = fmt.Fprintf(os.Stdout, "  PO:  %s\n", Path.Render(poPath))
}
