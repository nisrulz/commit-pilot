package lib

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Mode string

const (
	ModeAuto   Mode = ""
	ModeSingle Mode = "1"
)

type Config struct {
	Model            string
	APIBase          string
	APIKey           string
	DryRun           bool
	Cleanup          bool
	Yes              bool
	Mode             Mode
	Prompt           string
	ContextWindow    int
	Retries          int
	Timeout          time.Duration
	MaxFilesGroup    int
	Scope            ChangeScope
	PlanOut          string
	Apply            string
	PlanLint         string
	Include          []string
	Exclude          []string
	IncludeSensitive bool
	Conventional     bool
	TicketPrefix     string
	Imperative       bool
	MaxSubjectLength int
	BodyStyle        string
	Context          context.Context
	HTTPClient       HTTPDoer
	Input            io.Reader
	Output           io.Writer
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
	Mode             string
	DryRun           bool
	Cleanup          bool
	Yes              bool
	Doctor           bool
	ListModels       bool
	Staged           bool
	Unstaged         bool
	PlanOut          string
	Apply            string
	PlanLint         string
	Include          []string
	Exclude          []string
	IncludeSensitive bool
}

func ParseArgs(args []string) (RawFlags, bool) {
	var f RawFlags
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--dry-run", "--no-commit":
			f.DryRun = true
		case "--cleanup":
			f.Cleanup = true
		case "--single":
			f.Mode = "1"
		case "--yes":
			f.Yes = true
		case "--doctor":
			f.Doctor = true
		case "--list-models":
			f.ListModels = true
		case "--staged":
			f.Staged = true
		case "--unstaged":
			f.Unstaged = true
		case "--plan-out":
			if i+1 < len(args) {
				i++
				f.PlanOut = args[i]
			}
		case "--apply":
			if i+1 < len(args) {
				i++
				f.Apply = args[i]
			}
		case "--plan-lint":
			if i+1 < len(args) {
				i++
				f.PlanLint = args[i]
			}
		case "--include":
			if i+1 < len(args) {
				i++
				f.Include = append(f.Include, args[i])
			}
		case "--exclude":
			if i+1 < len(args) {
				i++
				f.Exclude = append(f.Exclude, args[i])
			}
		case "--include-sensitive":
			f.IncludeSensitive = true
		case "-h", "--help":
			return f, true
		}
	}
	return f, false
}

const (
	ConfigDirEnv          = "COMMIT_PILOT_CONFIG_DIR"
	TmpDirEnv             = "COMMIT_PILOT_TMP_DIR"
	configFileName        = "config.env"
	defaultConfigContents = "# Commit Pilot provider defaults\n" +
		"OPENAI_PROVIDER=lmstudio\n" +
		"OPENAI_MODEL=gemma-4-e2b-it-qat\n" +
		"OPENAI_BASE_URL=http://localhost:1234/v1\n"
	maxEnvModelLen       = 256
	maxEnvAPIBaseLen     = 2048
	maxEnvAPIKeyLen      = 512
	defaultContextWindow = 65536
	DefaultModel         = "gemma-4-e2b-it-qat"
	DefaultAPIBase       = "http://localhost:1234/v1"
	DefaultMaxTokens     = 4096
	defaultRetries       = 2
	defaultTimeout       = 180 * time.Second
)

func ConfigDir() string {
	if dir := os.Getenv(ConfigDirEnv); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "commit-pilot")
	}
	return filepath.Join(home, ".config", "commit-pilot")
}

func TmpDir() string {
	if dir := os.Getenv(TmpDirEnv); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".commit-pilot/tmp"
	}
	return filepath.Join(home, ".commit-pilot", "tmp")
}

func ConfigDefaults() map[string]string {
	path := filepath.Join(ConfigDir(), configFileName)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		err = os.MkdirAll(ConfigDir(), 0700)
		if err == nil {
			file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err == nil {
				_, err = file.WriteString(defaultConfigContents)
				file.Close()
				if err == nil {
					file, err = os.Open(path)
				}
			} else if os.IsExist(err) {
				file, err = os.Open(path)
			}
		}
	}
	if err != nil {
		return nil
	}
	values := readConfigFile(file)
	if root, err := GitRun("rev-parse", "--show-toplevel"); err == nil {
		projectPath := filepath.Join(strings.TrimSpace(root), ".commit-pilot", configFileName)
		if project, err := os.Open(projectPath); err == nil {
			for key, value := range readConfigFile(project) {
				values[key] = value
			}
		}
	}
	return values
}

func readConfigFile(file *os.File) map[string]string {
	defer file.Close()
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "  ! ignoring invalid config line: %s\n", line)
			continue
		}
		switch key = strings.TrimSpace(key); key {
		case "OPENAI_PROVIDER", "OPENAI_MODEL", "OPENAI_BASE_URL", "COMMIT_PILOT_CONVENTIONAL_COMMITS", "COMMIT_PILOT_TICKET_PREFIX", "COMMIT_PILOT_IMPERATIVE_TONE", "COMMIT_PILOT_MAX_SUBJECT_LENGTH", "COMMIT_PILOT_BODY_STYLE":
			values[key] = strings.TrimSpace(value)
		default:
			fmt.Fprintf(os.Stderr, "  ! ignoring unsupported config key: %s\n", key)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "  ! could not read config file: %v\n", err)
	}
	return values
}

func configValue(name string, defaults map[string]string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaults[name]
}

func ResolveConfig(f RawFlags) Config {
	defaults := ConfigDefaults()
	model := configValue("OPENAI_MODEL", defaults)
	if len(model) > maxEnvModelLen {
		model = model[:maxEnvModelLen]
	}
	apiBase := configValue("OPENAI_BASE_URL", defaults)
	if len(apiBase) > maxEnvAPIBaseLen {
		apiBase = apiBase[:maxEnvAPIBaseLen]
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if len(apiKey) > maxEnvAPIKeyLen {
		apiKey = apiKey[:maxEnvAPIKeyLen]
	}
	provider := configValue("OPENAI_PROVIDER", defaults)
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

	retries := defaultRetries
	if value, err := strconv.Atoi(os.Getenv("COMMIT_PILOT_RETRIES")); err == nil && value >= 0 {
		retries = value
	}
	timeout := defaultTimeout
	if value, err := strconv.Atoi(os.Getenv("COMMIT_PILOT_TIMEOUT_SECONDS")); err == nil && value > 0 {
		timeout = time.Duration(value) * time.Second
	}
	maxFilesGroup := MaxFilesPerGroup
	if value, err := strconv.Atoi(os.Getenv("COMMIT_PILOT_MAX_FILES_PER_GROUP")); err == nil && value >= 0 {
		maxFilesGroup = value
	}
	maxSubjectLength := MaxSubjectLength
	if value, err := strconv.Atoi(configValue("COMMIT_PILOT_MAX_SUBJECT_LENGTH", defaults)); err == nil && value > 0 {
		maxSubjectLength = value
	}
	scope := ScopeAuto
	if f.Staged {
		scope = ScopeStaged
	} else if f.Unstaged {
		scope = ScopeUnstaged
	}

	return Config{
		Model:            model,
		APIBase:          apiBase,
		APIKey:           apiKey,
		DryRun:           f.DryRun,
		Cleanup:          f.Cleanup,
		Yes:              f.Yes,
		Mode:             Mode(f.Mode),
		Prompt:           prompt,
		ContextWindow:    contextWindow,
		Retries:          retries,
		Timeout:          timeout,
		MaxFilesGroup:    maxFilesGroup,
		Scope:            scope,
		PlanOut:          f.PlanOut,
		Apply:            f.Apply,
		PlanLint:         f.PlanLint,
		Include:          f.Include,
		Exclude:          f.Exclude,
		IncludeSensitive: f.IncludeSensitive,
		Conventional:     configBool("COMMIT_PILOT_CONVENTIONAL_COMMITS", defaults, true),
		TicketPrefix:     configValue("COMMIT_PILOT_TICKET_PREFIX", defaults),
		Imperative:       configBool("COMMIT_PILOT_IMPERATIVE_TONE", defaults, true),
		MaxSubjectLength: maxSubjectLength,
		BodyStyle:        configValue("COMMIT_PILOT_BODY_STYLE", defaults),
		Context:          context.Background(),
		Input:            os.Stdin,
		Output:           os.Stdout,
	}
}

func configBool(name string, defaults map[string]string, fallback bool) bool {
	value := configValue(name, defaults)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func printHelp() {
	fmt.Print(`commit-pilot: AI-powered git commit messages that know what you changed.

Usage:
  commit-pilot                           # auto-chunk into logical commits
  commit-pilot --single                  # one commit for all changes
  commit-pilot --dry-run                 # preview only
  commit-pilot --no-commit               # preview only (alias for --dry-run)
  commit-pilot --yes                     # apply proposed commits without prompting
  commit-pilot --doctor                  # check Git and provider setup
  commit-pilot --list-models             # list models available from the provider
  commit-pilot --staged                  # use staged changes only
  commit-pilot --unstaged                # ignore staged changes
  commit-pilot --plan-out <path>         # save generated plan as JSON
  commit-pilot --apply <path>            # apply an edited JSON plan
  commit-pilot --plan-lint <path>        # validate a saved JSON plan
  commit-pilot --include <glob>          # include matching files only
  commit-pilot --exclude <glob>          # exclude matching files
  commit-pilot --include-sensitive       # allow sensitive-looking files to reach the model
  commit-pilot --cleanup                 # remove temp files on success

Environment variables:
  OPENAI_PROVIDER              Provider: ollama, lmstudio, openai, unsloth
  OPENAI_MODEL                 Model name (default: gemma-4-e2b-it-qat)
  OPENAI_BASE_URL              API base URL
  OPENAI_API_KEY               API key
  COMMIT_PILOT_PROMPT          Custom prompt text (overrides default)
  COMMIT_PILOT_PROMPT_FILE     Path to custom prompt file (overrides default)
  COMMIT_PILOT_CONTEXT_WINDOW  Model context window size in tokens (default: 65536)
  COMMIT_PILOT_RETRIES         Retries for transient provider failures (default: 2)
  COMMIT_PILOT_TIMEOUT_SECONDS Provider request timeout in seconds (default: 180)
  COMMIT_PILOT_MAX_FILES_PER_GROUP  Maximum files per generated commit, 0 disables the cap (default: 3)
  COMMIT_PILOT_CONFIG_DIR      Directory for configuration (default: ~/.config/commit-pilot)
  COMMIT_PILOT_TMP_DIR         Directory for temporary summaries (default: ~/.commit-pilot/tmp)

Config file:
  $COMMIT_PILOT_CONFIG_DIR/config.env    Optional provider/model defaults
`)
}
