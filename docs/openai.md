# OpenAI

## Prerequisites

An [OpenAI API key](https://platform.openai.com/api-keys) with access to `gpt-4o-mini` (or another model).

## Run commit-pilot

```bash
OPENAI_PROVIDER=openai \
  OPENAI_API_KEY=sk-proj-... \
  OPENAI_MODEL=gpt-4o-mini \
  commit-pilot
```

Verify the selected model before committing:

```bash
OPENAI_PROVIDER=openai OPENAI_API_KEY=sk-proj-... commit-pilot --doctor
```
