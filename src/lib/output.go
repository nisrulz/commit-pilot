package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/nisrulz/commit-pilot/internal/spinner"
)

var (
	quietOutput   bool
	jsonOutput    bool
	commitResults []CommitGroup
)

func SetOutputMode(quiet, json bool) {
	quietOutput = quiet || json
	jsonOutput = json
	commitResults = nil
}

func IsQuietOutput() bool { return quietOutput }

func IsJSONOutput() bool { return jsonOutput }

func RecordCommit(group CommitGroup) { commitResults = append(commitResults, group) }

func PrintRunResult(status string) {
	if jsonOutput {
		PrintJSON(map[string]any{"status": status, "commits": commitResults})
	}
}

func PrintError(message string) {
	if jsonOutput {
		PrintJSON(ErrorResult(message))
	}
}

func ErrorResult(message string) map[string]any {
	return map[string]any{"status": "error", "error": message}
}

func PrintJSON(value any) {
	data, _ := json.Marshal(value)
	fmt.Println(string(data))
}

// Die reports a fatal error in the current output mode and exits with code 1.
func Die(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if IsJSONOutput() {
		PrintError(message)
	} else {
		Error(message)
	}
	os.Exit(1)
}

// PrintContextError explains a context-window overflow and suggests fixes.
func PrintContextError(err *ContextLengthError) {
	if IsJSONOutput() {
		PrintJSON(map[string]any{"status": "error", "error": err.Message, "estimated_tokens": err.Estimated, "context_window": err.Available})
		return
	}
	fmt.Println()
	fmt.Fprintf(os.Stderr, "  %s %s\n", red("ERROR:"), sanitizeLine(err.Message, 2000))
	fmt.Fprintf(os.Stderr, "    Estimated tokens: %s\n", FormatNumber(err.Estimated))
	fmt.Fprintf(os.Stderr, "    Context window:   %s tokens\n", FormatNumber(err.Available))
	fmt.Println()
	fmt.Fprintf(os.Stderr, "  %s To fix this, you can:\n", yellow("SUGGESTIONS:"))
	fmt.Fprintf(os.Stderr, "    1. Increase context_window in your config file\n")
	fmt.Fprintf(os.Stderr, "    2. Stage fewer files at once\n")
	fmt.Fprintf(os.Stderr, "    3. Use a model with larger context window\n")
}

// FormatNumber renders a count compactly (e.g. 1500 -> "1k").
func FormatNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

// Pluralize appends "s" unless the count is exactly one.
func Pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// reportNoChanges prints the outcome when there is nothing to commit.
func reportNoChanges(cfg Config) {
	if cfg.JSON {
		PrintRunResult("no_changes")
	} else if !cfg.Quiet {
		fmt.Printf("  %s No changes to commit.\n", yellow("\u26a1"))
	}
}

const WrapWidth = 72

func WrapText(text string, width int) []string {
	var lines []string
	runes := []rune(text)
	for len(runes) > 0 {
		if len(runes) <= width {
			lines = append(lines, string(runes))
			break
		}
		idx := width
		for idx > 0 && runes[idx] != ' ' {
			idx--
		}
		if idx == 0 {
			idx = width
		}
		lines = append(lines, string(runes[:idx]))
		runes = runes[idx:]
		if len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return lines
}

var (
	green   = color.New(color.FgGreen).SprintfFunc()
	yellow  = color.New(color.FgYellow).SprintfFunc()
	cyan    = color.New(color.FgCyan).SprintfFunc()
	magenta = color.New(color.FgMagenta).SprintfFunc()
	bold    = color.New(color.Bold).SprintfFunc()
	red     = color.New(color.FgRed).SprintfFunc()
)

func PrintStep(msg string) {
	if quietOutput {
		return
	}
	fmt.Printf("  %s %s\n", green("*"), sanitizeLine(msg, 2000))
}

func PrintProcessing(msg string) {
	if quietOutput {
		return
	}
	fmt.Printf("  %s %s\n", yellow(">"), sanitizeLine(msg, 2000))
}

// Success prints a green confirmation line to stdout, honoring quiet mode.
func Success(msg string) {
	if quietOutput {
		return
	}
	fmt.Printf("  %s %s\n", green("✔"), sanitizeLine(msg, 2000))
}

// Successf prints a formatted green confirmation line to stdout.
func Successf(format string, args ...any) {
	Success(fmt.Sprintf(format, args...))
}

// Warning prints a yellow warning line to stderr.
func Warning(msg string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", yellow("!"), sanitizeLine(msg, 2000))
}

// Warningf prints a formatted yellow warning line to stderr.
func Warningf(format string, args ...any) {
	Warning(fmt.Sprintf(format, args...))
}

// Error prints a red error line to stderr.
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", red("!"), sanitizeLine(msg, 2000))
}

// Errorf prints a formatted red error line to stderr.
func Errorf(format string, args ...any) {
	Error(fmt.Sprintf(format, args...))
}

// startSpinner shows an animated working indicator during a long-running
// operation unless output is suppressed (quiet or JSON mode). It returns a
// function that stops and clears the spinner.
func startSpinner() func() {
	if quietOutput {
		return func() {}
	}
	return spinner.Start()
}

// PrintProbeHeader reports that provider reachability is being checked.
func PrintProbeHeader(msg string) {
	if quietOutput {
		return
	}
	fmt.Printf("  %s %s\n", cyan("•"), sanitizeLine(msg, 2000))
}

// PrintProbeResult prints one provider reachability row, green for reachable
// and red for unreachable.
func PrintProbeResult(name, url string, ok bool) {
	if quietOutput {
		return
	}
	mark := red("✗")
	if ok {
		mark = green("✓")
	}
	fmt.Printf("      %-10s %-34s %s\n", name, url, mark)
}

// PrintProviderSelected confirms the provider chosen for the run and the model
// it will use.
func PrintProviderSelected(name, base, model string) {
	if quietOutput {
		return
	}
	fmt.Printf("  %s Using provider: %s (%s)\n", green("✓"), bold(name), base)
	fmt.Printf("    -> Model: %s\n", sanitizeLine(model, 1024))
}

// PrintSeparator prints the rule that closes the startup header.
func PrintSeparator() {
	if quietOutput {
		return
	}
	fmt.Println(strings.Repeat("=", bannerSeparatorWidth))
}

func PrintCommitSection(subject, description string, filePaths []string, dryRun bool) {
	if quietOutput {
		return
	}
	subject = sanitizeText(subject, MaxSubjectLength)
	description = sanitizeText(description, MaxDescriptionLength)
	statusTag := "committed!"
	colorFn := green
	iconChar := "*"
	if dryRun {
		colorFn = yellow
		statusTag = "dry-run, skipped"
		iconChar = "!"
	}

	fmt.Printf("  %s %s\n", colorFn(iconChar), bold(subject))
	fmt.Println()

	for _, line := range strings.Split(description, "\n") {
		for _, wl := range WrapText(line, WrapWidth) {
			fmt.Printf("    %s\n", wl)
		}
	}

	fmt.Println()
	fmt.Printf("    %s %s\n", cyan(">"), cyan("files:"))
	for _, f := range filePaths {
		fmt.Printf("      %s %s\n", cyan("-"), cyan(sanitizePath(f)))
	}
	fmt.Println()
	fmt.Printf("  %s %s\n", colorFn(iconChar), colorFn(statusTag))
	fmt.Println()
}
