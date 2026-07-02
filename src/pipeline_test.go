package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSummary_validJSON(t *testing.T) {
	text := `{"summary": "Added new feature", "changes": ["added handler", "fixed bug"]}`
	s := parseSummary(text, "main.go")
	if s.Summary != "Added new feature" {
		t.Fatalf("expected summary 'Added new feature', got '%s'", s.Summary)
	}
	if len(s.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(s.Changes))
	}
	if s.File != "main.go" {
		t.Fatalf("expected file main.go, got '%s'", s.File)
	}
}

func TestParseSummary_jsonBlock(t *testing.T) {
	text := "Here is the summary:\n```json\n{\"summary\": \"Changed config\", \"changes\": [\"updated port\"]}\n```"
	s := parseSummary(text, "config.go")
	if s.Summary != "Changed config" {
		t.Fatalf("expected summary 'Changed config', got '%s'", s.Summary)
	}
	if len(s.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(s.Changes))
	}
}

func TestParseSummary_emptyJSON(t *testing.T) {
	text := `{"other": "value"}`
	s := parseSummary(text, "file.go")
	if s.File != "file.go" {
		t.Fatalf("expected file file.go, got '%s'", s.File)
	}
}

func TestParseSummary_invalidJSON(t *testing.T) {
	text := "this is not json at all"
	s := parseSummary(text, "foo.go")
	if s.File != "foo.go" {
		t.Fatalf("expected file foo.go, got '%s'", s.File)
	}
	if s.Summary != text {
		t.Fatalf("expected fallback summary to be raw text")
	}
}

func TestFallbackSummary_short(t *testing.T) {
	s := fallbackSummary("short text", "a.go")
	if s.Summary != "short text" {
		t.Fatalf("expected 'short text', got '%s'", s.Summary)
	}
	if s.File != "a.go" {
		t.Fatalf("expected file a.go, got '%s'", s.File)
	}
}

func TestFallbackSummary_long(t *testing.T) {
	long := strings.Repeat("x", 1000)
	s := fallbackSummary(long, "big.go")
	if len(s.Summary) >= len(long) {
		t.Fatalf("expected fallback summary to be truncated, got %d chars (original %d)", len(s.Summary), len(long))
	}
	if !strings.HasSuffix(s.Summary, "...") {
		t.Fatal("expected truncated summary to end with '...'")
	}
}

func TestFileSummary_jsonRoundTrip(t *testing.T) {
	original := FileSummary{
		File:    "main.go",
		Summary: "Refactored handler",
		Changes: []string{"removed old code", "added new logic"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded FileSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.File != original.File {
		t.Fatalf("File mismatch: %s != %s", decoded.File, original.File)
	}
	if decoded.Summary != original.Summary {
		t.Fatalf("Summary mismatch: %s != %s", decoded.Summary, original.Summary)
	}
	if len(decoded.Changes) != len(original.Changes) {
		t.Fatalf("Changes count mismatch: %d != %d", len(decoded.Changes), len(original.Changes))
	}
}
