# Testing

## Unit tests

Focused unit tests live in `tests/` (external test package `lib_test`). They
target individual functions in `src/lib`: plan validation, provider retries,
cancellation, confirmation input, config precedence, Git scope behavior, and
provider dispatch (including unknown-provider rejection and probe body draining).
They also cover JSON resilience: repair of truncated model output, structured
output negotiation (strict `json_schema` degrading to `json_object` and then a
plain request), and the retry paths for responses that are cut off or contain
no JSON.

End-to-end tests live in `tests/e2e/`. They build the real CLI binary and run it
against a mock OpenAI-compatible provider. Together they cover auto and single
mode commits, plan-out/apply/plan-lint, dry-run, sensitive-file filtering, scope
flags, partially staged files, option-like filenames, binary-only commits,
`--list-models`, `--doctor`, and provider failures. A resilience suite drives
the whole CLI through bad model output: a truncated plan that gets repaired, a
prose-only plan that gets a strict re-prompt, a provider that rejects
`json_object` and receives a plain retry, and a completion cut off at its token
budget that gets a larger-budget retry.

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

`make test` runs the full suite and reports the results grouped into readable
categories such as `Planning`, `LLM & Providers`, and `End To End`, with
coverage and pass, fail, and skip totals. It fails the build when any test
fails.

Coverage is measured for the `src/lib` package across both suites.

## Live test

The integration test runs commit-pilot against a real AI endpoint.

The script checks that your AI provider is reachable before starting. It probes
the default Ollama endpoint (trying both `localhost` and `127.0.0.1`) or, when
`OPENAI_BASE_URL` is set, that endpoint's `/models` route. A 401 counts as
reachable, so key-protected servers such as Unsloth Studio are recognized even
before the API key is configured. If nothing responds, it prints setup
instructions.

**Ollama (default):**
```bash
make test-live
```

**LM Studio (openai_compat):**
```bash
OPENAI_BASE_URL=http://localhost:1234/v1 make test-live
```

**OpenAI (or any OpenAI-compatible endpoint):**
```bash
OPENAI_BASE_URL=https://api.openai.com/v1 \
  OPENAI_API_KEY=sk-... \
  make test-live
```

**Unsloth Studio (openai_compat):**
```bash
OPENAI_BASE_URL=http://localhost:8888/v1 make test-live
```

The script sets up a temporary git repo with staged changes across docs,
config, and code, then runs commit-pilot in dry-run mode. Output streams as it
happens: the provider probe and build report appear immediately, then a working
spinner runs through the silent test phase. Results are grouped into titled
tables such as `repo & changes` and `binary files`, with pass and fail totals
when the run finishes. It checks for:

- **Repo & Changes**: running outside a git repo, an empty repo, multi-file
  changes that reach the AI stage, single mode, and a mixed staged and
  unstaged working tree
- **Binary Files**: detecting binary files, multiple binary formats, small
  binaries, and binary mixed with text
- **Large Diffs**: many files split into window-sized batches, a small context
  window that triggers batching, a very large single file, and a huge diff
  that chunks across LLM calls
- **Path Edge Cases**: unicode names, deleted files, renames, symlinks, deeply
  nested directories, and spaces in paths
- **Diff Edge Cases**: empty diffs, special characters, empty files,
  newline-only files, and subject-line truncation
- **Failure Modes**: a pre-commit hook rejection fails the run, and a file
  deleted mid-run

Use `commit-pilot --doctor` to verify a local provider before running the live
test. `commit-pilot --list-models` shows the model IDs the provider exposes.

The script keeps the temporary directory `.temp-test/` in the project root and cleans it up when it finishes.
