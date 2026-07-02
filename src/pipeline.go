package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func planFromSummaries(tmpl string, cfg Config, summariesJSON string) ([]CommitGroup, error) {
	printProcessing("Planning commits from summaries...")

	r := strings.NewReplacer("{diff}", summariesJSON)
	prompt := r.Replace(tmpl)

	result, err := callLLM(prompt, cfg, defaultMaxTokens)
	if err != nil {
		return nil, fmt.Errorf("plan commits: %w", err)
	}

	raw, err := extractJSON(result)
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
