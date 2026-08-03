# Testing

## Unit tests

Focused unit tests live in `tests/` (external test package `lib_test`). They
target individual functions in `src/lib`: plan validation, provider retries,
cancellation, confirmation input, config precedence, Git scope behavior, and
provider dispatch (including unknown-provider rejection and probe body draining).

End-to-end tests live in `tests/e2e/`. They build the real CLI binary and run it
against a mock OpenAI-compatible provider. Together they cover auto and single
mode commits, plan-out/apply/plan-lint, dry-run, sensitive-file filtering, scope
flags, partially staged files, option-like filenames, binary-only commits,
`--list-models`, `--doctor`, and provider failures.

The install script is covered by an end-to-end test too. It runs
`scripts/install.sh` against a local mock release server, so the real `curl`,
`tar`, and checksum tooling all get exercised. The test verifies the binary
lands in `~/go/bin`, the working directory stays untouched, a stale binary gets
replaced, and checksum mismatches or conflicting destinations abort the install.
Missing checksum entries fail too.

To point `install.sh` at a mirror, or at a mock server during testing, set
`COMMIT_PILOT_INSTALL_API_BASE` and `COMMIT_PILOT_INSTALL_DOWNLOAD_BASE`. The
defaults point at the official GitHub endpoints.

```bash
make test
```

We measure coverage for the `src/lib` package across both suites.

## Live test

The integration test runs commit-pilot against a real AI endpoint.

The script checks that your AI provider is reachable before starting. Each
known provider probes its own health URL, trying both `localhost` and
`127.0.0.1`. Key-protected providers such as Unsloth Studio are detected through
their `/health` route, so a running server is recognized even before the API key
is configured. If nothing responds, it prints setup instructions.

**LMStudio (default):**
```bash
make test-live
```

**Ollama:**
```bash
OPENAI_BASE_URL=http://localhost:11434/v1 make test-live
```

**OpenAI (or any OpenAI-compatible endpoint):**
```bash
OPENAI_BASE_URL=https://api.openai.com/v1 \
  OPENAI_API_KEY=sk-... \
  make test-live
```

**Unsloth Studio:**
```bash
OPENAI_BASE_URL=http://localhost:8888/v1 make test-live
```

The script sets up a temporary git repo with staged changes across docs, config, and code, then runs commit-pilot in dry-run mode. It checks for:

- Git repo detection (running outside a git repo prints an error)
- No changes (an empty repo prints a message)
- Multi-file changes (counts changed files and reaches the AI stage)
- Single mode (`--single` reaches the AI stage)
- Binary files (small binaries commit without a model call; mixed binary and text changes stay grouped)
- Large diffs (many files split into window-sized batches; oversized files and long lines chunk across LLM calls)
- Context window limits (a small window triggers batching warnings)
- Empty diffs (a staged file with no real change)
- Staged and unstaged changes (a mixed working tree)
- Partially staged files (rejected before a model call)
- Path edge cases (unicode names, spaces, deeply nested directories, symlinks, renames, deletions)
- Diff edge cases (special characters, empty files, newline-only files)
- Failure modes (a pre-commit hook rejection fails the run; a file deleted mid-run)
- Subject truncation (long commit subjects are cut to the configured limit)

Use `commit-pilot --doctor` to verify a local provider before running the live
test. `commit-pilot --list-models` shows the model IDs the provider exposes.

The script keeps the temporary directory `.temp-test/` in the project root and cleans it up when it finishes.
