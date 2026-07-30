package lib_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lib "github.com/nisrulz/commit-pilot/src/lib"
)

func TestParseCommitGroup(t *testing.T) {
	_, err := lib.ParseCommitGroup(`{"subject":"feat: add x","description":"desc","files":["a.go"]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = lib.ParseCommitGroup(`[{"subject":"feat: add x","description":"desc","files":["a.go"]}]`)
	if err != nil {
		t.Fatalf("unexpected error for array input: %v", err)
	}

	_, err = lib.ParseCommitGroup(`{"other":"value"}`)
	if err == nil {
		t.Fatal("expected error for missing subject field")
	}

	_, err = lib.ParseCommitGroup("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	_, err = lib.ParseCommitGroup("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestAllFilePaths(t *testing.T) {
	changes := &lib.Changes{
		AllFiles: []string{"a.go", "b.go", "c.bin"},
		FilesWithDiffs: []lib.FileDiff{
			{Path: "a.go", Diff: "+code"},
			{Path: "b.go", Diff: "-code"},
		},
		BinaryFiles: []string{"c.bin"},
	}
	result := lib.AllFilePaths(changes)
	if len(result) != 3 {
		t.Fatalf("expected 3 paths, got %d: %v", len(result), result)
	}
}

func TestBatchLabel(t *testing.T) {
	single := []lib.FileDiff{{Path: "a.go", Diff: "diff"}}
	label := lib.BatchLabel(single)
	if !strings.Contains(label, "file") {
		t.Fatalf("expected 'file' in label, got '%s'", label)
	}

	multi := []lib.FileDiff{{Path: "a.go", Diff: "d1"}, {Path: "b.go", Diff: "d2"}}
	label = lib.BatchLabel(multi)
	if !strings.Contains(label, "files") {
		t.Fatalf("expected 'files' in label, got '%s'", label)
	}

	chunked := []lib.FileDiff{{Path: "big.go", Diff: "c1"}, {Path: "big.go", Diff: "c2"}}
	label = lib.BatchLabel(chunked)
	if !strings.Contains(label, "chunk") {
		t.Fatalf("expected 'chunk' in label, got '%s'", label)
	}
}

func TestFormatNumber(t *testing.T) {
	if s := lib.FormatNumber(999); s != "999" {
		t.Fatalf("expected '999', got '%s'", s)
	}
	if s := lib.FormatNumber(1000); s != "1k" {
		t.Fatalf("expected '1k', got '%s'", s)
	}
	if s := lib.FormatNumber(1500); s != "1k" {
		t.Fatalf("expected '1k', got '%s'", s)
	}
	if s := lib.FormatNumber(10000); s != "10k" {
		t.Fatalf("expected '10k', got '%s'", s)
	}
}

func TestPluralize(t *testing.T) {
	if s := lib.Pluralize(1, "file"); s != "1 file" {
		t.Fatalf("expected '1 file', got '%s'", s)
	}
	if s := lib.Pluralize(2, "file"); s != "2 files" {
		t.Fatalf("expected '2 files', got '%s'", s)
	}
	if s := lib.Pluralize(0, "file"); s != "0 files" {
		t.Fatalf("expected '0 files', got '%s'", s)
	}
}

func TestContextLengthError(t *testing.T) {
	err := &lib.ContextLengthError{
		Message:   "test error",
		Estimated: 1000,
		Available: 500,
	}
	if err.Error() != "test error" {
		t.Fatalf("expected 'test error', got '%s'", err.Error())
	}
}

func TestParseArgs(t *testing.T) {
	_, showHelp := lib.ParseArgs([]string{"--help"})
	if !showHelp {
		t.Fatal("expected showHelp=true for --help")
	}

	_, showHelp = lib.ParseArgs([]string{"-h"})
	if !showHelp {
		t.Fatal("expected showHelp=true for -h")
	}

	flags, showHelp := lib.ParseArgs([]string{"--single", "--dry-run"})
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if string(flags.Mode) != "1" {
		t.Fatalf("expected mode '1', got '%s'", string(flags.Mode))
	}
	if !flags.DryRun {
		t.Fatal("expected DryRun=true")
	}

	flags, _ = lib.ParseArgs([]string{"--no-commit"})
	if !flags.DryRun {
		t.Fatal("expected --no-commit to enable dry-run")
	}

	flags, showHelp = lib.ParseArgs([]string{"--single", "--yes"})
	if showHelp || string(flags.Mode) != "1" || !flags.Yes {
		t.Fatalf("expected --single and --yes to be parsed, got %+v", flags)
	}

	flags, showHelp = lib.ParseArgs([]string{"--doctor"})
	if showHelp || !flags.Doctor {
		t.Fatalf("expected --doctor to be parsed, got %+v", flags)
	}
}

func TestAllFilePathsEmpty(t *testing.T) {
	changes := &lib.Changes{}
	result := lib.AllFilePaths(changes)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d", len(result))
	}
}

func TestFormatDiffSectionEdgeCases(t *testing.T) {
	r := lib.FormatDiffSection(nil)
	if r != "" {
		t.Fatalf("expected empty for nil, got '%s'", r)
	}

	r = lib.FormatDiffSection([]lib.FileDiff{})
	if r != "" {
		t.Fatalf("expected empty for empty, got '%s'", r)
	}
}

func TestCanFitInContextEdgeCases(t *testing.T) {
	if lib.CanFitInContext("tpl", []lib.FileDiff{{Path: "a.go", Diff: "small"}}, 0) {
		t.Fatal("expected false for zero context window")
	}
}

func TestAvailableDiffTokensEdgeCases(t *testing.T) {
	if b := lib.AvailableDiffTokens("template", 0); b != 0 {
		t.Fatalf("expected 0 for tiny context, got %d", b)
	}
}

func TestMergeCommitGroupsEdgeCases(t *testing.T) {
	merged := lib.MergeCommitGroups(nil)
	if merged.Subject != "" {
		t.Fatalf("expected empty subject for nil, got '%s'", merged.Subject)
	}
}

func TestIsChunkedBatchEdgeCases(t *testing.T) {
	if lib.IsChunkedBatch(nil) {
		t.Fatal("expected false for nil batch")
	}
	if lib.IsChunkedBatch([]lib.FileDiff{}) {
		t.Fatal("expected false for empty batch")
	}
}

func TestSanitizeDiffEdgeCases(t *testing.T) {
	input := "hello\x00world\nline2"
	got := lib.SanitizeDiff(input)
	if strings.Contains(got, "\x00") {
		t.Fatal("null bytes should be removed")
	}
	if !strings.Contains(got, "hello") {
		t.Fatal("non-null content should be preserved")
	}
}

func TestFormatPromptEdgeCases(t *testing.T) {
	result := lib.FormatPrompt("no placeholders", nil, "")
	if result != "no placeholders" {
		t.Fatalf("expected unchanged template, got '%s'", result)
	}
}

func TestLoadPromptAndSections(t *testing.T) {
	single := lib.LoadPrompt("1", "")
	if !strings.Contains(single, "subject") {
		t.Fatal("expected 'single' section to contain 'subject'")
	}

	groups := lib.LoadPrompt("", "")
	if !strings.Contains(groups, "files") {
		t.Fatal("expected 'groups' section to contain 'files'")
	}

	section := lib.LoadSection("summarize")
	if !strings.Contains(section, "summary") {
		t.Fatal("expected 'summarize' section to contain 'summary'")
	}

	section = lib.LoadSection("plan")
	if !strings.Contains(section, "subject") {
		t.Fatal("expected 'plan' section to contain 'subject'")
	}

	custom := lib.LoadPrompt("", "custom prompt text")
	if custom != "custom prompt text" {
		t.Fatalf("expected custom prompt, got '%s'", custom)
	}
}

func TestSummariesPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(lib.TmpDirEnv, tmpDir)
	path := lib.SummariesPath()
	if !strings.HasPrefix(path, tmpDir) {
		t.Fatalf("expected path %q to be inside temporary directory %q", path, tmpDir)
	}
	if !strings.Contains(path, "git_diff_summaries") {
		t.Fatal("expected path to contain git_diff_summaries")
	}
}

func TestConfigDir(t *testing.T) {
	t.Setenv(lib.ConfigDirEnv, "/tmp/commit-pilot")
	if got := lib.ConfigDir(); got != "/tmp/commit-pilot" {
		t.Fatalf("ConfigDir() = %q, want environment value", got)
	}
}

func TestConfigDirDefault(t *testing.T) {
	t.Setenv(lib.ConfigDirEnv, "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("user home directory unavailable")
	}
	if got, want := lib.ConfigDir(), filepath.Join(home, ".config", "commit-pilot"); got != want {
		t.Fatalf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestTmpDir(t *testing.T) {
	t.Setenv(lib.TmpDirEnv, "/tmp/commit-pilot")
	if got := lib.TmpDir(); got != "/tmp/commit-pilot" {
		t.Fatalf("TmpDir() = %q, want environment value", got)
	}
}

func TestTmpDirDefault(t *testing.T) {
	t.Setenv(lib.TmpDirEnv, "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("user home directory unavailable")
	}
	if got, want := lib.TmpDir(), filepath.Join(home, ".commit-pilot", "tmp"); got != want {
		t.Fatalf("TmpDir() = %q, want %q", got, want)
	}
}

func TestConfigDefaults(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(lib.ConfigDirEnv, configDir)
	content := "# local defaults\nOPENAI_PROVIDER=ollama\nOPENAI_MODEL=qwen\nOPENAI_BASE_URL=http://localhost:11434/v1\nIGNORED=value\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.env"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	defaults := lib.ConfigDefaults()
	if defaults["OPENAI_PROVIDER"] != "ollama" || defaults["OPENAI_MODEL"] != "qwen" {
		t.Fatalf("unexpected config defaults: %#v", defaults)
	}
	if _, ok := defaults["IGNORED"]; ok {
		t.Fatal("unsupported configuration key should be ignored")
	}
}

func TestConfigDefaultsRejectsInvalidPreferenceValues(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(lib.ConfigDirEnv, configDir)
	content := "COMMIT_PILOT_CONVENTIONAL_COMMITS=perhaps\nCOMMIT_PILOT_MAX_SUBJECT_LENGTH=0\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.env"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	defaults := lib.ConfigDefaults()
	if len(defaults) != 0 {
		t.Fatalf("invalid preferences should be ignored: %#v", defaults)
	}
}

func TestConfigDefaultsCreatesFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(lib.ConfigDirEnv, configDir)

	defaults := lib.ConfigDefaults()
	if defaults["OPENAI_PROVIDER"] != "lmstudio" {
		t.Fatalf("expected LM Studio default, got %#v", defaults)
	}
	data, err := os.ReadFile(filepath.Join(configDir, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "OPENAI_MODEL=gemma-4-e2b-it-qat") {
		t.Fatalf("unexpected generated config: %s", data)
	}
}

func TestProjectConfigOverridesUserConfig(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(lib.ConfigDirEnv, configDir)
	if err := os.WriteFile(filepath.Join(configDir, "config.env"), []byte("OPENAI_MODEL=user-model\n"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := lib.GitRun("rev-parse", "--show-toplevel")
	if err != nil {
		t.Skip("test requires Git repository")
	}
	projectDir := filepath.Join(strings.TrimSpace(root), ".commit-pilot")
	projectConfig := filepath.Join(projectDir, "config.env")
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectConfig, []byte("OPENAI_MODEL=project-model\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(projectConfig)
		_ = os.Remove(projectDir)
	})
	if got := lib.ConfigDefaults()["OPENAI_MODEL"]; got != "project-model" {
		t.Fatalf("project config should override user config, got %q", got)
	}
}

func TestLintPlan(t *testing.T) {
	groups := []lib.CommitGroup{{Subject: "feat: add plan lint", Files: []string{"a.go"}}}
	if err := lib.LintPlan(groups, []string{"a.go"}); err != nil {
		t.Fatalf("valid plan: %v", err)
	}
	groups[0].Subject = "add plan lint"
	if err := lib.LintPlan(groups, []string{"a.go"}); err == nil {
		t.Fatal("expected conventional subject error")
	}
}

func TestApplyMessagePreferences(t *testing.T) {
	got := lib.ApplyMessagePreferences("prompt", lib.Config{Conventional: false, TicketPrefix: "ABC-", Imperative: true, MaxSubjectLength: 55, BodyStyle: "bulleted"})
	for _, want := range []string{"Do not require", "ABC-", "imperative", "55", "bulleted"} {
		if !strings.Contains(got, want) {
			t.Fatalf("preferences missing %q: %s", want, got)
		}
	}
}

func TestConfirmCommitPlanDeclinesInjectedInput(t *testing.T) {
	changes, err := lib.GetGitChanges()
	if err != nil || len(changes.AllFiles) == 0 {
		t.Skip("test requires a changed Git worktree")
	}
	var output bytes.Buffer
	cfg := lib.Config{Input: strings.NewReader("n\n"), Output: &output}
	if lib.ConfirmCommitPlan([]lib.CommitGroup{{Subject: "test: plan", Files: []string{"x"}}}, cfg, changes.Fingerprint) {
		t.Fatal("expected declined confirmation")
	}
	if !strings.Contains(output.String(), "No commits created") {
		t.Fatalf("missing confirmation output: %s", output.String())
	}
}

func TestListProviderModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"z"}],"models":[{"key":"a"}]}`))
	}))
	defer server.Close()
	models, err := lib.ListProviderModels(lib.Config{APIBase: server.URL})
	if err != nil || strings.Join(models, ",") != "a,z" {
		t.Fatalf("models=%v err=%v", models, err)
	}
}

func TestWarnInsecureHTTP(t *testing.T) {
	lib.WarnInsecureHTTP("https://api.openai.com/v1", "sk-test")
	lib.WarnInsecureHTTP("http://localhost:1234/v1", "sk-test")
	lib.WarnInsecureHTTP("http://127.0.0.1:8000/v1", "sk-test")
	lib.WarnInsecureHTTP("http://example.com/v1", "sk-test")
	lib.WarnInsecureHTTP("http://example.com/v1", "")
}

func TestAssignBinaryFilesEdgeCases(t *testing.T) {
	groups := []lib.CommitGroup{
		{Subject: "feat: x", Files: []string{"dir/a.go"}},
	}
	result := lib.AssignBinaryFiles(groups, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 group for nil binaries, got %d", len(result))
	}

	result = lib.AssignBinaryFiles(nil, []string{"a.bin"})
	if result != nil {
		t.Fatalf("expected nil for nil groups, got %v", result)
	}
}

func TestFilterValidFilesEdgeCases(t *testing.T) {
	valid := []string{"a.go", "b.go"}
	result := lib.FilterValidFiles(nil, valid)
	if len(result) != 0 {
		t.Fatalf("expected empty for nil candidates, got %d", len(result))
	}
}

func TestGroupChunkedBatchesMoreCases(t *testing.T) {
	batches := [][]lib.FileDiff{
		{{Path: "big.go", Diff: "c1"}},
		{{Path: "big.go", Diff: "c2"}},
		{{Path: "other.go", Diff: "diff"}},
	}
	got := lib.GroupChunkedBatches(batches)
	if len(got) != 2 {
		t.Fatalf("expected 2 groups (merged + separate), got %d", len(got))
	}
}

func TestSplitFilesIntoBatchesSingleFileFits(t *testing.T) {
	files := []lib.FileDiff{
		{Path: "a.go", Diff: "small change"},
	}
	batches := lib.SplitFilesIntoBatches("template", files, 100000)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch for small file, got %d", len(batches))
	}
}

func TestTruncateDiffSmall(t *testing.T) {
	diff := "line1\nline2\nline3"
	result := lib.TruncateDiff(diff)
	if result != diff {
		t.Fatalf("expected unchanged diff, got truncated version")
	}
}

func TestIsBinaryDiffKnownPatterns(t *testing.T) {
	if !lib.IsBinaryDiff("Binary files a and b differ") {
		t.Fatal("expected true for 'Binary files' message")
	}
	if lib.IsBinaryDiff("text content\nnothing binary") {
		t.Fatal("expected false for plain text")
	}
	if !lib.IsBinaryDiff("GIT binary patch\nliteral 12") {
		t.Fatal("expected true for Git binary patch")
	}
}

func TestFilterChangesFiltersAllPlanFiles(t *testing.T) {
	changes := &lib.Changes{
		AllFiles:       []string{"main.go", "private.pem", "package-lock.json"},
		FilesWithDiffs: []lib.FileDiff{{Path: "main.go"}, {Path: "package-lock.json"}},
		BinaryFiles:    []string{"private.pem"},
	}
	lib.FilterChanges(changes, nil, nil, false)
	if got := lib.AllFilePaths(changes); len(got) != 1 || got[0] != "main.go" {
		t.Fatalf("unexpected selected files: %v", got)
	}
}

func TestFilterFilesHonorsIncludeAndExclude(t *testing.T) {
	files := []lib.FileDiff{{Path: "cmd/main.go"}, {Path: "docs/readme.md"}}
	got := lib.FilterFiles(files, []string{"cmd/*"}, []string{"*main.go"}, true)
	if len(got) != 0 {
		t.Fatalf("exclude should win, got: %v", got)
	}
}

func TestIsSensitivePathAvoidsOrdinaryNames(t *testing.T) {
	for _, path := range []string{"src/monkey.go", "src/keyboard.go", "src/clock.go"} {
		if lib.IsSensitivePath(path) {
			t.Fatalf("ordinary path marked sensitive: %s", path)
		}
	}
	for _, path := range []string{".env.local", "keys/id_rsa", "config/api_key.txt", "package-lock.json", "Cargo.lock"} {
		if !lib.IsSensitivePath(path) {
			t.Fatalf("sensitive path not detected: %s", path)
		}
	}
}

func TestContextLengthErrorFields(t *testing.T) {
	err := &lib.ContextLengthError{
		Message:   "msg",
		Estimated: 5000,
		Available: 4096,
	}
	if err.Estimated != 5000 || err.Available != 4096 {
		t.Fatal("ContextLengthError fields should be accessible")
	}
}
