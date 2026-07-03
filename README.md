# <img src="img/logo.svg"  height=24/> [Commit Pilot](https://nisrulz.com/commit-pilot/)

Never type `git commit -m "fix stuff"` again.

**Local-first.** Reads your uncommitted changes, groups related files, and writes conventional commit messages through LMStudio (default), Ollama, Unsloth Studio, or any OpenAI-compatible API. **Zero telemetry.** No data leaves your machine.

![Banner](img/github_banner.webp)

📖 Read the story: [I Hate Writing Commit Messages, So I Built Commit Pilot](https://crushingcode.nisrulz.com/blog/i-hate-writing-commit-messages-so-i-built-commmit-pilot/)

## Quick start

```bash
curl -sfL https://github.com/nisrulz/commit-pilot/releases/latest/download/install.sh | sh
```

No Go needed. The script picks the right binary for your OS and puts it in `~/go/bin`.

Or build from source:

```bash
make install
# or
go build -o commit-pilot ./src/
```

Requires [Go](https://go.dev/dl/) 1.21+ and GNU Make.

## Configuration

All configuration is done via environment variables.

| Setting | Env var | Default |
|---|---|---|
| Provider | `OPENAI_PROVIDER` | `lmstudio` |
| Model | `OPENAI_MODEL` | `gemma-4-e2b-it-qat` |
| API base | `OPENAI_BASE_URL` | `http://localhost:1234/v1` |
| API key | `OPENAI_API_KEY` | — |
| Prompt text | `COMMIT_PILOT_PROMPT` | built-in |
| Prompt file | `COMMIT_PILOT_PROMPT_FILE` | — |
| Context window | `COMMIT_PILOT_CONTEXT_WINDOW` | `65536` (64k tokens) |

## Handling large diffs

Commit Pilot automatically handles changes that exceed the model's context window:

1. **Estimates** token count and checks against the context window
2. **Batches** files into groups that fit within the window
3. **Splits oversized files** into line-aligned chunks processed across multiple LLM calls
4. **Merges** chunk results into a single commit message
5. **Shows progress**: `Processing batch 1/3 (2 files)...`

When an oversized single file is detected, it shows per-chunk progress: `Chunk 2/5 of big.go`.

### Dynamic context window (LM Studio)

When using LM Studio, commit-pilot automatically determines the optimal context window:
- Checks available system RAM (reserves 5 GB for OS and apps)
- Queries the loaded model's `max_context_length` via LM Studio's REST API
- Uses `lms load --estimate-only` to binary-search the largest context that fits your RAM

No configuration needed. The tool adapts to your hardware.

### Manual override

Set the context window explicitly to override automatic detection:

```bash
export COMMIT_PILOT_CONTEXT_WINDOW=131072  # 128k tokens
```

## Cleanup

Remove temp files automatically after a successful run:

```bash
commit-pilot --cleanup
```

## Custom prompt

Override the default prompt with inline text or a file:

```bash
COMMIT_PILOT_PROMPT="Write concise conventional commits" commit-pilot
COMMIT_PILOT_PROMPT_FILE=myprompt.txt commit-pilot
```

The prompt template uses `{files}` and `{diff}` placeholders for the file list and git diff.

## Provider setup

See the provider-specific guides:

- [LMStudio](docs/lmstudio.md) (default, gemma-4-e2b-it-qat)
- [Ollama](docs/ollama.md) (gemma4:e2b-it-qat)
- [OpenAI](docs/openai.md) (gpt-4o-mini) — or any OpenAI-compatible API
- [Unsloth Studio](docs/unsloth.md) (unsloth/gemma-4-E4B-it-qat-GGUF)

## How it works

See [how-it-works.md](docs/how-it-works.md).

## Privacy

**Zero telemetry.** Commit Pilot doesn't track, phone home, or collect data. All AI processing happens via the provider you configure — no callbacks, no analytics, no data leaves your machine.

## Requirements

- [LMStudio](https://lmstudio.ai) (default), [Ollama](https://ollama.com), [Unsloth Studio](https://unsloth.ai), or OpenAI
- A git repository

## Development

See [dev.md](docs/dev.md) for build instructions, project structure, and scripts.

Run tests:

```bash
make test
```

## License

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
