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
│   ├── lmstudio.md       # LM Studio setup (openai_compat example)
│   ├── ollama.md         # Ollama setup
│   ├── openai.md         # OpenAI setup (openai_compat example)
│   ├── openai_compat.md  # OpenAI-compatible providers setup
│   ├── testing.md        # Unit, e2e, and live test guide
│   ├── unsloth.md        # Unsloth Studio setup (openai_compat example)
│   └── usage.md          # Usage reference
├── img/
│   ├── github_banner.webp
│   └── logo.svg
├── scripts/
│   ├── install.sh        # One-line install script
│   ├── live-test.sh      # Live integration test driver
│   ├── livetable/        # Renders live-test results (make test-live)
│   ├── setup-lmstudio.sh # LMStudio model download
│   ├── setup-ollama.sh   # Ollama model download
│   ├── setup-path.sh     # PATH setup helper
│   └── testtable/        # Renders unit/e2e results (make test)
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
│       ├── banner.go     # Startup banner rendering and version
│       ├── banner_art.go # Banner ASCII art and tagline
│       ├── batch.go      # Batch splitting and diff chunking
│       ├── doctor.go     # Doctor checks and model listing
│       ├── output.go     # Terminal/JSON output helpers and fatal errors
│       ├── probe.go      # Provider reachability probing
│       ├── provider/     # Pluggable model-serving backends (one file per provider)
│       │   ├── provider.go       # Provider interface, registry, shared probe/listing helpers
│       │   └── openai_compat.go  # OpenAI-compatible endpoints (hosted or local)
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

### Live test

The integration test runs commit-pilot against a real AI endpoint.

The script checks that your AI provider is reachable before starting. If it is not, it prints setup instructions.

The script builds a YAML config file under a temp config base and runs the binary
with `COMMIT_PILOT_CONFIG_DIR` set. Local runs use Ollama. GitHub Actions starts
the Go mock server in `scripts/mock-openai` and uses the standard `CI` variable
to point the same harness at it.

**Ollama (default):**
```bash
make test-live
```

It sets up a temporary git repo with staged changes across docs, config, and code, then runs commit-pilot in dry-run mode. It checks for:

- Git repo detection (non-git dir says error)
- No changes (empty repo says message)
- File detection (counts multi-file changes)
- AI pipeline (git scan reaches AI call)
- Single commit mode (`--single` flag)
- Binary file detection (`.bin` file listed)

The temp directory `.temp-test/` lives in the project root and gets cleaned up when the script finishes.

## Releasing

Cut a release with the helper script, which bumps the version in the startup
banner (and the test that pins it), commits the bump, tags it, and pushes both
to the remote:

```bash
scripts/release.sh 1.1.0
```

The script refuses to run if the working tree is dirty or if the tag already
exists. Pushing the tag triggers the [GitHub Actions](../.github/workflows/release.yml)
workflow, which builds binaries for macOS, Linux, and Windows and creates a
GitHub Release with checksums. The workflow pins every action and the GoReleaser
version. The installer aborts if the matching archive checksum is missing or
invalid. GitHub also records build provenance for every release archive and the
checksum file.
