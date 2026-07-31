package lib_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIJSONOutput(t *testing.T) {
	binary := buildCLI(t)
	repo := t.TempDir()
	configDir := t.TempDir()
	runGit(t, repo, "init")

	stdout, stderr, err := runCLI(binary, repo, configDir, "--json")
	if err != nil {
		t.Fatalf("no-change run failed: %v, stdout: %s, stderr: %s", err, stdout, stderr)
	}
	assertJSONStatus(t, stdout, "no_changes")

	if err := os.WriteFile(filepath.Join(repo, "file.go"), []byte("package example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = runCLI(binary, repo, configDir, "--apply", "missing.json", "--json")
	if err == nil {
		t.Fatal("missing plan should fail")
	}
	assertJSONStatus(t, stdout, "error")
	if strings.Contains(stderr, "Proposed commit plan") {
		t.Fatalf("unexpected confirmation for failed plan: %s", stderr)
	}

	plan := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(plan, []byte(`[{"subject":"test: cancel plan","description":"","files":["file.go"]}]`), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = runCLI(binary, repo, configDir, "--apply", plan, "--json")
	if err != nil {
		t.Fatalf("cancelled plan should succeed: %v, stderr: %s", err, stderr)
	}
	assertJSONStatus(t, stdout, "cancelled")
	if !strings.Contains(stderr, "Apply this plan?") {
		t.Fatalf("expected confirmation on stderr: %s", stderr)
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Dir(root)
	binary := filepath.Join(t.TempDir(), "commit-pilot")
	cmd := exec.Command("go", "build", "-o", binary, "./src")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func runCLI(binary, dir, configDir string, args ...string) (string, string, error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "COMMIT_PILOT_CONFIG_DIR="+configDir)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func assertJSONStatus(t *testing.T, output, want string) {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output %q: %v", output, err)
	}
	if result["status"] != want {
		t.Fatalf("status = %v, want %s", result["status"], want)
	}
}
