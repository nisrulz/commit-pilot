package e2e_test

import (
	"strings"
	"testing"
)

func TestEndToEndSensitiveFilesNeverReachModel(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "main.go", "package main\n")
	writeFile(t, repo, ".env", "SECRET=value\n")
	runGit(t, repo, "add", "-A")

	mock := newMockOpenAI(t, mockRespond)
	_, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "y\n", "--json", "--yes", "--single")
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, stderr)
	}
	if joined := mock.joinedPrompts(); strings.Contains(joined, ".env") {
		t.Fatalf("sensitive path leaked into model prompt:\n%s", joined)
	}
	if got := lastCommitFiles(t, repo); len(got) != 1 || got[0] != "main.go" {
		t.Fatalf("expected only main.go committed, got %v", got)
	}
	// The .env file must remain uncommitted and untouched.
	out := runGitOutput(t, repo, "status", "--porcelain")
	if !strings.Contains(out, ".env") {
		t.Fatalf("expected .env to remain uncommitted, got:\n%s", out)
	}
	// Exactly one AI call: the second-pass check must not re-send the file.
	if mock.calls() != 1 {
		t.Fatalf("expected exactly 1 AI call, got %d", mock.calls())
	}
}

func TestEndToEndStagedScope(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "staged.txt", "s\n")
	writeFile(t, repo, "unstaged.txt", "u\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "base")
	writeFile(t, repo, "staged.txt", "s2\n")
	writeFile(t, repo, "unstaged.txt", "u2\n")
	runGit(t, repo, "add", "staged.txt")

	mock := newMockOpenAI(t, mockRespond)
	_, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "y\n", "--json", "--yes", "--single", "--staged")
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, stderr)
	}
	files := lastCommitFiles(t, repo)
	if len(files) != 1 || files[0] != "staged.txt" {
		t.Fatalf("expected only staged.txt committed, got %v", files)
	}
	out := runGitOutput(t, repo, "status", "--porcelain")
	if !strings.Contains(out, "unstaged.txt") {
		t.Fatalf("unstaged.txt should remain uncommitted, got:\n%s", out)
	}
}
