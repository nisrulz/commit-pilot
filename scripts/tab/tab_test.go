package tab

import (
	"bytes"
	"strings"
	"testing"
)

func TestRender_titlesAndRows(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, []Group{
		{Title: "repo & changes", Rows: []Row{{Name: "detects non-git directory", Status: "PASS"}}},
		{Title: "binary files", Rows: []Row{{Name: "detects binary files", Status: "FAIL"}, {Name: "search max context", Status: "SKIP"}}},
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
	var buf bytes.Buffer
	Render(&buf, []Group{{
		Title: "x",
		Rows: []Row{
			{Name: "a", Status: "PASS"},
			{Name: "b", Status: "FAIL"},
			{Name: "c", Status: "SKIP"},
		},
	}})
	out := buf.String()
	for code, text := range map[string]string{
		"\x1b[34m":  "blue title",
		"\x1b[36m":  "cyan header and border",
		"\x1b[33m":  "yellow test names",
		"\x1b[32m":  "green pass",
		"\x1b[31m":  "red fail",
		"\x1b[93m":  "orange skip",
	} {
		if !strings.Contains(out, code) {
			t.Errorf("missing %s (%s):\n%s", text, code, out)
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
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}
