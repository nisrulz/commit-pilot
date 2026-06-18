#!/bin/bash
set -euo pipefail

MODEL="gemma4:e2b-it-qat"

if ! command -v ollama &>/dev/null; then
  if command -v brew &>/dev/null; then
    echo "  ⏳ Installing Ollama..."
    brew install ollama
  else
    echo "  ✗ Ollama not found."
    echo "    Install from https://ollama.com"
    exit 1
  fi
fi

if ollama list 2>/dev/null | grep -qi "$MODEL"; then
  echo "  ✓ $MODEL already downloaded"
else
  echo "  ⏳ Downloading $MODEL..."
  ollama pull "$MODEL"
  echo "  ✓ $MODEL downloaded"
fi

echo "  ✓ Ollama ready"
echo "  ➜ Run: ollama serve"
