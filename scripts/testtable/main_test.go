package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseEvents_counts(t *testing.T) {
	in := `
{"Action":"run","Package":"github.com/nisrulz/commit-pilot/tests","Test":"TestFoo"}
{"Action":"pass","Package":"github.com/nisrulz/commit-pilot/tests","Test":"TestFoo","Elapsed":0.01}
{"Action":"pass","Package":"github.com/nisrulz/commit-pilot/tests","Test":"TestPlanFromSummariesSuccess","Elapsed":0.01}
{"Action":"fail","Package":"github.com/nisrulz/commit-pilot/tests/e2e","Test":"TestBaz","Elapsed":0.01}
{"Action":"skip","Package":"github.com/nisrulz/commit-pilot/tests","Test":"TestQuux","Elapsed":0.01}
{"Action":"output","Package":"github.com/nisrulz/commit-pilot/tests","Output":"ok  github.com/nisrulz/commit-pilot/tests 0.1s coverage: 70.8% of statements\n"}
{"Action":"pass","Package":"github.com/nisrulz/commit-pilot/tests"}
`
	rows, pass, fail, skip, coverage := parseEvents(strings.NewReader(in))
	if pass != 2 || fail != 1 || skip != 1 {
		t.Fatalf("pass=%d fail=%d skip=%d, want 2/1/1", pass, fail, skip)
	}
	if len(rows) != 4 {
		t.Fatalf("rows=%d, want 4", len(rows))
	}
	if rows[0].category != "other" || rows[0].test != "foo" || rows[1].category != "planning" || rows[2].category != "end to end" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if len(coverage) != 1 || !strings.HasPrefix(coverage[0], "coverage: 70.8%") {
		t.Fatalf("unexpected coverage: %v", coverage)
	}
}

func TestParseEvents_dedupesCoverage(t *testing.T) {
	in := `
{"Action":"output","Package":"github.com/nisrulz/commit-pilot/tests","Output":"ok  github.com/nisrulz/commit-pilot/tests 0.1s coverage: 70.8% of statements\n"}
{"Action":"output","Package":"github.com/nisrulz/commit-pilot/tests","Output":"coverage: 70.8% of statements\n"}
{"Action":"output","Package":"github.com/nisrulz/commit-pilot/tests","Output":"coverage: [no statements]\n"}
{"Action":"output","Package":"github.com/nisrulz/commit-pilot/tests","Output":"coverage: [no statements]\n"}
`
	_, _, _, _, coverage := parseEvents(strings.NewReader(in))
	if len(coverage) != 1 {
		t.Fatalf("expected only real coverage lines, got %v", coverage)
	}
	if !strings.HasPrefix(coverage[0], "coverage: 70.8%") {
		t.Fatalf("unexpected coverage: %v", coverage)
	}
}

func TestParseEvents_ignoresMalformed(t *testing.T) {
	in := "not json\n{\"Action\":\"output\"}\n"
	rows, pass, fail, skip, _ := parseEvents(strings.NewReader(in))
	if len(rows) != 0 || pass != 0 || fail != 0 || skip != 0 {
		t.Fatalf("expected no rows, got %#v %d/%d/%d", rows, pass, fail, skip)
	}
}

func TestShortPackage(t *testing.T) {
	if got := shortPackage("github.com/nisrulz/commit-pilot/tests/e2e"); got != "tests/e2e" {
		t.Fatalf("got %q, want tests/e2e", got)
	}
	if got := shortPackage("github.com/nisrulz/commit-pilot/tests"); got != "tests" {
		t.Fatalf("got %q, want tests", got)
	}
}

func TestHumanizeTestName(t *testing.T) {
	cases := map[string]string{
		"TestPlanFromSummariesSuccess":                                        "plan from summaries success",
		"TestEndToEndPlanOutAndApply":                                         "end to end plan out and apply",
		"TestCallLLMTruncation":                                               "call llm truncation",
		"TestCLIJSONOutput":                                                   "CLI JSON output",
		"TestEndToEndRejectsPartiallyStagedFile/--staged":                     "end to end rejects partially staged file / --staged",
		"TestEndToEndInstallScript/installs_binary_without_touching_cwd":      "end to end install script / installs binary without touching cwd",
	}
	for in, want := range cases {
		if got := humanizeTestName(in); got != want {
			t.Errorf("humanizeTestName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCategoryForTest(t *testing.T) {
	cases := map[string]string{
		"TestExtractJSON_nested":                                   "json parsing",
		"TestParseArgsValueFlags":                                  "cli & args",
		"TestCLIJSONOutput":                                        "cli & args",
		"TestConfigDefaults":                                       "config",
		"TestEstimateTokens_basic":                                 "context window",
		"TestParseSummary_validJSON":                               "summaries",
		"TestSplitFileIntoChunks":                                  "batching & chunking",
		"TestFilterFilesHonorsIncludeAndExclude":                   "filtering",
		"TestGetGitChangesScopes":                                  "git changes",
		"TestMergeCommitGroups_multiple":                           "commit groups",
		"TestPlanFromSummariesSuccess":                             "planning",
		"TestCallLLMTruncation":                                    "llm & providers",
		"TestResponseFormatSchemaPerPrompt":                        "llm & providers",
		"TestLoadPromptAndSections":                                "prompts",
		"TestPrintJSON":                                            "output & formatting",
		"TestSummarizeChangesChunksLargeFilesAndOwnsPath":          "summaries",
		"TestUnknownThing":                                         "other",
	}
	for in, want := range cases {
		if got := categoryForTest(in); got != want {
			t.Errorf("categoryForTest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderReport_containsRows(t *testing.T) {
	var buf bytes.Buffer
	renderReport(&buf, []outcome{
		{category: "planning", test: "foo", status: "PASS"},
		{category: "end to end", test: "baz", status: "FAIL"},
	}, 1, 1, 0, []string{"coverage: 70.8% of statements"})
	out := buf.String()
	for _, want := range []string{"Planning", "End To End", "TEST", "RESULT", "foo", "baz", "Passed: 1", "Failed: 1", "Total: 2", "coverage: 70.8%"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderReport output missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "Planning") < strings.Index(out, "End To End") {
		t.Errorf("expected sorted groups, end to end before planning:\n%s", out)
	}
}
