package lib

import (
	"bufio"
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

func ExecuteCommit(files []string, subject, description string, dryRun bool, maxSubjectLength int) bool {
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
	if maxSubjectLength <= 0 {
		maxSubjectLength = MaxSubjectLength
	}
	if utf8.RuneCountInString(subject) > maxSubjectLength {
		subject = string([]rune(subject)[:maxSubjectLength])
	}

	if !dryRun {
		if _, err := GitRun(append([]string{"add"}, files...)...); err != nil {
			fmt.Fprintf(os.Stderr, "  ! git add failed: %v\n", err)
			return false
		}
		commitArgs := append([]string{"commit", "--only", "-m", subject, "-m", description, "--"}, files...)
		if _, err := GitRun(commitArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "  ! git commit failed: %v\n", err)
			return false
		}
	}

	fmt.Println()
	PrintCommitSection(subject, description, files, dryRun)
	return true
}

func ConfirmCommitPlan(groups []CommitGroup, cfg Config, fingerprint string) bool {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	in := cfg.Input
	if in == nil {
		in = os.Stdin
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Proposed commit plan:")
	for i, group := range groups {
		fmt.Fprintf(out, "    %d. %s\n", i+1, group.Subject)
		if description := strings.TrimSpace(group.Description); description != "" {
			fmt.Fprintf(out, "       %s\n", description)
		}
		fmt.Fprintf(out, "       Files: %s\n", strings.Join(group.Files, ", "))
	}

	if cfg.DryRun {
		return true
	}
	current, err := GetGitChangesForScope(cfg.Scope)
	if err != nil || current.Fingerprint != fingerprint {
		fmt.Fprintln(out, "  Changes were updated while the plan was being generated. Please run commit-pilot again.")
		return false
	}
	if cfg.Yes {
		return true
	}

	fmt.Fprint(out, "  Apply this plan? [y/N] ")
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
		fmt.Fprintln(out, "  No commits created.")
		return false
	}
	return true
}
