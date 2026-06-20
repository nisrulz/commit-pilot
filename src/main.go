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

	// Check if diffs fit in context window
	estimatedTokens := estimatePromptTokens(tmpl, changes.FilesWithDiffs)
	if !canFitInContext(tmpl, changes.FilesWithDiffs, cfg.ContextWindow) {
		batches := splitFilesIntoBatches(tmpl, changes.FilesWithDiffs, cfg.ContextWindow)
		fmt.Printf("  %s Large diff detected (%s tokens estimated, %s token context)\n",
			yellow("!"),
			formatNumber(estimatedTokens),
			formatNumber(cfg.ContextWindow))
		fmt.Printf("    Processing in %d batches\n", len(batches))
	}

	if cfg.Mode == ModeSingle {
		runSingleMode(changes, cfg, tmpl)
	} else {
		runAutoMode(changes, cfg, tmpl)
	}
}

func runSingleMode(changes *Changes, cfg Config, tmpl string) {
	printProcessing("Generating commit message...")

	batches := splitFilesIntoBatches(tmpl, changes.FilesWithDiffs, cfg.ContextWindow)

	if len(batches) > 1 {
		var allGroups []CommitGroup
		for i, batch := range batches {
			printProcessing(fmt.Sprintf("Processing batch %d/%d (%s)...",
				i+1, len(batches), pluralize(len(batch), "file")))

			group, err := groupFromAI(tmpl, cfg, batch, 4096)
			if err != nil {
				if ctxErr, ok := err.(*ContextLengthError); ok {
					printContextError(ctxErr)
					return
				}
				die("AI call failed: %v", err)
			}
			allGroups = append(allGroups, group)
		}

		merged := mergeCommitGroups(allGroups)
		subject := merged.Subject
		if subject == "" {
			subject = "chore: update"
		}
		executeCommit(allFilePaths(changes), subject, merged.Description, cfg.DryRun)
	} else {
		group, err := groupFromAI(tmpl, cfg, changes.FilesWithDiffs, 4096)
		if err != nil {
			if ctxErr, ok := err.(*ContextLengthError); ok {
				printContextError(ctxErr)
				return
			}
			die("AI call failed: %v", err)
		}
		subject := group.Subject
		if subject == "" {
			subject = "chore: update"
		}
		executeCommit(allFilePaths(changes), subject, group.Description, cfg.DryRun)
	}
}

func allFilePaths(changes *Changes) []string {
	files := make([]string, 0, len(changes.FilesWithDiffs)+len(changes.BinaryFiles))
	for _, f := range changes.FilesWithDiffs {
		files = append(files, f.Path)
	}
	return append(files, changes.BinaryFiles...)
}

func runAutoMode(changes *Changes, cfg Config, tmpl string) {
	remaining := make([]FileDiff, len(changes.FilesWithDiffs))
	copy(remaining, changes.FilesWithDiffs)

	if len(remaining) == 0 {
		return
	}

	var groups []CommitGroup

	for len(remaining) > 0 {
		group, commitFiles := processNextBatch(tmpl, cfg, remaining, &groups)
		if len(commitFiles) == 0 && group.Subject == "" {
			return
		}

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

func processNextBatch(tmpl string, cfg Config, remaining []FileDiff, groups *[]CommitGroup) (CommitGroup, []string) {
	batches := splitFilesIntoBatches(tmpl, remaining, cfg.ContextWindow)
	batch := batches[0]

	if len(batches) > 1 {
		printProcessing(fmt.Sprintf("Processing batch of %s files...", pluralize(len(batch), "")))
	}

	group, err := groupFromAI(tmpl, cfg, batch, 4096)
	if err != nil {
		if ctxErr, ok := err.(*ContextLengthError); ok {
			printContextError(ctxErr)
			return CommitGroup{}, nil
		}
		die("AI call failed: %v", err)
	}

	if len(group.Files) == 0 {
		f := batch[0]
		*groups = append(*groups, CommitGroup{
			Subject:     "chore: update",
			Description: "Update " + f.Path,
			Files:       []string{f.Path},
		})
		return group, []string{f.Path}
	}

	remainingPaths := make([]string, len(remaining))
	for i, f := range remaining {
		remainingPaths[i] = f.Path
	}

	commitFiles := limitCommitScope(filterValidFiles(group.Files, remainingPaths))
	if len(commitFiles) == 0 {
		commitFiles = []string{batch[0].Path}
	}

	*groups = append(*groups, CommitGroup{
		Subject:     group.Subject,
		Description: group.Description,
		Files:       commitFiles,
	})

	return group, commitFiles
}

func mergeCommitGroups(groups []CommitGroup) CommitGroup {
	if len(groups) == 0 {
		return CommitGroup{}
	}
	if len(groups) == 1 {
		return groups[0]
	}

	// Combine all subjects and descriptions
	var subjects []string
	var descriptions []string
	for _, g := range groups {
		if g.Subject != "" {
			subjects = append(subjects, g.Subject)
		}
		if g.Description != "" {
			descriptions = append(descriptions, g.Description)
		}
	}

	subject := "chore: update"
	if len(subjects) > 0 {
		// Use the first subject as the main one
		subject = subjects[0]
	}

	description := strings.Join(descriptions, "\n\n")

	return CommitGroup{
		Subject:     subject,
		Description: description,
	}
}

func printContextError(err *ContextLengthError) {
	fmt.Println()
	fmt.Fprintf(os.Stderr, "  %s %s\n", red("ERROR:"), err.Message)
	fmt.Fprintf(os.Stderr, "    Estimated tokens: %s\n", formatNumber(err.Estimated))
	fmt.Fprintf(os.Stderr, "    Context window:   %s tokens\n", formatNumber(err.Available))
	fmt.Println()
	fmt.Fprintf(os.Stderr, "  %s To fix this, you can:\n", yellow("SUGGESTIONS:"))
	fmt.Fprintf(os.Stderr, "    1. Increase context window: export COMMIT_PILOT_CONTEXT_WINDOW=131072\n")
	fmt.Fprintf(os.Stderr, "    2. Stage fewer files at once\n")
	fmt.Fprintf(os.Stderr, "    3. Use a model with larger context window\n")
	os.Exit(1)
}

func formatNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
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
