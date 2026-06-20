package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type FileDiff struct {
	Path string
	Diff string
}

type Changes struct {
	AllFiles       []string
	FilesWithDiffs []FileDiff
	BinaryFiles    []string
}

func gitRun(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s",
				strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func gitOutputLines(args ...string) []string {
	out, err := gitRun(args...)
	if err != nil {
		return nil
	}
	var lines []string
	for _, f := range strings.Split(strings.TrimSpace(out), "\n") {
		if f != "" {
			lines = append(lines, f)
		}
	}
	return lines
}

// isBinaryDiff checks if diff content indicates a binary file
func isBinaryDiff(diff string) bool {
	// Check for explicit "Binary files" message
	if strings.Contains(diff, "Binary files") {
		return true
	}

	// Check for null bytes which indicate binary content
	for i := 0; i < len(diff)-1; i++ {
		if diff[i] == 0 {
			return true
		}
	}

	return false
}

func getGitChanges() (*Changes, error) {
	_, err := gitRun("rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	staged := gitOutputLines("diff", "--cached", "--name-only")
	hasStaged := len(staged) > 0

	var files []string
	if hasStaged {
		files = staged
	} else {
		files = gitOutputLines("diff", "--name-only")
		untracked := gitOutputLines("ls-files", "--others", "--exclude-standard")
		seen := make(map[string]bool, len(files))
		for _, f := range files {
			seen[f] = true
		}
		for _, f := range untracked {
			if !seen[f] {
				files = append(files, f)
			}
			seen[f] = true
		}
	}

	var withDiffs []FileDiff
	var binaryFiles []string

	for _, f := range files {
		var raw string
		var err error
		if hasStaged {
			raw, err = gitRun("diff", "--cached", "--", f)
		} else {
			raw, err = gitRun("diff", "--", f)
			if raw == "" && err == nil {
				raw, err = gitRun("diff", "--no-index", "/dev/null", f)
			}
		}

		if raw == "" {
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ! warning: could not diff %s: %v\n", f, err)
			}
			continue
		}

		if isBinaryDiff(raw) {
			binaryFiles = append(binaryFiles, f)
		} else {
			withDiffs = append(withDiffs, FileDiff{Path: f, Diff: raw})
		}
	}

	return &Changes{
		AllFiles:       files,
		FilesWithDiffs: withDiffs,
		BinaryFiles:    binaryFiles,
	}, nil
}
