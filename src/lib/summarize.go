package lib

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileSummary struct {
	File    string   `json:"file"`
	Summary string   `json:"summary"`
	Changes []string `json:"changes"`
}

func SummariesPath() string {
	home, _ := os.UserHomeDir()
	date := time.Now().Format("2006-01-02")

	name := "unknown"
	if out, err := GitRun("rev-parse", "--show-toplevel"); err == nil {
		name = filepath.Base(strings.TrimSpace(out))
	}

	id := make([]byte, 4)
	if _, err := rand.Read(id); err != nil {
		id = []byte{0, 0, 0, 1}
	}

	dir := filepath.Join(home, ".commit-pilot", "tmp")
	os.MkdirAll(dir, 0700)

	return filepath.Join(dir, fmt.Sprintf("git_diff_summaries_%s_%s_%s.json", date, name, hex.EncodeToString(id)))
}

func SummarizeChanges(cfg Config, tmpl string, files []FileDiff, dst string) (string, error) {
	var summaries []FileSummary

	for i, fd := range files {
		PrintProcessing(fmt.Sprintf("Summarizing %s (%d/%d)...", fd.Path, i+1, len(files)))

		prompt := FormatPrompt(tmpl, []string{fd.Path}, fd.Diff)
		result, err := CallLLM(prompt, cfg, DefaultMaxTokens)
		if err != nil {
			return "", fmt.Errorf("summarize %s: %w", fd.Path, err)
		}

		s := ParseSummary(result, fd.Path)
		summaries = append(summaries, s)

		data, _ := json.MarshalIndent(summaries, "", "  ")
		if err := os.WriteFile(dst, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "  ! warning: could not write summaries: %v\n", err)
		}
	}

	out, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal summaries: %w", err)
	}
	return string(out), nil
}

func ParseSummary(text, file string) FileSummary {
	raw, err := ExtractJSON(text)
	if err != nil {
		return FallbackSummary(text, file)
	}

	var s FileSummary
	if err := json.Unmarshal(raw, &s); err != nil {
		return FallbackSummary(text, file)
	}

	if s.File == "" {
		s.File = file
	}
	return s
}

const (
	MaxDumpLen     = 300
	MaxSummaryLen  = 500
)

func FallbackSummary(text, file string) FileSummary {
	dump := text
	if len(dump) > MaxDumpLen {
		dump = dump[:MaxDumpLen] + "..."
	}
	// First line only in the error message, full text goes into the summary
	first := dump
	if idx := strings.IndexByte(first, '\n'); idx > 0 {
		first = first[:min(idx, 200)]
	}
	fmt.Fprintf(os.Stderr, "  %s could not parse summary for %s\n", yellow("!"), file)
	fmt.Fprintf(os.Stderr, "    response: %s\n", first)

	summary := text
	if len(summary) > MaxSummaryLen {
		summary = summary[:MaxSummaryLen] + "..."
	}
	return FileSummary{File: file, Summary: summary}
}
