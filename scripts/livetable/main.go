// Command livetable runs the live test suite under a working spinner and
// renders the results as grouped tables. It drives scripts/live-test.sh, which
// makes the real AI calls, and prints the script's output once it finishes so
// the spinner stays clean on screen.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nisrulz/commit-pilot/internal/spinner"
	"github.com/nisrulz/commit-pilot/internal/table"
)

// outcome is the result of a single live test.
type outcome struct {
	category string
	name     string
	status   string
}

func main() {
	results, err := os.CreateTemp("", "commit-pilot-live.XXXXXX")
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create results file: %v\n", err)
		os.Exit(1)
	}
	resultsPath := results.Name()
	defer os.Remove(resultsPath)

	script := filepath.Join(repoRoot(), "scripts", "live-test.sh")
	var outBuf, errBuf bytes.Buffer
	runErr := runScript(script, resultsPath, &outBuf, &errBuf)

	os.Stdout.Write(outBuf.Bytes())
	os.Stderr.Write(errBuf.Bytes())

	rows, pass, fail := readResultsFile(resultsPath)
	if len(rows) > 0 {
		renderReport(os.Stdout, rows, pass, fail)
	}
	if fail > 0 || runErr != nil {
		os.Exit(1)
	}
}

// runScript executes the given shell script under a working spinner, capturing
// its stdout and stderr into the given writers. The results file path is passed
// to the script so it knows where to record test outcomes.
func runScript(script, resultsPath string, out, errOut io.Writer) error {
	cmd := exec.Command(script)
	cmd.Env = append(os.Environ(), "COMMIT_PILOT_LIVE_RESULTS="+resultsPath)
	cmd.Stdout = out
	cmd.Stderr = errOut
	stop := spinner.Start()
	defer stop()
	return cmd.Run()
}

// repoRoot returns the repository root. make targets run from the root, so the
// current working directory is the repo root.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// readResultsFile loads the outcomes from the results file the script wrote.
func readResultsFile(path string) ([]outcome, int, int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0
	}
	defer f.Close()
	return readResults(f)
}

// readResults parses "category|name|status" lines from r, skipping blank and
// malformed lines, and returns the outcomes plus pass and fail counts.
func readResults(r io.Reader) ([]outcome, int, int) {
	var rows []outcome
	var pass, fail int
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			continue
		}
		category := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		status := strings.TrimSpace(parts[2])
		if status != "PASS" && status != "FAIL" {
			continue
		}
		rows = append(rows, outcome{category: category, name: name, status: status})
		if status == "PASS" {
			pass++
		} else {
			fail++
		}
	}
	return rows, pass, fail
}

// renderReport prints the outcomes grouped by category, followed by totals.
func renderReport(w io.Writer, rows []outcome, pass, fail int) {
	type group struct {
		title string
		rows  []table.Row
	}
	var groups []group
	index := map[string]int{}
	for _, row := range rows {
		tr := table.Row{Cells: []string{row.name, row.status}}
		if i, ok := index[row.category]; ok {
			groups[i].rows = append(groups[i].rows, tr)
			continue
		}
		index[row.category] = len(groups)
		groups = append(groups, group{title: row.category, rows: []table.Row{tr}})
	}

	tgs := make([]table.Group, len(groups))
	for i, g := range groups {
		tgs[i] = table.Group{Title: table.TitleCase(g.title), Rows: g.rows}
	}
	table.Render(w, []string{"TEST", "RESULT"}, tgs)

	fmt.Fprintf(w, "\n  Passed: %d   Failed: %d   Total: %d\n", pass, fail, pass+fail)
}
