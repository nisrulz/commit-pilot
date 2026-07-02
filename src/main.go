package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultModel    = "gemma-4-e2b-it-qat"
	defaultAPIBase  = "http://localhost:1234/v1"
	defaultMaxTokens = 4096
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

	estimatedTokens := estimatePromptTokens(tmpl, changes.FilesWithDiffs)
	if !canFitInContext(tmpl, changes.FilesWithDiffs, cfg.ContextWindow) {
		fmt.Printf("  %s Large diff detected (%s tokens estimated, %s token context)\n",
			yellow("!"),
			formatNumber(estimatedTokens),
			formatNumber(cfg.ContextWindow))
	}

	if cfg.Mode == ModeSingle {
		runSingleMode(changes, cfg, tmpl)
	} else {
		runAutoMode(changes, cfg, tmpl)
	}

	checkAndCommitRemainingChanges(cfg, tmpl)
}

func runSingleMode(changes *Changes, cfg Config, tmpl string) {
	printProcessing("Generating commit message...")

	batches := splitFilesIntoBatches(tmpl, changes.FilesWithDiffs, cfg.ContextWindow)
	grouped := groupChunkedBatches(batches)

	var allGroups []CommitGroup
	for i, g := range grouped {
		printProcessing(fmt.Sprintf("Processing batch %d/%d (%s)...", i+1, len(grouped), batchLabel(g)))

		group, err := groupFromAI(tmpl, cfg, g, defaultMaxTokens)
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
	if !executeCommit(allFilePaths(changes), subject, merged.Description, cfg.DryRun) {
		os.Exit(1)
	}
}

func allFilePaths(changes *Changes) []string {
	files := make([]string, 0, len(changes.FilesWithDiffs)+len(changes.BinaryFiles))
	for _, f := range changes.FilesWithDiffs {
		files = append(files, f.Path)
	}
	return append(files, changes.BinaryFiles...)
}

func batchLabel(batch []FileDiff) string {
	if isChunkedBatch(batch) {
		return fmt.Sprintf("chunk group (%s)", batch[0].Path)
	}
	return pluralize(len(batch), "file")
}

func groupChunkedBatches(batches [][]FileDiff) [][]FileDiff {
	if len(batches) <= 1 {
		return batches
	}

	var result [][]FileDiff
	current := batches[0]

	for i := 1; i < len(batches); i++ {
		if len(current) > 0 && len(batches[i]) == 1 &&
			current[0].Path == batches[i][0].Path {
			current = append(current, batches[i]...)
		} else {
			result = append(result, current)
			current = batches[i]
		}
	}
	result = append(result, current)
	return result
}

func checkAndCommitRemainingChanges(cfg Config, tmpl string) {
	fmt.Println()
	fmt.Println("  Checking for any remaining uncommitted changes...")

	status, err := gitRun("status", "--porcelain")
	if err != nil {
		die("git status failed: %v", err)
	}

	if status == "" {
		fmt.Printf("  %s Working directory is clean. Exiting successfully.\n", green("✔"))
		return
	}

	fmt.Printf("  %s Found uncommitted changes. Attempting to group and commit.\n", yellow("⚠️"))

	remainingChanges, err := getGitChanges()
	if err != nil {
		die("git status check failed: %v", err)
	}

	if len(remainingChanges.AllFiles) == 0 {
		fmt.Printf("  %s Status check indicated changes, but getGitChanges found none. Exiting.\n", green("✔"))
		return
	}

	fmt.Printf("  %s Re-analyzing remaining changes for a final commit...\n", yellow("🧠"))

	runAutoMode(remainingChanges, cfg, tmpl)

	finalStatus, _ := gitRun("status", "--porcelain")
	if finalStatus == "" {
		fmt.Printf("  %s Clean Checkout Successful\n", green("✔"))
	} else {
		fmt.Fprintf(os.Stderr, "  %s CRITICAL WARNING: Final git status still shows uncommitted changes:\n", red("🚨"))
		fmt.Fprintf(os.Stderr, "  %s %s\n", yellow("!"), finalStatus)
		os.Exit(1)
	}
}

func runAutoMode(changes *Changes, cfg Config, tmpl string) {
	files := changes.FilesWithDiffs
	if len(files) == 0 {
		return
	}

	target := summariesPath()
	fmt.Fprintf(os.Stderr, "  %s Summaries -> %s\n", yellow("~"), target)

	summarizeTmpl := loadSection("summarize")
	planTmpl := loadSection("plan")

	summariesJSON, err := summarizeChanges(cfg, summarizeTmpl, files, target)
	if err != nil {
		if ctxErr, ok := err.(*ContextLengthError); ok {
			printContextError(ctxErr)
			return
		}
		die("summarization failed: %v", err)
	}

	groups, err := planFromSummaries(planTmpl, cfg, summariesJSON)
	if err != nil {
		if ctxErr, ok := err.(*ContextLengthError); ok {
			printContextError(ctxErr)
			return
		}
		die("planning failed: %v", err)
	}

	allPaths := allFilePaths(changes)
	for i, g := range groups {
		groups[i].Files = limitCommitScope(filterValidFiles(g.Files, allPaths))
	}

	groups = assignBinaryFiles(groups, changes.BinaryFiles)
	groups = mergeGroups(groups)

	if len(groups) == 0 {
		return
	}

	printStep(fmt.Sprintf("Found %s", pluralize(len(groups), "logical work package")))
	commitFailed := false
	for _, g := range groups {
		if !executeCommit(g.Files, g.Subject, g.Description, cfg.DryRun) {
			commitFailed = true
		}
	}
	if commitFailed {
		os.Exit(1)
	}
}

func mergeCommitGroups(groups []CommitGroup) CommitGroup {
	if len(groups) == 0 {
		return CommitGroup{}
	}
	if len(groups) == 1 {
		return groups[0]
	}

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
