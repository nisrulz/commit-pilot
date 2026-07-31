package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
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
		fmt.Fprintf(os.Stderr, "  ! %s\n", message)
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
	fmt.Fprintf(os.Stderr, "  %s %s\n", red("ERROR:"), err.Message)
	fmt.Fprintf(os.Stderr, "    Estimated tokens: %s\n", FormatNumber(err.Estimated))
	fmt.Fprintf(os.Stderr, "    Context window:   %s tokens\n", FormatNumber(err.Available))
	fmt.Println()
	fmt.Fprintf(os.Stderr, "  %s To fix this, you can:\n", yellow("SUGGESTIONS:"))
	fmt.Fprintf(os.Stderr, "    1. Increase context window: export COMMIT_PILOT_CONTEXT_WINDOW=131072\n")
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

// reportBinaryOnly prints the outcome when only binary files changed, which
// cannot be summarized into an AI commit message.
func reportBinaryOnly(cfg Config) {
	if cfg.JSON {
		PrintRunResult("binary_only")
	} else if !cfg.Quiet {
		fmt.Printf("  %s Only binary files changed \u2014 cannot generate AI commit message.\n", yellow("\u26a1"))
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
	green  = color.New(color.FgGreen).SprintfFunc()
	yellow = color.New(color.FgYellow).SprintfFunc()
	cyan   = color.New(color.FgCyan).SprintfFunc()
	bold   = color.New(color.Bold).SprintfFunc()
	red    = color.New(color.FgRed).SprintfFunc()
)

func PrintStep(msg string) {
	if quietOutput {
		return
	}
	fmt.Printf("  %s %s\n", green("*"), msg)
}

func PrintProcessing(msg string) {
	if quietOutput {
		return
	}
	fmt.Printf("  %s %s\n", yellow(">"), msg)
}

func PrintCommitSection(subject, description string, filePaths []string, dryRun bool) {
	if quietOutput {
		return
	}
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
		fmt.Printf("      %s %s\n", cyan("-"), cyan(f))
	}
	fmt.Println()
	fmt.Printf("  %s %s\n", colorFn(iconChar), colorFn(statusTag))
	fmt.Println()
}
