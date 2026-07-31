package lib_test

import (
	"context"
	"errors"
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"net/http"
	"strings"
	"testing"
)

type failingDoer struct{}

func (failingDoer) Do(*http.Request) (*http.Response, error) { return nil, errors.New("unavailable") }

func TestCallLLMContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := lib.CallLLMContext(ctx, "prompt", lib.Config{APIBase: "http://localhost", Timeout: 1, Retries: 1, HTTPClient: failingDoer{}}, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestExtractJSON_depthLimit(t *testing.T) {
	depth := lib.MaxJSONDepth + 10
	text := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	_, err := lib.ExtractJSON(text)
	if err == nil {
		t.Fatal("expected error for deeply nested JSON")
	}
}

func TestExtractJSON_object(t *testing.T) {
	raw, err := lib.ExtractJSON(`{"key": "value"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"key": "value"}` {
		t.Fatalf("expected '{\"key\": \"value\"}', got '%s'", string(raw))
	}
}

func TestExtractJSON_array(t *testing.T) {
	raw, err := lib.ExtractJSON(`[1, 2, 3]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `[1, 2, 3]` {
		t.Fatalf("expected '[1, 2, 3]', got '%s'", string(raw))
	}
}

func TestExtractJSON_codeBlock(t *testing.T) {
	raw, err := lib.ExtractJSON("Here:\n```json\n{\"key\": \"val\"}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"key": "val"}` {
		t.Fatalf("expected '{\"key\": \"val\"}', got '%s'", string(raw))
	}
}

func TestExtractJSON_noJSON(t *testing.T) {
	_, err := lib.ExtractJSON("just plain text")
	if err == nil {
		t.Fatal("expected error for text with no JSON")
	}
}

func TestExtractJSON_unmatched(t *testing.T) {
	_, err := lib.ExtractJSON(`{"key": "val"`)
	if err == nil {
		t.Fatal("expected error for unmatched brackets")
	}
}

func TestExtractJSON_empty(t *testing.T) {
	_, err := lib.ExtractJSON("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestExtractJSON_nested(t *testing.T) {
	raw, err := lib.ExtractJSON(`{"a": {"b": [1, 2, {"c": 3}]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"a": {"b": [1, 2, {"c": 3}]}}` {
		t.Fatalf("unexpected result: %s", string(raw))
	}
}

func TestExtractJSONAllowsBracketsInsideStrings(t *testing.T) {
	raw, err := lib.ExtractJSON(`prefix {"subject":"fix: handle } and ] in text","description":"ok"} suffix`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"subject":"fix: handle } and ] in text","description":"ok"}` {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestExtractJSONSkipsInvalidBalancedCandidate(t *testing.T) {
	raw, err := lib.ExtractJSON(`prefix {not-json} then {"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("unexpected result: %s", raw)
	}
}

func TestExtractJSONRejectsLargeUnclosedInput(t *testing.T) {
	if _, err := lib.ExtractJSON(strings.Repeat("[", 50_000)); err == nil {
		t.Fatal("expected error for unclosed JSON")
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
		if !lib.IsContextLengthError(kw) {
			t.Errorf("lib.IsContextLengthError(%q) should be true", kw)
		}
	}
}

func TestIsContextLengthError_noMatch(t *testing.T) {
	if lib.IsContextLengthError("rate limit exceeded") {
		t.Error("should not match rate limit errors")
	}
	if lib.IsContextLengthError("") {
		t.Error("should not match empty string")
	}
}
