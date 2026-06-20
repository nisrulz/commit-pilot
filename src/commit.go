package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type CommitGroup struct {
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

func groupFromAI(tmpl string, cfg Config, files []FileDiff, maxTokens int) (CommitGroup, error) {
	fileList := make([]string, len(files))
	for i, f := range files {
		fileList[i] = f.Path
	}

	prompt := formatPrompt(tmpl, fileList, formatDiffSection(files))
	result, err := callLLM(prompt, cfg, maxTokens)
	if err != nil {
		return CommitGroup{}, fmt.Errorf("AI call: %w", err)
	}

	return parseCommitGroup(result)
}

func parseCommitGroup(text string) (CommitGroup, error) {
	raw, err := extractJSON(text)
	if err != nil {
		return CommitGroup{}, fmt.Errorf("extract JSON: %w", err)
	}

	var g CommitGroup
	if err := json.Unmarshal(raw, &g); err != nil {
		return CommitGroup{}, fmt.Errorf("parse commit group: %w", err)
	}

	if g.Subject == "" {
		var groups []CommitGroup
		if err := json.Unmarshal(raw, &groups); err == nil && len(groups) > 0 {
			return groups[0], nil
		}
	}

	return g, nil
}

func executeCommit(files []string, subject, description string, dryRun bool) bool {
	if len(files) == 0 {
		return false
	}

	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "chore: update"
	}
	if utf8.RuneCountInString(subject) > 100 {
		subject = string([]rune(subject)[:100])
	}

	if !dryRun {
		addArgs := append([]string{"add", "--"}, files...)
		if _, err := gitRun(addArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "  ! git add failed: %v\n", err)
			return false
		}
		if _, err := gitRun("commit", "-m", subject, "-m", description); err != nil {
			fmt.Fprintf(os.Stderr, "  ! git commit failed: %v\n", err)
			return false
		}
	}

	fmt.Println()
	printCommitSection(subject, description, files, dryRun)
	return true
}
