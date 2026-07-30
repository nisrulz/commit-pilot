package lib

import (
	"encoding/json"
	"fmt"
	"os"
)

func WritePlan(path string, groups []CommitGroup) error {
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func ReadPlan(path string) ([]CommitGroup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var groups []CommitGroup
	if err := json.Unmarshal(data, &groups); err != nil || len(groups) == 0 {
		return nil, fmt.Errorf("plan must contain at least one commit group")
	}
	return groups, nil
}
