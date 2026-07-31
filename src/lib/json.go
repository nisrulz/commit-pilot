package lib

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MaxJSONDepth is the deepest JSON structure ExtractJSON will scan.
const MaxJSONDepth = 100

// ExtractJSON pulls the first balanced JSON object or array out of a model
// response, tolerating surrounding prose and markdown fences.
func ExtractJSON(text string) (json.RawMessage, error) {
	text = strings.TrimSpace(text)
	start := -1
	var stack []byte
	inString := false
	escaped := false
	for i := 0; i < len(text); i++ {
		if start < 0 {
			if text[i] != '{' && text[i] != '[' {
				continue
			}
			start = i
			stack = append(stack, text[i])
			continue
		}

		if inString {
			if escaped {
				escaped = false
			} else if text[i] == '\\' {
				escaped = true
			} else if text[i] == '"' {
				inString = false
			}
			continue
		}

		switch text[i] {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, text[i])
			if len(stack) > MaxJSONDepth {
				return nil, fmt.Errorf("JSON nesting exceeds max depth %d", MaxJSONDepth)
			}
		case '}', ']':
			open := stack[len(stack)-1]
			if (open == '{' && text[i] != '}') || (open == '[' && text[i] != ']') {
				start = -1
				stack = stack[:0]
				continue
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				raw := json.RawMessage(text[start : i+1])
				if json.Valid(raw) {
					return raw, nil
				}
				start = -1
			}
		}
	}
	return nil, fmt.Errorf("no valid JSON object or array found in AI response")
}

// cleanAPIError extracts a user-facing message from a provider JSON error body.
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
