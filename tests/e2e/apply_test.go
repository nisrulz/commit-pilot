package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEndToEndPlanOutAndApply(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "main.go", "package main\n")
	writeFile(t, repo, "util.go", "package main\n")
	runGit(t, repo, "add", "-A")

	planPath := filepath.Join(t.TempDir(), "plan.json")
	mock := newMockOpenAI(t, mockRespond)
	_, stderr, err := runCLI(t, bin, repo, nil, "y\n", "--plan-out", planPath, "--json", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("plan-out run failed: %v\n%s", err, stderr)
	}
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("plan file not written: %v", err)
	}
	if count := commitCount(t, repo); count != 1 {
		t.Fatalf("--plan-out should not commit, count=%d", count)
	}

	stdout, stderr, err := runCLI(t, bin, repo, nil, "", "--apply", planPath, "--dry-run", "--json", "--yes", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("dry-run apply failed: %v\n%s", err, stderr)
	}
	var dryRunResult map[string]any
	if err := json.Unmarshal([]byte(stdout), &dryRunResult); err != nil {
		t.Fatalf("invalid dry-run JSON: %v", err)
	}
	if dryRunResult["status"] != "dry_run" || commitCount(t, repo) != 1 {
		t.Fatalf("dry-run apply changed the repository: %s", stdout)
	}

	// Apply the saved plan.
	stdout, stderr, err = runCLI(t, bin, repo, nil, "y\n", "--apply", planPath, "--json", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("apply run failed: %v\n%s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed", result["status"])
	}
	if got := lastCommitFiles(t, repo); len(got) != 2 {
		t.Fatalf("expected 2 files committed via apply, got %v", got)
	}
}

func TestEndToEndSingleModeWritesPlan(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "main.go", "package main\n")
	runGit(t, repo, "add", "main.go")

	planPath := filepath.Join(t.TempDir(), "plan.json")
	mock := newMockOpenAI(t, mockRespond)
	_, stderr, err := runCLI(t, bin, repo, nil, "", "--single", "--plan-out", planPath, "--json", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("single plan-out run failed: %v\n%s", err, stderr)
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("plan file not written: %v", err)
	}
	var groups []struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(data, &groups); err != nil {
		t.Fatalf("invalid plan JSON: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Files) != 1 || groups[0].Files[0] != "main.go" {
		t.Fatalf("unexpected plan: %s", data)
	}
	if count := commitCount(t, repo); count != 1 {
		t.Fatalf("--plan-out should not commit, count=%d", count)
	}
}

func TestEndToEndApplyBinaryOnlyPlan(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	binary := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46}
	if err := os.WriteFile(filepath.Join(repo, "logo.bin"), binary, 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "logo.bin")
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, []byte(`[{"subject":"chore: update logo","description":"","files":["logo.bin"]}]`), 0600); err != nil {
		t.Fatal(err)
	}

	mock := newMockOpenAI(t, mockRespond)
	stdout, stderr, err := runCLI(t, bin, repo, nil, "y\n", "--apply", planPath, "--json", "--yes", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("apply run failed: %v\n%s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed (not binary_only)", result["status"])
	}
	if mock.calls() != 0 {
		t.Fatalf("apply must not call the model, got %d calls", mock.calls())
	}
	if got := lastCommitFiles(t, repo); len(got) != 1 || got[0] != "logo.bin" {
		t.Fatalf("expected logo.bin committed, got %v", got)
	}
}

func TestEndToEndCommitsBinaryOnlyChangesWithoutModel(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	binary := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}
	if err := os.WriteFile(filepath.Join(repo, "logo.bin"), binary, 0644); err != nil {
		t.Fatal(err)
	}

	mock := newMockOpenAI(t, mockRespond)
	stdout, stderr, err := runCLI(t, bin, repo, nil, "", "--json", "--yes", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("binary-only run failed: %v\n%s", err, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "completed" {
		t.Fatalf("status = %v, want completed", result["status"])
	}
	if mock.calls() != 0 {
		t.Fatalf("binary-only run must not call the model, got %d calls", mock.calls())
	}
	if got := lastCommitFiles(t, repo); len(got) != 1 || got[0] != "logo.bin" {
		t.Fatalf("expected logo.bin committed, got %v", got)
	}
}

func TestEndToEndPlanLint(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, []byte(`[{"subject":"feat: add a","description":"","files":["a.go"]}]`), 0600); err != nil {
		t.Fatal(err)
	}
	mock := newMockOpenAI(t, mockRespond)
	stdout, _, err := runCLI(t, bin, repo, nil, "", "--plan-lint", planPath, "--json", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("plan-lint failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "valid" {
		t.Fatalf("status = %v, want valid", result["status"])
	}
	if mock.calls() != 0 {
		t.Fatalf("plan-lint must not call the model, got %d calls", mock.calls())
	}
}

func TestEndToEndApplyInvalidPlanFails(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, []byte(`[{"subject":"feat: add a","description":"","files":["invented.go"]}]`), 0600); err != nil {
		t.Fatal(err)
	}
	mock := newMockOpenAI(t, mockRespond)
	stdout, _, err := runCLI(t, bin, repo, nil, "y\n", "--apply", planPath, "--json", "--config", mockConfig(t, mock))
	if err == nil {
		t.Fatal("expected failure for invalid plan")
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON error, got: %q", stdout)
	}
	if result["status"] != "error" {
		t.Fatalf("status = %v, want error", result["status"])
	}
}
