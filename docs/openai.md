# OpenAI

OpenAI provides a hosted OpenAI-compatible API. Commit Pilot sends the
configured diff to OpenAI when you use this setup.

## Create an API key

Create a key in the [OpenAI API dashboard](https://platform.openai.com/api-keys).

Commit Pilot reads the key from this environment variable only:

```bash
export COMMIT_PILOT_OPENAI_COMPAT_API_KEY=sk-...
```

Do not put the key in `config.yaml`, a repository file, or a command line.

## Select a model

The OpenAI model catalog lists these GPT-5.6 models:

- `gpt-5.6-sol` for complex reasoning and coding
- `gpt-5.6-terra` for a balance of capability and cost
- `gpt-5.6-luna` for cost-sensitive, high-volume work

This guide uses `gpt-5.6-luna`. Confirm that the model is available to your
OpenAI project before you run Commit Pilot.

## Configure Commit Pilot

Edit `~/.config/commit-pilot/config.yaml`:

```yaml
provider: openai_compat
base_url: https://api.openai.com/v1
model: gpt-5.6-luna
```

## Check the connection

```bash
commit-pilot --doctor
commit-pilot --list-models
```

`--doctor` checks the API and confirms that the selected model appears in the
model list. OpenAI may return an error if the model is not enabled for your
project.

## Run Commit Pilot

```bash
commit-pilot
```

Use [OpenAI's model catalog](https://platform.openai.com/docs/models) to choose
a different model.
