package lib_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lib "github.com/nisrulz/commit-pilot/src/lib"
)

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

func TestReadPlanSingleObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`{"subject":"feat: single","description":"","files":["a.go"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	groups, err := lib.ReadPlan(path)
	if err != nil {
		t.Fatalf("single-object plan should be accepted: %v", err)
	}
	if len(groups) != 1 || groups[0].Subject != "feat: single" {
		t.Fatalf("unexpected plan: %#v", groups)
	}
}

func TestReadPlanEmptyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`[]`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.ReadPlan(path); err == nil {
		t.Fatal("expected error for empty plan array")
	}
}

func TestReadPlanMissingFile(t *testing.T) {
	if _, err := lib.ReadPlan(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing plan file")
	}
}

func TestReadPlanInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`not json`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.ReadPlan(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestConventionalSubject(t *testing.T) {
	valid := []string{
		"feat: add search",
		"fix(parser): handle unicode",
		"chore(deps): bump color to 1.19",
		"refactor!: drop legacy flag",
		"feat(api)!: change response shape",
		"docs: update README",
	}
	for _, s := range valid {
		if err := lib.LintPlan([]lib.CommitGroup{{Subject: s, Files: []string{"a.go"}}}, []string{"a.go"}, lib.Config{Conventional: true}); err != nil {
			t.Errorf("subject %q should be conventional: %v", s, err)
		}
	}

	invalid := []string{
		"",
		"feat",
		"feat:",
		"feat: ",
		"feat no colon",
		"feat:no space",
		"Feat: uppercase type",
		"feat(scope):",
		"():bad scope",
	}
	for _, s := range invalid {
		groups := []lib.CommitGroup{{Subject: s, Files: []string{"a.go"}}}
		if err := lib.LintPlan(groups, []string{"a.go"}, lib.Config{Conventional: true}); err == nil {
			t.Errorf("subject %q should be rejected", s)
		}
	}
}

func TestLintPlanSubjectLengthRunes(t *testing.T) {
	// 50 multi-byte runes exceeds a 40-rune limit.
	subject := strings.Repeat("界", 50)
	groups := []lib.CommitGroup{{Subject: subject, Files: []string{"a.go"}}}
	if err := lib.LintPlan(groups, []string{"a.go"}, lib.Config{Conventional: false, MaxSubjectLength: 40}); err == nil {
		t.Fatal("expected subject length error")
	}
	if err := lib.LintPlan(groups, []string{"a.go"}, lib.Config{Conventional: false, MaxSubjectLength: 60}); err != nil {
		t.Fatalf("subject within limit should pass: %v", err)
	}
}

func TestValidatePlanDuplicateFiles(t *testing.T) {
	groups := []lib.CommitGroup{
		{Subject: "feat: a", Files: []string{"a.go"}},
		{Subject: "feat: b", Files: []string{"a.go"}},
	}
	if err := lib.ValidatePlan(groups, []string{"a.go", "b.go"}); err == nil {
		t.Fatal("expected duplicate-file error")
	}
}

func TestValidatePlanMissingCoverage(t *testing.T) {
	groups := []lib.CommitGroup{{Subject: "feat: a", Files: []string{"a.go"}}}
	err := lib.ValidatePlan(groups, []string{"a.go", "b.go"})
	if err == nil || !strings.Contains(err.Error(), "does not cover") {
		t.Fatalf("expected missing-coverage error, got %v", err)
	}
}

func TestSplitFileIntoChunks(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString("some fairly long line of diff content for token estimation purposes\n")
	}
	fd := lib.FileDiff{Path: "big.go", Diff: sb.String()}
	chunks := lib.SplitFileIntoChunks(fd, 2000)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	var joined []string
	for _, c := range chunks {
		if c.Path != "big.go" {
			t.Fatalf("chunk path = %q, want big.go", c.Path)
		}
		joined = append(joined, c.Diff)
	}
	if strings.Join(joined, "\n") != fd.Diff {
		t.Fatal("chunked content must reassemble to the original diff")
	}
}

func TestSplitFileIntoChunksNegativeBudget(t *testing.T) {
	if chunks := lib.SplitFileIntoChunks(lib.FileDiff{Path: "a.go", Diff: "x"}, 0); chunks != nil {
		t.Fatalf("expected nil for non-positive budget, got %v", chunks)
	}
}

func TestParseArgsValueFlags(t *testing.T) {
	flags, _ := lib.ParseArgs([]string{"--plan-out", "p.json", "--apply", "a.json", "--plan-lint", "l.json", "--include", "*.go", "--include", "*.md", "--exclude", "vendor/"})
	if flags.PlanOut != "p.json" {
		t.Fatalf("PlanOut = %q, want p.json", flags.PlanOut)
	}
	if flags.Apply != "a.json" {
		t.Fatalf("Apply = %q, want a.json", flags.Apply)
	}
	if flags.PlanLint != "l.json" {
		t.Fatalf("PlanLint = %q, want l.json", flags.PlanLint)
	}
	if len(flags.Include) != 2 || flags.Include[0] != "*.go" || flags.Include[1] != "*.md" {
		t.Fatalf("Include = %v, want two globs", flags.Include)
	}
	if len(flags.Exclude) != 1 || flags.Exclude[0] != "vendor/" {
		t.Fatalf("Exclude = %v, want [vendor/]", flags.Exclude)
	}
}

func TestParseArgsMissingValueDoesNotPanic(t *testing.T) {
	flags, _ := lib.ParseArgs([]string{"--plan-out"})
	if flags.PlanOut != "" {
		t.Fatalf("expected empty PlanOut when value missing, got %q", flags.PlanOut)
	}
}

func TestParseArgsListAndDoctorFlags(t *testing.T) {
	flags, _ := lib.ParseArgs([]string{"--list-models", "--json", "--quiet", "--staged", "--unstaged", "--include-sensitive", "--cleanup"})
	if !flags.ListModels || !flags.JSON || !flags.Quiet || !flags.Staged || !flags.Unstaged || !flags.IncludeSensitive || !flags.Cleanup {
		t.Fatalf("unexpected flags: %+v", flags)
	}
}

func TestShouldIncludePath(t *testing.T) {
	// Includes restrict.
	if !lib.ShouldIncludePath("cmd/main.go", []string{"cmd/*"}, nil, true) {
		t.Error("include glob should match cmd/main.go")
	}
	if lib.ShouldIncludePath("docs/x.md", []string{"cmd/*"}, nil, true) {
		t.Error("include glob should reject docs/x.md")
	}
	// Excludes win over includes.
	if lib.ShouldIncludePath("cmd/main.go", []string{"cmd/*"}, []string{"*main.go"}, true) {
		t.Error("exclude should win over include")
	}
	// Sensitive paths are filtered unless explicitly allowed.
	if lib.ShouldIncludePath(".env", nil, nil, false) {
		t.Error("sensitive path should be excluded by default")
	}
	if !lib.ShouldIncludePath(".env", nil, nil, true) {
		t.Error("--include-sensitive should allow .env")
	}
	if !lib.ShouldIncludePath("package-lock.json", nil, nil, false) {
		t.Error("lock files should be included by default")
	}
	if !lib.ShouldIncludePath("src/main.go", nil, nil, false) {
		t.Error("ordinary source path should be included")
	}
}

func TestIgnorePatternsReadsProjectFile(t *testing.T) {
	dir := newGitRepo(t)
	t.Chdir(dir)
	ignorePath := filepath.Join(dir, ".commitpilotignore")
	if err := os.WriteFile(ignorePath, []byte("# comment\n*.log\ngenerated/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	patterns := lib.IgnorePatterns()
	if len(patterns) != 2 || patterns[0] != "*.log" || patterns[1] != "generated/" {
		t.Fatalf("unexpected ignore patterns: %v", patterns)
	}
	if lib.ShouldIncludePath("app.log", nil, nil, true) {
		t.Error("*.log from project ignore file should be excluded")
	}
	if !lib.ShouldIncludePath("src/main.go", nil, nil, true) {
		t.Error("unignored file should be included")
	}
}

func TestExecuteCommitTruncatesSubject(t *testing.T) {
	dir := newGitRepo(t)
	t.Chdir(dir)
	writeFile(t, dir, "a.go", "package a\n")
	runGit(t, dir, "add", "a.go")

	subject := strings.Repeat("x", 150)
	if ok := lib.ExecuteCommit([]string{"a.go"}, subject, "", false, 100, lib.ScopeStaged); !ok {
		t.Fatal("ExecuteCommit failed")
	}
	head := strings.TrimSpace(runGitOutput(t, dir, "log", "-1", "--format=%s"))
	if len([]rune(head)) != 100 {
		t.Fatalf("subject length = %d, want 100: %q", len([]rune(head)), head)
	}
}

func TestExecuteCommitEmptySubjectFallback(t *testing.T) {
	dir := newGitRepo(t)
	t.Chdir(dir)
	writeFile(t, dir, "a.go", "package a\n")
	runGit(t, dir, "add", "a.go")
	if ok := lib.ExecuteCommit([]string{"a.go"}, "---\n", "body", false, 100, lib.ScopeStaged); !ok {
		t.Fatal("ExecuteCommit failed")
	}
	head := strings.TrimSpace(runGitOutput(t, dir, "log", "-1", "--format=%s"))
	if head != "chore: update" {
		t.Fatalf("fallback subject = %q, want 'chore: update'", head)
	}
}

func TestExecuteCommitReportsRejectedHookWithStdout(t *testing.T) {
	dir := newGitRepo(t)
	t.Chdir(dir)
	writeFile(t, dir, "a.go", "package a\n")
	runGit(t, dir, "add", "a.go")

	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho rejected\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if lib.ExecuteCommit([]string{"a.go"}, "test: reject", "", false, 100, lib.ScopeStaged) {
		t.Fatal("rejected commit was reported as successful")
	}
	if subject := strings.TrimSpace(runGitOutput(t, dir, "log", "-1", "--format=%s")); subject != "init" {
		t.Fatalf("unexpected commit created: %q", subject)
	}
}

func TestGetGitChangesScopes(t *testing.T) {
	dir := newGitRepo(t)
	t.Chdir(dir)
	writeFile(t, dir, "staged.txt", "s\n")
	writeFile(t, dir, "unstaged.txt", "u\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "base")
	writeFile(t, dir, "staged.txt", "s2\n")
	writeFile(t, dir, "unstaged.txt", "u2\n")
	runGit(t, dir, "add", "staged.txt")

	staged, err := lib.GetGitChangesForScope(lib.ScopeStaged)
	if err != nil {
		t.Fatalf("staged scope: %v", err)
	}
	if len(staged.AllFiles) != 1 || staged.AllFiles[0] != "staged.txt" {
		t.Fatalf("staged scope files = %v, want [staged.txt]", staged.AllFiles)
	}

	unstaged, err := lib.GetGitChangesForScope(lib.ScopeUnstaged)
	if err != nil {
		t.Fatalf("unstaged scope: %v", err)
	}
	if len(unstaged.AllFiles) != 1 || unstaged.AllFiles[0] != "unstaged.txt" {
		t.Fatalf("unstaged scope files = %v, want [unstaged.txt]", unstaged.AllFiles)
	}

	auto, err := lib.GetGitChangesForScope(lib.ScopeAuto)
	if err != nil {
		t.Fatalf("auto scope: %v", err)
	}
	// Auto prefers staged changes.
	if len(auto.AllFiles) != 1 || auto.AllFiles[0] != "staged.txt" {
		t.Fatalf("auto scope files = %v, want [staged.txt]", auto.AllFiles)
	}
}

func TestGetGitChangesFingerprintTracksContent(t *testing.T) {
	dir := newGitRepo(t)
	t.Chdir(dir)
	writeFile(t, dir, "a.go", "package a\n")
	runGit(t, dir, "add", "a.go")
	first, err := lib.GetGitChanges()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.go", "package a\n// changed\n")
	runGit(t, dir, "add", "a.go")
	second, err := lib.GetGitChanges()
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == second.Fingerprint {
		t.Fatal("fingerprint should change when content changes")
	}
}

func TestCheckProviderMatchesModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
	}))
	defer server.Close()
	found, err := lib.CheckProvider(lib.Config{APIBase: server.URL, Model: "test-model"})
	if err != nil || !found {
		t.Fatalf("expected model found: found=%v err=%v", found, err)
	}
	found, err = lib.CheckProvider(lib.Config{APIBase: server.URL, Model: "missing-model"})
	if err != nil || found {
		t.Fatalf("expected model not found: found=%v err=%v", found, err)
	}
}

func TestPlanFromSummariesFallback(t *testing.T) {
	groups, err := lib.PlanFromSummaries("Summaries:\n{diff}", lib.Config{}, `not json`)
	if err == nil || groups != nil {
		t.Fatalf("expected error for invalid AI plan response, got %v", groups)
	}
}

func TestLoadPromptCustomFile(t *testing.T) {
	// The config file's prompt key is honored in ResolveConfig.
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "prompt: custom prompt\n")
	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.Prompt != "custom prompt" {
		t.Fatalf("Prompt = %q, want custom prompt", cfg.Prompt)
	}
}

func TestIsBinaryDiffNullByte(t *testing.T) {
	if !lib.IsBinaryDiff("text\x00more") {
		t.Fatal("null byte should mark binary")
	}
	if lib.IsBinaryDiff("plain text\nline two") {
		t.Fatal("plain text should not be marked binary")
	}
}
