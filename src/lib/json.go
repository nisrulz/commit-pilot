package lib

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MaxJSONDepth is the deepest JSON structure ExtractJSON will scan.
const MaxJSONDepth = 100

// ExtractJSON pulls the first balanced JSON object or array out of a model
// response, tolerating surrounding prose, markdown fences, and responses that
// were cut off mid-JSON. A truncated but otherwise valid structure is repaired
// by closing any unterminated string and remaining brackets before validating.
func ExtractJSON(text string) (json.RawMessage, error) {
	text = strings.TrimSpace(text)
	start := -1
	var stack []byte
	inString := false
	escaped := false
	strStart := 0
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
			strStart = i
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

	if repaired, ok := repairTruncatedJSON(text, start, stack, inString, escaped, strStart); ok {
		return repaired, nil
	}
	return nil, fmt.Errorf("no valid JSON object or array found in AI response")
}

// repairTruncatedJSON turns a JSON prefix that ends mid-structure into valid
// JSON. A dangling string is either closed as a value or, when it is an
// incomplete object key, dropped together with its trailing comma; remaining
// open brackets are then closed and the result validated.
func repairTruncatedJSON(text string, start int, stack []byte, inString, escaped bool, strStart int) (json.RawMessage, bool) {
	if start < 0 || len(stack) == 0 {
		return nil, false
	}

	var base string
	if inString {
		prefix := text[start:strStart]
		fragment := text[strStart:]
		if escaped {
			fragment = fragment[:len(fragment)-1]
		}
		inner := stack[len(stack)-1]
		trimmedPrefix := strings.TrimRight(prefix, " \t")
		if inner == '[' || strings.HasSuffix(trimmedPrefix, ":") {
			base = prefix + fragment + `"`
		} else {
			base = trimTrailingComma(prefix)
		}
	} else {
		base = trimTrailingComma(text[start:])
	}

	candidate := base
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i] {
		case '{':
			candidate += "}"
		case '[':
			candidate += "]"
		}
	}
	if !json.Valid([]byte(candidate)) {
		return nil, false
	}
	return json.RawMessage(candidate), true
}

// trimTrailingComma removes trailing whitespace and a dangling comma so a
// structure cut off right after a separator can still be closed.
func trimTrailingComma(s string) string {
	s = strings.TrimRight(s, " \t")
	if strings.HasSuffix(s, ",") {
		s = s[:len(s)-1]
	}
	return s
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
