package lib_test

import (
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

	flags, showHelp := lib.ParseArgs([]string{"1", "--dry-run"})
	if showHelp {
		t.Fatal("expected showHelp=false")
	}
	if string(flags.Mode) != "1" {
		t.Fatalf("expected mode '1', got '%s'", string(flags.Mode))
	}
	if !flags.DryRun {
		t.Fatal("expected DryRun=true")
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
	path := lib.SummariesPath()
	if !strings.Contains(path, ".commit-pilot") {
		t.Fatal("expected path to contain .commit-pilot")
	}
	if !strings.Contains(path, "git_diff_summaries") {
		t.Fatal("expected path to contain git_diff_summaries")
	}
}

func TestWarnInsecureHTTP(t *testing.T) {
	lib.WarnInsecureHTTP("https://api.openai.com/v1", "sk-test")
	lib.WarnInsecureHTTP("http://localhost:1234/v1", "sk-test")
	lib.WarnInsecureHTTP("http://127.0.0.1:8000/v1", "sk-test")
	lib.WarnInsecureHTTP("http://example.com/v1", "sk-test")
	lib.WarnInsecureHTTP("http://example.com/v1", "")
}

func TestFileCategoryEdgeCases(t *testing.T) {
	got := lib.FileCategory("config.js")
	if got != "config" {
		t.Fatalf("expected 'config' for config.js, got '%s'", got)
	}

	got = lib.FileCategory("README")
	if got != "docs" {
		t.Fatalf("expected 'docs' for README, got '%s'", got)
	}

	got = lib.FileCategory(".gitignore")
	if got != "config" {
		t.Fatalf("expected 'config' for .gitignore, got '%s'", got)
	}

	got = lib.FileCategory("run.sh")
	if got != "scripts" {
		t.Fatalf("expected 'scripts' for run.sh, got '%s'", got)
	}

	got = lib.FileCategory("main.go")
	if got != "code" {
		t.Fatalf("expected 'code' for main.go, got '%s'", got)
	}
}

func TestLimitCommitScopeEdgeCases(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go", "d.go", "e.go"}
	result := lib.LimitCommitScope(files)
	if len(result) > 3 {
		t.Fatalf("expected at most 3 files, got %d", len(result))
	}
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

func TestMergeGroupsEdgeCases(t *testing.T) {
	groups := []lib.CommitGroup{
		{Subject: "feat: add login", Files: []string{"login.go"}},
		{Subject: "feat: add login", Files: []string{"session.go"}},
	}
	result := lib.MergeGroups(groups)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged group, got %d", len(result))
	}
}

func TestSubjectsRelatedEdgeCases(t *testing.T) {
	if lib.SubjectsRelated("feat: a", "") {
		t.Fatal("expected false for empty second subject")
	}
	if lib.SubjectsRelated("", "feat: a") {
		t.Fatal("expected false for empty first subject")
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
}

func TestParseFloatValues(t *testing.T) {
	if v := lib.ParseFloat("0"); v != 0 {
		t.Fatalf("expected 0, got %f", v)
	}
	if v := lib.ParseFloat("3.14"); v != 3.14 {
		t.Fatalf("expected 3.14, got %f", v)
	}
	if v := lib.ParseFloat("abc"); v != 0 {
		t.Fatalf("expected 0 for invalid input, got %f", v)
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
