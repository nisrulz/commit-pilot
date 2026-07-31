# LMStudio

## Install

```bash
brew install --cask lm-studio
```

Or download from [lmstudio.ai](https://lmstudio.ai).

## Download model

```bash
lms get gemma-4-e2b-it-qat -y
lms server start
```

## Run commit-pilot

This is the default provider, so you don't need any env vars:

```bash
commit-pilot
```

Explicitly:

```bash
OPENAI_PROVIDER=lmstudio OPENAI_MODEL=gemma-4-e2b-it-qat commit-pilot
```

Check the server and model name:

```bash
commit-pilot --doctor
commit-pilot --list-models
```

Custom API base:

```bash
OPENAI_BASE_URL=http://localhost:1234/v1 \
  OPENAI_MODEL=gemma-4-e2b-it-qat \
  commit-pilot
```
