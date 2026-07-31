package lib

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// ExecuteCommit stages the given files and creates a commit with the provided
// subject and description. In dry-run mode nothing is committed. It returns
// false when there is nothing to commit or the commit fails.
func ExecuteCommit(files []string, subject, description string, dryRun bool, maxSubjectLength int, scope ChangeScope) bool {
	if len(files) == 0 {
		return false
	}

	normalized := NormalizeCommitGroup(CommitGroup{Subject: subject, Description: description})
	subject = normalized.Subject
	description = normalized.Description
	subject = strings.TrimSpace(subject)
	subject = strings.ReplaceAll(subject, "\n", " ")
	subject = strings.ReplaceAll(subject, "\r", "")
	subject = strings.TrimLeft(subject, "-")
	if subject == "" {
		subject = "chore: update"
	}
	if maxSubjectLength <= 0 {
		maxSubjectLength = MaxSubjectLength
	}
	if utf8.RuneCountInString(subject) > maxSubjectLength {
		subject = string([]rune(subject)[:maxSubjectLength])
	}

	if !dryRun {
		if scope != ScopeStaged {
			if _, err := GitRun(append([]string{"add", "--"}, files...)...); err != nil {
				fmt.Fprintf(os.Stderr, "  ! git add failed: %v\n", err)
				return false
			}
		}
		commitArgs := append([]string{"commit", "--only", "-m", subject, "-m", description, "--"}, files...)
		if _, err := GitRun(commitArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "  ! git commit failed: %v\n", err)
			return false
		}
	}

	if !IsQuietOutput() {
		fmt.Println()
	}
	PrintCommitSection(subject, description, files, dryRun)
	RecordCommit(CommitGroup{Subject: subject, Description: description, Files: files})
	return true
}

// ConfirmCommitPlan prints the proposed commit groups and asks the user to
// approve them. It verifies that the underlying changes have not changed since
// the plan was generated, and auto-approves in dry-run and --yes modes.
func ConfirmCommitPlan(groups []CommitGroup, cfg Config, fingerprint string) bool {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	in := cfg.Input
	if in == nil {
		in = os.Stdin
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Proposed commit plan:")
	for i, group := range groups {
		group = NormalizeCommitGroup(group)
		fmt.Fprintf(out, "    %d. %s\n", i+1, group.Subject)
		if description := strings.TrimSpace(group.Description); description != "" {
			fmt.Fprintf(out, "       %s\n", description)
		}
		fmt.Fprintf(out, "       Files: %s\n", strings.Join(sanitizePaths(group.Files), ", "))
	}

	if cfg.DryRun {
		return true
	}
	current, err := GetGitChangesForScope(cfg.Scope)
	if err != nil || current.Fingerprint != fingerprint {
		fmt.Fprintln(out, "  Changes were updated while the plan was being generated. Please run commit-pilot again.")
		return false
	}
	if cfg.Yes {
		return true
	}

	fmt.Fprint(out, "  Apply this plan? [y/N] ")
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
		fmt.Fprintln(out, "  No commits created.")
		return false
	}
	return true
}
