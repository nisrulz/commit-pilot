package main

import (
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxFilesPerGroup = 3
	minCommonPrefix  = 20
)

func fileCategory(path string) string {
	name := strings.ToLower(filepath.Base(path))
	ext := filepath.Ext(name)

	switch name {
	case "readme", "license", "changelog", "plan", "manual":
		return "docs"
	case ".gitignore", ".gitattributes", ".env":
		return "config"
	case "config.js", "config.json", "config.yaml", "config.yml":
		return "config"
	}

	switch ext {
	case ".md", ".txt", ".rst":
		return "docs"
	case ".json", ".yaml", ".yml", ".toml":
		return "config"
	}

	if name == "run.sh" || name == "stop.sh" || name == "deploy.sh" ||
		name == "build.sh" || name == "test.sh" {
		return "scripts"
	}

	return "code"
}

func filterValidFiles(candidateFiles, validFiles []string) []string {
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

func limitCommitScope(files []string) []string {
	if len(files) <= maxFilesPerGroup {
		return files
	}

	cats := make(map[string]string, len(files))
	counts := make(map[string]int)
	for _, f := range files {
		c := fileCategory(f)
		cats[f] = c
		counts[c]++
	}

	primary := ""
	maxCount := 0
	for c, n := range counts {
		if n > maxCount {
			maxCount = n
			primary = c
		}
	}

	var same []string
	for _, f := range files {
		if cats[f] == primary {
			same = append(same, f)
		}
	}

	if len(same) > maxFilesPerGroup {
		same = same[:maxFilesPerGroup]
	}
	return same
}

func subjectsRelated(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
		return true
	}

	common := 0
	minLen := min(len(a), len(b))
	for common < minLen && a[common] == b[common] {
		common++
	}

	return common >= minCommonPrefix
}

func mergeGroups(groups []CommitGroup) []CommitGroup {
	if len(groups) <= 1 {
		return groups
	}

	used := make([]bool, len(groups))
	var merged []CommitGroup

	for i, g := range groups {
		if used[i] {
			continue
		}
		used[i] = true

		subj := strings.ToLower(strings.TrimRight(g.Subject, "."))
		files := make(map[string]bool)
		for _, f := range g.Files {
			files[f] = true
		}

		for j := i + 1; j < len(groups); j++ {
			if used[j] {
				continue
			}
			other := strings.ToLower(strings.TrimRight(groups[j].Subject, "."))
			if subjectsRelated(subj, other) {
				used[j] = true
				for _, f := range groups[j].Files {
					files[f] = true
				}
			}
		}

		sorted := make([]string, 0, len(files))
		for f := range files {
			sorted = append(sorted, f)
		}
		sort.Strings(sorted)

		merged = append(merged, CommitGroup{
			Subject:     g.Subject,
			Description: g.Description,
			Files:       sorted,
		})
	}

	return merged
}

func assignBinaryFiles(groups []CommitGroup, binaryFiles []string) []CommitGroup {
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
