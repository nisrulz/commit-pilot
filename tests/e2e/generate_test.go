package e2e_test

import (
	"encoding/json"
	"testing"
)

// mockConfig writes a YAML config file pointing at the mock provider and
// returns its path to pass via --config.
func mockConfig(t *testing.T, mock *mockOpenAI) string {
	t.Helper()
	return writeConfig(t, "provider: openai_compat\nmodel: test-model\nbase_url: "+mock.url()+"\n")
}

func TestEndToEndAutoModeCommits(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "src/app.go", "package app\nfunc Run() {}\n")
	writeFile(t, repo, "docs/readme.md", "# readme\n")
	writeFile(t, repo, "go.mod", "module example\n")
	runGit(t, repo, "add", "-A")

	mock := newMockOpenAI(t, mockRespond)
	stdout, stderr, err := runCLI(t, bin, repo, nil, "y\n", "--json", "--yes", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed", result["status"])
	}
	// Auto mode: 3 summarizes + 1 plan.
	if mock.calls() != 4 {
		t.Fatalf("expected 4 AI calls (3 summarize + 1 plan), got %d", mock.calls())
	}
	if got := lastCommitFiles(t, repo); len(got) != 3 {
		t.Fatalf("expected all 3 files committed, got %v", got)
	}
	if out := runGitOutput(t, repo, "status", "--porcelain"); out != "" {
		t.Fatalf("working tree not clean after run:\n%s", out)
	}
}

func TestEndToEndSingleModeOneCommit(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	writeFile(t, repo, "b.go", "package b\n")
	runGit(t, repo, "add", "-A")

	mock := newMockOpenAI(t, mockRespond)
	stdout, _, err := runCLI(t, bin, repo, nil, "y\n", "--json", "--yes", "--single", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed", result["status"])
	}
	if mock.calls() != 1 {
		t.Fatalf("expected 1 AI call, got %d", mock.calls())
	}
	// Single mode adds exactly one commit on top of the initial one.
	if got := commitCount(t, repo); got != 2 {
		t.Fatalf("expected 1 commit, got %d", got)
	}
	// Single mode committed both files in one commit.
	if got := lastCommitFiles(t, repo); len(got) != 2 {
		t.Fatalf("expected 2 files in commit, got %v", got)
	}
}

func TestEndToEndDryRunCreatesNoCommits(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	mock := newMockOpenAI(t, mockRespond)
	stdout, stderr, err := runCLI(t, bin, repo, nil, "", "--json", "--dry-run", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "dry_run" {
		t.Fatalf("status = %v, want dry_run", result["status"])
	}
	if commits := result["commits"]; commits == nil {
		t.Fatalf("expected proposed commits in dry-run output: %s", stdout)
	}
	// Only the initial commit should exist; nothing was committed.
	if count := commitCount(t, repo); count != 1 {
		t.Fatalf("dry-run created commits: %d", count)
	}
	if out := runGitOutput(t, repo, "status", "--porcelain"); out == "" {
		t.Fatal("expected changes to remain uncommitted after dry-run")
	}
}

func TestEndToEndDeclinedConfirmation(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	mock := newMockOpenAI(t, mockRespond)
	stdout, stderr, err := runCLI(t, bin, repo, nil, "n\n", "--json", "--single", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "cancelled" {
		t.Fatalf("status = %v, want cancelled", result["status"])
	}
	// Only the initial commit should exist; the declined run committed nothing.
	if count := commitCount(t, repo); count != 1 {
		t.Fatalf("declined run created commits: %d", count)
	}
}
