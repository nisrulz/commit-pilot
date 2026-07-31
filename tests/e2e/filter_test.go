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

func TestEndToEndRejectsPartiallyStagedFile(t *testing.T) {
	bin := cliBinary(t)
	for _, scope := range []string{"--staged", "--unstaged"} {
		t.Run(scope, func(t *testing.T) {
			repo := newGitRepo(t)
			writeFile(t, repo, "partial.txt", "one\ntwo\n")
			runGit(t, repo, "add", "partial.txt")
			runGit(t, repo, "commit", "-q", "-m", "base")
			writeFile(t, repo, "partial.txt", "ONE\ntwo\n")
			runGit(t, repo, "add", "partial.txt")
			writeFile(t, repo, "partial.txt", "ONE\nTWO\n")

			mock := newMockOpenAI(t, mockRespond)
			_, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "", "--single", "--yes", scope)
			if err == nil {
				t.Fatal("partially staged file should be rejected")
			}
			if !strings.Contains(stderr, "both staged and unstaged") {
				t.Fatalf("missing actionable error: %s", stderr)
			}
			if mock.calls() != 0 {
				t.Fatalf("model was called before scope validation: %d", mock.calls())
			}
			if count := commitCount(t, repo); count != 2 {
				t.Fatalf("unexpected commit created, count=%d", count)
			}
		})
	}
}

func TestEndToEndTreatsGitOptionFilenameLiterally(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "--all", "literal path\n")
	writeFile(t, repo, ".env", "SECRET=value\n")

	mock := newMockOpenAI(t, mockRespond)
	_, stderr, err := runCLI(t, bin, repo, mockEnv(mock), "", "--single", "--yes")
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, stderr)
	}
	if got := lastCommitFiles(t, repo); len(got) != 1 || got[0] != "--all" {
		t.Fatalf("expected only literal --all path committed, got %v", got)
	}
	if staged := strings.TrimSpace(runGitOutput(t, repo, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("filtered files were staged through option injection: %s", staged)
	}
	if status := runGitOutput(t, repo, "status", "--porcelain"); !strings.Contains(status, "?? .env") {
		t.Fatalf("expected .env to remain untracked, got:\n%s", status)
	}
}
