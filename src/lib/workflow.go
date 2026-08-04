package lib

import (
	"fmt"
	"os"
	"strings"
)

// runWorkflow is the main commit flow: it collects the current git changes and
// either lints a saved plan, applies a saved plan, or generates new commits
// through the configured AI provider.
func runWorkflow(cfg Config) {
	tmpl := ApplyMessagePreferences(LoadPrompt(cfg.Mode, cfg.Prompt), cfg)

	changes, err := GetGitChangesForScope(cfg.Scope)
	if err != nil {
		Die("git: %v", err)
	}
	if len(changes.AllFiles) == 0 {
		reportNoChanges(cfg)
		return
	}
	filtered := FilterChanges(changes, cfg.Include, cfg.Exclude, cfg.IncludeSensitive)
	if len(filtered) > 0 && !IsQuietOutput() {
		fmt.Printf("  %s Skipped: %s\n", yellow("~"), strings.Join(sanitizePaths(filtered), ", "))
	}
	if len(changes.AllFiles) == 0 {
		reportNoChanges(cfg)
		return
	}
	if err := ValidateChangeScope(changes); err != nil {
		Die("unsafe change scope: %v", err)
	}

	switch {
	case cfg.PlanLint != "":
		runPlanLint(cfg, changes)
	case cfg.Apply != "":
		runApply(cfg, changes)
	case len(changes.FilesWithDiffs) == 0 && len(changes.BinaryFiles) > 0:
		runBinaryOnly(cfg, changes)
	default:
		runGenerate(cfg, changes, tmpl)
	}
}

func runBinaryOnly(cfg Config, changes *Changes) {
	groups := AssignBinaryFiles(nil, changes.BinaryFiles)
	writePlanIfRequested(cfg.PlanOut, groups)
	if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
		if cfg.JSON {
			PrintRunResult("cancelled")
		}
		return
	}
	group := groups[0]
	if !ExecuteCommit(group.Files, group.Subject, group.Description, cfg.DryRun, cfg.MaxSubjectLength, changes.EffectiveScope) {
		os.Exit(1)
	}
	if cfg.JSON {
		status := "completed"
		if cfg.DryRun {
			status = "dry_run"
		}
		PrintRunResult(status)
	}
}

// runPlanLint validates a saved plan against the current changes and the
// configured message preferences, without applying it.
func runPlanLint(cfg Config, changes *Changes) {
	groups, err := ReadPlan(cfg.PlanLint)
	if err != nil {
		Die("read plan: %v", err)
	}
	if err := LintPlan(groups, AllFilePaths(changes), cfg); err != nil {
		Die("invalid plan: %v", err)
	}
	if cfg.JSON {
		PrintJSON(map[string]any{"status": "valid"})
	} else if !cfg.Quiet {
		Success("Plan is valid.")
	}
}

// runApply validates a saved plan against the current changes and commits each
// group after the user confirms it.
func runApply(cfg Config, changes *Changes) {
	groups, err := ReadPlan(cfg.Apply)
	if err != nil {
		Die("read plan: %v", err)
	}
	allPaths := AllFilePaths(changes)
	if err := ValidatePlan(groups, allPaths); err != nil {
		Die("invalid plan: %v", err)
	}
	if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
		if cfg.JSON {
			PrintRunResult("cancelled")
		}
		return
	}
	for _, group := range groups {
		if !ExecuteCommit(group.Files, group.Subject, group.Description, cfg.DryRun, cfg.MaxSubjectLength, changes.EffectiveScope) {
			os.Exit(1)
		}
	}
	if cfg.JSON {
		status := "completed"
		if cfg.DryRun {
			status = "dry_run"
		}
		PrintRunResult(status)
	}
}

// runGenerate turns the current changes into commits using the AI provider: it
// warns about oversized diffs, dispatches to single or auto mode, then checks
// for any changes left behind and cleans up temp files on success.
func runGenerate(cfg Config, changes *Changes, tmpl string) {
	if cfg.AutoContextWindow {
		if detected := DetectContextWindow(cfg.APIBase); detected > 0 {
			cfg.ContextWindow = detected
		}
	}
	PrintStep(fmt.Sprintf("Found %s", Pluralize(len(changes.AllFiles), "changed file")))
	if len(changes.BinaryFiles) > 0 && !IsQuietOutput() {
		fmt.Printf("    (binary: %s)\n", strings.Join(sanitizePaths(changes.BinaryFiles), ", "))
	}

	estimatedTokens := EstimatePromptTokens(tmpl, changes.FilesWithDiffs)
	if !CanFitInContext(tmpl, changes.FilesWithDiffs, cfg.ContextWindow) && !IsQuietOutput() {
		fmt.Printf("  %s Large diff detected (%s tokens estimated, %s token context)\n",
			yellow("!"),
			FormatNumber(estimatedTokens),
			FormatNumber(cfg.ContextWindow))
	}

	var summariesPaths []string
	committed := true
	if cfg.Mode == ModeSingle {
		committed = RunSingleMode(changes, cfg, tmpl)
	} else {
		var path string
		path, committed = RunAutoMode(changes, cfg, tmpl)
		summariesPaths = append(summariesPaths, path)
	}

	if !cfg.DryRun && committed {
		summariesPaths = append(summariesPaths, CheckAndCommitRemainingChanges(cfg, tmpl))
	}

	if cfg.Cleanup && committed {
		for _, p := range summariesPaths {
			if p != "" {
				os.Remove(p)
			}
		}
	}

	if cfg.JSON {
		status := "completed"
		if cfg.DryRun {
			status = "dry_run"
		} else if !committed {
			status = "cancelled"
		}
		PrintRunResult(status)
	}
}

// RunSingleMode puts every change into one commit: it processes the changes in
// context-sized batches, merges the results, and creates a single commit.
func RunSingleMode(changes *Changes, cfg Config, tmpl string) bool {
	PrintProcessing("Generating commit message...")

	batches := SplitFilesIntoBatches(tmpl, changes.FilesWithDiffs, cfg.ContextWindow)
	grouped := GroupChunkedBatches(batches)

	var allGroups []CommitGroup
	for i, g := range grouped {
		PrintProcessing(fmt.Sprintf("Processing batch %d/%d (%s)...", i+1, len(grouped), BatchLabel(g)))

		group, err := GroupFromAI(tmpl, cfg, g, DefaultMaxTokens)
		if err != nil {
			if ctxErr, ok := err.(*ContextLengthError); ok {
				PrintContextError(ctxErr)
				os.Exit(1)
				return false
			}
			Die("AI call failed: %v", err)
		}
		allGroups = append(allGroups, group)
	}

	merged := MergeCommitGroups(allGroups)
	subject := merged.Subject
	if subject == "" {
		subject = "chore: update"
	}
	group := CommitGroup{Subject: subject, Description: merged.Description, Files: AllFilePaths(changes)}
	writePlanIfRequested(cfg.PlanOut, []CommitGroup{group})
	if !ConfirmCommitPlan([]CommitGroup{group}, cfg, changes.Fingerprint) {
		return false
	}
	if !ExecuteCommit(AllFilePaths(changes), subject, merged.Description, cfg.DryRun, cfg.MaxSubjectLength, changes.EffectiveScope) {
		os.Exit(1)
	}
	return true
}

// RunAutoMode asks the model to organize changes into logical commit groups and
// commits each group. It returns the summaries temp-file path and whether the
// user approved the plan.
func RunAutoMode(changes *Changes, cfg Config, tmpl string) (string, bool) {
	files := changes.FilesWithDiffs
	if len(files) == 0 {
		return "", true
	}

	target := SummariesPath()
	if !IsQuietOutput() {
		fmt.Fprintf(os.Stderr, "  %s Summaries -> %s\n", yellow("~"), sanitizeText(target, 1024))
	}

	summarizeTmpl := LoadSection("summarize")
	planTmpl := ApplyMessagePreferences(LoadSection("plan"), cfg)

	summariesJSON, err := SummarizeChanges(cfg, summarizeTmpl, files, target)
	if err != nil {
		if ctxErr, ok := err.(*ContextLengthError); ok {
			PrintContextError(ctxErr)
			os.Exit(1)
		}
		Die("summarization failed: %v", err)
	}

	groups, err := PlanFromSummaries(planTmpl, cfg, summariesJSON)
	if err != nil {
		if ctxErr, ok := err.(*ContextLengthError); ok {
			PrintContextError(ctxErr)
			os.Exit(1)
		}
		Die("planning failed: %v", err)
	}

	allPaths := AllFilePaths(changes)
	groups = AssignBinaryFiles(groups, changes.BinaryFiles)
	if err := ValidatePlan(groups, allPaths); err != nil {
		Die("invalid generated plan: %v", err)
	}
	writePlanIfRequested(cfg.PlanOut, groups)

	if len(groups) == 0 {
		return target, true
	}

	PrintStep(fmt.Sprintf("Found %s", Pluralize(len(groups), "logical work package")))
	if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
		return target, false
	}
	commitFailed := false
	for _, g := range groups {
		if !ExecuteCommit(g.Files, g.Subject, g.Description, cfg.DryRun, cfg.MaxSubjectLength, changes.EffectiveScope) {
			commitFailed = true
		}
	}
	if commitFailed {
		os.Exit(1)
	}
	return target, true
}

func writePlanIfRequested(path string, groups []CommitGroup) {
	if path == "" {
		return
	}
	if err := WritePlan(path, groups); err != nil {
		Die("write plan: %v", err)
	}
	if !IsQuietOutput() {
		fmt.Printf("  %s Plan -> %s\n", yellow("~"), sanitizePath(path))
	}
}

// CheckAndCommitRemainingChanges verifies that no changes are left behind after
// the main commit pass and commits anything that remains, reusing the same
// filters so excluded or sensitive files never reach the model.
func CheckAndCommitRemainingChanges(cfg Config, tmpl string) string {
	if !IsQuietOutput() {
		fmt.Println()
		PrintProcessing("Checking for any remaining uncommitted changes...")
	}

	status, err := GitRun("status", "--porcelain")
	if err != nil {
		Die("git status failed: %v", err)
	}

	if status == "" {
		if !IsQuietOutput() {
			Success("Working directory is clean. Exiting successfully.")
		}
		return ""
	}

	if !IsQuietOutput() {
		fmt.Printf("  %s Found uncommitted changes. Attempting to group and commit.\n", yellow("⚠️"))
	}

	remainingChanges, err := GetGitChangesForScope(cfg.Scope)
	if err != nil {
		Die("git status check failed: %v", err)
	}

	FilterChanges(remainingChanges, cfg.Include, cfg.Exclude, cfg.IncludeSensitive)

	if len(remainingChanges.AllFiles) == 0 {
		if !IsQuietOutput() {
			Success("No changes remain to commit after filtering. Exiting.")
		}
		return ""
	}

	if !IsQuietOutput() {
		fmt.Printf("  %s Re-analyzing remaining changes for a final commit...\n", yellow("🧠"))
	}

	path, committed := RunAutoMode(remainingChanges, cfg, tmpl)
	if !committed {
		return path
	}

	finalStatus, _ := GitRun("status", "--porcelain")
	if finalStatus == "" {
		if !IsQuietOutput() {
			Success("Clean Checkout Successful")
		}
	} else {
		fmt.Fprintf(os.Stderr, "  %s CRITICAL WARNING: Final git status still shows uncommitted changes:\n", red("🚨"))
		Warning(finalStatus)
		os.Exit(1)
	}
	return path
}

// BatchLabel describes a batch for progress output: either a chunk group for a
// single oversized file, or the number of files in the batch.
func BatchLabel(batch []FileDiff) string {
	if IsChunkedBatch(batch) {
		return fmt.Sprintf("chunk group (%s)", batch[0].Path)
	}
	return Pluralize(len(batch), "file")
}

// GroupChunkedBatches merges consecutive single-file batches that belong to the
// same oversized file back into one batch.
func GroupChunkedBatches(batches [][]FileDiff) [][]FileDiff {
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
