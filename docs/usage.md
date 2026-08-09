# Usage

This page is the reference for running commit-pilot. For how the tool decides
what to commit, see [how-it-works.md](how-it-works.md). For provider-specific
setup, see the provider guides listed at the bottom.

## Configuration

The CLI ships no config. Everything except the API key lives in a single YAML
config file that is created on first run with the default values and is the
source of truth. User config lives at `$COMMIT_PILOT_CONFIG_DIR/commit-pilot/config.yaml`.
`COMMIT_PILOT_CONFIG_DIR` defaults to `~/.config`, so by default the config
file is `~/.config/commit-pilot/config.yaml`. The `commit-pilot` subdir is
created on first run; the tool also keeps its own working files (e.g. tmp
summaries) there, so they move together.

```yaml
provider: openai_compat
model: lfm2.5:8b
base_url: http://localhost:11434/v1
context_window: 65536     # 64k tokens
retries: 2
timeout_seconds: 180
conventional: true
ticket_prefix: ""
imperative: true
max_subject_length: 100
body_style: ""
prompt: ...              # optional custom prompt text
mode: auto               # auto | single
dry_run: false
cleanup: false
```

Missing keys fall back to the built-in defaults. The file is written with `0600`
permissions; edit it directly to configure the tool. `dry_run` and `cleanup`
are flags (`--dry-run`, `--cleanup`) rather than config values.

The API key is never stored in the config file. Commit Pilot reads it from
`COMMIT_PILOT_OPENAI_COMPAT_API_KEY` only, so it stays out of config files and shell
history.

### Upgrade from older versions

Move `COMMIT_PILOT_OPENAI_API_KEY` to `COMMIT_PILOT_OPENAI_COMPAT_API_KEY`.
Commit Pilot reports an error if the old variable is still set. The only
provider value is now `openai_compat`; the generated config points it at Ollama
by default.

| Env var | Purpose | Default |
|---|---|---|
| `COMMIT_PILOT_CONFIG_DIR` | Base config directory | `~/.config` |
| `COMMIT_PILOT_OPENAI_COMPAT_API_KEY` | API key (never stored in config) | unset |

### Resolution precedence (highest first)

1. `--config <path>` flag (config file for this invocation)
2. `COMMIT_PILOT_CONFIG_DIR` env var
3. `~/.config/commit-pilot/config.yaml`

An explicit `--config` pointing at a missing file is a hard error. A missing
file from the env var or default path is created with the default values on
first run, so a fresh install works before any config exists and the file is
then the source of truth.

### Session scope

- `--config <path>` applies to a single invocation.
- `COMMIT_PILOT_CONFIG_DIR=/path commit-pilot ...` applies to that command line
  only (the value is not exported to the shell).
- Only `export COMMIT_PILOT_CONFIG_DIR=/path` makes it persist for the shell
  session.

### Repository config (untrusted)

For repository-specific preferences, add `.commit-pilot.yaml` to the
repository. It is treated as untrusted and may only set commit-message
preferences (`conventional`, `ticket_prefix`, `imperative`,
`max_subject_length`, `body_style`) and output defaults (`default_branch`,
`output_format`). Any other key is rejected, so a cloned repository cannot
change the provider, model, or API base. Those come from your user config, and
Commit Pilot never creates the repository file.

For example:

```yaml
ticket_prefix: PLAT-
max_subject_length: 72
body_style: short bullet list
```

## Provider selection

The only provider is `openai_compat`. The default config points it at Ollama
(`http://localhost:11434/v1`) with model `lfm2.5:8b`. For LM Studio, Unsloth
Studio, OpenAI, or another compatible server, change `base_url` and `model` in
the config file. An explicit `provider` or customized `base_url` is always
respected and probed on its own.

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

## Reliable model output

Commit Pilot asks for JSON and, when your provider supports it, requests strict
structured output through `response_format`. Built-in prompts send a strict
`json_schema` that pins the exact response shape; custom prompts and providers
without structured-output support fall back to `json_object` and then a plain
request automatically, so nothing breaks on older servers. Every prompt also
spells out the JSON schema it expects, so the model returns the right shape
even without structured output.

Input is sized to the model before it is sent. The summaries that drive plan
grouping are compacted to fit the configured context window, reserving room for
the template, the response, and a safety margin, so the JSON never exceeds what
the model can process.

On top of that, three safeguards keep malformed model responses from failing
the run:

- A response that hits the output budget is retried automatically with a
  larger budget.
- A response with no JSON at all is asked once more with a strict
  only-JSON instruction.
- JSON cut off mid-structure is repaired by closing the unfinished brackets
  and dropping a dangling key before parsing.

## Custom prompt

Override the default prompt with a `prompt` key in the config file:

```yaml
prompt: Write concise conventional commits with {files} and {diff}.
```

The prompt template uses `{files}` and `{diff}` placeholders for the file list
and git diff.

## Cleanup

Remove temp files automatically after a successful run:

```bash
commit-pilot --cleanup
```

## Providers

The `provider` config key accepts only `openai_compat`. Any other value aborts
the run, so a typo cannot silently point commit-pilot at the wrong endpoint.

See the provider-specific guides:

- [Ollama](ollama.md) (default endpoint, lfm2.5:8b)
- [OpenAI-compatible](openai_compat.md) (any OpenAI-compatible API)
- [OpenAI](openai.md) (gpt-5.6-luna)
- [LM Studio](lmstudio.md) (openai_compat example, lfm2.5:8b)
- [Unsloth Studio](unsloth.md) (openai_compat example)
