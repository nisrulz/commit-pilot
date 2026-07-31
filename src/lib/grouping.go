package lib

import (
	"sort"
	"strings"
)

func AssignBinaryFiles(groups []CommitGroup, binaryFiles []string) []CommitGroup {
	if len(binaryFiles) == 0 || len(groups) == 0 {
		return groups
	}
	files := append([]string(nil), binaryFiles...)
	sort.Strings(files)
	var desc strings.Builder
	desc.WriteString("Update binary files:")
	for _, file := range files {
		desc.WriteString("\n- ")
		desc.WriteString(file)
	}
	return append(groups, CommitGroup{
		Subject:     "chore: update binary assets",
		Description: desc.String(),
		Files:       files,
	})
}
