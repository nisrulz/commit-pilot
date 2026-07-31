package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func PlanFromSummaries(tmpl string, cfg Config, summariesJSON string) ([]CommitGroup, error) {
	PrintProcessing("Planning commits from summaries...")

	r := strings.NewReplacer("{diff}", summariesJSON)
	prompt := r.Replace(tmpl)

	result, err := CallLLM(prompt, cfg, DefaultMaxTokens)
	if err != nil {
		return nil, fmt.Errorf("plan commits: %w", err)
	}

	raw, err := ExtractJSON(result)
	if err != nil {
		return nil, fmt.Errorf("extract plan: %w", err)
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
			fmt.Fprintf(os.Stderr, "  %s Could not parse AI plan, grouping all files into one commit\n", yellow("!"))
		}
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("plan returned empty groups")
	}

	return NormalizeCommitGroups(groups), nil
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
