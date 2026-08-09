# LM Studio

LM Studio is a local, OpenAI-compatible server. Point the `openai_compat`
provider at it with a `base_url`.

## Install

```bash
brew install --cask lm-studio
```

Or download from [lmstudio.ai](https://lmstudio.ai).

## Download model

The script downloads the default model (`LiquidAI/LFM2.5-8B-A1B-GGUF`, the
same model as Ollama's `lfm2.5:8b`) and starts the server:

```bash
make setup-lmstudio
```

## Configure commit-pilot

The default config points `openai_compat` at Ollama. To use LM Studio instead, edit your config file
(`~/.config/commit-pilot/config.yaml`):

```yaml
provider: openai_compat
model: LiquidAI/LFM2.5-8B-A1B-GGUF
base_url: http://localhost:1234/v1
```

Check the server and model name:

```bash
commit-pilot --doctor
commit-pilot --list-models
```
