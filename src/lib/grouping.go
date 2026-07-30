package lib

import (
	"path/filepath"
	"sort"
	"strings"
)

func FilterValidFiles(candidateFiles, validFiles []string) []string {
	valid := make(map[string]bool, len(validFiles))
	for _, f := range validFiles {
		valid[f] = true
	}
	var out []string
	for _, f := range candidateFiles {
		if valid[f] {
			out = append(out, f)
		}
	}
	return out
}

func AssignBinaryFiles(groups []CommitGroup, binaryFiles []string) []CommitGroup {
	if len(binaryFiles) == 0 || len(groups) == 0 {
		return groups
	}

	unassigned := make(map[string]bool, len(binaryFiles))
	for _, f := range binaryFiles {
		unassigned[f] = true
	}

	for i := range groups {
		if len(unassigned) == 0 {
			break
		}
		if len(groups[i].Files) == 0 {
			continue
		}

		dirs := make(map[string]bool)
		for _, f := range groups[i].Files {
			dirs[filepath.Dir(f)] = true
		}

		for bf := range unassigned {
			if dirs[filepath.Dir(bf)] {
				groups[i].Files = append(groups[i].Files, bf)
				delete(unassigned, bf)
			}
		}
	}

	if len(unassigned) > 0 {
		remaining := make([]string, 0, len(unassigned))
		for f := range unassigned {
			remaining = append(remaining, f)
		}
		sort.Strings(remaining)

		var desc strings.Builder
		desc.WriteString("Update binary files:")
		for _, f := range remaining {
			desc.WriteString("\n- ")
			desc.WriteString(f)
		}

		groups = append(groups, CommitGroup{
			Subject:     "chore: update binary assets",
			Description: desc.String(),
			Files:       remaining,
		})
	}

	return groups
}
