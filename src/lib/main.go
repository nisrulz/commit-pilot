package lib

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func Main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println()
		fmt.Printf("  %s Interrupted — no changes committed.\n", yellow("!"))
		os.Exit(1)
	}()

	flags, showHelp := ParseArgs(os.Args[1:])
	if showHelp {
		printHelp()
		return
	}

	cfg := ResolveConfig(flags)
	if flags.Doctor {
		if !RunDoctor(cfg) {
			os.Exit(1)
		}
		return
	}

	tmpl := LoadPrompt(cfg.Mode, cfg.Prompt)

	changes, err := GetGitChangesForScope(cfg.Scope)
	if err != nil {
		Die("git: %v", err)
	}

	if len(changes.AllFiles) == 0 {
		fmt.Printf("  %s No changes to commit.\n", yellow("\u26a1"))
		return
	}
	changes.FilesWithDiffs = FilterFiles(changes.FilesWithDiffs, cfg.Include, cfg.Exclude, cfg.IncludeSensitive)

	if len(changes.FilesWithDiffs) == 0 && len(changes.BinaryFiles) > 0 {
		fmt.Printf("  %s Only binary files changed \u2014 cannot generate AI commit message.\n", yellow("\u26a1"))
		return
	}
	if cfg.Apply != "" {
		groups, err := ReadPlan(cfg.Apply)
		if err != nil {
			Die("read plan: %v", err)
		}
		allPaths := AllFilePaths(changes)
		if err := ValidatePlan(groups, allPaths); err != nil {
			Die("invalid plan: %v", err)
		}
		if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
			return
		}
		for _, group := range groups {
			if !ExecuteCommit(group.Files, group.Subject, group.Description, cfg.DryRun) {
				os.Exit(1)
			}
		}
		return
	}

	PrintStep(fmt.Sprintf("Found %s", Pluralize(len(changes.AllFiles), "changed file")))
	if len(changes.BinaryFiles) > 0 {
		fmt.Printf("    (binary: %s)\n", strings.Join(changes.BinaryFiles, ", "))
	}

	estimatedTokens := EstimatePromptTokens(tmpl, changes.FilesWithDiffs)
	if !CanFitInContext(tmpl, changes.FilesWithDiffs, cfg.ContextWindow) {
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
}

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
	if !ConfirmCommitPlan([]CommitGroup{{Subject: subject, Description: merged.Description, Files: AllFilePaths(changes)}}, cfg, changes.Fingerprint) {
		return false
	}
	if !ExecuteCommit(AllFilePaths(changes), subject, merged.Description, cfg.DryRun) {
		os.Exit(1)
	}
	return true
}

func AllFilePaths(changes *Changes) []string {
	files := make([]string, 0, len(changes.FilesWithDiffs)+len(changes.BinaryFiles))
	for _, f := range changes.FilesWithDiffs {
		files = append(files, f.Path)
	}
	return append(files, changes.BinaryFiles...)
}

func BatchLabel(batch []FileDiff) string {
	if IsChunkedBatch(batch) {
		return fmt.Sprintf("chunk group (%s)", batch[0].Path)
	}
	return Pluralize(len(batch), "file")
}

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

func CheckAndCommitRemainingChanges(cfg Config, tmpl string) string {
	fmt.Println()
	fmt.Println("  Checking for any remaining uncommitted changes...")

	status, err := GitRun("status", "--porcelain")
	if err != nil {
		Die("git status failed: %v", err)
	}

	if status == "" {
		fmt.Printf("  %s Working directory is clean. Exiting successfully.\n", green("✔"))
		return ""
	}

	fmt.Printf("  %s Found uncommitted changes. Attempting to group and commit.\n", yellow("⚠️"))

	remainingChanges, err := GetGitChangesForScope(cfg.Scope)
	if err != nil {
		Die("git status check failed: %v", err)
	}

	if len(remainingChanges.AllFiles) == 0 {
		fmt.Printf("  %s Status check indicated changes, but GetGitChanges found none. Exiting.\n", green("✔"))
		return ""
	}

	fmt.Printf("  %s Re-analyzing remaining changes for a final commit...\n", yellow("🧠"))

	path, committed := RunAutoMode(remainingChanges, cfg, tmpl)
	if !committed {
		return path
	}

	finalStatus, _ := GitRun("status", "--porcelain")
	if finalStatus == "" {
		fmt.Printf("  %s Clean Checkout Successful\n", green("✔"))
	} else {
		fmt.Fprintf(os.Stderr, "  %s CRITICAL WARNING: Final git status still shows uncommitted changes:\n", red("🚨"))
		fmt.Fprintf(os.Stderr, "  %s %s\n", yellow("!"), finalStatus)
		os.Exit(1)
	}
	return path
}

func RunAutoMode(changes *Changes, cfg Config, tmpl string) (string, bool) {
	files := changes.FilesWithDiffs
	if len(files) == 0 {
		return "", true
	}

	target := SummariesPath()
	fmt.Fprintf(os.Stderr, "  %s Summaries -> %s\n", yellow("~"), target)

	summarizeTmpl := LoadSection("summarize")
	planTmpl := LoadSection("plan")

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
	for i, g := range groups {
		groups[i].Files = LimitCommitScopeTo(FilterValidFiles(g.Files, allPaths), cfg.MaxFilesGroup)
	}

	groups = AssignBinaryFiles(groups, changes.BinaryFiles)
	groups = MergeGroups(groups)
	if cfg.PlanOut != "" {
		if err := WritePlan(cfg.PlanOut, groups); err != nil {
			Die("write plan: %v", err)
		}
		fmt.Printf("  %s Plan -> %s\n", yellow("~"), cfg.PlanOut)
	}

	if len(groups) == 0 {
		return target, true
	}

	PrintStep(fmt.Sprintf("Found %s", Pluralize(len(groups), "logical work package")))
	if !ConfirmCommitPlan(groups, cfg, changes.Fingerprint) {
		return target, false
	}
	commitFailed := false
	for _, g := range groups {
		if !ExecuteCommit(g.Files, g.Subject, g.Description, cfg.DryRun) {
			commitFailed = true
		}
	}
	if commitFailed {
		os.Exit(1)
	}
	return target, true
}

func PrintContextError(err *ContextLengthError) {
	fmt.Println()
	fmt.Fprintf(os.Stderr, "  %s %s\n", red("ERROR:"), err.Message)
	fmt.Fprintf(os.Stderr, "    Estimated tokens: %s\n", FormatNumber(err.Estimated))
	fmt.Fprintf(os.Stderr, "    Context window:   %s tokens\n", FormatNumber(err.Available))
	fmt.Println()
	fmt.Fprintf(os.Stderr, "  %s To fix this, you can:\n", yellow("SUGGESTIONS:"))
	fmt.Fprintf(os.Stderr, "    1. Increase context window: export COMMIT_PILOT_CONTEXT_WINDOW=131072\n")
	fmt.Fprintf(os.Stderr, "    2. Stage fewer files at once\n")
	fmt.Fprintf(os.Stderr, "    3. Use a model with larger context window\n")
}

func FormatNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

func Die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  ! "+format+"\n", args...)
	os.Exit(1)
}

func Pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
