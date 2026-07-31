# Developer guide

## Build

```bash
make build
```

Build and install to `$GOPATH/bin`:

```bash
make install
```

Run directly after build:

```bash
make build && ./commit-pilot --dry-run
make build && ./commit-pilot --single --dry-run
```

## Project structure

```
commit-pilot/
├── .github/workflows/
│   └── release.yml       # Release automation
├── docs/
│   ├── dev.md            # Development guide
│   ├── github-pages.md   # Website deployment
│   ├── how-it-works.md   # How commit-pilot works
│   ├── lmstudio.md       # LMStudio setup
│   ├── ollama.md         # Ollama setup
│   ├── openai.md         # OpenAI setup
│   └── unsloth.md        # Unsloth Studio setup
├── img/
│   ├── github_banner.webp
│   └── logo.svg
├── scripts/
│   ├── install.sh        # One-line install script
│   ├── setup-lmstudio.sh # LMStudio model download
│   ├── setup-ollama.sh   # Ollama model download
│   └── setup-path.sh     # PATH setup helper
├── src/
│   ├── main.go           # CLI entry point (thin wrapper)
│   └── lib/
│       ├── main.go       # Entry point: flag parsing, dispatch, signal handling
│       ├── args.go       # Command-line flag parsing
│       ├── config.go     # Config struct and env/config-file resolution
│       ├── help.go       # Usage/help text
│       ├── workflow.go   # Run flows: plan-lint, apply, generate, single/auto mode
│       ├── git.go        # Git operations and change collection
│       ├── filter.go     # Include/exclude and sensitive-path filtering
│       ├── llm.go        # LLM API client with retries and cancellation
│       ├── json.go       # JSON extraction from model responses
│       ├── prompt.go     # Prompt loading and formatting
│       ├── group.go      # Commit group domain: parsing, merging, AI grouping
│       ├── commit.go     # Commit execution and plan confirmation
│       ├── grouping.go   # Binary-file assignment to commit groups
│       ├── plan_file.go  # Plan read/write/validate/lint
│       ├── pipeline.go   # Summarization & planning pipeline
│       ├── summarize.go  # Per-file diff summarization
│       ├── tokens.go     # Token estimation and context-fit checks
│       ├── batch.go      # Batch splitting and diff chunking
│       ├── context.go    # Dynamic context window detection
│       ├── doctor.go     # Doctor checks and model listing
│       ├── output.go     # Terminal/JSON output helpers and fatal errors
│       └── prompt.txt    # Default prompt templates (embedded)
├── tests/
│   ├── *_test.go           # Unit tests (package lib_test)
│   └── e2e/                # End-to-end CLI tests against a mock provider
├── .gitignore
├── .goreleaser.yaml
├── go.mod
├── go.sum
├── index.html
├── LICENSE
├── Makefile
└── README.md
```

## Makefile targets

| Target | Description |
|---|---|
| `make build` | Build the binary |
| `make install` | Build and copy to `~/go/bin` |
| `make vet` | Run static analysis |
| `make test` | Run unit and end-to-end tests |
| `make clean` | Remove the binary |
| `make test-live` | Run live integration test (requires AI provider running) |
| `make setup-lmstudio` | Download default model for LMStudio |
| `make setup-ollama` | Download default model for Ollama |
| `make uninstall` | Remove from `~/go/bin` |

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
# or
go test -count=1 ./tests/... -coverpkg=./src/lib/
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
OPENAI_BASE_URL=http://localhost:8888/v1 \
  OPENAI_API_KEY=sk-unsloth-... \
  make test-live
```

It sets up a temporary git repo with staged changes across docs, config, and code, then runs commit-pilot in dry-run mode. It checks for:

- Git repo detection (non-git dir says error)
- No changes (empty repo says message)
- File detection (counts multi-file changes)
- AI pipeline (git scan reaches AI call)
- Single commit mode (`--single`)
- Binary file detection (`.bin` file listed)

Use `commit-pilot --doctor` to verify a local provider before running the live
test. `commit-pilot --list-models` shows the model IDs the provider exposes.

The temp directory `.temp-test/` lives in the project root and gets cleaned up when the script finishes.

## Releasing

Tag a commit and push to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This triggers the [GitHub Actions](../.github/workflows/release.yml) workflow.
It builds binaries for macOS, Linux, and Windows and creates a GitHub Release
with checksums.
