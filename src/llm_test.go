package main

import (
	"testing"
)

func TestExtractJSON_depthLimit(t *testing.T) {
	depth := maxJSONDepth + 10
	text := "{"
	for i := 0; i < depth; i++ {
		text += "{"
	}
	_, err := extractJSON(text)
	if err == nil {
		t.Fatal("expected error for deeply nested JSON")
	}
}

func TestExtractJSON_object(t *testing.T) {
	raw, err := extractJSON(`{"key": "value"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"key": "value"}` {
		t.Fatalf("expected '{\"key\": \"value\"}', got '%s'", string(raw))
	}
}

func TestExtractJSON_array(t *testing.T) {
	raw, err := extractJSON(`[1, 2, 3]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `[1, 2, 3]` {
		t.Fatalf("expected '[1, 2, 3]', got '%s'", string(raw))
	}
}

func TestExtractJSON_codeBlock(t *testing.T) {
	raw, err := extractJSON("Here:\n```json\n{\"key\": \"val\"}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"key": "val"}` {
		t.Fatalf("expected '{\"key\": \"val\"}', got '%s'", string(raw))
	}
}

func TestExtractJSON_noJSON(t *testing.T) {
	_, err := extractJSON("just plain text")
	if err == nil {
		t.Fatal("expected error for text with no JSON")
	}
}

func TestExtractJSON_unmatched(t *testing.T) {
	_, err := extractJSON(`{"key": "val"`)
	if err == nil {
		t.Fatal("expected error for unmatched brackets")
	}
}

func TestExtractJSON_empty(t *testing.T) {
	_, err := extractJSON("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestExtractJSON_nested(t *testing.T) {
	raw, err := extractJSON(`{"a": {"b": [1, 2, {"c": 3}]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"a": {"b": [1, 2, {"c": 3}]}}` {
		t.Fatalf("unexpected result: %s", string(raw))
	}
}

func TestIsContextLengthError_match(t *testing.T) {
	keywords := []string{
		"context length exceeded",
		"context_length_exceeded",
		"max_tokens exceeded",
		"too many tokens",
		"request too large",
	}
	for _, kw := range keywords {
		if !isContextLengthError(kw) {
			t.Errorf("isContextLengthError(%q) should be true", kw)
		}
	}
}

func TestIsContextLengthError_noMatch(t *testing.T) {
	if isContextLengthError("rate limit exceeded") {
		t.Error("should not match rate limit errors")
	}
	if isContextLengthError("") {
		t.Error("should not match empty string")
	}
}
