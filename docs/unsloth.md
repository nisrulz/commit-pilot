# Unsloth Studio

## Install

Follow the instructions at [unsloth.ai](https://unsloth.ai) to install Unsloth Studio.

## Load a model

```bash
unsloth run --model unsloth/gemma-4-E4B-it-qat-GGUF -p 8888
```

Or open the Unsloth Studio UI, load a GGUF model via New Chat, and create an API key from **Settings → API**.

## Run commit-pilot

```bash
OPENAI_PROVIDER=unsloth \
  OPENAI_API_KEY=sk-unsloth-... \
  commit-pilot
```

Check the server and available model names:

```bash
OPENAI_PROVIDER=unsloth OPENAI_API_KEY=sk-unsloth-... commit-pilot --doctor
OPENAI_PROVIDER=unsloth OPENAI_API_KEY=sk-unsloth-... commit-pilot --list-models
```

The default model is `unsloth/gemma-4-E4B-it-qat-GGUF`. To use a different model name, set it explicitly:

```bash
OPENAI_PROVIDER=unsloth \
  OPENAI_MODEL=your-model-name \
  OPENAI_API_KEY=sk-unsloth-... \
  commit-pilot
```

Custom API base:

```bash
OPENAI_BASE_URL=http://localhost:8000/v1 \
  OPENAI_API_KEY=sk-unsloth-... \
  commit-pilot
```
