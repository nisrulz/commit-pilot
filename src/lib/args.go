package lib

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
			if i+1 < len(args) {
				i++
				f.PlanOut = args[i]
			}
		case "--apply":
			if i+1 < len(args) {
				i++
				f.Apply = args[i]
			}
		case "--plan-lint":
			if i+1 < len(args) {
				i++
				f.PlanLint = args[i]
			}
		case "--include":
			if i+1 < len(args) {
				i++
				f.Include = append(f.Include, args[i])
			}
		case "--exclude":
			if i+1 < len(args) {
				i++
				f.Exclude = append(f.Exclude, args[i])
			}
		case "--include-sensitive":
			f.IncludeSensitive = true
		case "-h", "--help":
			return f, true
		}
	}
	return f, false
}
