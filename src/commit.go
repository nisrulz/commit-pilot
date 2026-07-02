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

func isChunkedBatch(batch []FileDiff) bool {
	if len(batch) <= 1 {
		return false
	}
	p := batch[0].Path
	for _, fd := range batch[1:] {
		if fd.Path != p {
			return false
		}
	}
	return true
}

func groupFromAI(tmpl string, cfg Config, files []FileDiff, maxTokens int) (CommitGroup, error) {
	if isChunkedBatch(files) {
		return groupFromAIChunked(tmpl, cfg, files, maxTokens)
	}

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

func groupFromAIChunked(tmpl string, cfg Config, chunks []FileDiff, maxTokens int) (CommitGroup, error) {
	if len(chunks) == 0 {
		return CommitGroup{}, nil
	}

	path := chunks[0].Path
	var groups []CommitGroup
	for i, ch := range chunks {
		printProcessing(fmt.Sprintf("Chunk %d/%d of %s", i+1, len(chunks), path))
		g, err := groupFromAI(tmpl, cfg, []FileDiff{ch}, maxTokens)
		if err != nil {
			return CommitGroup{}, err
		}
		groups = append(groups, g)
	}
	return mergeCommitGroups(groups), nil
}

func parseCommitGroup(text string) (CommitGroup, error) {
	raw, err := extractJSON(text)
	if err != nil {
		return CommitGroup{}, fmt.Errorf("extract JSON: %w", err)
	}

	var g CommitGroup
	if err := json.Unmarshal(raw, &g); err == nil {
		if g.Subject != "" {
			return g, nil
		}
	}

	var groups []CommitGroup
	if err := json.Unmarshal(raw, &groups); err == nil && len(groups) > 0 {
		return groups[0], nil
	}

	return CommitGroup{}, fmt.Errorf("parse commit group: expected JSON object with 'subject' field")
}

func executeCommit(files []string, subject, description string, dryRun bool) bool {
	if len(files) == 0 {
		return false
	}

	subject = strings.TrimSpace(subject)
	subject = strings.ReplaceAll(subject, "\n", " ")
	subject = strings.ReplaceAll(subject, "\r", "")
	subject = strings.TrimLeft(subject, "-")
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
