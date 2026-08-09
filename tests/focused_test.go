package lib_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	lib "github.com/nisrulz/commit-pilot/src/lib"
)

// staticDoer returns a canned HTTP response for every request, letting tests
// drive LLM calls without a network.
type staticDoer struct {
	status int
	body   string
	err    error
}

func (d staticDoer) Do(*http.Request) (*http.Response, error) {
	if d.err != nil {
		return nil, d.err
	}
	return &http.Response{
		StatusCode: d.status,
		Body:       io.NopCloser(strings.NewReader(d.body)),
		Header:     make(http.Header),
	}, nil
}

func chatContent(content string) string {
	body, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]any{"role": "assistant", "content": content},
		}},
	})
	return string(body)
}

func cfgWithDoer(doer staticDoer) lib.Config {
	return lib.Config{
		APIBase:    "http://localhost/v1",
		Model:      "test-model",
		HTTPClient: doer,
		Retries:    1,
		Timeout:    5,
	}
}

// --- WritePlan ---

func TestWritePlanRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	groups := []lib.CommitGroup{
		{Subject: "feat: a", Description: "desc a", Files: []string{"a.go"}},
		{Subject: "fix: b", Description: "", Files: []string{"b.go", "c.go"}},
	}
	if err := lib.WritePlan(path, groups); err != nil {
		t.Fatalf("WritePlan: %v", err)
	}
	got, err := lib.ReadPlan(path)
	if err != nil {
		t.Fatalf("ReadPlan: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(got))
	}
	if got[0].Subject != "feat: a" || got[1].Files[1] != "c.go" {
		t.Fatalf("round-trip mismatch: %#v", got)
	}
}

func TestWritePlanErrorPath(t *testing.T) {
	// A directory as the target forces a write error.
	if err := lib.WritePlan(t.TempDir(), []lib.CommitGroup{{Subject: "feat: a", Files: []string{"a.go"}}}); err == nil {
		t.Fatal("expected error writing plan to a directory")
	}
}

// --- SummarizeChanges ---

func TestSummarizeChangesWritesFileAndReturnsJSON(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "summaries.json")
	cfg := cfgWithDoer(staticDoer{status: 200, body: chatContent(`{"summary":"changed it","changes":["fixed thing"]}`)})
	files := []lib.FileDiff{{Path: "a.go", Diff: "+code"}, {Path: "b.go", Diff: "-old"}}

	out, err := lib.SummarizeChanges(cfg, "Summarize: {files}\n{diff}", files, dst)
	if err != nil {
		t.Fatalf("SummarizeChanges: %v", err)
	}
	var summaries []lib.FileSummary
	if err := json.Unmarshal([]byte(out), &summaries); err != nil {
		t.Fatalf("invalid summary JSON: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].File != "a.go" || summaries[1].File != "b.go" {
		t.Fatalf("unexpected summary files: %v", summaries)
	}
	// The incremental file must exist and contain the same data.
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("summaries file not written: %v", err)
	}
	var fromFile []lib.FileSummary
	if err := json.Unmarshal(data, &fromFile); err != nil {
		t.Fatalf("invalid file JSON: %v", err)
	}
	if len(fromFile) != 2 {
		t.Fatalf("expected 2 summaries in file, got %d", len(fromFile))
	}
}

func TestSummarizeChangesProviderFailure(t *testing.T) {
	cfg := cfgWithDoer(staticDoer{err: io.ErrUnexpectedEOF})
	_, err := lib.SummarizeChanges(cfg, "tpl", []lib.FileDiff{{Path: "a.go", Diff: "x"}}, filepath.Join(t.TempDir(), "s.json"))
	if err == nil {
		t.Fatal("expected error when provider call fails")
	}
}

func TestSummarizeChangesChunksLargeFilesAndOwnsPath(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(chatContent(`{"file":"invented.go","summary":"chunk","changes":["change"]}`)))
	}))
	defer server.Close()

	dst := filepath.Join(t.TempDir(), "summaries.json")
	cfg := lib.Config{APIBase: server.URL, Model: "test", ContextWindow: 8192}
	out, err := lib.SummarizeChanges(cfg, "Summarize: {files}\n{diff}", []lib.FileDiff{{Path: "huge.go", Diff: generateHugeDiff(500)}}, dst)
	if err != nil {
		t.Fatalf("SummarizeChanges: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected multiple chunk requests, got %d", calls.Load())
	}
	var summaries []lib.FileSummary
	if err := json.Unmarshal([]byte(out), &summaries); err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].File != "huge.go" {
		t.Fatalf("model replaced code-owned path: %#v", summaries)
	}
}

// --- PlanFromSummaries ---

const summariesJSON = `[{"file":"a.go","summary":"added a","changes":["a"]},{"file":"b.go","summary":"fixed b","changes":["b"]}]`

func TestPlanFromSummariesSuccess(t *testing.T) {
	cfg := cfgWithDoer(staticDoer{status: 200, body: chatContent(`[{"subject":"feat: ab","description":"d","files":["a.go","b.go"]}]`)})
	groups, err := lib.PlanFromSummaries("Plan:\n{diff}", cfg, summariesJSON)
	if err != nil {
		t.Fatalf("PlanFromSummaries: %v", err)
	}
	if len(groups) != 1 || groups[0].Subject != "feat: ab" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
	if len(groups[0].Files) != 2 {
		t.Fatalf("expected 2 files, got %v", groups[0].Files)
	}
}

func TestPlanFromSummariesSingleObject(t *testing.T) {
	cfg := cfgWithDoer(staticDoer{status: 200, body: chatContent(`{"subject":"feat: one","description":"d","files":["a.go","b.go"]}`)})
	groups, err := lib.PlanFromSummaries("Plan:\n{diff}", cfg, summariesJSON)
	if err != nil {
		t.Fatalf("PlanFromSummaries: %v", err)
	}
	if len(groups) != 1 || groups[0].Subject != "feat: one" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
}

func TestPlanFromSummariesFallsBackWhenAIUnparseable(t *testing.T) {
	// A JSON array of scalars is extractable but not a valid plan, so it
	// triggers the fallback that groups all files into one commit.
	cfg := cfgWithDoer(staticDoer{status: 200, body: chatContent(`[1,2,3]`)})
	groups, err := lib.PlanFromSummaries("Plan:\n{diff}", cfg, summariesJSON)
	if err != nil {
		t.Fatalf("expected fallback plan, got error: %v", err)
	}
	if len(groups) != 1 || groups[0].Subject != "chore: update changes" {
		t.Fatalf("expected fallback plan, got %#v", groups)
	}
}

func TestPlanFromSummariesEmptyGroups(t *testing.T) {
	cfg := cfgWithDoer(staticDoer{status: 200, body: chatContent(`[]`)})
	_, err := lib.PlanFromSummaries("Plan:\n{diff}", cfg, summariesJSON)
	if err == nil {
		t.Fatal("expected error for empty groups response")
	}
}

// --- ResolveConfig coverage ---

func TestResolveConfigFileSettings(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home),
		"provider: openai_compat\nmodel: custom-model\nbase_url: https://custom.test/v1\n"+
			"context_window: 16000\nretries: 5\ntimeout_seconds: 42\n"+
			"max_subject_length: 72\nticket_prefix: \"PLAT-\"\nbody_style: bulleted\n"+
			"conventional: false\nimperative: false\n")
	t.Setenv(lib.EnvAPIKey, "sk-test")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.Model != "custom-model" || cfg.APIBase != "https://custom.test/v1" || cfg.APIKey != "sk-test" {
		t.Fatalf("unexpected provider settings: %+v", cfg)
	}
	if cfg.ContextWindow != 16000 || cfg.Retries != 5 || cfg.Timeout.Seconds() != 42 {
		t.Fatalf("unexpected numeric settings: %+v", cfg)
	}
	if cfg.TicketPrefix != "PLAT-" || cfg.BodyStyle != "bulleted" {
		t.Fatalf("unexpected message preferences: %+v", cfg)
	}
	if cfg.Conventional || cfg.Imperative {
		t.Fatalf("boolean prefs should be false: %+v", cfg)
	}
	if cfg.MaxSubjectLength != 72 {
		t.Fatalf("MaxSubjectLength = %d, want 72", cfg.MaxSubjectLength)
	}
}

func TestResolveConfigProviderDefaults(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "provider: ollama\n")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.Model != "lfm2.5:8b" {
		t.Fatalf("expected Ollama default model, got %q", cfg.Model)
	}
	if cfg.APIBase != "http://localhost:11434/v1" {
		t.Fatalf("expected Ollama default base, got %q", cfg.APIBase)
	}
}

func TestResolveConfigUnknownProviderRejected(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "provider: nonsense\n")

	_, err := lib.ResolveConfig(lib.RawFlags{})
	if err == nil {
		t.Fatal("expected a resolve error for an unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider \"nonsense\"") {
		t.Fatalf("resolve error = %q", err)
	}
}

func TestResolveConfigCustomProviderRequiresBase(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "provider: openai_compat\nbase_url: http://127.0.0.1:9999/v1\n")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if cfg.Provider != "openai_compat" {
		t.Fatalf("Provider = %q, want openai_compat", cfg.Provider)
	}
	if cfg.APIBase != "http://127.0.0.1:9999/v1" {
		t.Fatalf("APIBase = %q, want custom base", cfg.APIBase)
	}
	if cfg.Model != lib.DefaultModel {
		t.Fatalf("expected default model fallback, got %q", cfg.Model)
	}
}

func TestResolveConfigCustomProviderRejectsMissingBase(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "provider: openai_compat\n")

	_, err := lib.ResolveConfig(lib.RawFlags{})
	if err == nil || !strings.Contains(err.Error(), "requires base_url") {
		t.Fatalf("expected missing-base error, got %v", err)
	}
}

func TestResolveConfigKnownProviderNoError(t *testing.T) {
	for _, name := range []string{"ollama", "openai_compat"} {
		home := isolatedHome(t)
		content := "provider: " + name + "\n"
		if name == "openai_compat" {
			content += "base_url: http://127.0.0.1:9999/v1\n"
		}
		writeConfigFile(t, defaultPath(home), content)
		if _, err := lib.ResolveConfig(lib.RawFlags{}); err != nil {
			t.Fatalf("provider %q: unexpected resolve error %v", name, err)
		}
	}
}

// --- Batch edge branches ---

func TestSplitFilesIntoBatchesTinyContext(t *testing.T) {
	files := []lib.FileDiff{{Path: "a.go", Diff: "change"}}
	batches := lib.SplitFilesIntoBatches("tpl", files, 100)
	if len(batches) != 1 || len(batches[0]) != 1 {
		t.Fatalf("expected single batch fallback, got %v", batches)
	}
}

func TestSplitFileIntoChunksTinyBudget(t *testing.T) {
	// Per-chunk budget (budget - 60) goes non-positive.
	if chunks := lib.SplitFileIntoChunks(lib.FileDiff{Path: "a.go", Diff: "x"}, 50); chunks != nil {
		t.Fatalf("expected nil for tiny budget, got %v", chunks)
	}
}

// --- stdout capture helpers ---

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return <-done
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	done := make(chan string)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return <-done
}

func TestPrintErrorJSON(t *testing.T) {
	lib.SetOutputMode(false, true)
	defer lib.SetOutputMode(false, false)

	out := captureStdout(t, func() {
		lib.PrintError("provider unavailable")
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON error: %q", out)
	}
	if result["status"] != "error" || result["error"] != "provider unavailable" {
		t.Fatalf("unexpected error JSON: %q", out)
	}
}

func TestPrintContextErrorText(t *testing.T) {
	err := &lib.ContextLengthError{Message: "too big", Estimated: 9000, Available: 4096}
	out := captureStderr(t, func() {
		lib.PrintContextError(err)
	})
	if !strings.Contains(out, "too big") || !strings.Contains(out, "SUGGESTIONS") {
		t.Fatalf("unexpected text output: %q", out)
	}
}

func TestPrintJSON(t *testing.T) {
	out := captureStdout(t, func() {
		lib.PrintJSON(map[string]any{"key": "value"})
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("PrintJSON output not valid JSON: %q", out)
	}
	if got["key"] != "value" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestPrintRunResultJSONIncludesCommits(t *testing.T) {
	lib.SetOutputMode(false, true)
	lib.RecordCommit(lib.CommitGroup{Subject: "feat: x", Files: []string{"a.go"}})
	defer lib.SetOutputMode(false, false)

	out := captureStdout(t, func() {
		lib.PrintRunResult("completed")
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON result: %q", out)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed", result["status"])
	}
	if commits, ok := result["commits"].([]any); !ok || len(commits) != 1 {
		t.Fatalf("expected 1 recorded commit, got %q", out)
	}
}

func TestPrintContextErrorJSON(t *testing.T) {
	lib.SetOutputMode(false, true)
	defer lib.SetOutputMode(false, false)

	err := &lib.ContextLengthError{Message: "too big", Estimated: 9000, Available: 4096}
	out := captureStdout(t, func() {
		lib.PrintContextError(err)
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %q", out)
	}
	if result["status"] != "error" || result["estimated_tokens"] != float64(9000) {
		t.Fatalf("unexpected context error JSON: %q", out)
	}
}
