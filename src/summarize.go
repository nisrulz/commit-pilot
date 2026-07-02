package main

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

func summariesPath() string {
	home, _ := os.UserHomeDir()
	date := time.Now().Format("2006-01-02")

	name := "unknown"
	if out, err := gitRun("rev-parse", "--show-toplevel"); err == nil {
		name = filepath.Base(strings.TrimSpace(out))
	}

	id := make([]byte, 4)
	rand.Read(id)

	dir := filepath.Join(home, ".commit-pilot", "tmp")
	os.MkdirAll(dir, 0700)

	return filepath.Join(dir, fmt.Sprintf("git_diff_summaries_%s_%s_%s.json", date, name, hex.EncodeToString(id)))
}

func summarizeChanges(cfg Config, tmpl string, files []FileDiff, dst string) (string, error) {
	var summaries []FileSummary

	for i, fd := range files {
		printProcessing(fmt.Sprintf("Summarizing %s (%d/%d)...", fd.Path, i+1, len(files)))

		prompt := formatPrompt(tmpl, []string{fd.Path}, fd.Diff)
		result, err := callLLM(prompt, cfg, defaultMaxTokens)
		if err != nil {
			return "", fmt.Errorf("summarize %s: %w", fd.Path, err)
		}

		s := parseSummary(result, fd.Path)
		summaries = append(summaries, s)

		data, _ := json.MarshalIndent(summaries, "", "  ")
		os.WriteFile(dst, data, 0600)
	}

	out, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal summaries: %w", err)
	}
	return string(out), nil
}

func parseSummary(text, file string) FileSummary {
	raw, err := extractJSON(text)
	if err != nil {
		return fallbackSummary(text, file)
	}

	var s FileSummary
	if err := json.Unmarshal(raw, &s); err != nil {
		return fallbackSummary(string(raw), file)
	}

	if s.File == "" {
		s.File = file
	}
	return s
}

const (
	maxDumpLen     = 300
	maxSummaryLen  = 500
)

func fallbackSummary(text, file string) FileSummary {
	dump := text
	if len(dump) > maxDumpLen {
		dump = dump[:maxDumpLen] + "..."
	}
	fmt.Fprintf(os.Stderr, "  %s could not parse summary for %s, using raw response\n", yellow("!"), file)
	fmt.Fprintf(os.Stderr, "    raw: %s\n", dump)

	summary := text
	if len(summary) > maxSummaryLen {
		summary = summary[:maxSummaryLen] + "..."
	}
	return FileSummary{File: file, Summary: summary}
}
