# Ollama

Ollama is the default provider and requires no configuration.

## Install

```bash
brew install ollama
```

Or download from [ollama.com](https://ollama.com).

## Download model

The script downloads the default model:

```bash
make setup-ollama
```

## Serve

```bash
ollama serve
```

## Configure commit-pilot

Ollama is the default provider (`ollama`, model `lfm2.5:8b`, which is Liquid
AI's LFM2.5-8B-A1B), so a fresh install works out of the box. Your config file
(`~/.config/commit-pilot/config.yaml`) already carries the defaults; add or
edit these values to change them:

```yaml
provider: ollama
model: lfm2.5:8b
```

Check the server and available models:

```bash
commit-pilot --doctor
commit-pilot --list-models
```

Custom API base:

```yaml
provider: ollama
model: lfm2.5:8b
base_url: http://localhost:11434/v1
```

## Run commit-pilot

```bash
commit-pilot
```
