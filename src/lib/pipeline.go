package lib

import (
	"encoding/json"
	"fmt"
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
			return nil, fmt.Errorf("parse plan: %w", err)
		}
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("plan returned empty groups")
	}

	return groups, nil
}
