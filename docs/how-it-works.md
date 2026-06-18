# How it works

1. **Git scan** - collects staged or unstaged changes (staged preferred)
2. **AI analysis** - sends diffs to the LLM you configured
3. **Grouping** - the AI returns conventional commit messages with file groupings
4. **Execution** - stages and commits each logical group

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
