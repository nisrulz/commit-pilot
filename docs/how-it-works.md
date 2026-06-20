# How it works

1. **Git scan** - collects staged or unstaged changes (staged preferred)
2. **Token estimation** - estimates if diffs fit within model context window
3. **Batching** - splits large diffs into manageable batches
4. **AI analysis** - sends diffs to the LLM you configured
5. **Grouping** - the AI returns conventional commit messages with file groupings
6. **Execution** - stages and commits each logical group

## Auto-chunk mode (default)

The AI groups related file changes into logical commits. Change a bug fix and a refactor in different files? They become separate commits.

## Single commit mode

Pass `1` to put all changes into one commit:

```bash
commit-pilot 1
```

## Dry run

Preview without committing:

```bash
commit-pilot --dry-run
```

## Large diff handling

When changes exceed the model's context window (default 64k tokens), commit-pilot automatically:

- Splits files into batches
- Processes each batch sequentially
- Merges results into final commits

Configure the context window size:

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
