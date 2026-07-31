package lib

import "fmt"

// RawFlags holds the command-line flags parsed from the process arguments.
type RawFlags struct {
	Mode             string
	DryRun           bool
	Cleanup          bool
	Yes              bool
	Doctor           bool
	ListModels       bool
	JSON             bool
	Quiet            bool
	Staged           bool
	Unstaged         bool
	PlanOut          string
	Apply            string
	PlanLint         string
	Include          []string
	Exclude          []string
	IncludeSensitive bool
	Error            string
}

// ParseArgs walks the command-line arguments and records every recognized
// flag. It returns showHelp=true when the user asked for help output.
func ParseArgs(args []string) (RawFlags, bool) {
	var f RawFlags
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--dry-run", "--no-commit":
			f.DryRun = true
		case "--cleanup":
			f.Cleanup = true
		case "--single":
			f.Mode = "1"
		case "--yes":
			f.Yes = true
		case "--doctor":
			f.Doctor = true
		case "--list-models":
			f.ListModels = true
		case "--json":
			f.JSON = true
		case "--quiet":
			f.Quiet = true
		case "--staged":
			f.Staged = true
		case "--unstaged":
			f.Unstaged = true
		case "--plan-out":
			if i+1 < len(args) && args[i+1] != "" {
				i++
				f.PlanOut = args[i]
			} else {
				f.Error = "--plan-out requires a path"
				return f, false
			}
		case "--apply":
			if i+1 < len(args) && args[i+1] != "" {
				i++
				f.Apply = args[i]
			} else {
				f.Error = "--apply requires a path"
				return f, false
			}
		case "--plan-lint":
			if i+1 < len(args) && args[i+1] != "" {
				i++
				f.PlanLint = args[i]
			} else {
				f.Error = "--plan-lint requires a path"
				return f, false
			}
		case "--include":
			if i+1 < len(args) && args[i+1] != "" {
				i++
				f.Include = append(f.Include, args[i])
			} else {
				f.Error = "--include requires a glob"
				return f, false
			}
		case "--exclude":
			if i+1 < len(args) && args[i+1] != "" {
				i++
				f.Exclude = append(f.Exclude, args[i])
			} else {
				f.Error = "--exclude requires a glob"
				return f, false
			}
		case "--include-sensitive":
			f.IncludeSensitive = true
		case "-h", "--help":
			return f, true
		default:
			f.Error = fmt.Sprintf("unknown argument %q", a)
			return f, false
		}
	}
	if f.Staged && f.Unstaged {
		f.Error = "--staged and --unstaged cannot be used together"
	}
	actions := 0
	for _, selected := range []bool{f.Doctor, f.ListModels, f.Apply != "", f.PlanLint != ""} {
		if selected {
			actions++
		}
	}
	if actions > 1 {
		f.Error = "--doctor, --list-models, --apply, and --plan-lint are mutually exclusive"
	}
	if f.PlanOut != "" && actions > 0 {
		f.Error = "--plan-out cannot be combined with --doctor, --list-models, --apply, or --plan-lint"
	}
	if f.PlanOut != "" {
		f.DryRun = true
	}
	return f, false
}
