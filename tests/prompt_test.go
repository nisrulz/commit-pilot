package lib_test

import (
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"strings"
	"testing"
)

func TestSectionByName_exists(t *testing.T) {
	s := lib.SectionByName("single")
	if s == "" {
		t.Fatal("expected non-empty section 'single'")
	}
	if !contains(s, "subject") {
		t.Fatal("expected 'single' section to contain 'subject'")
	}
}

func TestSectionByName_groups(t *testing.T) {
	s := lib.SectionByName("groups")
	if s == "" {
		t.Fatal("expected non-empty section 'groups'")
	}
	if !contains(s, "files") {
		t.Fatal("expected 'groups' section to contain 'files'")
	}
}

func TestSectionByName_summarize(t *testing.T) {
	s := lib.SectionByName("summarize")
	if s == "" {
		t.Fatal("expected non-empty section 'summarize'")
	}
}

func TestSectionByName_plan(t *testing.T) {
	s := lib.SectionByName("plan")
	if s == "" {
		t.Fatal("expected non-empty section 'plan'")
	}
}

func TestSectionByName_missing(t *testing.T) {
	s := lib.SectionByName("nonexistent")
	if s == "" {
		t.Fatal("expected fallback to last section for missing name")
	}
}

func TestSanitizeDiff_removesNull(t *testing.T) {
	input := "hello\x00world"
	got := lib.SanitizeDiff(input)
	if contains(got, "\x00") {
		t.Fatal("lib.SanitizeDiff should remove null bytes")
	}
}

func TestSanitizeDiff_keepsNewlines(t *testing.T) {
	input := "line1\nline2\nline3"
	got := lib.SanitizeDiff(input)
	if got != input {
		t.Fatalf("expected '%s', got '%s'", input, got)
	}
}

func TestFormatPrompt_basic(t *testing.T) {
	result := lib.FormatPrompt("Files: {files}\nDiff: {diff}", []string{"a.go"}, "+func foo()")
	if !contains(result, "a.go") {
		t.Fatal("expected file list in formatted prompt")
	}
	if !contains(result, "+func foo()") {
		t.Fatal("expected diff in formatted prompt")
	}
}

func TestFormatDiffSection_single(t *testing.T) {
	result := lib.FormatDiffSection([]lib.FileDiff{{Path: "a.go", Diff: "+func"}})
	if !contains(result, "a.go") {
		t.Fatal("expected file name in diff section")
	}
}

func TestFormatDiffSection_empty(t *testing.T) {
	result := lib.FormatDiffSection(nil)
	if result != "" {
		t.Fatal("expected empty string for nil input")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
