package lib

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// MaxJSONDepth is the deepest JSON structure ExtractJSON will scan.
const MaxJSONDepth = 100

// JSONBlockRE matches fenced code blocks that hold JSON in a model response.
var JSONBlockRE = regexp.MustCompile("```(?:json)?\\s*\n(.+?)\n```")

// ExtractJSON pulls the first balanced JSON object or array out of a model
// response, tolerating surrounding prose and markdown fences.
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
