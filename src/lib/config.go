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

// Mode selects how changes are turned into commits.
type Mode string

const (
	ModeAuto   Mode = ""
	ModeSingle Mode = "1"
)

// Config is the fully resolved runtime configuration for a single run. Every
// value is filled in by ResolveConfig before a run starts.
type Config struct {
	Model             string
	Provider          string
	APIBase           string
	APIKey            string
	DryRun            bool
	Cleanup           bool
	Yes               bool
	Mode              Mode
	Prompt            string
	ContextWindow     int
	AutoContextWindow bool
	Retries           int
	Timeout           time.Duration
	Scope             ChangeScope
	PlanOut           string
	Apply             string
	PlanLint          string
	Include           []string
	Exclude           []string
	IncludeSensitive  bool
	Conventional      bool
	TicketPrefix      string
	Imperative        bool
	MaxSubjectLength  int
	BodyStyle         string
	Context           context.Context
	HTTPClient        HTTPDoer
	Input             io.Reader
	Output            io.Writer
	JSON              bool
	Quiet             bool
}

// KnownProviders maps provider names to their default API base URLs.
var KnownProviders = map[string]string{
	"ollama":   "http://localhost:11434/v1",
	"lmstudio": "http://localhost:1234/v1",
	"openai":   "https://api.openai.com/v1",
	"unsloth":  "http://localhost:8888/v1",
}

// ProviderDefaults maps provider names to their default model identifiers.
var ProviderDefaults = map[string]string{
	"ollama":   "gemma4:e2b-it-qat",
	"lmstudio": "gemma-4-e2b-it-qat",
	"openai":   "gpt-4o-mini",
	"unsloth":  "unsloth/gemma-4-E4B-it-qat-GGUF",
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

// ConfigDir returns the directory that holds the user configuration file.
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

// TmpDir returns the directory where per-run summaries are written.
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

// ConfigDefaults loads provider and message-preference defaults from the user
// config file, overlaid with any project-level .commit-pilot/config.env.
// The user config file is created with the built-in defaults when missing.
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
				if isProjectPreference(key) {
					values[key] = value
				} else {
					fmt.Fprintf(os.Stderr, "  ! ignoring non-message setting %s from project config\n", key)
				}
			}
		}
	}
	return values
}

func isProjectPreference(key string) bool {
	switch key {
	case "COMMIT_PILOT_CONVENTIONAL_COMMITS", "COMMIT_PILOT_TICKET_PREFIX", "COMMIT_PILOT_IMPERATIVE_TONE", "COMMIT_PILOT_MAX_SUBJECT_LENGTH", "COMMIT_PILOT_BODY_STYLE":
		return true
	default:
		return false
	}
}

// readConfigFile parses a KEY=VALUE config file, skipping comments and empty
// lines and rejecting unknown or invalid entries.
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
			fmt.Fprintln(os.Stderr, "  ! ignoring invalid config line")
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "OPENAI_PROVIDER", "OPENAI_MODEL", "OPENAI_BASE_URL", "COMMIT_PILOT_CONVENTIONAL_COMMITS", "COMMIT_PILOT_TICKET_PREFIX", "COMMIT_PILOT_IMPERATIVE_TONE", "COMMIT_PILOT_MAX_SUBJECT_LENGTH", "COMMIT_PILOT_BODY_STYLE":
			if !validConfigValue(key, value) {
				fmt.Fprintf(os.Stderr, "  ! ignoring invalid config value for %s\n", key)
				continue
			}
			values[key] = value
		default:
			fmt.Fprintf(os.Stderr, "  ! ignoring unsupported config key: %s\n", key)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "  ! could not read config file: %v\n", err)
	}
	return values
}

// validConfigValue reports whether a config value passes the type checks for
// its key (booleans parse, subject lengths are positive).
func validConfigValue(key, value string) bool {
	switch key {
	case "COMMIT_PILOT_CONVENTIONAL_COMMITS", "COMMIT_PILOT_IMPERATIVE_TONE":
		_, err := strconv.ParseBool(value)
		return err == nil
	case "COMMIT_PILOT_MAX_SUBJECT_LENGTH":
		length, err := strconv.Atoi(value)
		return err == nil && length > 0
	default:
		return true
	}
}

// configValue returns the value for a setting, preferring the environment over
// the config file defaults.
func configValue(name string, defaults map[string]string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaults[name]
}

// ResolveConfig builds the effective configuration from command-line flags,
// environment variables, and the config file, applying defaults for anything
// left unset.
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
	autoContextWindow := provider == "lmstudio"
	if cw := os.Getenv("COMMIT_PILOT_CONTEXT_WINDOW"); cw != "" {
		autoContextWindow = false
		if v, err := strconv.Atoi(cw); err == nil && v > 0 {
			contextWindow = v
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
		Model:             model,
		Provider:          provider,
		APIBase:           apiBase,
		APIKey:            apiKey,
		DryRun:            f.DryRun,
		Cleanup:           f.Cleanup,
		Yes:               f.Yes,
		Mode:              Mode(f.Mode),
		Prompt:            prompt,
		ContextWindow:     contextWindow,
		AutoContextWindow: autoContextWindow,
		Retries:           retries,
		Timeout:           timeout,
		Scope:             scope,
		PlanOut:           f.PlanOut,
		Apply:             f.Apply,
		PlanLint:          f.PlanLint,
		Include:           f.Include,
		Exclude:           f.Exclude,
		IncludeSensitive:  f.IncludeSensitive,
		Conventional:      configBool("COMMIT_PILOT_CONVENTIONAL_COMMITS", defaults, true),
		TicketPrefix:      configValue("COMMIT_PILOT_TICKET_PREFIX", defaults),
		Imperative:        configBool("COMMIT_PILOT_IMPERATIVE_TONE", defaults, true),
		MaxSubjectLength:  maxSubjectLength,
		BodyStyle:         configValue("COMMIT_PILOT_BODY_STYLE", defaults),
		Context:           context.Background(),
		Input:             os.Stdin,
		Output:            os.Stdout,
		JSON:              f.JSON,
		Quiet:             f.Quiet,
	}
}

// configBool resolves a boolean setting with a fallback for unset values.
func configBool(name string, defaults map[string]string, fallback bool) bool {
	value := configValue(name, defaults)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
