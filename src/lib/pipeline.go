package lib

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func PlanFromSummaries(tmpl string, cfg Config, summariesJSON string) ([]CommitGroup, error) {
	PrintProcessing("Planning commits from summaries...")

	summariesJSON = CompactSummariesForPlan(summariesJSON, tmpl, cfg.ContextWindow)

	r := strings.NewReplacer("{diff}", summariesJSON)
	prompt := r.Replace(tmpl)

	result, err := callPlanLLM(prompt, cfg)
	if err != nil {
		return nil, fmt.Errorf("plan commits: %w", err)
	}

	raw, err := ExtractJSON(result)
	if err != nil {
		// The model returned no JSON at all. Ask once more with an explicit
		// instruction before giving up.
		PrintProcessing("Plan response contained no JSON, retrying...")
		strict := prompt + "\n\nIMPORTANT: Respond with ONLY a single valid JSON array. No prose, no markdown fences, no commentary."
		result, err = callPlanLLM(strict, cfg)
		if err != nil {
			return nil, fmt.Errorf("plan commits: %w", err)
		}
		raw, err = ExtractJSON(result)
		if err != nil {
			return nil, fmt.Errorf("extract plan: %w", err)
		}
	}

	var groups []CommitGroup
	if err := json.Unmarshal(raw, &groups); err != nil {
		var single CommitGroup
		if err2 := json.Unmarshal(raw, &single); err2 == nil {
			groups = []CommitGroup{single}
		} else {
			groups = FallbackPlan(summariesJSON)
			if len(groups) == 0 {
				return nil, fmt.Errorf("AI response was not in the expected format")
			}
			Warning("Could not parse AI plan, grouping all files into one commit")
		}
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("plan returned empty groups")
	}

	return NormalizeCommitGroups(groups), nil
}

// callPlanLLM sends the planning prompt under a working spinner, retrying once
// with a larger output budget if the model's response was cut off mid-
// generation.
func callPlanLLM(prompt string, cfg Config) (string, error) {
	result, err := callLLMWithSpinner(prompt, cfg, DefaultMaxTokens)
	if err == nil {
		return result, nil
	}
	var trunc *TruncatedError
	if !errors.As(err, &trunc) {
		return "", err
	}
	PrintProcessing("Plan was cut off, retrying with a larger output budget...")
	return callLLMWithSpinner(prompt, cfg, DefaultMaxTokens*2)
}

// callLLMWithSpinner runs a single LLM call under a working spinner.
func callLLMWithSpinner(prompt string, cfg Config, maxTokens int) (string, error) {
	stop := startSpinner()
	defer stop()
	return CallLLM(prompt, cfg, maxTokens)
}

// CompactSummariesForPlan shrinks the summaries so the planning prompt fits the
// model's context window, after reserving tokens for the template, the
// response, and a safety margin. Inputs that already fit, are unparseable, or
// have no usable context window are returned unchanged.
func CompactSummariesForPlan(summariesJSON, template string, contextWindow int) string {
	available := AvailableDiffTokens(template, contextWindow)
	if available <= 0 {
		return summariesJSON
	}
	if EstimateTokens(summariesJSON) <= available {
		return summariesJSON
	}
	var summaries []FileSummary
	if err := json.Unmarshal([]byte(summariesJSON), &summaries); err != nil || len(summaries) == 0 {
		return summariesJSON
	}

	// Each entry costs tokens beyond its summary text: the JSON keys, the file
	// path, and the changes array wrappers.
	const framingPerFile = 40
	perFile := available/len(summaries) - framingPerFile
	const minPerFile = 64
	if perFile < minPerFile {
		perFile = minPerFile
	}
	compact := make([]FileSummary, len(summaries))
	for i, s := range summaries {
		compact[i] = s
		compact[i].Summary = trimSummaryToTokens(s.Summary, perFile)
	}
	out, err := json.Marshal(compact)
	if err != nil {
		return summariesJSON
	}
	return string(out)
}

// trimSummaryToTokens shortens a summary to fit roughly within a token budget,
// using a conservative characters-per-token ratio.
func trimSummaryToTokens(text string, budget int) string {
	if budget <= 0 {
		return ""
	}
	maxRunes := budget * 3
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func FallbackPlan(summariesJSON string) []CommitGroup {
	var summaries []FileSummary
	if err := json.Unmarshal([]byte(summariesJSON), &summaries); err != nil || len(summaries) == 0 {
		return nil
	}
	files := make([]string, len(summaries))
	for i, s := range summaries {
		files[i] = s.File
	}
	return []CommitGroup{{
		Subject:     "chore: update changes",
		Description: "Automated commit from commit-pilot",
		Files:       files,
	}}
}
