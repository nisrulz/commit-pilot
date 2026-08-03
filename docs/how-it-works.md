# How it works

1. **Git scan**: collects staged or unstaged changes (staged preferred)
2. **Token estimation**: checks if the diffs fit within the model context window
3. **Batching**: splits large diffs into manageable batches
4. **AI analysis**: shows the configured destination, then sends selected diffs to it
5. **Grouping**: the AI returns conventional commit messages with file groupings
6. **Review and execution**: shows the plan, waits for confirmation, then stages and commits each logical group

## Auto-chunk mode (default)

The AI decides the logical commit groups. Commit Pilot summarizes up to four files at a time, preserves their original order, and validates that every selected file is covered. Oversized files use the same chunking path as single mode.

Binary-only changes skip the provider and use `chore: update binary assets`.

## Temp files

In auto mode, commit-pilot writes per-file summaries to `~/.commit-pilot/tmp/` as it processes each file. It uses these files to plan the groupings, and you can safely delete them after a run. Set `COMMIT_PILOT_TMP_DIR` to store them elsewhere:

```bash
COMMIT_PILOT_TMP_DIR=/path/to/commit-pilot-tmp commit-pilot
rm -rf ~/.commit-pilot/tmp
```

Configuration lives separately in `~/.config/commit-pilot/`. Set
`COMMIT_PILOT_CONFIG_DIR` to point elsewhere. Commit Pilot reads provider,
model, and API-base defaults from the `config.env` there. If the file is
missing, it creates one with the LM Studio defaults. Environment variables
always win. This directory never holds temporary summaries.

For repository-specific message preferences, add `.commit-pilot/config.env`.
The project file cannot set `OPENAI_PROVIDER`, `OPENAI_MODEL`, or
`OPENAI_BASE_URL`. Provider settings come from your environment or user config.
Commit Pilot does not create the project file.

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
retry.

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

### Dynamic context detection (LM Studio)

When generating with LM Studio and no explicit `COMMIT_PILOT_CONTEXT_WINDOW`, the tool automatically:

1. Reads available system RAM (reserving 5 GB for OS and apps)
2. Queries LM Studio's REST API for the loaded model's `max_context_length`
3. Uses `lms load --estimate-only` to binary-search the largest context length that fits

Configure the context window size to override:

```bash
export COMMIT_PILOT_CONTEXT_WINDOW=131072  # 128k tokens
```

Plan linting and plan application do not contact the provider or run context
detection.

## Provider safety

Commit Pilot only accepts HTTPS provider URLs outside the local machine. Plain
HTTP remains available for loopback addresses such as `localhost`, `127.0.0.1`,
and `::1`.

The tool has no telemetry. Selected diffs still go to the configured provider,
so use LM Studio, Ollama, or another local provider when the code must stay on
your machine.

`OPENAI_PROVIDER` accepts `lmstudio`, `ollama`, `openai`, `unsloth`, and
`custom`. An unknown provider name aborts before anything runs, so a typo can
never silently fall back to a different endpoint. `custom` targets an
OpenAI-compatible API without a dedicated backend and requires
`OPENAI_BASE_URL`.

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
