package lib

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nisrulz/commit-pilot/src/lib/provider"
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
	Model            string
	Provider         string
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
	JSON             bool
	Quiet            bool
	// ProviderExplicit reports that the user chose a provider by name, so the
	// startup probe must respect that name instead of identifying what runs at
	// the endpoint.
	ProviderExplicit bool
	// APIBaseExplicit reports that a non-default base URL was configured, so
	// the startup probe must identify which provider is running there.
	APIBaseExplicit bool
	// ConfigPath is the user config file that was loaded, if any.
	ConfigPath string
	// DefaultBranch and OutputFormat come from the untrusted repository config.
	DefaultBranch string
	OutputFormat  string

	contextSet      bool
	retriesSet      bool
	timeoutSet      bool
	conventionalSet bool
	imperativeSet   bool
}

const (
	EnvConfigDir    = "COMMIT_PILOT_CONFIG_DIR"
	EnvAPIKey       = "COMMIT_PILOT_OPENAI_COMPAT_API_KEY"
	LegacyEnvAPIKey = "COMMIT_PILOT_OPENAI_API_KEY"
)

const (
	RepoConfigFile    = ".commit-pilot.yaml"
	ToolDirName       = "commit-pilot"
	DefaultConfigFile = "config.yaml"

	// DefaultProviderName is used when no provider is configured. Ollama is the
	// default endpoint, exposed through the OpenAI-compatible provider.
	DefaultProviderName = "openai_compat"

	maxEnvAPIKeyLen = 512
	maxConfigSize   = 1 << 20 // 1 MiB
	configPerm      = 0o600

	defaultContextWindow = 65536
	minContextWindow     = 256
	maxContextWindow     = 2_000_000

	defaultRetries = 2
	defaultTimeout = 180 * time.Second

	DefaultModel     = "lfm2.5:8b"
	DefaultAPIBase   = "http://localhost:11434/v1"
	DefaultMaxTokens = 4096

	// defaultConfigYAML is written to the user config path on first run. It is
	// the source of truth for the tool's settings; the CLI ships no config of
	// its own.
	defaultConfigYAML = `# commit-pilot configuration (created on first run).
# Edit this file; it is the source of truth for the tool's settings.
provider: openai_compat
model: lfm2.5:8b
base_url: http://localhost:11434/v1
context_window: 65536
retries: 2
timeout_seconds: 180
conventional: true
imperative: true
mode: auto
`
)

// KnownProviders maps provider names to their default API base URLs.
var KnownProviders = map[string]string{
	"openai_compat": DefaultAPIBase,
}

// ProviderDefaults maps provider names to their default model identifiers.
var ProviderDefaults = map[string]string{
	"openai_compat": DefaultModel,
}

var knownOutputFormats = map[string]bool{
	"": true, "text": true, "json": true,
}

// ResolveConfig builds the effective runtime config from the config file, the
// repository config, flags, and built-in defaults.
//
// Precedence (highest first): command-line flags, the user config file, the
// untrusted repository config, then profile defaults.
//
// The user config file is resolved from (highest first): the --config flag, the
// COMMIT_PILOT_CONFIG_DIR env var, or ~/.config/commit-pilot/config.yaml. A
// missing file is only a hard error when the path came from --config; env and
// default misses create the file with the default values on first run, so a
// fresh install works before any config exists and the file is then the source
// of truth. The API key is read from COMMIT_PILOT_OPENAI_COMPAT_API_KEY, never from a
// config file or the command line.
func ResolveConfig(f RawFlags) (Config, error) {
	if f.Error != "" {
		return Config{}, fmt.Errorf("%s", f.Error)
	}

	path, source := resolveConfigFile(f.ConfigPath)
	if source != "flag" {
		if env := os.Getenv(EnvConfigDir); env != "" {
			if fi, err := os.Stat(env); err == nil && !fi.IsDir() {
				return Config{}, fmt.Errorf("%s must point at a directory, got file %q", EnvConfigDir, env)
			}
		}
		// create the tool subdir on first run so config and working files
		// have a home even before any config exists
		if path != "" {
			os.MkdirAll(filepath.Dir(path), 0o700)
		}
	}
	if isDir(path) {
		return Config{}, fmt.Errorf("config path %q is a directory, expected a file", path)
	}

	// The CLI ships no config. On first run the default config file is created
	// with the default values so the file is always present and is the source
	// of truth for the tool's settings.
	if path != "" && !fileExists(path) && source != "flag" {
		if err := os.WriteFile(path, []byte(defaultConfigYAML), configPerm); err != nil {
			return Config{}, fmt.Errorf("could not create config file %s: %v", path, err)
		}
	}

	var cfg Config
	var providerConfigured, baseConfigured string

	if repoPath := discoverRepoConfig(); repoPath != "" {
		rc, err := loadRepoConfig(repoPath)
		if err != nil {
			return Config{}, err
		}
		applyRepoConfig(&cfg, rc)
	}

	if path != "" && fileExists(path) {
		fc, err := loadUserConfig(path)
		if err != nil {
			return Config{}, err
		}
		providerConfigured = fc.Provider
		baseConfigured = fc.BaseURL
		cfg.ConfigPath = path
		applyFileConfig(&cfg, fc)
	} else if source == "flag" && path != "" {
		return Config{}, fmt.Errorf("config file %q not found", path)
	}

	applyFlags(&cfg, f)
	applyProfileDefaults(&cfg)

	cfg.ProviderExplicit = providerConfigured != ""
	cfg.APIBaseExplicit = baseConfigured != "" && baseConfigured != DefaultAPIBase

	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	apiKey, err := apiKeyFromEnv()
	if err != nil {
		return Config{}, err
	}
	cfg.APIKey = apiKey

	return cfg, nil
}

func validateConfig(c Config) error {
	if err := validateProvider(c.Provider); err != nil {
		return err
	}
	if err := provider.ValidateURL(c.APIBase); err != nil {
		return err
	}
	if err := validateMode(string(c.Mode)); err != nil {
		return err
	}
	if err := validateOutputFormat(c.OutputFormat); err != nil {
		return err
	}
	if c.contextSet {
		if err := validateContextWindow(c.ContextWindow); err != nil {
			return err
		}
	}
	if c.retriesSet && c.Retries < 0 {
		return fmt.Errorf("retries must be >= 0, got %d", c.Retries)
	}
	if c.timeoutSet && c.Timeout <= 0 {
		return fmt.Errorf("timeout_seconds must be > 0, got %d", int(c.Timeout.Seconds()))
	}
	if c.MaxSubjectLength > 0 && c.MaxSubjectLength < 10 {
		return fmt.Errorf("max_subject_length must be at least 10, got %d", c.MaxSubjectLength)
	}
	return nil
}

func validateProvider(p string) error {
	if p == "" {
		return nil
	}
	if !provider.Known(p) {
		return fmt.Errorf("unknown provider %q (known: %s)", p, strings.Join(provider.Names(), ", "))
	}
	return nil
}

func validateContextWindow(n int) error {
	if n < minContextWindow || n > maxContextWindow {
		return fmt.Errorf("context_window %d out of bounds (%d-%d)", n, minContextWindow, maxContextWindow)
	}
	return nil
}

func validateMode(m string) error {
	if m == "" || m == string(ModeAuto) || m == string(ModeSingle) || m == "auto" || m == "single" {
		return nil
	}
	return fmt.Errorf("invalid mode %q (want auto or single)", m)
}

func validateOutputFormat(f string) error {
	if !knownOutputFormats[f] {
		return fmt.Errorf("invalid output_format %q (want text or json)", f)
	}
	return nil
}
