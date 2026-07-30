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
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxResponseSize = 1 << 20

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type ChatChoice struct {
	Message ChatMessage `json:"message"`
}

type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

// ContextLengthError indicates the input exceeded the model's context window
type ContextLengthError struct {
	Message   string
	Estimated int
	Available int
}

func (e *ContextLengthError) Error() string {
	return e.Message
}

const MaxJSONDepth = 100

var JSONBlockRE = regexp.MustCompile("```(?:json)?\\s*\n(.+?)\n```")

func CallLLM(prompt string, cfg Config, maxTokens int) (string, error) {
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

	client := &http.Client{}
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
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
		if retryAfter > 0 {
			time.Sleep(retryAfter)
		} else {
			time.Sleep(time.Second << attempt)
		}
	}

	if status != http.StatusOK {
		errMsg := strings.TrimSpace(string(respBody))

		// Detect context length errors from various providers
		if IsContextLengthError(errMsg) {
			return "", &ContextLengthError{
				Message:   fmt.Sprintf("Input too large for model context window (%s)", cfg.Model),
				Estimated: EstimateTokens(prompt),
				Available: cfg.ContextWindow,
			}
		}

		// Try to extract a clean message from provider JSON error responses
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

// IsContextLengthError checks if an error message indicates context length exceeded
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

// cleanAPIError extracts a user-facing message from provider JSON error responses
func cleanAPIError(body string) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err == nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	return ""
}

func ExtractJSON(text string) (json.RawMessage, error) {
	text = strings.TrimSpace(text)

	if m := JSONBlockRE.FindStringSubmatch(text); m != nil {
		text = strings.TrimSpace(m[1])
	}

	start := -1
	for i, c := range text {
		if c == '{' || c == '[' {
			start = i
			break
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("no JSON structure found in AI response")
	}

	openChar := text[start]
	closeChar := byte('}')
	if openChar == '[' {
		closeChar = ']'
	}

	depth := 0
	end := -1
	for i := start; i < len(text); i++ {
		if text[i] == openChar {
			depth++
			if depth > MaxJSONDepth {
				return nil, fmt.Errorf("JSON nesting exceeds max depth %d", MaxJSONDepth)
			}
		} else if text[i] == closeChar {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end == -1 {
		return nil, fmt.Errorf("unmatched brackets in AI response")
	}

	return json.RawMessage(text[start:end]), nil
}
