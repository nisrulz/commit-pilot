# OpenAI-compatible providers

`openai_compat` targets any server that exposes an OpenAI-compatible
`/chat/completions` and `/models` API. That includes hosted endpoints such as
OpenAI and local servers such as [LM Studio](lmstudio.md) and
[Unsloth Studio](unsloth.md). The default `base_url` points at Ollama.

## Configure commit-pilot

Commit Pilot reads the API key from the `COMMIT_PILOT_OPENAI_COMPAT_API_KEY`
environment variable, never from a config file or the command line. That keeps
it out of shell history. Export it from a secret manager or your shell profile:

```bash
export COMMIT_PILOT_OPENAI_COMPAT_API_KEY=sk-...
```

Add to your config file (`~/.config/commit-pilot/config.yaml`):

```yaml
provider: openai_compat
base_url: https://api.openai.com/v1
model: gpt-5.6-luna
```

## Run commit-pilot

```bash
commit-pilot
```

Verify the selected model before committing:

```bash
commit-pilot --doctor
```

## Examples

- [OpenAI](openai.md) is the hosted example
- [LM Studio](lmstudio.md) and [Unsloth Studio](unsloth.md) are two local
  examples of the same setup
