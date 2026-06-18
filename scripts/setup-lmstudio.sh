#!/bin/bash
set -euo pipefail

MODEL="gemma-4-e2b-it-qat"

if ! command -v lms &>/dev/null; then
  if command -v brew &>/dev/null; then
    echo "  ⏳ Installing LMStudio..."
    brew install --cask lm-studio
  else
    echo "  ✗ LM Studio CLI (lms) not found."
    echo "    Install from https://lmstudio.ai"
    exit 1
  fi
fi

if lms ls 2>/dev/null | grep -qi "$MODEL"; then
  echo "  ✓ $MODEL already downloaded"
else
  echo "  ⏳ Downloading $MODEL..."
  lms get "$MODEL" -y
  echo "  ✓ $MODEL downloaded"
fi

lms server start 2>/dev/null || true
echo "  ✓ LMStudio server ready"
