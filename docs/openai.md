# OpenAI

OpenAI is a hosted, OpenAI-compatible API. Point the `openai_compat` provider
at it with a `base_url` and an API key.

## Get an API key

Create an API key at [platform.openai.com](https://platform.openai.com).

Commit Pilot reads the API key from the `COMMIT_PILOT_OPENAI_COMPAT_API_KEY`
environment variable, never from a config file or the command line. That keeps
it out of shell history. Export it from a secret manager or your shell profile:

```bash
export COMMIT_PILOT_OPENAI_COMPAT_API_KEY=sk-...
```

## Configure commit-pilot

Ollama is the default provider. To use OpenAI instead, edit your config file
(`~/.config/commit-pilot/config.yaml`):

```yaml
provider: openai_compat
model: gpt-5.6-luna
base_url: https://api.openai.com/v1
```

`gpt-5.6-luna` is OpenAI's cost-efficient model in the GPT-5.6 family, built
for high-volume workloads. Use `gpt-5.6-terra` to balance intelligence and
cost, or `gpt-5.6-sol` for the flagship tier.

Check the connection and available models:

```bash
commit-pilot --doctor
commit-pilot --list-models
```

## Run commit-pilot

```bash
commit-pilot
```

Your diff goes to OpenAI's servers, so use a local provider (Ollama, LM Studio,
Unsloth Studio) when the code must stay on your machine.
