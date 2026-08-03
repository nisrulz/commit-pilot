// Command livetable renders the live-test results as grouped tables. It reads
// one "category|test|status" line per test from stdin and prints a table per
// category, so the shell-driven live test keeps a clean, readable report.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nisrulz/commit-pilot/scripts/tab"
)

// outcome is the result of a single live test.
type outcome struct {
	category string
	name     string
	status   string
}

func main() {
	rows, pass, fail := readResults(os.Stdin)
	renderReport(os.Stdout, rows, pass, fail)
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
		rows  []tab.Row
	}
	var groups []group
	index := map[string]int{}
	for _, row := range rows {
		tr := tab.Row{Name: row.name, Status: row.status}
		if i, ok := index[row.category]; ok {
			groups[i].rows = append(groups[i].rows, tr)
			continue
		}
		index[row.category] = len(groups)
		groups = append(groups, group{title: row.category, rows: []tab.Row{tr}})
	}

	tgs := make([]tab.Group, len(groups))
	for i, g := range groups {
		tgs[i] = tab.Group{Title: g.title, Rows: g.rows}
	}
	tab.Render(w, tgs)

	fmt.Fprintf(w, "\n  Passed: %d   Failed: %d   Total: %d\n", pass, fail, pass+fail)
}
