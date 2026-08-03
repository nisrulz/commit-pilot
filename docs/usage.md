# Usage

This page is the reference for running commit-pilot. For how the tool decides
what to commit, see [how-it-works.md](how-it-works.md). For provider-specific
setup, see the provider guides listed at the bottom.

## Configuration

Configuration comes from environment variables or a local config file.
Environment values always win.

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

### Config files

Temporary AI summaries and configuration use separate locations.
`COMMIT_PILOT_TMP_DIR` controls the disposable summaries created in auto mode.
`COMMIT_PILOT_CONFIG_DIR` contains reusable provider defaults.

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

Message preferences work in either config file:
`COMMIT_PILOT_CONVENTIONAL_COMMITS`, `COMMIT_PILOT_TICKET_PREFIX`,
`COMMIT_PILOT_IMPERATIVE_TONE`, `COMMIT_PILOT_MAX_SUBJECT_LENGTH`, and
`COMMIT_PILOT_BODY_STYLE`. Environment values override config values.

For example:

```dotenv
COMMIT_PILOT_TICKET_PREFIX=PLAT-
COMMIT_PILOT_MAX_SUBJECT_LENGTH=72
COMMIT_PILOT_BODY_STYLE=short bullet list
```

## Review the commit plan

Before committing anything, Commit Pilot prints the proposed subjects and files
and asks for confirmation. Use `--yes` for non-interactive or fully autonomous
runs:

```bash
commit-pilot --yes
commit-pilot --single --yes
```

Use `--single` to create one commit for all changes.

Use `--plan-out plan.json` to save a generated plan for review. It never creates
commits. Edit the JSON, then run `commit-pilot --apply plan.json` to validate it
against the current changes and apply it.

Run `commit-pilot --plan-lint plan.json` to validate an edited plan without
applying it. It checks file coverage, duplicate files, and your configured
subject format and length.

## Control files sent to the model

Use `--include` and `--exclude` with glob patterns to limit the files sent to
the model. Add one pattern per line to `.commitpilotignore` for project
defaults. Commit Pilot skips paths that look like secrets, keys, or
certificates unless you pass `--include-sensitive`. It prints every skipped
path. Dependency lockfiles are included by default.

Use `--no-commit` when you want to generate and review a plan without creating
commits. It is an alias for `--dry-run`.

## Check your setup

Run `commit-pilot --doctor` to check the current Git repository, resolved
provider settings, and provider connection. It never prints your API key.

Run `commit-pilot --list-models` to see the models reported by the configured
provider. Add `--json` for a machine-readable result. Regular JSON runs emit one
result object with a status and the commit groups. Confirmation prompts stay on
stderr. Use `--quiet` to hide nonessential progress output during a regular run.

## Custom prompt

Override the default prompt with inline text or a file:

```bash
COMMIT_PILOT_PROMPT="Write concise conventional commits" commit-pilot
COMMIT_PILOT_PROMPT_FILE=myprompt.txt commit-pilot
```

The prompt template uses `{files}` and `{diff}` placeholders for the file list
and git diff.

## Cleanup

Remove temp files automatically after a successful run:

```bash
commit-pilot --cleanup
```

## Providers

`OPENAI_PROVIDER` accepts `lmstudio`, `ollama`, `openai`, `unsloth`, and
`custom`. Any other value aborts the run, so a typo cannot silently point
commit-pilot at the wrong endpoint. Use `custom` for an OpenAI-compatible API
that is not one of the named providers; it requires `OPENAI_BASE_URL`.

See the provider-specific guides:

- [LMStudio](lmstudio.md) (default, gemma-4-e2b-it-qat)
- [Ollama](ollama.md) (gemma4:e2b-it-qat)
- [OpenAI](openai.md) (gpt-4o-mini) or any OpenAI-compatible API
- [Unsloth Studio](unsloth.md) (unsloth/gemma-4-E4B-it-qat-GGUF)
