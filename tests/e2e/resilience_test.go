package e2e_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// jsonPlan returns a complete plan covering every file listed in the prompt.
func jsonPlan(prompt string) string {
	seen := map[string]bool{}
	var files []string
	for _, m := range summaryRE.FindAllStringSubmatch(prompt, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			files = append(files, m[1])
		}
	}
	body, _ := json.Marshal([]map[string]any{{
		"subject":     "feat: update files",
		"description": "updated files",
		"files":       files,
	}})
	return string(body)
}

// summarizeJSON returns a valid summary response for the requested file.
func summarizeJSON(prompt string) string {
	file := "unknown.go"
	if m := fileTagRE.FindStringSubmatch(prompt); m != nil {
		if files := parseFilesFromList(m[1]); len(files) > 0 {
			file = files[0]
		}
	}
	body, _ := json.Marshal(map[string]any{
		"file":    file,
		"summary": "summary for " + file,
		"changes": []string{"changed " + file},
	})
	return string(body)
}

// resilientRespond dispatches the usual summarize response and a plan response
// selected by the test through the planRespond closure.
func resilientRespond(planRespond func(prompt string) string) func(prompt string) string {
	return func(prompt string) string {
		if strings.Contains(prompt, "code change summarizer") {
			return summarizeJSON(prompt)
		}
		return planRespond(prompt)
	}
}

func TestEndToEndRepairsTruncatedPlan(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	// The plan response is cut off mid-array; ExtractJSON must repair it.
	mock := newMockOpenAI(t, resilientRespond(func(prompt string) string {
		return `[{"subject":"feat: update files","description":"updated files","files":["a.go"]`
	}))
	stdout, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "", "--json", "--yes")
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed\n%s", result["status"], stdout)
	}
	if got := lastCommitFiles(t, repo); len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("expected a.go committed, got %v", got)
	}
	if mock.calls() != 2 {
		t.Fatalf("expected 2 AI calls (1 summarize + 1 plan), got %d", mock.calls())
	}
}

func TestEndToEndRetriesPlanWithoutJSON(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	// First plan response is pure prose; the strict re-prompt must elicit JSON.
	mock := newMockOpenAI(t, resilientRespond(func(prompt string) string {
		if strings.Contains(prompt, "ONLY a single valid JSON array") {
			return jsonPlan(prompt)
		}
		return "I looked at the files but I will not output JSON."
	}))
	stdout, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "", "--json", "--yes")
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed\n%s", result["status"], stdout)
	}
	if mock.calls() != 3 {
		t.Fatalf("expected 3 AI calls (1 summarize + 2 plan), got %d", mock.calls())
	}
}

func TestEndToEndFallsBackWhenResponseFormatRejected(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	mock := newMockOpenAI(t, resilientRespond(jsonPlan))
	// Reject any request that asks for structured output; the client must
	// degrade json_schema -> json_object -> plain and still succeed.
	mock.status = func(prompt string, hasResponseFormat bool) int {
		if hasResponseFormat {
			return 400
		}
		return 200
	}
	stdout, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "", "--json", "--yes")
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed\n%s", result["status"], stdout)
	}
	// Summarize and plan each try json_schema, then json_object, then plain.
	if mock.calls() != 6 {
		t.Fatalf("expected 6 AI calls (3 per stage with fallbacks), got %d", mock.calls())
	}
	if got := lastCommitFiles(t, repo); len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("expected a.go committed, got %v", got)
	}
}

func TestEndToEndRetriesTruncatedPlan(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	mock := newMockOpenAI(t, resilientRespond(jsonPlan))
	// The first plan completion reports finish_reason "length"; the client must
	// retry the plan with a larger output budget. Later plan calls succeed.
	planCalls := 0
	mock.reason = func(prompt string) string {
		if strings.Contains(prompt, "git commit planner") {
			planCalls++
			if planCalls == 1 {
				return "length"
			}
		}
		return "stop"
	}
	stdout, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "", "--json", "--yes")
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed\n%s", result["status"], stdout)
	}
	if mock.calls() != 3 {
		t.Fatalf("expected 3 AI calls (1 summarize + 2 plan), got %d", mock.calls())
	}
	if got := lastCommitFiles(t, repo); len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("expected a.go committed, got %v", got)
	}
}
