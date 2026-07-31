# Testing

## Unit tests

Focused unit tests live in `tests/` (external test package `lib_test`). They
target individual functions in `src/lib`: plan validation, provider retries,
cancellation, confirmation input, config precedence, and Git scope behavior.

End-to-end tests live in `tests/e2e/`. They build the real CLI binary and run it
against a mock OpenAI-compatible provider. Together they cover auto and single
mode commits, plan-out/apply/plan-lint, dry-run, sensitive-file filtering, scope
flags, `--list-models`, `--doctor`, and provider failures.

```bash
make test
```

We measure coverage for the `src/lib` package across both suites.

## Live test

The integration test runs commit-pilot against a real AI endpoint.

The script checks that your AI provider is reachable before starting. If it is not, it prints setup instructions.

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
OPENAI_BASE_URL=http://localhost:8888 make test-live
```

The script sets up a temporary git repo with staged changes across docs, config, and code, then runs commit-pilot in dry-run mode. It checks for:

- Git repo detection (running outside a git repo prints an error)
- No changes (an empty repo prints a message)
- Multi-file changes (counts changed files and reaches the AI stage)
- Single mode (`--single` reaches the AI stage)
- Binary files (detected and reported; small binaries don't crash; mixed binary and text handled)
- Large diffs (many files split into window-sized batches; oversized single files chunked across LLM calls)
- Context window limits (a small window triggers batching warnings)
- Empty diffs (a staged file with no real change)
- Staged and unstaged changes (a mixed working tree)
- Path edge cases (unicode names, spaces, deeply nested directories, symlinks, renames, deletions)
- Diff edge cases (special characters, empty files, newline-only files)
- Failure modes (a pre-commit hook rejection fails the run; a file deleted mid-run)
- Subject truncation (long commit subjects are cut to the configured limit)

Use `commit-pilot --doctor` to verify a local provider before running the live
test. `commit-pilot --list-models` shows the model IDs the provider exposes.

The script keeps the temporary directory `.temp-test/` in the project root and cleans it up when it finishes.
