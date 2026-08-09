# Ollama

Commit Pilot uses Ollama through its OpenAI-compatible API. The generated
config points to Ollama by default:

```yaml
provider: openai_compat
base_url: http://localhost:11434/v1
model: lfm2.5:8b
```

Ollama's OpenAI-compatible API uses the `/v1/chat/completions` and `/v1/models`
routes. Ollama does not require an API key for a local server.

## Install Ollama

Install Ollama from [ollama.com/download](https://ollama.com/download). On
macOS with Homebrew:

```bash
brew install ollama
```

## Start Ollama

```bash
ollama serve
```

Keep the server running while you use Commit Pilot.

## Download a model

Pull the default model (`lfm2.5:8b`) from the
[Ollama library](https://ollama.com/library/lfm2.5):

```bash
ollama pull lfm2.5:8b
ollama ls
```

The value in `model` must match the model name shown by `ollama ls`.

## Check the connection

```bash
commit-pilot --doctor
commit-pilot --list-models
```

`--doctor` checks the server and confirms that the configured model appears in
the model list.

## Use another Ollama model

Edit `~/.config/commit-pilot/config.yaml`:

```yaml
provider: openai_compat
base_url: http://localhost:11434/v1
model: qwen3:8b
```

Pull the model before you run Commit Pilot:

```bash
ollama pull qwen3:8b
```

## Run Commit Pilot

```bash
commit-pilot
```
