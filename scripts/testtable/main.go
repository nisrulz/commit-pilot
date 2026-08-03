// Command testtable runs the unit test suite and prints each test as a row in
// a result table. It mirrors `make test` (with coverage) but renders an
// aligned, readable report instead of the default Go test output.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"github.com/nisrulz/commit-pilot/internal/spinner"
	"github.com/nisrulz/commit-pilot/scripts/tab"
)

// testEvent is one newline-delimited `go test -json` event.
type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

// outcome is the result of a single test.
type outcome struct {
	category string
	test     string
	status   string
}

// overrides maps test names that do not split into readable words to a clear
// title, so the report reads like the curated live-test names.
var overrides = map[string]string{
	"TestCLIJSONOutput": "CLI JSON output",
}

func main() {
	type testResult struct {
		out, errOut []byte
		err         error
	}
	done := make(chan testResult, 1)
	go func() {
		out, errOut, err := runGoTest()
		done <- testResult{out: out, errOut: errOut, err: err}
	}()

	stop := spinner.Start()
	res := <-done
	stop()

	rows, pass, fail, skip, coverage := parseEvents(bytes.NewReader(res.out))
	renderReport(os.Stdout, rows, pass, fail, skip, coverage)
	if res.err != nil {
		if fail == 0 {
			fmt.Fprint(os.Stderr, string(res.errOut))
		}
		os.Exit(1)
	}
	if fail > 0 {
		os.Exit(1)
	}
}

// runGoTest runs the same suite as `make test` and returns its stdout (the
// JSON events), its stderr, and the command error. The suite can fail while
// still emitting valid events, so the output is returned regardless.
func runGoTest() ([]byte, []byte, error) {
	cmd := exec.Command("go", "test", "-count=1", "-json", "./tests/...", "-coverpkg=./src/lib/,./src/lib/provider/")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// parseEvents turns test events into outcome rows and collects the coverage
// summary lines. Events without a test (package-level actions) are skipped.
func parseEvents(r io.Reader) ([]outcome, int, int, int, []string) {
	var rows []outcome
	var pass, fail, skip int
	var coverage []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var ev testEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			if ev.Test == "" {
				continue
			}
			status := strings.ToUpper(ev.Action)
			rows = append(rows, outcome{category: categoryFor(ev), test: humanizeTestName(ev.Test), status: status})
			switch ev.Action {
			case "pass":
				pass++
			case "fail":
				fail++
			case "skip":
				skip++
			}
		case "output":
			if idx := strings.Index(ev.Output, "coverage:"); idx >= 0 {
				line := strings.TrimSpace(ev.Output[idx:])
				if strings.Contains(line, "[no statements]") {
					continue
				}
				if !slices.Contains(coverage, line) {
					coverage = append(coverage, line)
				}
			}
		}
	}
	return rows, pass, fail, skip, coverage
}

// shortPackage strips the module path so "github.com/nisrulz/commit-pilot/tests"
// reads as "tests".
func shortPackage(pkg string) string {
	const prefix = "github.com/nisrulz/commit-pilot/"
	return strings.TrimPrefix(pkg, prefix)
}

// categoryFor assigns a human-readable group to a test event. End-to-end tests
// form one group; unit tests are grouped by the area they cover, in the same
// style as the curated live-test categories.
func categoryFor(ev testEvent) string {
	if shortPackage(ev.Package) == "tests/e2e" {
		return "end to end"
	}
	return categoryForTest(ev.Test)
}

// categoryForTest maps a unit test name to the responsibility it covers. Rules
// are checked in order so names like "PlanFromSummaries" land in planning
// rather than summaries, and "ResponseFormat..." in llm rather than prompts.
func categoryForTest(name string) string {
	switch {
	case strings.Contains(name, "ExtractJSON"):
		return "json parsing"
	case strings.Contains(name, "ParseArgs"), strings.Contains(name, "CLI"):
		return "cli & args"
	case strings.Contains(name, "Config"), strings.Contains(name, "ConfigDir"), strings.Contains(name, "ResolveConfig"), strings.Contains(name, "TmpDir"):
		return "config"
	case strings.Contains(name, "ContextLength"), strings.Contains(name, "QueryModelInfo"),
		strings.Contains(name, "SearchMaxContext"), strings.Contains(name, "GetSystemRAM"),
		strings.Contains(name, "ModelInfo"), strings.Contains(name, "ParseEstimatedMemory"),
		strings.Contains(name, "CanFitInContext"), strings.Contains(name, "AvailableDiffTokens"),
		strings.Contains(name, "EstimatePromptTokens"), strings.Contains(name, "EstimateTokens"):
		return "context window"
	case strings.Contains(name, "Plan"):
		return "planning"
	case strings.Contains(name, "Summary"), strings.Contains(name, "Summaries"), strings.Contains(name, "Summarize"):
		return "summaries"
	case strings.Contains(name, "SplitFile"), strings.Contains(name, "SplitFiles"),
		strings.Contains(name, "Chunk"), strings.Contains(name, "TruncateDiff"),
		strings.Contains(name, "IsChunkedBatch"), strings.Contains(name, "Batch"):
		return "batching & chunking"
	case strings.Contains(name, "Filter"), strings.Contains(name, "ShouldIncludePath"),
		strings.Contains(name, "IgnorePatterns"), strings.Contains(name, "IsSensitivePath"):
		return "filtering"
	case strings.Contains(name, "GetGitChanges"), strings.Contains(name, "AllFilePaths"),
		strings.Contains(name, "IsBinaryDiff"):
		return "git changes"
	case strings.Contains(name, "AssignBinary"), strings.Contains(name, "MergeCommitGroup"),
		strings.Contains(name, "ParseCommitGroup"), strings.Contains(name, "ConventionalSubject"),
		strings.Contains(name, "ExecuteCommit"), strings.Contains(name, "ConfirmCommitPlan"):
		return "commit groups"
	case strings.Contains(name, "Plan"):
		return "planning"
	case strings.Contains(name, "CallLLM"), strings.Contains(name, "ResponseFormat"),
		strings.Contains(name, "Provider"), strings.Contains(name, "Probe"),
		strings.Contains(name, "ValidateProviderURL"), strings.Contains(name, "Unsloth"),
		strings.Contains(name, "OpenAI"):
		return "llm & providers"
	case strings.Contains(name, "Prompt"), strings.Contains(name, "SectionByName"),
		strings.Contains(name, "ApplyMessagePreferences"), strings.Contains(name, "FormatDiffSection"),
		strings.Contains(name, "SanitizeDiff"):
		return "prompts"
	case strings.Contains(name, "Print"), strings.Contains(name, "WrapText"),
		strings.Contains(name, "FormatNumber"), strings.Contains(name, "Pluralize"), strings.Contains(name, "ErrorResult"):
		return "output & formatting"
	default:
		return "other"
	}
}

// humanizeTestName turns a Go test function name into a readable title, the
// same style as the curated live-test names. It strips the Test prefix, splits
// camelCase into words, lowercases the result, and appends any subtest name.
func humanizeTestName(name string) string {
	if title, ok := overrides[name]; ok {
		return title
	}
	parent, sub, _ := strings.Cut(name, "/")
	title := splitWords(strings.TrimPrefix(parent, "Test"))
	if sub != "" {
		title += " / " + strings.ReplaceAll(sub, "_", " ")
	}
	return strings.ToLower(title)
}

// splitWords splits a CamelCase string into words, breaking before an
// uppercase letter that follows a lowercase one or that ends an acronym run.
func splitWords(s string) string {
	if s == "" {
		return ""
	}
	var words []string
	start := 0
	for i := 1; i < len(s); i++ {
		prev, cur := s[i-1], s[i]
		if cur >= 'A' && cur <= 'Z' {
			prevLower := prev >= 'a' && prev <= 'z'
			nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
			prevUpper := prev >= 'A' && prev <= 'Z'
			if prevLower || (prevUpper && nextLower) {
				words = append(words, s[start:i])
				start = i
			}
		}
	}
	words = append(words, s[start:])
	return strings.Join(words, " ")
}

// renderReport prints the outcomes grouped by category, followed by coverage
// and totals. Groups are sorted so the report is stable across runs.
func renderReport(w io.Writer, rows []outcome, pass, fail, skip int, coverage []string) {
	type group struct {
		title string
		rows  []tab.Row
	}
	var groups []group
	index := map[string]int{}
	for _, row := range rows {
		tr := tab.Row{Name: row.test, Status: row.status}
		if i, ok := index[row.category]; ok {
			groups[i].rows = append(groups[i].rows, tr)
			continue
		}
		index[row.category] = len(groups)
		groups = append(groups, group{title: row.category, rows: []tab.Row{tr}})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].title < groups[j].title })

	tgs := make([]tab.Group, len(groups))
	for i, g := range groups {
		tgs[i] = tab.Group{Title: g.title, Rows: g.rows}
	}
	tab.Render(w, tgs)

	for _, line := range coverage {
		fmt.Fprintf(w, "  %s\n", line)
	}
	fmt.Fprintf(w, "\n  Passed: %d   Failed: %d   Skipped: %d   Total: %d\n", pass, fail, skip, pass+fail+skip)
}
