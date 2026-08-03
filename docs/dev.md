# Developer guide

## Build

Commit Pilot requires Go 1.25 or newer.

```bash
make build
```

Build and install to `$GOPATH/bin`:

```bash
make install
```

## Project structure

```
commit-pilot/
├── .github/workflows/
│   ├── pages.yml         # GitHub Pages deployment
│   ├── release.yml       # Release automation
│   └── test.yml          # Test suite
├── docs/
│   ├── dev.md            # Development guide
│   ├── github-pages.md   # Website deployment
│   ├── how-it-works.md   # How commit-pilot works
│   ├── lmstudio.md       # LMStudio setup
│   ├── ollama.md         # Ollama setup
│   ├── openai.md         # OpenAI setup
│   ├── testing.md        # Unit, e2e, and live test guide
│   ├── unsloth.md        # Unsloth Studio setup
│   └── usage.md          # Usage reference
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
│       ├── provider/     # Pluggable model-serving backends (one file per provider)
│       │   ├── provider.go   # Provider interface, registry, shared probe/listing helpers
│       │   ├── openai.go     # OpenAI-compatible hosted endpoints
│       │   ├── ollama.go     # Ollama local API
│       │   ├── lmstudio.go   # LM Studio local API
│       │   ├── unsloth.go    # Unsloth Studio (health-probed, key-protected API)
│       │   └── custom.go     # Generic OpenAI-compatible endpoint (OPENAI_BASE_URL)
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

## Testing

See [testing.md](testing.md) for the unit, end-to-end, and live test suites and how to run them.

## Releasing

Tag a commit and push to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This triggers the [GitHub Actions](../.github/workflows/release.yml) workflow.
It builds binaries for macOS, Linux, and Windows and creates a GitHub Release
with checksums. The workflow pins every action and the GoReleaser version. The
installer aborts if the matching archive checksum is missing or invalid. GitHub
also records build provenance for every release archive and the checksum file.
