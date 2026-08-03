# <img src="img/logo.svg"  height=24/> [Commit Pilot](https://nisrulz.com/commit-pilot/)

Never type `git commit -m "fix stuff"` again.

**Local-first.** Reads your uncommitted changes, groups related files, and writes conventional commit messages through LMStudio (default), Ollama, Unsloth Studio, or any OpenAI-compatible API. **Zero telemetry.** Diffs only go to the provider you configure. Use a local provider to keep them on your machine.

![Banner](img/github_banner.webp)

📖 Read the story: [I Hate Writing Commit Messages, So I Built Commit Pilot](https://crushingcode.nisrulz.com/blog/i-hate-writing-commit-messages-so-i-built-commmit-pilot/)

## Quick start

```bash
curl -sfL https://github.com/nisrulz/commit-pilot/releases/latest/download/install.sh | sh
```

No Go needed. The script picks the right binary for your OS and puts it in `~/go/bin`.

Or build from source:

```bash
make install
```

Requires [Go](https://go.dev/dl/) 1.25+ and GNU Make.

The installer requires a matching SHA-256 checksum before it installs a release.
Release archives also have GitHub build provenance. You can verify a downloaded
archive with:

```bash
gh attestation verify <archive> --repo nisrulz/commit-pilot
```

## Configuration

You can supply configuration through environment variables or a local config file.

| Setting | Env var | Default |
|---|---|---|
| Provider | `OPENAI_PROVIDER` | `lmstudio` |
| Model | `OPENAI_MODEL` | `gemma-4-e2b-it-qat` |
| API base | `OPENAI_BASE_URL` | `http://localhost:1234/v1` |
| API key | `OPENAI_API_KEY` | unset |
| Prompt text | `COMMIT_PILOT_PROMPT` | built-in |
| Prompt file | `COMMIT_PILOT_PROMPT_FILE` | unset |
| Context window | `COMMIT_PILOT_CONTEXT_WINDOW` | `65536` (64k tokens) |
| Retries | `COMMIT_PILOT_RETRIES` | `2` |
| Request timeout | `COMMIT_PILOT_TIMEOUT_SECONDS` | `180` |
| Configuration directory | `COMMIT_PILOT_CONFIG_DIR` | `~/.config/commit-pilot` |
| Temporary summaries directory | `COMMIT_PILOT_TMP_DIR` | `~/.commit-pilot/tmp` |
| Conventional commits | `COMMIT_PILOT_CONVENTIONAL_COMMITS` | `true` |
| Ticket prefix | `COMMIT_PILOT_TICKET_PREFIX` | unset |
| Imperative subject tone | `COMMIT_PILOT_IMPERATIVE_TONE` | `true` |
| Subject limit | `COMMIT_PILOT_MAX_SUBJECT_LENGTH` | `100` |
| Commit body style | `COMMIT_PILOT_BODY_STYLE` | model default |

Temporary AI summaries and configuration use separate locations. `COMMIT_PILOT_TMP_DIR`
controls the disposable summaries created in auto mode. `COMMIT_PILOT_CONFIG_DIR`
contains reusable provider defaults.

For reusable provider defaults, create `config.env` in `COMMIT_PILOT_CONFIG_DIR`:

```dotenv
OPENAI_PROVIDER=ollama
OPENAI_MODEL=gemma4:e2b-it-qat
OPENAI_BASE_URL=http://localhost:11434/v1
```

If the file does not exist, Commit Pilot creates it with the default LM Studio
provider, model, and API base on its first run.

Values set in the environment take precedence over this file. API keys stay
environment-only.

For repository-specific message preferences, add `.commit-pilot/config.env`.
Project files cannot change the provider, model, or API base. Those settings
come from your environment or user config, so a cloned repository cannot choose
where your diff is sent. Commit Pilot never creates the project file.

Message preferences work in either config file: `COMMIT_PILOT_CONVENTIONAL_COMMITS`, `COMMIT_PILOT_TICKET_PREFIX`, `COMMIT_PILOT_IMPERATIVE_TONE`, `COMMIT_PILOT_MAX_SUBJECT_LENGTH`, and `COMMIT_PILOT_BODY_STYLE`. Environment values override config values.

For example:

```dotenv
COMMIT_PILOT_TICKET_PREFIX=PLAT-
COMMIT_PILOT_MAX_SUBJECT_LENGTH=72
COMMIT_PILOT_BODY_STYLE=short bullet list
```

## Review the commit plan

Before committing anything, Commit Pilot prints the proposed subjects and files and asks for confirmation. Use `--yes` for non-interactive or fully autonomous runs:

```bash
commit-pilot --yes
commit-pilot --single --yes
```

Use `--single` to create one commit for all changes.

Use `--plan-out plan.json` to save a generated plan for review. It never creates commits. Edit the JSON, then run `commit-pilot --apply plan.json` to validate it against the current changes and apply it.

Run `commit-pilot --plan-lint plan.json` to validate an edited plan without applying it. It checks file coverage, duplicate files, and your configured subject format and length.

## Control files sent to the model

Use `--include` and `--exclude` with glob patterns to limit the files sent to the model. Add one pattern per line to `.commitpilotignore` for project defaults. Commit Pilot skips paths that look like secrets, keys, or certificates unless you pass `--include-sensitive`. It prints every skipped path. Dependency lockfiles are included by default.

Use `--no-commit` when you want to generate and review a plan without creating commits. It is an alias for `--dry-run`.

## Check your setup

Run `commit-pilot --doctor` to check the current Git repository, resolved provider
settings, and provider connection. It never prints your API key.

Run `commit-pilot --list-models` to see the models reported by the configured
provider. Add `--json` for a machine-readable result. Regular JSON runs emit one
result object with a status and the commit groups. Confirmation prompts stay on
stderr. Use `--quiet` to hide nonessential progress output during a regular run.

## Handling large diffs

Commit Pilot automatically handles changes that exceed the model's context window:

1. Estimates the token count and checks it against the context window
2. Batches files into groups that fit within the window
3. Splits oversized files into line-aligned chunks across multiple LLM calls
4. Merges chunk results into a single commit message
5. Shows progress: `Processing batch 1/3 (2 files)...`

When a single file is too large, you see per-chunk progress: `Chunk 2/5 of big.go`.

Auto mode summarizes up to four files at a time and keeps their original order for planning. Binary-only changes use `chore: update binary assets` without calling the model.

### Dynamic context window (LM Studio)

When generating with LM Studio, commit-pilot automatically determines the optimal context window:
- Checks available system RAM (reserves 5 GB for OS and apps)
- Queries the loaded model's `max_context_length` via LM Studio's REST API
- Uses `lms load --estimate-only` to binary-search the largest context that fits your RAM

You don't need to configure anything. The tool adapts to your hardware.

### Manual override

Set the context window explicitly to override automatic detection:

```bash
export COMMIT_PILOT_CONTEXT_WINDOW=131072  # 128k tokens
```

## Cleanup

Remove temp files automatically after a successful run:

```bash
commit-pilot --cleanup
```

## Custom prompt

Override the default prompt with inline text or a file:

```bash
COMMIT_PILOT_PROMPT="Write concise conventional commits" commit-pilot
COMMIT_PILOT_PROMPT_FILE=myprompt.txt commit-pilot
```

The prompt template uses `{files}` and `{diff}` placeholders for the file list and git diff.

## Provider setup

See the provider-specific guides:

- [LMStudio](docs/lmstudio.md) (default, gemma-4-e2b-it-qat)
- [Ollama](docs/ollama.md) (gemma4:e2b-it-qat)
- [OpenAI](docs/openai.md) (gpt-4o-mini) or any OpenAI-compatible API
- [Unsloth Studio](docs/unsloth.md) (unsloth/gemma-4-E4B-it-qat-GGUF)

`OPENAI_PROVIDER` accepts `lmstudio`, `ollama`, `openai`, `unsloth`, and `custom`.
Any other value aborts the run, so a typo cannot silently point commit-pilot at
the wrong endpoint. Use `custom` for an OpenAI-compatible API that is not one of
the named providers; it requires `OPENAI_BASE_URL`.

## How it works

See [how-it-works.md](docs/how-it-works.md).

## Privacy

**Zero telemetry.** Commit Pilot has no analytics or callbacks. It sends selected diffs to the provider shown at the start of a run. Local providers keep that data on your machine. Remote providers receive it over HTTPS. Project config files cannot change the provider endpoint.

Commit Pilot refuses partially staged files because widening a hunk selection would commit code you did not review. Commit or stash one side of the file, then run the command again.

## Requirements

- [LMStudio](https://lmstudio.ai) (default), [Ollama](https://ollama.com), [Unsloth Studio](https://unsloth.ai), or OpenAI
- A git repository

## Development

See [dev.md](docs/dev.md) for build instructions, project structure, and scripts, and [testing.md](docs/testing.md) for the test suites.

Run tests:

```bash
make test
```

## License

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
