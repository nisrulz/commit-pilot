# How it works

1. **Git scan** - collects staged or unstaged changes (staged preferred)
2. **Token estimation** - estimates if diffs fit within model context window
3. **Batching** - splits large diffs into manageable batches
4. **AI analysis** - sends diffs to the LLM you configured
5. **Grouping** - the AI returns conventional commit messages with file groupings
6. **Execution** - stages and commits each logical group

## Auto-chunk mode (default)

The AI groups related file changes into logical commits. Change a bug fix and a refactor in different files? They become separate commits.

## Temp files

In auto mode, commit-pilot writes per-file summaries to `~/.commit-pilot/tmp/` as it processes each file. These JSON files are used to plan logical commit groupings and can be safely deleted after a run. Set `COMMIT_PILOT_TMP_DIR` to store them elsewhere:

```bash
COMMIT_PILOT_TMP_DIR=/path/to/commit-pilot-tmp commit-pilot
rm -rf ~/.commit-pilot/tmp
```

Configuration belongs separately in `~/.config/commit-pilot/`. Set
`COMMIT_PILOT_CONFIG_DIR` to use another configuration directory. Commit Pilot
loads provider, model, and API-base defaults from `config.env` in that directory;
creates the file with the LM Studio defaults when absent, and gives environment
variables precedence. This directory is not used for temporary summaries.

## Commit confirmation

Before committing, Commit Pilot prints the proposed commit subjects and files, then
asks for confirmation. Use `--yes` when running non-interactively or when you want
to apply the plan without a prompt:

```bash
commit-pilot --yes
```

Use `--cleanup` to remove the temp file automatically on success:

```bash
commit-pilot --cleanup
```

## Interrupt handling

Press `Ctrl+C` at any point. Commit-pilot exits cleanly with a message and no changes get committed.

## Single commit mode

Pass `--single` to put all changes into one commit:

```bash
commit-pilot --single
```

## Dry run

Preview without committing:

```bash
commit-pilot --dry-run
```

## Cleanup

Remove temp files automatically on success:

```bash
commit-pilot --cleanup
```

## Large diff handling

When changes exceed the model's context window (default 64k tokens), commit-pilot automatically:

- **Batches** files into groups that fit within the window
- **Chunks** oversized single files into line-aligned pieces, processed across multiple LLM calls
- **Merges** chunk results into a single commit message

### Dynamic context detection (LM Studio)

When using LM Studio with no explicit `COMMIT_PILOT_CONTEXT_WINDOW`, the tool automatically:

1. Reads available system RAM (reserving 5 GB for OS and apps)
2. Queries LM Studio's REST API for the loaded model's `max_context_length`
3. Uses `lms load --estimate-only` to binary-search the largest context length that fits

Configure the context window size to override:

```bash
export COMMIT_PILOT_CONTEXT_WINDOW=131072  # 128k tokens
```

## Output

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
