package lib

import "fmt"

// printHelp prints usage information to stdout.
func printHelp() {
	fmt.Print(`commit-pilot: AI-powered git commit messages that know what you changed.

Usage:
  commit-pilot                           # auto-chunk into logical commits
  commit-pilot --single                  # one commit for all changes
  commit-pilot --dry-run                 # preview only
  commit-pilot --no-commit               # preview only (alias for --dry-run)
  commit-pilot --yes                     # apply proposed commits without prompting
  commit-pilot --doctor                  # check Git and provider setup
  commit-pilot --list-models             # list models available from the provider
  commit-pilot --json                    # emit machine-readable output
  commit-pilot --quiet                   # hide progress output
  commit-pilot --staged                  # use staged changes only
  commit-pilot --unstaged                # ignore staged changes
  commit-pilot --plan-out <path>         # save generated plan without committing
  commit-pilot --apply <path>            # apply an edited JSON plan
  commit-pilot --plan-lint <path>        # validate a saved JSON plan
  commit-pilot --include <glob>          # include matching files only
  commit-pilot --exclude <glob>          # exclude matching files
  commit-pilot --include-sensitive       # allow sensitive-looking files to reach the model
  commit-pilot --cleanup                 # remove temp files on success

Environment variables:
  OPENAI_PROVIDER              Provider: ollama, lmstudio, openai, unsloth
  OPENAI_MODEL                 Model name (default: gemma-4-e2b-it-qat)
  OPENAI_BASE_URL              API base URL
  OPENAI_API_KEY               API key
  COMMIT_PILOT_PROMPT          Custom prompt text (overrides default)
  COMMIT_PILOT_PROMPT_FILE     Path to custom prompt file (overrides default)
  COMMIT_PILOT_CONTEXT_WINDOW  Model context window size in tokens (default: 65536)
  COMMIT_PILOT_RETRIES         Retries for transient provider failures (default: 2)
  COMMIT_PILOT_TIMEOUT_SECONDS Provider request timeout in seconds (default: 180)
  COMMIT_PILOT_CONFIG_DIR      Directory for configuration (default: ~/.config/commit-pilot)
  COMMIT_PILOT_TMP_DIR         Directory for temporary summaries (default: ~/.commit-pilot/tmp)

Config file:
  $COMMIT_PILOT_CONFIG_DIR/config.env    Optional provider/model defaults
`)
}
