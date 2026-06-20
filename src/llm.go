package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const maxResponseSize = 1 << 20

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

// ContextLengthError indicates the input exceeded the model's context window
type ContextLengthError struct {
	Message    string
	Estimated  int
	Available  int
}

func (e *ContextLengthError) Error() string {
	return e.Message
}

var jsonBlockRE = regexp.MustCompile("```(?:json)?\\s*\n(.+?)\n```")

func callLLM(prompt string, cfg Config, maxTokens int) (string, error) {
	warnInsecureHTTP(cfg.APIBase, cfg.APIKey)

	body, err := json.Marshal(chatRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.APIBase, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		errMsg := strings.TrimSpace(string(respBody))

		// Detect context length errors from various providers
		if isContextLengthError(errMsg) {
			return "", &ContextLengthError{
				Message:   fmt.Sprintf("Input too large for model context window (%s)", cfg.Model),
				Estimated: estimateTokens(prompt),
				Available: cfg.ContextWindow,
			}
		}

		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errMsg)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from AI")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// isContextLengthError checks if an error message indicates context length exceeded
func isContextLengthError(errMsg string) bool {
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

func warnInsecureHTTP(apiBase, apiKey string) {
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

func extractJSON(text string) (json.RawMessage, error) {
	text = strings.TrimSpace(text)

	if m := jsonBlockRE.FindStringSubmatch(text); m != nil {
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
