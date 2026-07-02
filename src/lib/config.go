package lib

import (
	"fmt"
	"os"
	"strconv"
)

type Mode string

const (
	ModeAuto   Mode = ""
	ModeSingle Mode = "1"
)

type Config struct {
	Model         string
	APIBase       string
	APIKey        string
	DryRun        bool
	Mode          Mode
	Prompt        string
	ContextWindow int
}

var KnownProviders = map[string]string{
	"ollama":   "http://localhost:11434/v1",
	"lmstudio": "http://localhost:1234/v1",
	"openai":   "https://api.openai.com/v1",
	"unsloth":  "http://localhost:8888/v1",
}

var ProviderDefaults = map[string]string{
	"ollama":   "gemma4:e2b-it-qat",
	"lmstudio": "gemma-4-e2b-it-qat",
	"openai":   "gpt-4o-mini",
	"unsloth":  "unsloth/gemma-4-E4B-it-qat-GGUF",
}

type RawFlags struct {
	Mode   string
	DryRun bool
}

func ParseArgs(args []string) (RawFlags, bool) {
	var f RawFlags
	if len(args) > 0 && args[0] == "1" {
		f.Mode = "1"
		args = args[1:]
	}
	for _, a := range args {
		switch a {
		case "--dry-run":
			f.DryRun = true
		case "-h", "--help":
			return f, true
		}
	}
	return f, false
}

const (
	maxEnvModelLen       = 256
	maxEnvAPIBaseLen     = 2048
	maxEnvAPIKeyLen      = 512
	defaultContextWindow = 65536
	DefaultModel         = "gemma-4-e2b-it-qat"
	DefaultAPIBase       = "http://localhost:1234/v1"
	DefaultMaxTokens     = 4096
)

func ResolveConfig(f RawFlags) Config {
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
	if provider == "" && apiBase == "" {
		provider = "lmstudio"
	}
	if provider != "" {
		if apiBase == "" {
			apiBase = KnownProviders[provider]
		}
		if model == "" {
			model = ProviderDefaults[provider]
		}
	}

	if model == "" {
		model = DefaultModel
	}
	if apiBase == "" {
		apiBase = DefaultAPIBase
	}

	prompt := os.Getenv("COMMIT_PILOT_PROMPT")
	if p := os.Getenv("COMMIT_PILOT_PROMPT_FILE"); p != "" {
		data, err := os.ReadFile(p)
		if err == nil {
			prompt = string(data)
		}
	}

	contextWindow := defaultContextWindow
	if cw := os.Getenv("COMMIT_PILOT_CONTEXT_WINDOW"); cw != "" {
		if v, err := strconv.Atoi(cw); err == nil && v > 0 {
			contextWindow = v
		}
	} else if provider == "lmstudio" {
		d := DetectContextWindow(apiBase)
		if d > 0 {
			contextWindow = d
		}
	}

	return Config{
		Model:         model,
		APIBase:       apiBase,
		APIKey:        apiKey,
		DryRun:        f.DryRun,
		Mode:          Mode(f.Mode),
		Prompt:        prompt,
		ContextWindow: contextWindow,
	}
}

func printHelp() {
	fmt.Print(`commit-pilot: AI-powered git commit messages that know what you changed.

Usage:
  commit-pilot                           # auto-chunk into logical commits
  commit-pilot 1                         # one commit for all changes
  commit-pilot --dry-run                 # preview only

Environment variables:
  OPENAI_PROVIDER              Provider: ollama, lmstudio, openai, unsloth
  OPENAI_MODEL                 Model name (default: gemma-4-e2b-it-qat)
  OPENAI_BASE_URL              API base URL
  OPENAI_API_KEY               API key
  COMMIT_PILOT_PROMPT          Custom prompt text (overrides default)
  COMMIT_PILOT_PROMPT_FILE     Path to custom prompt file (overrides default)
  COMMIT_PILOT_CONTEXT_WINDOW  Model context window size in tokens (default: 65536)
`)
}
