package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce    sync.Once
	buildBinPath string
	buildBinErr  error
)

// cliBinary builds the commit-pilot binary once per test run and returns its
// path. The build is shared across all E2E tests to keep the suite fast.
func cliBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		cwd, err := os.Getwd()
		if err != nil {
			buildBinErr = err
			return
		}
		root := filepath.Clean(filepath.Join(cwd, "..", ".."))
		dir, err := os.MkdirTemp("", "commit-pilot-e2e-*")
		if err != nil {
			buildBinErr = err
			return
		}
		bin := filepath.Join(dir, "commit-pilot")
		cmd := exec.Command("go", "build", "-o", bin, "./src")
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			buildBinErr = fmt.Errorf("build CLI: %v\n%s", err, output)
			return
		}
		buildBinPath = bin
	})
	if buildBinErr != nil {
		t.Fatal(buildBinErr)
	}
	return buildBinPath
}

// runCLI invokes the built binary in dir with the given environment overrides,
// optional stdin, and args, returning stdout, stderr, and the error.
func runCLI(t *testing.T, bin, dir string, env map[string]string, stdin string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"COMMIT_PILOT_CONFIG_DIR="+t.TempDir(),
		"COMMIT_PILOT_TMP_DIR="+t.TempDir(),
	)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// newGitRepo creates a git repository in a temp directory with an initial
// commit and a configured identity.
func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@test")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init", "-q")
	return dir
}

// writeFile writes content to a path inside dir, creating parent directories.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

// runGitOutput runs a git command in dir and returns its combined output.
func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// lastCommitFiles returns the file paths touched by the most recent commit.
func lastCommitFiles(t *testing.T, dir string) []string {
	t.Helper()
	out := strings.TrimSpace(runGitOutput(t, dir, "show", "--name-only", "--format=", "HEAD"))
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// commitCount returns the total number of commits in the repository.
func commitCount(t *testing.T, dir string) int {
	out := strings.TrimSpace(runGitOutput(t, dir, "rev-list", "--count", "HEAD"))
	count := 0
	_, _ = fmt.Sscanf(out, "%d", &count)
	return count
}
