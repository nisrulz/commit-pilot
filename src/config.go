package main

import (
	"fmt"
	"os"
)

type Mode string

const (
	ModeAuto   Mode = ""
	ModeSingle Mode = "1"
)

type Config struct {
	Model   string
	APIBase string
	APIKey  string
	DryRun  bool
	Mode    Mode
	Prompt  string
}

var knownProviders = map[string]string{
	"ollama":   "http://localhost:11434/v1",
	"lmstudio": "http://localhost:1234/v1",
	"openai":   "https://api.openai.com/v1",
}

var providerDefaults = map[string]string{
	"ollama":   "gemma4:e2b-it-qat",
	"lmstudio": "gemma-4-e2b-it-qat",
	"openai":   "gpt-4o-mini",
}

type rawFlags struct {
	Mode   string
	DryRun bool
}

func parseArgs(args []string) (rawFlags, bool) {
	var f rawFlags

	if len(args) > 0 && args[0] == "1" {
		f.Mode = "1"
		args = args[1:]
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			f.DryRun = true
		case "-h", "--help":
			return f, true
		}
	}

	return f, false
}

const (
	maxEnvModelLen   = 256
	maxEnvAPIBaseLen = 2048
	maxEnvAPIKeyLen  = 512
)

func resolveConfig(f rawFlags) Config {
	model := os.Getenv("OPENAI_MODEL")
	if len(model) > maxEnvModelLen {
		model = model[:maxEnvModelLen]
	}
	apiBase := os.Getenv("OPENAI_BASE_URL")
	if len(apiBase) > maxEnvAPIBaseLen {
		apiBase = apiBase[:maxEnvAPIBaseLen]
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if len(apiKey) > maxEnvAPIKeyLen {
		apiKey = apiKey[:maxEnvAPIKeyLen]
	}
	provider := os.Getenv("OPENAI_PROVIDER")

	if provider != "" {
		if apiBase == "" {
			apiBase = knownProviders[provider]
		}
		if model == "" {
			model = providerDefaults[provider]
		}
	}

	if model == "" {
		model = defaultModel
	}
	if apiBase == "" {
		apiBase = defaultAPIBase
	}

	prompt := os.Getenv("COMMIT_PILOT_PROMPT")
	if p := os.Getenv("COMMIT_PILOT_PROMPT_FILE"); p != "" {
		data, err := os.ReadFile(p)
		if err == nil {
			prompt = string(data)
		}
	}

	return Config{
		Model:   model,
		APIBase: apiBase,
		APIKey:  apiKey,
		DryRun:  f.DryRun,
		Mode:    Mode(f.Mode),
		Prompt:  prompt,
	}
}

func printHelp() {
	fmt.Print(`commit-pilot: AI-powered git commit messages that know what you changed.

Usage:
  commit-pilot                           # auto-chunk into logical commits
  commit-pilot 1                         # one commit for all changes
  commit-pilot --dry-run                 # preview only

Environment variables:
  OPENAI_PROVIDER         Provider: ollama, lmstudio, openai
  OPENAI_MODEL            Model name (default: gemma-4-e2b-it-qat)
  OPENAI_BASE_URL         API base URL
  OPENAI_API_KEY          API key
  COMMIT_PILOT_PROMPT     Custom prompt text (overrides default)
  COMMIT_PILOT_PROMPT_FILE Path to custom prompt file (overrides default)
`)
}
