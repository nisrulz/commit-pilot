// Package tab renders the two-column result tables shared by the live-test and
// make-test reporters. Group titles are blue, table headers and borders cyan,
// test names yellow, and results green, red, or orange, so pass, fail, and
// skipped results read at a glance.
package tab

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Test statuses understood by the renderer.
const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
	StatusSkip = "SKIP"
)

// ANSI color codes applied to the report.
const (
	reset  = "\x1b[0m"
	blue   = "\x1b[34m"
	cyan   = "\x1b[36m"
	cyanB  = "\x1b[1;36m"
	green  = "\x1b[32m"
	red    = "\x1b[31m"
	yellow = "\x1b[33m"
	orange = "\x1b[93m"
)

// Row is a single test result.
type Row struct {
	Name   string
	Status string
}

// Group is a titled set of rows rendered as one table.
type Group struct {
	Title string
	Rows  []Row
}

// Render prints each group as a titled table. Titles are title-cased, so
// "repo & changes" reads as "Repo & Changes".
func Render(w io.Writer, groups []Group) {
	first := true
	for _, g := range groups {
		if !first {
			fmt.Fprintln(w)
		}
		first = false
		fmt.Fprintf(w, "  %s%s%s\n", blue, titleCase(g.Title), reset)
		renderTable(w, g.Rows)
	}
}

// renderTable prints one group's rows as a bordered table.
func renderTable(w io.Writer, rows []Row) {
	left := runeLen("TEST")
	right := runeLen("RESULT")
	for _, r := range rows {
		if n := runeLen(r.Name); n > left {
			left = n
		}
		if n := runeLen(r.Status); n > right {
			right = n
		}
	}

	border := fmt.Sprintf("+%s+%s+", dashes(left), dashes(right))
	fmt.Fprintln(w, paint(cyan, border))
	fmt.Fprint(w, paint(cyan, "| "), paint(cyanB, runePad("TEST", left)), paint(cyan, " | "), paint(cyanB, runePad("RESULT", right)), paint(cyan, " |"), "\n")
	fmt.Fprintln(w, paint(cyan, border))
	for _, r := range rows {
		fmt.Fprint(w, paint(cyan, "| "), paint(yellow, runePad(r.Name, left)), paint(cyan, " | "), paint(statusColor(r.Status), runePad(r.Status, right)), paint(cyan, " |"), "\n")
	}
	fmt.Fprintln(w, paint(cyan, border))
}

// paint wraps s in an ANSI color code.
func paint(code, s string) string {
	return code + s + reset
}

// dashes builds the horizontal run between the two columns.
func dashes(width int) string {
	return strings.Repeat("-", width+2)
}

// runePad right-pads s to width using spaces.
func runePad(s string, width int) string {
	n := runeLen(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// runeLen returns the display width of s in runes.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// statusColor returns the ANSI code for a status value.
func statusColor(status string) string {
	switch status {
	case StatusFail:
		return red
	case StatusSkip:
		return orange
	default:
		return green
	}
}

// acronyms keeps short technical terms in their proper casing when a title is
// title-cased, so "llm & providers" reads as "LLM & Providers".
var acronyms = map[string]string{
	"llm":  "LLM",
	"cli":  "CLI",
	"json": "JSON",
	"ai":   "AI",
}

// titleCase capitalizes the first letter of each word, so "repo & changes"
// reads as "Repo & Changes".
func titleCase(s string) string {
	fields := strings.Fields(s)
	for i, field := range fields {
		if field == "&" {
			continue
		}
		if proper, ok := acronyms[field]; ok {
			fields[i] = proper
			continue
		}
		fields[i] = strings.ToUpper(field[:1]) + field[1:]
	}
	return strings.Join(fields, " ")
}
