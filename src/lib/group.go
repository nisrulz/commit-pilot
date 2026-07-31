package lib

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CommitGroup is a proposed commit: a subject line, an optional multi-line
// description, and the list of file paths it covers.
type CommitGroup struct {
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// IsChunkedBatch reports whether a batch holds multiple diff chunks of a single
// oversized file (as opposed to one diff per file).
func IsChunkedBatch(batch []FileDiff) bool {
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

// GroupFromAI asks the model to produce a commit group for the given files. An
// oversized file split into chunks is handled by GroupFromAIChunked.
func GroupFromAI(tmpl string, cfg Config, files []FileDiff, maxTokens int) (CommitGroup, error) {
	if IsChunkedBatch(files) {
		return GroupFromAIChunked(tmpl, cfg, files, maxTokens)
	}

	fileList := make([]string, len(files))
	for i, f := range files {
		fileList[i] = f.Path
	}

	prompt := FormatPrompt(tmpl, fileList, FormatDiffSection(files))
	result, err := CallLLM(prompt, cfg, maxTokens)
	if err != nil {
		return CommitGroup{}, fmt.Errorf("AI call: %w", err)
	}

	return ParseCommitGroup(result)
}

// GroupFromAIChunked summarizes each chunk of an oversized file separately and
// merges the results into one commit group.
func GroupFromAIChunked(tmpl string, cfg Config, chunks []FileDiff, maxTokens int) (CommitGroup, error) {
	if len(chunks) == 0 {
		return CommitGroup{}, nil
	}

	path := chunks[0].Path
	var groups []CommitGroup
	for i, ch := range chunks {
		PrintProcessing(fmt.Sprintf("Chunk %d/%d of %s", i+1, len(chunks), path))
		g, err := GroupFromAI(tmpl, cfg, []FileDiff{ch}, maxTokens)
		if err != nil {
			return CommitGroup{}, err
		}
		groups = append(groups, g)
	}
	return MergeCommitGroups(groups), nil
}

// MergeCommitGroups combines per-chunk results into a single group, keeping the
// first subject and concatenating the descriptions.
func MergeCommitGroups(groups []CommitGroup) CommitGroup {
	if len(groups) == 0 {
		return CommitGroup{}
	}
	if len(groups) == 1 {
		return groups[0]
	}

	var subjects []string
	var descriptions []string
	for _, g := range groups {
		if g.Subject != "" {
			subjects = append(subjects, g.Subject)
		}
		if g.Description != "" {
			descriptions = append(descriptions, g.Description)
		}
	}

	subject := "chore: update"
	if len(subjects) > 0 {
		subject = subjects[0]
	}

	description := strings.Join(descriptions, "\n\n")

	return CommitGroup{
		Subject:     subject,
		Description: description,
	}
}

// ParseCommitGroup extracts a commit group from an AI response, accepting
// either a single commit object or an array of commits (first one wins).
func ParseCommitGroup(text string) (CommitGroup, error) {
	raw, err := ExtractJSON(text)
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
