package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const MaxSubjectLength = 100

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

func ValidatePlan(groups []CommitGroup, validFiles []string) error {
	valid := make(map[string]bool, len(validFiles))
	for _, file := range validFiles {
		valid[file] = true
	}
	seen := make(map[string]bool, len(validFiles))
	for i, group := range groups {
		if strings.TrimSpace(group.Subject) == "" || len(group.Files) == 0 {
			return fmt.Errorf("group %d must have a subject and at least one file", i+1)
		}
		for _, file := range group.Files {
			if !valid[file] {
				return fmt.Errorf("group %d contains unknown file %q", i+1, file)
			}
			if seen[file] {
				return fmt.Errorf("file %q appears more than once", file)
			}
			seen[file] = true
		}
	}
	var missing []string
	for _, file := range validFiles {
		if !seen[file] {
			missing = append(missing, file)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("plan does not cover: %s", strings.Join(missing, ", "))
	}
	return nil
}

func LintPlan(groups []CommitGroup, validFiles []string) error {
	if err := ValidatePlan(groups, validFiles); err != nil {
		return err
	}
	var issues []string
	for i, group := range groups {
		if len([]rune(group.Subject)) > MaxSubjectLength {
			issues = append(issues, fmt.Sprintf("group %d subject exceeds %d characters", i+1, MaxSubjectLength))
		}
		if !conventionalSubject(group.Subject) {
			issues = append(issues, fmt.Sprintf("group %d subject is not a conventional commit", i+1))
		}
	}
	if len(issues) > 0 {
		return fmt.Errorf("%s", strings.Join(issues, "; "))
	}
	return nil
}

func conventionalSubject(subject string) bool {
	typeEnd := strings.IndexByte(subject, ':')
	if typeEnd < 1 || typeEnd+2 > len(subject) || subject[typeEnd+1] != ' ' {
		return false
	}
	typeName := subject[:typeEnd]
	if scopeStart := strings.IndexByte(typeName, '('); scopeStart >= 0 {
		if !strings.HasSuffix(typeName, ")") || scopeStart == 0 || scopeStart == len(typeName)-2 {
			return false
		}
		typeName = typeName[:scopeStart]
	}
	for _, r := range typeName {
		if !(r >= 'a' && r <= 'z') && r != '-' {
			return false
		}
	}
	return strings.TrimSpace(subject[typeEnd+2:]) != ""
}
