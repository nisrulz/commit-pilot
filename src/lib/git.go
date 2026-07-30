package lib

import (
	"crypto/sha256"
	"encoding/hex"
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
	Fingerprint    string
}

func GitRun(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if len(out) > 0 {
				return string(out), nil
			}
			msg := strings.TrimSpace(string(ee.Stderr))
			return "", fmt.Errorf(msg)
		}
		return "", fmt.Errorf("git is not installed or not working")
	}
	return string(out), nil
}

func GitOutputLines(args ...string) []string {
	out, err := GitRun(args...)
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

// IsBinaryDiff checks if diff content indicates a binary file
func IsBinaryDiff(diff string) bool {
	// Check for explicit "Binary files" message
	if strings.Contains(diff, "Binary files") {
		return true
	}

	// Check for null bytes which indicate binary content
	for i := 0; i < len(diff); i++ {
		if diff[i] == 0 {
			return true
		}
	}

	return false
}

func GetGitChanges() (*Changes, error) {
	_, err := GitRun("rev-parse", "--git-dir")
	if err != nil {
		return nil, err
	}

	staged := GitOutputLines("diff", "--cached", "--name-only")
	hasStaged := len(staged) > 0

	var files []string
	if hasStaged {
		files = staged
	} else {
		files = GitOutputLines("diff", "--name-only")
		untracked := GitOutputLines("ls-files", "--others", "--exclude-standard")
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
	hash := sha256.New()

	for _, f := range files {
		var raw string
		var err error
		if hasStaged {
			raw, err = GitRun("diff", "--cached", "--", f)
		} else {
			raw, err = GitRun("diff", "--", f)
			if raw == "" && err == nil {
				raw, err = GitRun("diff", "--no-index", "/dev/null", f)
			}
		}

		if raw == "" {
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ! warning: could not diff %s: %v\n", f, err)
			}
			continue
		}
		hash.Write([]byte(f))
		hash.Write([]byte{0})
		hash.Write([]byte(raw))

		if IsBinaryDiff(raw) {
			binaryFiles = append(binaryFiles, f)
		} else {
			withDiffs = append(withDiffs, FileDiff{Path: f, Diff: raw})
		}
	}

	return &Changes{
		AllFiles:       files,
		FilesWithDiffs: withDiffs,
		BinaryFiles:    binaryFiles,
		Fingerprint:    hex.EncodeToString(hash.Sum(nil)),
	}, nil
}
