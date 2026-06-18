package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultModel   = "gemma-4-e2b-it-qat"
	defaultAPIBase = "http://localhost:1234/v1"
)

func main() {
	flags, showHelp := parseArgs(os.Args[1:])
	if showHelp {
		printHelp()
		return
	}

	cfg := resolveConfig(flags)

	tmpl := loadPrompt(cfg.Mode, cfg.Prompt)

	changes, err := getGitChanges()
	if err != nil {
		die("git: %v", err)
	}

	if len(changes.AllFiles) == 0 {
		fmt.Printf("  %s No changes to commit.\n", yellow("\u26a1"))
		return
	}

	if len(changes.FilesWithDiffs) == 0 && len(changes.BinaryFiles) > 0 {
		fmt.Printf("  %s Only binary files changed \u2014 cannot generate AI commit message.\n", yellow("\u26a1"))
		return
	}

	printStep(fmt.Sprintf("Found %s", pluralize(len(changes.AllFiles), "changed file")))
	if len(changes.BinaryFiles) > 0 {
		fmt.Printf("    (binary: %s)\n", strings.Join(changes.BinaryFiles, ", "))
	}

	if cfg.Mode == ModeSingle {
		runSingleMode(changes, cfg, tmpl)
	} else {
		runAutoMode(changes, cfg, tmpl)
	}
}

func runSingleMode(changes *Changes, cfg Config, tmpl string) {
	printProcessing("Generating commit message...")

	group, err := groupFromAI(tmpl, cfg, changes.FilesWithDiffs, 2048)
	if err != nil {
		die("AI call failed: %v", err)
	}

	subject := group.Subject
	if subject == "" {
		subject = "chore: update"
	}

	allFiles := make([]string, 0, len(changes.FilesWithDiffs)+len(changes.BinaryFiles))
	for _, f := range changes.FilesWithDiffs {
		allFiles = append(allFiles, f.Path)
	}
	allFiles = append(allFiles, changes.BinaryFiles...)

	executeCommit(allFiles, subject, group.Description, cfg.DryRun)
}

func runAutoMode(changes *Changes, cfg Config, tmpl string) {
	remaining := make([]FileDiff, len(changes.FilesWithDiffs))
	copy(remaining, changes.FilesWithDiffs)

	if len(remaining) == 0 {
		return
	}

	var groups []CommitGroup

	printProcessing(fmt.Sprintf("Processing %s...", pluralize(len(remaining), "file")))

	for len(remaining) > 0 {
		group, err := groupFromAI(tmpl, cfg, remaining, 3072)
		if err != nil {
			die("AI call failed: %v", err)
		}

		if len(group.Files) == 0 {
			f := remaining[0]
			groups = append(groups, CommitGroup{
				Subject:     "chore: update",
				Description: "Update " + f.Path,
				Files:       []string{f.Path},
			})
			remaining = remaining[1:]
			continue
		}

		remainingPaths := make([]string, len(remaining))
		for i, f := range remaining {
			remainingPaths[i] = f.Path
		}

		commitFiles := limitCommitScope(filterValidFiles(group.Files, remainingPaths))
		if len(commitFiles) == 0 {
			commitFiles = []string{remaining[0].Path}
		}

		groups = append(groups, CommitGroup{
			Subject:     group.Subject,
			Description: group.Description,
			Files:       commitFiles,
		})

		committed := make(map[string]bool, len(commitFiles))
		for _, f := range commitFiles {
			committed[f] = true
		}
		var next []FileDiff
		for _, f := range remaining {
			if !committed[f.Path] {
				next = append(next, f)
			}
		}
		remaining = next
	}

	groups = assignBinaryFiles(groups, changes.BinaryFiles)
	groups = mergeGroups(groups)

	printStep(fmt.Sprintf("Found %s", pluralize(len(groups), "logical work package")))
	for _, g := range groups {
		executeCommit(g.Files, g.Subject, g.Description, cfg.DryRun)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  ! "+format+"\n", args...)
	os.Exit(1)
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
