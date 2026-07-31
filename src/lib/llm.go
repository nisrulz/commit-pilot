package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// MaxResponseSize caps how much of a provider response is read.
const MaxResponseSize = 1 << 20

// HTTPDoer abstracts HTTP calls so tests can inject a fake client.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ChatMessage is one message in an OpenAI-compatible chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body sent to the provider's chat completions API.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// ChatChoice is a single completion returned by the provider.
type ChatChoice struct {
	Message ChatMessage `json:"message"`
}

// ChatResponse is the parsed provider completion response.
type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

// ContextLengthError indicates the input exceeded the model's context window.
type ContextLengthError struct {
	Message   string
	Estimated int
	Available int
}

func (e *ContextLengthError) Error() string {
	return e.Message
}

// CallLLM sends a prompt to the configured provider and returns the response
// text, using the config's context for cancellation and retry handling.
func CallLLM(prompt string, cfg Config, maxTokens int) (string, error) {
	return CallLLMContext(cfg.Context, prompt, cfg, maxTokens)
}

// CallLLMContext sends a prompt to the provider with explicit parent context.
// Transient failures (429, 5xx, network errors) are retried with backoff up to
// cfg.Retries times; the call can be cancelled through the parent context.
func CallLLMContext(parent context.Context, prompt string, cfg Config, maxTokens int) (string, error) {
	if parent == nil {
		parent = context.Background()
	}
	WarnInsecureHTTP(cfg.APIBase, cfg.APIKey)

	body, err := json.Marshal(ChatRequest{
		Model: cfg.Model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	apiURL := strings.TrimRight(cfg.APIBase, "/") + "/chat/completions"

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	retries := cfg.Retries
	if retries < 0 {
		retries = 0
	}
	var respBody []byte
	var status int
	for attempt := 0; attempt <= retries; attempt++ {
		var retryAfter time.Duration
		ctx, cancel := context.WithTimeout(parent, timeout)
		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
		if err != nil {
			cancel()
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}

		resp, err := client.Do(req)
		if err == nil {
			status = resp.StatusCode
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				retryAfter = time.Duration(seconds) * time.Second
			}
			respBody, err = io.ReadAll(io.LimitReader(resp.Body, MaxResponseSize))
			resp.Body.Close()
		}
		cancel()
		if err == nil && status == http.StatusOK {
			break
		}
		if attempt == retries || (err == nil && status != http.StatusTooManyRequests && status < http.StatusInternalServerError) {
			if err != nil {
				if _, ok := err.(*url.Error); ok {
					return "", fmt.Errorf("could not reach provider at %s", cfg.APIBase)
				}
				return "", fmt.Errorf("http request: %w", err)
			}
			break
		}
		if retryAfter == 0 {
			retryAfter = time.Second << attempt
		}
		select {
		case <-parent.Done():
			return "", parent.Err()
		case <-time.After(retryAfter):
		}
	}

	if status != http.StatusOK {
		errMsg := strings.TrimSpace(string(respBody))

		// Detect context length errors from various providers.
		if IsContextLengthError(errMsg) {
			return "", &ContextLengthError{
				Message:   fmt.Sprintf("Input too large for model context window (%s)", cfg.Model),
				Estimated: EstimateTokens(prompt),
				Available: cfg.ContextWindow,
			}
		}

		// Try to extract a clean message from provider JSON error responses.
		clean := cleanAPIError(errMsg)
		if clean != "" {
			fmt.Fprintf(os.Stderr, "  %s %s\n", yellow("!"), clean)
			return "", fmt.Errorf("request failed")
		}
		return "", fmt.Errorf("request failed (status %d)", status)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("could not parse AI response")
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from AI")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// IsContextLengthError reports whether a provider error message indicates the
// input exceeded the model's context window.
func IsContextLengthError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	contextKeywords := []string{
		"context length",
		"context_length",
		"contextwindow",
		"max_tokens",
		"maximum context",
		"too many tokens",
		"token limit",
		"request too large",
		"payload too large",
		"input too long",
	}
	for _, keyword := range contextKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// WarnInsecureHTTP warns when an API key would be sent over plain HTTP to a
// remote host. Local endpoints (localhost/127.0.0.1) are exempt.
func WarnInsecureHTTP(apiBase, apiKey string) {
	if apiKey == "" {
		return
	}
	u, err := url.Parse(apiBase)
	if err != nil || u.Scheme != "http" {
		return
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return
	}
	fmt.Fprintf(os.Stderr, "  ! Warning: sending API key over plain HTTP to %s\n", u.Host)
}
