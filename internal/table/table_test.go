package table

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestRender_titlesAndRows(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, []string{"TEST", "RESULT"}, []Group{
		{Title: "Repo & Changes", Rows: []Row{{Cells: []string{"detects non-git directory", "PASS"}}}},
		{Title: "Binary Files", Rows: []Row{{Cells: []string{"detects binary files", "FAIL"}}, {Cells: []string{"search max context", "SKIP"}}}},
	})
	out := buf.String()
	for _, want := range []string{"Repo & Changes", "Binary Files", "TEST", "RESULT", "detects non-git directory", "detects binary files", "search max context"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "Binary Files") < strings.Index(out, "Repo & Changes") {
		t.Errorf("expected first group before second:\n%s", out)
	}
}

func TestRender_colors(t *testing.T) {
	prev := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = prev }()

	var buf bytes.Buffer
	Render(&buf, []string{"TEST", "RESULT"}, []Group{{
		Title: "x",
		Rows: []Row{
			{Cells: []string{"a", "PASS"}},
			{Cells: []string{"b", "FAIL"}},
			{Cells: []string{"c", "SKIP"}},
		},
	}})
	out := buf.String()
	for code, text := range map[string]string{
		"\x1b[34m": "blue title",
		"\x1b[36m": "cyan header and border",
		"\x1b[33m": "yellow first column",
		"\x1b[32m": "green pass",
		"\x1b[31m": "red fail",
		"\x1b[93m": "orange skip",
	} {
		if !strings.Contains(out, code) {
			t.Errorf("missing %s (%s):\n%s", text, code, out)
		}
	}
}

func TestRender_uncoloredFirstColumn(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, []string{"SUBJECT", "FILES"}, []Group{{
		Title: "Plan",
		Rows:  []Row{{Cells: []string{"feat: add widget", "widget.go, widget_test.go"}}},
	}})
	out := buf.String()
	for _, want := range []string{"SUBJECT", "FILES", "feat: add widget", "widget.go, widget_test.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"repo & changes":  "Repo & Changes",
		"binary files":    "Binary Files",
		"end to end":      "End To End",
		"planning":        "Planning",
		"llm & providers": "LLM & Providers",
		"cli & args":      "CLI & Args",
		"json parsing":    "JSON Parsing",
	}
	for in, want := range cases {
		if got := TitleCase(in); got != want {
			t.Errorf("TitleCase(%q) = %q, want %q", in, got, want)
		}
	}
}
