package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadResults_counts(t *testing.T) {
	in := "repo & changes|detects non-git directory|PASS\nrepo & changes|empty repo|FAIL\n\nmalformed line\nbinary files|detects binary files|PASS\n"
	rows, pass, fail := readResults(strings.NewReader(in))
	if pass != 2 || fail != 1 {
		t.Fatalf("pass=%d fail=%d, want 2/1", pass, fail)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(rows))
	}
	if rows[0].category != "repo & changes" || rows[0].name != "detects non-git directory" || rows[1].status != "FAIL" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestReadResults_empty(t *testing.T) {
	rows, pass, fail := readResults(strings.NewReader(""))
	if len(rows) != 0 || pass != 0 || fail != 0 {
		t.Fatalf("expected empty result set, got %#v %d %d", rows, pass, fail)
	}
}

func TestRenderReport_groupsByCategory(t *testing.T) {
	var buf bytes.Buffer
	renderReport(&buf, []outcome{
		{category: "binary files", name: "detects binary files", status: "PASS"},
		{category: "repo & changes", name: "detects non-git directory", status: "PASS"},
		{category: "binary files", name: "mixed binary/text handled", status: "FAIL"},
	}, 2, 1)
	out := buf.String()
	for _, want := range []string{"Binary Files", "Repo & Changes", "detects binary files", "detects non-git directory", "mixed binary/text handled", "Passed: 2", "Failed: 1", "Total: 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderReport output missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "Binary Files") > strings.Index(out, "Repo & Changes") {
		t.Errorf("expected binary files group before repo & changes:\n%s", out)
	}
}

func TestRunScript_RecordsResults(t *testing.T) {
	dir := t.TempDir()
	results := filepath.Join(dir, "results")
	script := filepath.Join(dir, "suite.sh")
	body := "#!/bin/sh\n" +
		"printf 'suite output line\\n'\n" +
		"echo 'repo & changes|detects non-git directory|PASS' >> \"$COMMIT_PILOT_LIVE_RESULTS\"\n" +
		"echo 'repo & changes|detects no changes|FAIL' >> \"$COMMIT_PILOT_LIVE_RESULTS\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := runScript(script, results, &out, &errOut); err != nil {
		t.Fatalf("runScript: %v", err)
	}

	if !strings.Contains(out.String(), "suite output line") {
		t.Fatalf("script stdout not captured: %q", out.String())
	}
	rows, pass, fail := readResultsFile(results)
	if pass != 1 || fail != 1 || len(rows) != 2 {
		t.Fatalf("pass=%d fail=%d rows=%d, want 1/1/2", pass, fail, len(rows))
	}
	if rows[0].category != "repo & changes" || rows[0].name != "detects non-git directory" {
		t.Fatalf("unexpected first row: %#v", rows[0])
	}
}

func TestReadResultsFile_missing(t *testing.T) {
	rows, pass, fail := readResultsFile(filepath.Join(t.TempDir(), "nope"))
	if len(rows) != 0 || pass != 0 || fail != 0 {
		t.Fatalf("expected empty result set, got %#v %d %d", rows, pass, fail)
	}
}
