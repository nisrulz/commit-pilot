package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// chatCall carries one prompt and its retry state across provider attempts.
type chatCall struct {
	parent    context.Context
	client    HTTPDoer
	apiURL    string
	cfg       Config
	prompt    string
	maxTokens int
	tier      formatTier
	retries   int
	timeout   time.Duration
}

// newChatCall resolves the per-call defaults the retry loop needs.
func newChatCall(parent context.Context, prompt string, cfg Config, maxTokens int) chatCall {
	client := cfg.HTTPClient
	if client == nil {
		client = newProviderHTTPClient(0)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	retries := cfg.Retries
	if retries < 0 {
		retries = 0
	}
	tier := formatTierObject
	if jsonSchemaForPrompt(prompt) != nil {
		tier = formatTierSchema
	}
	return chatCall{
		parent:    parent,
		client:    client,
		apiURL:    strings.TrimRight(cfg.APIBase, "/") + "/chat/completions",
		cfg:       cfg,
		prompt:    prompt,
		maxTokens: maxTokens,
		tier:      tier,
		retries:   retries,
		timeout:   timeout,
	}
}

// send performs the request with retries and format downgrades. It reports
// whether a 2xx response was received.
func (c chatCall) send() (chatAttempt, bool) {
	var last chatAttempt
	for attempt := 0; attempt <= c.retries; attempt++ {
		body, err := c.requestBody()
		if err != nil {
			return chatAttempt{err: fmt.Errorf("marshal request: %w", err)}, false
		}
		ctx, cancel := context.WithTimeout(c.parent, c.timeout)
		last = c.post(ctx, body)
		cancel()

		if last.err == nil && last.status == http.StatusOK {
			return last, true
		}
		// Providers without structured output reject the format with a 4xx.
		// Degrade one step (json_schema -> json_object -> plain) and retry
		// before reporting a failure.
		if last.err == nil && c.tier > formatTierNone && isResponseFormatRejection(last.status) {
			c.tier--
			attempt--
			continue
		}
		if attempt == c.retries || (last.err == nil && !isRetryableStatus(last.status)) {
			break
		}
		select {
		case <-c.parent.Done():
			return chatAttempt{err: c.parent.Err()}, false
		case <-time.After(c.waitFor(attempt, last)):
		}
	}
	return last, false
}

// waitFor returns the delay before the next attempt: the server's Retry-After
// when present, otherwise exponential backoff.
func (c chatCall) waitFor(attempt int, last chatAttempt) time.Duration {
	if last.retryAfter > 0 {
		return last.retryAfter
	}
	return backoffFor(attempt)
}

// chatAttempt is the outcome of one HTTP call to the provider.
type chatAttempt struct {
	status     int
	body       []byte
	retryAfter time.Duration
	err        error
}

// requestBody renders the JSON body for one chat completion request.
func (c chatCall) requestBody() ([]byte, error) {
	return json.Marshal(ChatRequest{
		Model: c.cfg.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: llmSystemInstruction},
			{Role: "user", Content: c.prompt},
		},
		Temperature:    0.2,
		MaxTokens:      c.maxTokens,
		ResponseFormat: responseFormatForTier(c.tier, c.prompt),
	})
}

// post performs one HTTP POST and reads a bounded response body.
func (c chatCall) post(ctx context.Context, body []byte) chatAttempt {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewReader(body))
	if err != nil {
		return chatAttempt{err: fmt.Errorf("create request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return chatAttempt{err: err}
	}
	defer resp.Body.Close()

	attempt := chatAttempt{status: resp.StatusCode}
	if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
		attempt.retryAfter = time.Duration(seconds) * time.Second
	}
	attempt.body, attempt.err = io.ReadAll(io.LimitReader(resp.Body, MaxResponseSize))
	return attempt
}
