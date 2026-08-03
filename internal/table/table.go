// Package table renders titled, bordered tables with aligned columns. It is
// shared by the test reporters (testtable, livetable) and the commit-pilot CLI,
// so reports and the commit plan share one look. Colors come from fatih/color
// and drop out automatically when the output is not a terminal.
package table

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
)

// Known test statuses colored by the renderer.
const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
	StatusSkip = "SKIP"
)

// Row is a single table row. Cells must line up with the header columns.
type Row struct {
	Cells []string
}

// Group is a titled set of rows rendered as one bordered table.
type Group struct {
	Title string
	Rows  []Row
}

// paintFn colors a formatted string, matching color.SprintfFunc.
type paintFn func(string, ...any) string

var (
	blue   paintFn = color.New(color.FgBlue).SprintfFunc()
	cyan   paintFn = color.New(color.FgCyan).SprintfFunc()
	cyanB  paintFn = color.New(color.FgCyan, color.Bold).SprintfFunc()
	yellow paintFn = color.New(color.FgYellow).SprintfFunc()
	green  paintFn = color.New(color.FgGreen).SprintfFunc()
	red    paintFn = color.New(color.FgRed).SprintfFunc()
	orange paintFn = color.New(color.FgHiYellow).SprintfFunc()
)

// Render prints each group as a titled, bordered table. Titles are blue,
// borders and headers cyan, the first column yellow, and cells whose value is a
// known status are green, red, or orange, so pass, fail, and skipped results
// read at a glance.
func Render(w io.Writer, headers []string, groups []Group) {
	first := true
	for _, g := range groups {
		if !first {
			fmt.Fprintln(w)
		}
		first = false
		if g.Title != "" {
			fmt.Fprintf(w, "  %s\n", blue(g.Title))
		}
		renderTable(w, headers, g.Rows)
	}
}

// renderTable prints one group's rows as a bordered table with aligned columns.
func renderTable(w io.Writer, headers []string, rows []Row) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = runeLen(h)
	}
	for _, r := range rows {
		for i, cell := range r.Cells {
			if i < len(widths) && runeLen(cell) > widths[i] {
				widths[i] = runeLen(cell)
			}
		}
	}

	border := "+"
	for _, width := range widths {
		border += dashes(width) + "+"
	}
	fmt.Fprintln(w, paint(cyan, border))
	writeRow(w, headers, widths, func(i int, cell string) paintFn { return cyanB })
	fmt.Fprintln(w, paint(cyan, border))
	for _, r := range rows {
		writeRow(w, r.Cells, widths, cellStyle)
	}
	fmt.Fprintln(w, paint(cyan, border))
}

// writeRow prints one row of padded cells between cyan pipes. styleFor picks
// the color of each cell, or nil for an uncolored cell.
func writeRow(w io.Writer, cells []string, widths []int, styleFor func(i int, cell string) paintFn) {
	fmt.Fprint(w, paint(cyan, "|"))
	for i, cell := range cells {
		fmt.Fprint(w, paint(cyan, " "), paint(styleFor(i, cell), runePad(cell, widths[i])), paint(cyan, " |"))
	}
	fmt.Fprintln(w)
}

// cellStyle picks the color for a data cell: the first column is yellow, and a
// cell whose value is a known status is green, red, or orange.
func cellStyle(i int, cell string) paintFn {
	switch cell {
	case StatusPass:
		return green
	case StatusFail:
		return red
	case StatusSkip:
		return orange
	}
	if i == 0 {
		return yellow
	}
	return nil
}

// paint wraps s in an ANSI color code, or returns it unchanged when no color
// applies.
func paint(style paintFn, s string) string {
	if style == nil {
		return s
	}
	return style(s)
}

// dashes builds the horizontal run between two columns.
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

// acronyms keeps short technical terms in their proper casing when a title is
// title-cased, so "llm & providers" reads as "LLM & Providers".
var acronyms = map[string]string{
	"llm":  "LLM",
	"cli":  "CLI",
	"json": "JSON",
	"ai":   "AI",
}

// TitleCase capitalizes the first letter of each word, so "repo & changes"
// reads as "Repo & Changes".
func TitleCase(s string) string {
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
