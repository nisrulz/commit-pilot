# OpenAI-compatible API

Commit Pilot has one provider: `openai_compat`. Use it for Ollama, LM Studio,
Unsloth Studio, OpenAI, or another server that implements the OpenAI
`/v1/chat/completions` and `/v1/models` endpoints.

## Configuration

The generated config uses Ollama as the default endpoint:

```yaml
provider: openai_compat
base_url: http://localhost:11434/v1
model: lfm2.5:8b
```

For another server, change `base_url` and `model` to values exposed by that
server. Use `commit-pilot --list-models` to get the exact model IDs.

## API keys

Commit Pilot reads the API key from:

```bash
export COMMIT_PILOT_OPENAI_COMPAT_API_KEY=...
```

Local Ollama and most local LM Studio servers do not need a key. OpenAI and
Unsloth Studio require one. Never put an API key in `config.yaml` or a
repository file.

## Verify the endpoint

The provider must expose these routes under the configured base URL:

```text
GET  /models
POST /chat/completions
```

Check the endpoint and model:

```bash
commit-pilot --doctor
commit-pilot --list-models
```

## Provider guides

- [Ollama](ollama.md)
- [LM Studio](lmstudio.md)
- [Unsloth Studio](unsloth.md)
- [OpenAI](openai.md)
