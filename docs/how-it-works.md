# How it works

1. **Provider probe**: verifies the AI endpoint is reachable; with no explicit provider, probes the default Ollama server
2. **Git scan**: collects staged or unstaged changes (staged preferred)
3. **Token estimation**: checks if the diffs fit within the model context window
4. **Batching**: splits large diffs into manageable batches
5. **AI analysis**: sends selected diffs to the provider
6. **Grouping**: the AI returns conventional commit messages with file groupings
7. **Review and execution**: shows the plan, waits for confirmation, then stages and commits each logical group

## Provider selection

Ollama is the default provider (`lfm2.5:8b`). With no explicit provider, the
default Ollama server is probed at its default endpoint and selected when it
answers. A named `provider` or a customized `base_url` in the config file is
always respected and probed on its own. `openai_compat` points at any
OpenAI-compatible server. If nothing is reachable the run continues with the
configured provider so the provider's own error surfaces at the API call.

## Auto-chunk mode (default)

The AI decides the logical commit groups. Commit Pilot summarizes up to four files at a time, preserves their original order, and validates that every selected file is covered. Oversized files use the same chunking path as single mode.

Binary-only changes skip the provider and use `chore: update binary assets`.

## Temp files

In auto mode, commit-pilot writes per-file summaries to
`$COMMIT_PILOT_CONFIG_DIR/commit-pilot/tmp/` as it processes each file. It uses
these files to plan the groupings, and you can safely delete them after a run.
They live inside the same directory as the config file, so moving
`COMMIT_PILOT_CONFIG_DIR` relocates both:

```bash
rm -rf ~/.config/commit-pilot/tmp
```

Configuration lives in `$COMMIT_PILOT_CONFIG_DIR/commit-pilot/config.yaml`
(`COMMIT_PILOT_CONFIG_DIR` defaults to `~/.config`). Commit Pilot reads
provider, model, API-base, context, retry, timeout, and message-preference
settings from that YAML file. The file is created on first run with the default
values and is the source of truth; missing keys fall back to the built-in
defaults. The API key is never stored there. Commit Pilot reads it from the
`COMMIT_PILOT_OPENAI_COMPAT_API_KEY` environment variable.

For repository-specific message preferences, add `.commit-pilot.yaml` to the
repository. The project file cannot set `provider`, `model`, or `base_url`, so
a cloned repository cannot redirect where your diff is sent. Commit Pilot does
not create the project file.

## Commit confirmation

Before committing, Commit Pilot prints the proposed commit subjects and files, then
asks for confirmation. Use `--yes` when running non-interactively or when you want
to apply the plan without a prompt:

```bash
commit-pilot --yes
```

Commit Pilot rejects files with both staged and unstaged hunks before calling the
provider. Commit or stash one side first so the plan matches the exact content
that will be committed.

## Interrupt handling

Press `Ctrl+C` during a provider request to cancel the request and any pending
retry. While the model is generating, an animated working indicator shows
progress; it is hidden in `--quiet` and `--json` runs and whenever the output
is not a terminal.

## Single commit mode

Pass `--single` to put all changes into one commit:

```bash
commit-pilot --single
```

## Dry run

Preview without committing:

```bash
commit-pilot --dry-run
# equivalent
commit-pilot --no-commit
```

`--plan-out <path>` also implies a dry run. It writes the plan without creating
commits.

## Cleanup

Remove temp files automatically on success:

```bash
commit-pilot --cleanup
```

## Large diff handling

When changes exceed the model's context window (default 64k tokens), commit-pilot automatically:

- Batches files into groups that fit within the window
- Chunks oversized single files into line-aligned pieces where possible, including very long single lines
- Merges chunk results into a single commit message

The planning step applies the same budget discipline. The file summaries that
drive grouping are compacted to fit the model's context window, so the JSON
sent to the planner is always sized to what the model can process. Every prompt
carries an explicit JSON schema (field names, types, and which fields are
required). Built-in prompts request strict `json_schema` structured output so
the model must conform to that shape, and the request degrades to `json_object`
and then plain output when a provider does not support it.

Configure the context window size to override the default:

```yaml
context_window: 131072  # 128k tokens
```

Plan linting and plan application do not contact the provider.

## Provider safety

Commit Pilot only accepts HTTPS provider URLs outside the local machine. Plain
HTTP remains available for loopback addresses such as `localhost`, `127.0.0.1`,
and `::1`.

The tool has no telemetry. Selected diffs still go to the configured provider,
so use Ollama or another local OpenAI-compatible server when the code must stay
on your machine.

The `provider` config key accepts `ollama` and `openai_compat`. An unknown
provider name aborts before anything runs, so a typo can never silently fall
back to a different endpoint. `openai_compat` targets an OpenAI-compatible API
without a dedicated backend and requires a `base_url`.

## Output

Use `--quiet` to hide nonessential progress output. `--json` prints one result
object for scripts and editor tooling. It includes the run status and generated
commit groups. Interactive confirmation remains on stderr so stdout stays valid
JSON.

```
  * feat(api): add user search endpoint

    Add GET /api/users/:id endpoint.
    Implement search query builder.

    > files:
      - src/api/users.go
      - src/api/query.go

  * committed!
```

Dry-run output uses yellow `!` icons and says `dry-run, skipped` instead of `committed!`:

```
  ! docs: update readme

    Fix typo in installation section.

    > files:
      - README.md

  ! dry-run, skipped
```
