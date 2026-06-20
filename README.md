# <img src="img/logo.svg"  height=24/> [Commit Pilot](https://nisrulz.com/commit-pilot/)

Never type `git commit -m "fix stuff"` again.

**Local-first.** Reads your uncommitted changes, groups related files, and writes conventional commit messages through LMStudio (default), Ollama, or any OpenAI-compatible API. **Zero telemetry — no data leaves your machine.**

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

Commit Pilot automatically batches large diffs that exceed the model's context window. When processing many files, it will:

1. Estimate token count for your changes
2. Split files into batches that fit the context window
3. Process each batch sequentially
4. Show progress: `Processing batch 1/3 (2 files)...`

If you encounter context length errors, increase the window:

```bash
export COMMIT_PILOT_CONTEXT_WINDOW=131072  # 128k tokens
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

## How it works

See [how-it-works.md](docs/how-it-works.md).

## Privacy

**Zero telemetry.** Commit Pilot doesn't track, phone home, or collect data. All AI processing happens via the provider you configure — no callbacks, no analytics, no data leaves your machine.

## Requirements

- [LMStudio](https://lmstudio.ai) (default), Ollama, or OpenAI
- A git repository

## Development

See [dev.md](docs/dev.md) for build instructions, project structure, and scripts.

## License

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
