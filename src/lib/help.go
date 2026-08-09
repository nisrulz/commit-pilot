package lib

import "fmt"

// printHelp prints usage information to stdout.
func printHelp() {
	fmt.Print(`commit-pilot: AI-powered git commit messages that know what you changed.

Usage:
  commit-pilot                           # auto-chunk into logical commits
  commit-pilot --single                  # one commit for all changes
  commit-pilot --config <path>           # use a specific config file
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

Configuration:
  The CLI ships no config. User config lives at
  $COMMIT_PILOT_CONFIG_DIR/commit-pilot/config.yaml, created on first run with
  the default values. COMMIT_PILOT_CONFIG_DIR defaults to ~/.config, so by
  default the file is ~/.config/commit-pilot/config.yaml. Edit that file; it is
  the source of truth. Values: provider, model, base_url, context_window,
  prompt, mode, retries, timeout_seconds, conventional, ticket_prefix,
  imperative, max_subject_length, body_style. The tool keeps its own working
  files (e.g. tmp summaries) in that directory too, so they move together.
  Providers: ollama (default, model lfm2.5:8b) or openai_compat for any
  OpenAI-compatible API (requires base_url).
  Resolution precedence (highest first):
    1. --config <path> flag (explicit config file for this invocation)
    2. COMMIT_PILOT_CONFIG_DIR env var (base config dir, e.g. ~/.config)
    3. ~/.config/commit-pilot/config.yaml
  An explicit --config pointing at a missing file is an error. A missing
  config file from the env var or default path is created with the default
  values on first run.

API key:
  The api_key is read from the COMMIT_PILOT_OPENAI_COMPAT_API_KEY environment
  variable only. It is never read from a config file and never accepted on
  the command line (keeps it out of config files and shell history).

Repository config:
  An optional .commit-pilot.yaml in the working directory tree is treated
  as untrusted and may only set commit-message preferences
  (conventional, ticket_prefix, imperative, max_subject_length, body_style)
  and output defaults (default_branch, output_format). Any other key
  (provider, base_url, api_key, ...) is rejected.

Session scope:
  --config applies to a single invocation. COMMIT_PILOT_CONFIG_DIR=/path \
  commit-pilot applies to that command line only (not exported). Only
  "export COMMIT_PILOT_CONFIG_DIR=/path" makes it persist for the shell session.

Environment variables:
  COMMIT_PILOT_CONFIG_DIR        Base config dir (commit-pilot subdir holds
                                 config.yaml and working files)
  COMMIT_PILOT_OPENAI_COMPAT_API_KEY    API key (never a config file value)
`)
}
