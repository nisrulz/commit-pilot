# <img src="img/logo.svg"  height=24/> Commit Pilot

Never type `git commit -m "fix stuff"` again.

Reads your uncommitted changes, groups related files, and writes conventional commit messages through LMStudio, Ollama, or OpenAI.

![Banner](img/github_banner.webp)

## Quick start

### Install script

```bash
curl -sfL https://github.com/nisrulz/commit-pilot/releases/latest/download/install.sh | sh
```

No Go needed. The script picks the right binary for your OS and puts it in /usr/local/bin.

### Go install

```bash
go install github.com/nisrulz/commit-pilot/src@latest
```

Requires [Go](https://go.dev/dl/) 1.21+.

### Build from source

```bash
make install
```

Requires Go 1.21+ and GNU Make.

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
- [OpenAI](docs/openai.md) (gpt-4o-mini)

## How it works

See [how-it-works.md](docs/how-it-works.md).

## Requirements

- [LMStudio](https://lmstudio.ai) (default), Ollama, or OpenAI
- A git repository

## Development

See [dev.md](docs/dev.md) for build instructions, project structure, and scripts.

## License

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
