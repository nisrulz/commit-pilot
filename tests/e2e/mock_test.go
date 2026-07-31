package e2e_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// mockOpenAI is an OpenAI-compatible HTTP server whose /chat/completions
// responses are derived from the incoming prompt via a responder function. It
// also records every call so tests can assert on call counts and prompts.
type mockOpenAI struct {
	server      *httptest.Server
	chatCalls   int
	chatPrompts []string
	mu          sync.Mutex
}

// newMockOpenAI starts the mock server. The responder maps a prompt to the
// assistant content string returned for it.
func newMockOpenAI(t *testing.T, respond func(prompt string) string) *mockOpenAI {
	t.Helper()
	m := &mockOpenAI{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
			return
		case "/chat/completions":
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.Unmarshal(body, &payload)
			prompt := ""
			if len(payload.Messages) > 0 {
				prompt = payload.Messages[0].Content
			}
			m.mu.Lock()
			m.chatCalls++
			m.chatPrompts = append(m.chatPrompts, prompt)
			m.mu.Unlock()
			resp, _ := json.Marshal(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{"role": "assistant", "content": respond(prompt)},
				}},
			})
			_, _ = w.Write(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockOpenAI) url() string { return m.server.URL }

func (m *mockOpenAI) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chatCalls
}

func (m *mockOpenAI) joinedPrompts() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strings.Join(m.chatPrompts, "\n")
}

var (
	fileListRE = regexp.MustCompile(`Files changed: (\[[^\]]*\])`)
	fileTagRE  = regexp.MustCompile(`\bFile: (\[[^\]]*\])`)
	summaryRE  = regexp.MustCompile(`"file":\s*"([^"]+)"`)
)

func parseFilesFromList(match string) []string {
	var files []string
	_ = json.Unmarshal([]byte(match), &files)
	return files
}

// mockRespond dispatches AI responses based on which embedded prompt template
// the request uses, so a single responder drives every mode:
//   - summarize: return a per-file summary
//   - plan:      return one group covering every summarized file
//   - single/groups: return a single commit object
func mockRespond(prompt string) string {
	switch {
	case strings.Contains(prompt, "code change summarizer"):
		m := fileTagRE.FindStringSubmatch(prompt)
		file := "unknown.go"
		if m != nil {
			if files := parseFilesFromList(m[1]); len(files) > 0 {
				file = files[0]
			}
		}
		body, _ := json.Marshal(map[string]any{
			"file":    file,
			"summary": "summary for " + file,
			"changes": []string{"changed " + file},
		})
		return string(body)
	case strings.Contains(prompt, "git commit planner"):
		seen := map[string]bool{}
		var files []string
		for _, m := range summaryRE.FindAllStringSubmatch(prompt, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				files = append(files, m[1])
			}
		}
		body, _ := json.Marshal([]map[string]any{{
			"subject":     "feat: update files",
			"description": "updated files",
			"files":       files,
		}})
		return string(body)
	default:
		// single mode and batch "groups" template both accept an object
		body, _ := json.Marshal(map[string]any{
			"subject":     "feat: test changes",
			"description": "made test changes",
			"files":       []string{},
		})
		return string(body)
	}
}
