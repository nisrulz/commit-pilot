package lib

import (
	"fmt"
	"os"
	"strings"
)

// runWorkflow is the main commit flow: it collects the current git changes and
// either lints a saved plan, applies a saved plan, or generates new commits
// through the configured AI provider. Every failure is returned so the entry
// point reports it once.
func runWorkflow(cfg Config) error {
	tmpl := ApplyMessagePreferences(LoadPrompt(cfg.Mode, cfg.Prompt), cfg)

	changes, err := GetGitChangesForScope(cfg.Scope)
	if err != nil {
		return fmt.Errorf("git: %w", err)
	}
	if len(changes.AllFiles) == 0 {
		reportNoChanges(cfg)
		return nil
	}
	filtered := FilterChanges(changes, cfg.Include, cfg.Exclude, cfg.IncludeSensitive)
	if len(filtered) > 0 && !IsQuietOutput() {
		fmt.Printf("  %s Skipped: %s\n", yellow("~"), strings.Join(sanitizePaths(filtered), ", "))
	}
	if len(changes.AllFiles) == 0 {
		reportNoChanges(cfg)
		return nil
	}
	if err := ValidateChangeScope(changes); err != nil {
		return fmt.Errorf("unsafe change scope: %w", err)
	}

	switch {
	case cfg.PlanLint != "":
		return runPlanLint(cfg, changes)
	case cfg.Apply != "":
		return runApply(cfg, changes)
	case len(changes.FilesWithDiffs) == 0 && len(changes.BinaryFiles) > 0:
		return runBinaryOnly(cfg, changes)
	default:
		return runGenerate(cfg, changes, tmpl)
	}
}

// runBinaryOnly commits changes that hold no diff text (binary files only),
// without calling the model.
func runBinaryOnly(cfg Config, changes *Changes) error {
	groups := AssignBinaryFiles(nil, changes.BinaryFiles)
	if err := writePlanIfRequested(cfg.PlanOut, groups); err != nil {
		return err
	}
	if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
		if cfg.JSON {
			PrintRunResult("cancelled")
		}
		return nil
	}
	if err := commitGroups(groups, cfg, changes); err != nil {
		return err
	}
	if cfg.JSON {
		PrintRunResult(commitStatus(cfg, true))
	}
	return nil
}

// runPlanLint validates a saved plan against the current changes and the
// configured message preferences, without applying it.
func runPlanLint(cfg Config, changes *Changes) error {
	groups, err := ReadPlan(cfg.PlanLint)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}
	if err := LintPlan(groups, AllFilePaths(changes), cfg); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}
	if cfg.JSON {
		PrintJSON(map[string]any{"status": "valid"})
	} else if !cfg.Quiet {
		Success("Plan is valid.")
	}
	return nil
}

// runApply validates a saved plan against the current changes and commits each
// group after the user confirms it.
func runApply(cfg Config, changes *Changes) error {
	groups, err := ReadPlan(cfg.Apply)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}
	if err := ValidatePlan(groups, AllFilePaths(changes)); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}
	if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
		if cfg.JSON {
			PrintRunResult("cancelled")
		}
		return nil
	}
	if err := commitGroups(groups, cfg, changes); err != nil {
		return err
	}
	if cfg.JSON {
		PrintRunResult(commitStatus(cfg, true))
	}
	return nil
}

// runGenerate turns the current changes into commits using the AI provider: it
// warns about oversized diffs, dispatches to single or auto mode, then checks
// for any changes left behind and cleans up temp files on success.
func runGenerate(cfg Config, changes *Changes, tmpl string) error {
	PrintStep(fmt.Sprintf("Found %s", Pluralize(len(changes.AllFiles), "changed file")))
	if len(changes.BinaryFiles) > 0 && !IsQuietOutput() {
		fmt.Printf("    (binary: %s)\n", strings.Join(sanitizePaths(changes.BinaryFiles), ", "))
	}

	// Auto mode sends diffs to the summarize stage, not the single-mode
	// template, so size the warning against the stage that carries the diff.
	estimateTemplate := tmpl
	if cfg.Mode != ModeSingle {
		estimateTemplate = LoadSection("summarize")
	}
	if !CanFitInContext(estimateTemplate, changes.FilesWithDiffs, cfg.ContextWindow) && !IsQuietOutput() {
		fmt.Printf("  %s Large diff detected (%s tokens estimated, %s token context)\n",
			yellow("!"),
			FormatNumber(EstimatePromptTokens(estimateTemplate, changes.FilesWithDiffs)),
			FormatNumber(cfg.ContextWindow))
	}

	var summariesPaths []string
	committed := true
	var err error
	if cfg.Mode == ModeSingle {
		committed, err = RunSingleMode(changes, cfg, tmpl)
	} else {
		var path string
		path, committed, err = RunAutoMode(changes, cfg)
		summariesPaths = append(summariesPaths, path)
	}
	if err != nil {
		return err
	}

	if !cfg.DryRun && committed {
		path, err := CheckAndCommitRemainingChanges(cfg)
		if err != nil {
			return err
		}
		summariesPaths = append(summariesPaths, path)
	}

	if cfg.Cleanup && committed {
		for _, p := range summariesPaths {
			if p != "" {
				os.Remove(p)
			}
		}
	}

	if cfg.JSON {
		PrintRunResult(commitStatus(cfg, committed))
	}
	return nil
}

// RunSingleMode puts every change into one commit: it processes the changes in
// context-sized batches, merges the results, and creates a single commit. A
// batch that the model cannot handle is skipped; the merged subject then falls
// back to a generic message instead of failing the run.
func RunSingleMode(changes *Changes, cfg Config, tmpl string) (bool, error) {
	PrintProcessing("Generating commit message...")

	batches := SplitFilesIntoBatches(tmpl, changes.FilesWithDiffs, cfg.ContextWindow)
	grouped := GroupChunkedBatches(batches)

	var allGroups []CommitGroup
	for i, g := range grouped {
		PrintProcessing(fmt.Sprintf("Processing batch %d/%d (%s)...", i+1, len(grouped), BatchLabel(g)))

		group, err := GroupFromAI(tmpl, cfg, g, DefaultMaxTokens)
		if err != nil {
			if IsUnreachable(err) {
				return false, err
			}
			reportStageError(err)
			continue
		}
		allGroups = append(allGroups, group)
	}

	merged := MergeCommitGroups(allGroups)
	subject := merged.Subject
	if subject == "" {
		subject = "chore: update"
	}
	group := CommitGroup{Subject: subject, Description: merged.Description, Files: AllFilePaths(changes)}
	if err := writePlanIfRequested(cfg.PlanOut, []CommitGroup{group}); err != nil {
		return false, err
	}
	if !ConfirmCommitPlan([]CommitGroup{group}, cfg, changes.Fingerprint) {
		return false, nil
	}
	if err := commitGroups([]CommitGroup{group}, cfg, changes); err != nil {
		return false, err
	}
	return true, nil
}

// RunAutoMode asks the model to organize changes into logical commit groups and
// commits each group. It returns the summaries temp-file path and whether the
// user approved the plan. Model failures degrade to a locally built plan so the
// change set is still committed.
func RunAutoMode(changes *Changes, cfg Config) (string, bool, error) {
	files := changes.FilesWithDiffs
	if len(files) == 0 {
		return "", true, nil
	}

	target := SummariesPath()
	if !IsQuietOutput() {
		fmt.Fprintf(os.Stderr, "  %s Summaries -> %s\n", yellow("~"), sanitizeText(target, 1024))
	}

	summarizeTmpl := LoadSection("summarize")
	planTmpl := ApplyMessagePreferences(LoadSection("plan"), cfg)

	summariesJSON, err := SummarizeChanges(cfg, summarizeTmpl, files, target)
	if err != nil {
		if IsUnreachable(err) {
			return target, false, err
		}
		Warningf("summarization failed: %v", err)
	}

	var groups []CommitGroup
	if summariesJSON == "" {
		groups = localFallbackGroups(changes)
	} else {
		groups, err = planGroups(planTmpl, cfg, summariesJSON, changes)
		if err != nil {
			return target, false, err
		}
	}
	if err := writePlanIfRequested(cfg.PlanOut, groups); err != nil {
		return target, false, err
	}

	if len(groups) == 0 {
		return target, true, nil
	}

	PrintStep(fmt.Sprintf("Found %s", Pluralize(len(groups), "logical work package")))
	if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
		return target, false, nil
	}
	if err := commitGroups(groups, cfg, changes); err != nil {
		return target, false, err
	}
	return target, true, nil
}

// commitGroups runs the commit for each approved group and stops at the first
// failure, returning it to the entry point.
func commitGroups(groups []CommitGroup, cfg Config, changes *Changes) error {
	for _, group := range groups {
		if err := ExecuteCommit(group.Files, group.Subject, group.Description, cfg.DryRun, cfg.MaxSubjectLength, changes.EffectiveScope); err != nil {
			return err
		}
	}
	return nil
}

func writePlanIfRequested(path string, groups []CommitGroup) error {
	if path == "" {
		return nil
	}
	if err := WritePlan(path, groups); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	if !IsQuietOutput() {
		fmt.Printf("  %s Plan -> %s\n", yellow("~"), sanitizePath(path))
	}
	return nil
}

// CheckAndCommitRemainingChanges verifies that no changes are left behind after
// the main commit pass and commits anything that remains, reusing the same
// filters so excluded or sensitive files never reach the model.
func CheckAndCommitRemainingChanges(cfg Config) (string, error) {
	if !IsQuietOutput() {
		fmt.Println()
		PrintProcessing("Checking for any remaining uncommitted changes...")
	}

	status, err := GitRun("status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}

	if status == "" {
		if !IsQuietOutput() {
			Success("Working directory is clean. Exiting successfully.")
		}
		return "", nil
	}

	if !IsQuietOutput() {
		fmt.Printf("  %s Found uncommitted changes. Attempting to group and commit.\n", yellow("⚠️"))
	}

	remainingChanges, err := GetGitChangesForScope(cfg.Scope)
	if err != nil {
		return "", fmt.Errorf("git status check failed: %w", err)
	}

	FilterChanges(remainingChanges, cfg.Include, cfg.Exclude, cfg.IncludeSensitive)

	if len(remainingChanges.AllFiles) == 0 {
		if !IsQuietOutput() {
			Success("No changes remain to commit after filtering. Exiting.")
		}
		return "", nil
	}

	if !IsQuietOutput() {
		fmt.Printf("  %s Re-analyzing remaining changes for a final commit...\n", yellow("🧠"))
	}

	path, committed, err := RunAutoMode(remainingChanges, cfg)
	if err != nil {
		return path, err
	}
	if !committed {
		return path, nil
	}

	finalStatus, _ := GitRun("status", "--porcelain")
	if finalStatus == "" {
		if !IsQuietOutput() {
			Success("Clean Checkout Successful")
		}
		return path, nil
	}
	return path, fmt.Errorf("working tree still has uncommitted changes after the run")
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
