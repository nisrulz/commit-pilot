// Command mock-openai serves the small OpenAI-compatible API used by CI live tests.
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

var (
	fileListRE    = regexp.MustCompile(`File: (\[[^\]]*\])`)
	summaryFileRE = regexp.MustCompile(`"file"\s*:\s*"([^"]+)"`)
)

type chatRequest struct {
	Messages []struct {
		Content string `json:"content"`
	} `json:"messages"`
}

func main() {
	server := &http.Server{
		Addr:     "127.0.0.1:18080",
		Handler:  http.HandlerFunc(handle),
		ErrorLog: log.New(io.Discard, "", 0),
	}
	log.Fatal(server.ListenAndServe())
}

func handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models"):
		writeJSON(w, map[string]any{"data": []map[string]string{{"id": "lfm2.5:8b"}}})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/chat/completions"):
		var request chatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		prompt := ""
		if len(request.Messages) > 0 {
			prompt = request.Messages[len(request.Messages)-1].Content
		}
		content, err := json.Marshal(responseFor(prompt))
		if err != nil {
			http.Error(w, "could not create response", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": string(content)}}},
		})
	default:
		http.NotFound(w, r)
	}
}

func responseFor(prompt string) any {
	if strings.Contains(prompt, "code change summarizer") {
		path := "unknown.go"
		match := fileListRE.FindStringSubmatch(prompt)
		if len(match) == 2 {
			var files []string
			if err := json.Unmarshal([]byte(match[1]), &files); err == nil && len(files) > 0 {
				path = files[0]
			}
		}
		return map[string]any{
			"file":    path,
			"summary": "summary for " + path,
			"changes": []string{"changed " + path},
		}
	}

	if strings.Contains(prompt, "git commit planner") {
		files := summaryFileRE.FindAllStringSubmatch(prompt, -1)
		unique := make([]string, 0, len(files))
		for _, match := range files {
			if len(match) == 2 && !slices.Contains(unique, match[1]) {
				unique = append(unique, match[1])
			}
		}
		return []map[string]any{{
			"subject":     "feat: update files",
			"description": "updated files",
			"files":       unique,
		}}
	}

	return map[string]any{
		"subject":     "feat: test changes",
		"description": "made test changes",
		"files":       []string{},
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
