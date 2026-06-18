package main

import (
	"encoding/json"
	"fmt"
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

func executeCommit(files []string, subject, description string, dryRun bool) {
	if len(files) == 0 {
		return
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
		gitRun(addArgs...)
		gitRun("commit", "-m", subject, "-m", description)
	}

	fmt.Println()
	printCommitSection(subject, description, files, dryRun)
}
