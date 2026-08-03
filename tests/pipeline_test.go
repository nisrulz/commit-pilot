package lib_test

import (
	"encoding/json"
	"fmt"
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"strings"
	"testing"
)

func TestParseSummary_validJSON(t *testing.T) {
	text := `{"summary": "Added new feature", "changes": ["added handler", "fixed bug"]}`
	s := lib.ParseSummary(text, "main.go")
	if s.Summary != "Added new feature" {
		t.Fatalf("expected summary 'Added new feature', got '%s'", s.Summary)
	}
	if len(s.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(s.Changes))
	}
	if s.File != "main.go" {
		t.Fatalf("expected file main.go, got '%s'", s.File)
	}
}

func TestParseSummary_jsonBlock(t *testing.T) {
	text := "Here is the summary:\n```json\n{\"summary\": \"Changed config\", \"changes\": [\"updated port\"]}\n```"
	s := lib.ParseSummary(text, "config.go")
	if s.Summary != "Changed config" {
		t.Fatalf("expected summary 'Changed config', got '%s'", s.Summary)
	}
	if len(s.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(s.Changes))
	}
}

func TestParseSummary_emptyJSON(t *testing.T) {
	text := `{"other": "value"}`
	s := lib.ParseSummary(text, "file.go")
	if s.File != "file.go" {
		t.Fatalf("expected file file.go, got '%s'", s.File)
	}
}

func TestParseSummary_invalidJSON(t *testing.T) {
	text := "this is not json at all"
	s := lib.ParseSummary(text, "foo.go")
	if s.File != "foo.go" {
		t.Fatalf("expected file foo.go, got '%s'", s.File)
	}
	if s.Summary != text {
		t.Fatalf("expected fallback summary to be raw text")
	}
}

func TestFallbackSummary_short(t *testing.T) {
	s := lib.FallbackSummary("short text", "a.go")
	if s.Summary != "short text" {
		t.Fatalf("expected 'short text', got '%s'", s.Summary)
	}
	if s.File != "a.go" {
		t.Fatalf("expected file a.go, got '%s'", s.File)
	}
}

func TestFallbackSummary_long(t *testing.T) {
	long := strings.Repeat("x", 1000)
	s := lib.FallbackSummary(long, "big.go")
	if len(s.Summary) >= len(long) {
		t.Fatalf("expected fallback summary to be truncated, got %d chars (original %d)", len(s.Summary), len(long))
	}
	if !strings.HasSuffix(s.Summary, "...") {
		t.Fatal("expected truncated summary to end with '...'")
	}
}

func TestFileSummary_jsonRoundTrip(t *testing.T) {
	original := lib.FileSummary{
		File:    "main.go",
		Summary: "Refactored handler",
		Changes: []string{"removed old code", "added new logic"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded lib.FileSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.File != original.File {
		t.Fatalf("File mismatch: %s != %s", decoded.File, original.File)
	}
	if decoded.Summary != original.Summary {
		t.Fatalf("Summary mismatch: %s != %s", decoded.Summary, original.Summary)
	}
	if len(decoded.Changes) != len(original.Changes) {
		t.Fatalf("lib.Changes count mismatch: %d != %d", len(decoded.Changes), len(original.Changes))
	}
}

func TestFallbackPlan_withFiles(t *testing.T) {
	summaries := `[{"file":"a.go","summary":"added foo","changes":["added foo"]},{"file":"b.go","summary":"fixed bar","changes":["fixed bar"]}]`
	groups := lib.FallbackPlan(summaries)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(groups[0].Files))
	}
	if groups[0].Files[0] != "a.go" || groups[0].Files[1] != "b.go" {
		t.Fatalf("unexpected file list: %v", groups[0].Files)
	}
	if groups[0].Subject != "chore: update changes" {
		t.Fatalf("expected 'chore: update changes', got '%s'", groups[0].Subject)
	}
}

func TestFallbackPlan_emptyJSON(t *testing.T) {
	groups := lib.FallbackPlan(`[]`)
	if groups != nil {
		t.Fatal("expected nil for empty summaries")
	}
}

func TestFallbackPlan_invalidJSON(t *testing.T) {
	groups := lib.FallbackPlan(`not json`)
	if groups != nil {
		t.Fatal("expected nil for invalid JSON")
	}
}

func TestPlanFromSummariesFailsWhenNoJSONOnRetry(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{
		{status: 200, body: chatContent("no json here")},
		{status: 200, body: chatContent("still no json")},
	}}
	cfg := cfgWithDoer(staticDoer{})
	cfg.HTTPClient = doer
	_, err := lib.PlanFromSummaries("Plan:\n{diff}", cfg, summariesJSON)
	if err == nil {
		t.Fatal("expected error when both attempts return no JSON")
	}
	if !strings.Contains(err.Error(), "extract plan") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func bigSummariesJSON(n, summaryLen int) string {
	summaries := make([]lib.FileSummary, n)
	for i := range summaries {
		summaries[i] = lib.FileSummary{
			File:    fmt.Sprintf("file%d.go", i),
			Summary: strings.Repeat("x", summaryLen),
			Changes: []string{"c"},
		}
	}
	b, _ := json.Marshal(summaries)
	return string(b)
}

func TestCompactSummariesForPlan_fits(t *testing.T) {
	out := lib.CompactSummariesForPlan(summariesJSON, "Plan:\n{diff}", 65536)
	if out != summariesJSON {
		t.Fatal("expected unchanged input when it already fits")
	}
}

func TestCompactSummariesForPlan_trims(t *testing.T) {
	in := bigSummariesJSON(20, 5000)
	origTokens := lib.EstimateTokens(in)
	ctx := 16384
	out := lib.CompactSummariesForPlan(in, "Plan:\n{diff}", ctx)
	if out == in {
		t.Fatal("expected compaction for oversized summaries")
	}
	var parsed []lib.FileSummary
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("compacted output is invalid JSON: %v", err)
	}
	if len(parsed) != 20 {
		t.Fatalf("expected 20 summaries, got %d", len(parsed))
	}
	for i, s := range parsed {
		if s.File != fmt.Sprintf("file%d.go", i) {
			t.Fatalf("file path lost at index %d: %q", i, s.File)
		}
		if len([]rune(s.Summary)) >= 5000 {
			t.Fatalf("summary %d not trimmed (%d runes)", i, len([]rune(s.Summary)))
		}
	}
	if got := lib.EstimateTokens(out); got >= origTokens {
		t.Fatalf("compaction did not reduce tokens: %d -> %d", origTokens, got)
	}
	if got := lib.EstimateTokens("Plan:\n" + out); got > ctx {
		t.Fatalf("compacted prompt still exceeds context: %d > %d", got, ctx)
	}
}

func TestCompactSummariesForPlan_keepsShortSummaries(t *testing.T) {
	var original []lib.FileSummary
	_ = json.Unmarshal([]byte(bigSummariesJSON(20, 5000)), &original)
	original[0].Summary = "short"
	b, _ := json.Marshal(original)
	out := lib.CompactSummariesForPlan(string(b), "Plan:\n{diff}", 16384)
	var parsed []lib.FileSummary
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("compacted output is invalid JSON: %v", err)
	}
	if parsed[0].Summary != "short" {
		t.Fatalf("short summary should be kept as-is, got %q", parsed[0].Summary)
	}
}

func TestCompactSummariesForPlan_zeroContext(t *testing.T) {
	in := bigSummariesJSON(5, 2000)
	out := lib.CompactSummariesForPlan(in, "Plan:\n{diff}", 0)
	if out != in {
		t.Fatal("expected unchanged input when no context window is known")
	}
}

func TestCompactSummariesForPlan_invalidJSON(t *testing.T) {
	in := strings.Repeat("nonsense ", 2000)
	out := lib.CompactSummariesForPlan(in, "Plan:\n{diff}", 8000)
	if out != in {
		t.Fatal("expected unchanged input when summaries are unparseable")
	}
}
