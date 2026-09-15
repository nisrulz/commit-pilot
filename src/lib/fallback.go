package lib

import (
	"errors"
	"strings"
)

// planGroups turns summaries into a validated plan. A model failure or an
// invalid plan degrades to a single local group covering every diff and binary
// file, so the run always has a usable plan. An unreachable provider is
// returned as an error, because that is a setup problem, not a grouping one.
func planGroups(planTmpl string, cfg Config, summariesJSON string, changes *Changes) ([]CommitGroup, error) {
	groups, err := PlanFromSummaries(planTmpl, cfg, summariesJSON)
	if err != nil {
		if IsUnreachable(err) {
			return nil, err
		}
		reportStageError(err)
		Warning("Grouping all files into one commit.")
		groups = FallbackPlan(summariesJSON)
	}

	allPaths := AllFilePaths(changes)
	groups = AssignBinaryFiles(groups, changes.BinaryFiles)
	if err := ValidatePlan(groups, allPaths); err != nil {
		Warningf("generated plan is invalid: %v; grouping all files into one commit", err)
		groups = localFallbackGroups(changes)
	}
	return groups, nil
}

// localFallbackGroups builds one commit group from the diff files alone, plus
// the binary group, with no model output.
func localFallbackGroups(changes *Changes) []CommitGroup {
	diffFiles := make([]string, len(changes.FilesWithDiffs))
	for i, f := range changes.FilesWithDiffs {
		diffFiles[i] = f.Path
	}
	return AssignBinaryFiles([]CommitGroup{FallbackCommitGroup(diffFiles)}, changes.BinaryFiles)
}

// reportStageError prints a stage failure, adding context-window guidance when
// the model rejected the input size. It never writes to stdout under --json, so
// the single JSON result at the end stays machine-readable.
func reportStageError(err error) {
	var ctxErr *ContextLengthError
	if errors.As(err, &ctxErr) {
		if IsJSONOutput() {
			Warning(ctxErr.Message)
		} else {
			PrintContextError(ctxErr)
		}
		Warning("Falling back to a locally generated message.")
		return
	}
	Warningf("AI stage failed: %v", err)
}

// FallbackCommitGroup builds a commit group from file paths alone, with no
// model call. The description names the files, so a run still produces a useful
// commit when the provider is unavailable.
func FallbackCommitGroup(files []string) CommitGroup {
	return CommitGroup{
		Subject:     "chore: update changes",
		Description: fallbackDescription(files),
		Files:       files,
	}
}

func fallbackDescription(files []string) string {
	if len(files) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Update ")
	b.WriteString(Pluralize(len(files), "file"))
	b.WriteString(":")
	for _, file := range files {
		b.WriteString("\n- ")
		b.WriteString(sanitizePath(file))
	}
	return b.String()
}
