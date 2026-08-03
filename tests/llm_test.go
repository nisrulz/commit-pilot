package lib_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type failingDoer struct{}

func (failingDoer) Do(*http.Request) (*http.Response, error) { return nil, errors.New("unavailable") }

// recordedResponse is one canned HTTP response in a scripted sequence.
type recordedResponse struct {
	status int
	body   string
}

// recordingDoer returns responses in order and records every request body,
// letting tests assert what was actually sent to the provider.
type recordingDoer struct {
	mu        sync.Mutex
	bodies    []string
	responses []recordedResponse
}

func (d *recordingDoer) Do(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bodies = append(d.bodies, string(body))
	resp := recordedResponse{status: http.StatusOK, body: "{}"}
	if len(d.responses) > 0 {
		resp = d.responses[0]
		d.responses = d.responses[1:]
	}
	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
	}, nil
}

func chatContentReason(content, reason string) string {
	body, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": reason,
		}},
	})
	return string(body)
}

func TestResponseFormatJSONObjectForUnknownPrompt(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{{status: 200, body: chatContent(`{"ok":true}`)}}}
	cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", HTTPClient: doer, Timeout: 5}
	if _, err := lib.CallLLMContext(context.Background(), "p", cfg, 100); err != nil {
		t.Fatalf("CallLLMContext: %v", err)
	}
	if len(doer.bodies) != 1 || !strings.Contains(doer.bodies[0], `"response_format":{"type":"json_object"}`) {
		t.Fatalf("expected json_object response_format in request body, got %v", doer.bodies)
	}
}

func TestResponseFormatJSONSchemaForKnownPrompt(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{{status: 200, body: chatContent(`{"ok":true}`)}}}
	cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", HTTPClient: doer, Timeout: 5}
	prompt := "You are a git commit planner. Group these file change summaries."
	if _, err := lib.CallLLMContext(context.Background(), prompt, cfg, 100); err != nil {
		t.Fatalf("CallLLMContext: %v", err)
	}
	if len(doer.bodies) != 1 {
		t.Fatalf("expected 1 call, got %d", len(doer.bodies))
	}
	if !strings.Contains(doer.bodies[0], `"type":"json_schema"`) {
		t.Fatalf("expected json_schema response_format, got %v", doer.bodies[0])
	}
	if !strings.Contains(doer.bodies[0], `"commit_plan"`) || !strings.Contains(doer.bodies[0], `"strict":true`) {
		t.Fatalf("expected strict commit_plan schema, got %v", doer.bodies[0])
	}
}

func TestResponseFormatSchemaPerPrompt(t *testing.T) {
	cases := []struct{ prompt, schemaName string }{
		{"You are a git commit planner.", "commit_plan"},
		{"You are a git commit organizer.", "commit_group"},
		{"You are a git commit message generator.", "commit_message"},
		{"You are a code change summarizer.", "file_summary"},
	}
	for _, tc := range cases {
		doer := &recordingDoer{responses: []recordedResponse{{status: 200, body: chatContent(`{}`)}}}
		cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", HTTPClient: doer, Timeout: 5}
		if _, err := lib.CallLLMContext(context.Background(), tc.prompt, cfg, 100); err != nil {
			t.Fatalf("%s: %v", tc.prompt, err)
		}
		if !strings.Contains(doer.bodies[0], `"type":"json_schema"`) ||
			!strings.Contains(doer.bodies[0], `"`+tc.schemaName+`"`) ||
			!strings.Contains(doer.bodies[0], `"strict":true`) {
			t.Errorf("prompt %q: expected strict json_schema for %s, got %s", tc.prompt, tc.schemaName, doer.bodies[0])
		}
	}
}

func TestResponseFormatFallsBackSchemaToObject(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{
		{status: 400, body: `{"error":{"message":"json_schema not supported"}}`},
		{status: 200, body: chatContent(`{"ok":true}`)},
	}}
	cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", HTTPClient: doer, Timeout: 5}
	prompt := "You are a git commit planner. Group these file change summaries."
	if _, err := lib.CallLLMContext(context.Background(), prompt, cfg, 100); err != nil {
		t.Fatalf("CallLLMContext: %v", err)
	}
	if len(doer.bodies) != 2 {
		t.Fatalf("expected fallback retry, got %d calls", len(doer.bodies))
	}
	if !strings.Contains(doer.bodies[0], `"type":"json_schema"`) {
		t.Fatal("first attempt should request json_schema")
	}
	if !strings.Contains(doer.bodies[1], `"type":"json_object"`) || strings.Contains(doer.bodies[1], "json_schema") {
		t.Fatal("fallback attempt should request json_object")
	}
}

func TestResponseFormatFallsBackObjectToPlain(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{
		{status: 400, body: `{"error":{"message":"response_format not supported"}}`},
		{status: 200, body: chatContent(`{"ok":true}`)},
	}}
	cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", HTTPClient: doer, Timeout: 5}
	if _, err := lib.CallLLMContext(context.Background(), "custom prompt", cfg, 100); err != nil {
		t.Fatalf("CallLLMContext: %v", err)
	}
	if len(doer.bodies) != 2 {
		t.Fatalf("expected fallback retry, got %d calls", len(doer.bodies))
	}
	if !strings.Contains(doer.bodies[0], "response_format") {
		t.Fatal("first attempt should request json_object")
	}
	if strings.Contains(doer.bodies[1], "response_format") {
		t.Fatal("fallback attempt should omit response_format")
	}
}

func TestResponseFormatKeepsFormatOnServerError(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{
		{status: 500, body: "boom"},
		{status: 200, body: chatContent(`{"ok":true}`)},
	}}
	cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", Retries: 1, HTTPClient: doer, Timeout: 5}
	if _, err := lib.CallLLMContext(context.Background(), "You are a git commit planner.", cfg, 100); err != nil {
		t.Fatalf("CallLLMContext: %v", err)
	}
	for _, b := range doer.bodies {
		if !strings.Contains(b, "response_format") {
			t.Fatalf("server errors must keep response_format, got %v", doer.bodies)
		}
	}
}

func TestCallLLMTruncation(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{{status: 200, body: chatContentReason(`{"a": 1`, "length")}}}
	cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", HTTPClient: doer, Timeout: 5}
	_, err := lib.CallLLMContext(context.Background(), "p", cfg, 100)
	var trunc *lib.TruncatedError
	if !errors.As(err, &trunc) || trunc.MaxTokens != 100 {
		t.Fatalf("expected TruncatedError, got %v", err)
	}
	if msg := trunc.Error(); !strings.Contains(msg, "100") {
		t.Fatalf("unexpected truncation message: %q", msg)
	}
}

func TestCallLLMFinishReasonStop(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{{status: 200, body: chatContentReason(`{"ok":true}`, "stop")}}}
	cfg := lib.Config{APIBase: "http://localhost/v1", Model: "m", HTTPClient: doer, Timeout: 5}
	out, err := lib.CallLLMContext(context.Background(), "p", cfg, 100)
	if err != nil {
		t.Fatalf("CallLLMContext: %v", err)
	}
	if out != `{"ok":true}` {
		t.Fatalf("unexpected content: %q", out)
	}
}

func TestPlanFromSummariesRetriesTruncation(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{
		{status: 200, body: chatContentReason(`[{"subject":"cut`, "length")},
		{status: 200, body: chatContent(`[{"subject":"feat: ok","description":"d","files":["a.go"]}]`)},
	}}
	cfg := cfgWithDoer(staticDoer{})
	cfg.HTTPClient = doer
	groups, err := lib.PlanFromSummaries("Plan:\n{diff}", cfg, summariesJSON)
	if err != nil {
		t.Fatalf("PlanFromSummaries: %v", err)
	}
	if len(groups) != 1 || groups[0].Subject != "feat: ok" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
	if len(doer.bodies) != 2 {
		t.Fatalf("expected a truncation retry, got %d calls", len(doer.bodies))
	}
	if !strings.Contains(doer.bodies[0], `"max_tokens":4096`) || !strings.Contains(doer.bodies[1], `"max_tokens":8192`) {
		t.Fatalf("expected retry with doubled max_tokens, got %v", doer.bodies)
	}
}

func TestPlanFromSummariesRetriesNoJSON(t *testing.T) {
	doer := &recordingDoer{responses: []recordedResponse{
		{status: 200, body: chatContent("I refuse to plan this. Sorry.")},
		{status: 200, body: chatContent(`[{"subject":"feat: retried","description":"d","files":["a.go"]}]`)},
	}}
	cfg := cfgWithDoer(staticDoer{})
	cfg.HTTPClient = doer
	groups, err := lib.PlanFromSummaries("Plan:\n{diff}", cfg, summariesJSON)
	if err != nil {
		t.Fatalf("PlanFromSummaries: %v", err)
	}
	if len(groups) != 1 || groups[0].Subject != "feat: retried" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
	if len(doer.bodies) != 2 {
		t.Fatalf("expected a no-JSON retry, got %d calls", len(doer.bodies))
	}
	if !strings.Contains(doer.bodies[1], "ONLY a single valid JSON array") {
		t.Fatalf("expected strict re-prompt, got %v", doer.bodies[1])
	}
}

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

func TestExtractJSON_truncatedObject(t *testing.T) {
	raw, err := lib.ExtractJSON(`{"key": "val`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `{"key": "val"}` {
		t.Fatalf("expected repaired '{\"key\": \"val\"}', got '%s'", string(raw))
	}
}

func TestExtractJSON_truncatedArray(t *testing.T) {
	raw, err := lib.ExtractJSON(`[{"subject":"feat: ab","des`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `[{"subject":"feat: ab"}]` {
		t.Fatalf("expected repaired array, got '%s'", string(raw))
	}
}

func TestExtractJSON_truncatedMidString(t *testing.T) {
	raw, err := lib.ExtractJSON(`{"a": "test\`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `{"a": "test"}` {
		t.Fatalf("expected repaired '{\"a\": \"test\"}', got '%s'", string(raw))
	}
}

func TestExtractJSON_unrecoverableTruncation(t *testing.T) {
	_, err := lib.ExtractJSON(`{"a": 1, }`)
	if err == nil {
		t.Fatal("expected error for unrecoverable truncated JSON")
	}
}

func TestExtractJSON_truncatedNested(t *testing.T) {
	raw, err := lib.ExtractJSON(`{"a":{"b":[1,2`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `{"a":{"b":[1,2]}}` {
		t.Fatalf("expected repaired nested JSON, got '%s'", string(raw))
	}
}

func TestExtractJSON_truncatedAfterObjectComma(t *testing.T) {
	raw, err := lib.ExtractJSON(`{"a":1,`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `{"a":1}` {
		t.Fatalf("expected repaired object, got '%s'", string(raw))
	}
}

func TestExtractJSON_truncatedAfterArrayComma(t *testing.T) {
	raw, err := lib.ExtractJSON(`[1,2,`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `[1,2]` {
		t.Fatalf("expected repaired array, got '%s'", string(raw))
	}
}

func TestExtractJSON_truncatedArrayElementString(t *testing.T) {
	raw, err := lib.ExtractJSON(`["a", "b`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `["a", "b"]` {
		t.Fatalf("expected repaired array, got '%s'", string(raw))
	}
}

func TestExtractJSON_bareOpenBrace(t *testing.T) {
	raw, err := lib.ExtractJSON(`{`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `{}` {
		t.Fatalf("expected repaired empty object, got '%s'", string(raw))
	}
}

func TestExtractJSON_validJSONWinsOverTruncated(t *testing.T) {
	raw, err := lib.ExtractJSON(`{"a":1} [1,2`)
	if err != nil {
		t.Fatalf("expected first valid JSON, got error: %v", err)
	}
	if string(raw) != `{"a":1}` {
		t.Fatalf("expected '{\"a\":1}', got '%s'", string(raw))
	}
}

func TestExtractJSON_keyWithoutValueUnrecoverable(t *testing.T) {
	_, err := lib.ExtractJSON(`{"a":`)
	if err == nil {
		t.Fatal("expected error when a key has no value to repair")
	}
}

func TestExtractJSON_balancedButInvalidNotRepaired(t *testing.T) {
	_, err := lib.ExtractJSON(`[1, 2, ]`)
	if err == nil {
		t.Fatal("expected error for balanced but invalid JSON")
	}
}

func TestExtractJSON_skipsMismatchedThenRepairsTruncated(t *testing.T) {
	raw, err := lib.ExtractJSON(`nonsense {broken] then [1,`)
	if err != nil {
		t.Fatalf("expected repaired JSON, got error: %v", err)
	}
	if string(raw) != `[1]` {
		t.Fatalf("expected repaired '[1]', got '%s'", string(raw))
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
