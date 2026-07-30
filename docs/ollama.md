# Ollama

## Install

```bash
brew install ollama
```

Or download from [ollama.com](https://ollama.com).

## Download model

```bash
ollama pull gemma4:e2b-it-qat
```

## Serve

```bash
ollama serve
```

## Run commit-pilot

```bash
OPENAI_PROVIDER=ollama OPENAI_MODEL=gemma4:e2b-it-qat commit-pilot
```

Check the server and available models:

```bash
OPENAI_PROVIDER=ollama commit-pilot --doctor
OPENAI_PROVIDER=ollama commit-pilot --list-models
```

Custom API base:

```bash
OPENAI_BASE_URL=http://localhost:11434/v1 \
  OPENAI_MODEL=gemma4:e2b-it-qat \
  commit-pilot
```
