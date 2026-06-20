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
make build && ./commit-pilot 1 --dry-run
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
│   └── openai.md         # OpenAI setup
├── img/
│   ├── github_banner.webp
│   └── logo.svg
├── scripts/
│   ├── install.sh        # One-line install script
│   ├── setup-lmstudio.sh # LMStudio model download
│   ├── setup-ollama.sh   # Ollama model download
│   └── setup-path.sh     # PATH setup helper
├── src/
│   ├── main.go           # Entry point, orchestration
│   ├── config.go         # CLI parsing, config resolution
│   ├── git.go            # Git operations
│   ├── llm.go            # LLM API client, JSON extraction
│   ├── prompt.go         # Prompt loading and formatting
│   ├── commit.go         # AI commit group parsing and execution
│   ├── grouping.go       # File categorization, grouping, merging logic
│   ├── tokens.go         # Token estimation, batch splitting
│   ├── output.go         # Terminal output helpers (colors, formatting)
│   └── prompt.txt        # Default prompt templates (embedded)
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
| `make clean` | Remove the binary |
| `make test-live` | Run live integration test (requires AI provider running) |
| `make setup-lmstudio` | Download default model for LMStudio |
| `make setup-ollama` | Download default model for Ollama |
| `make uninstall` | Remove from `~/go/bin` |

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

It sets up a temporary git repo with staged changes across docs, config, and code, then runs commit-pilot in dry-run mode. It checks for:

- Git repo detection (non-git dir says error)
- No changes (empty repo says message)
- File detection (counts multi-file changes)
- AI pipeline (git scan reaches AI call)
- Single commit mode (positional `1` arg)
- Binary file detection (`.bin` file listed)

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
