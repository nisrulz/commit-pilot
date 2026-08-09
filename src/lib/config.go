package lib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nisrulz/commit-pilot/src/lib/provider"
	"gopkg.in/yaml.v3"
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

// fileConfig mirrors the YAML config file schema. Values not present in the
// file are left nil/empty and fall back to profile defaults.
type fileConfig struct {
	Provider         string `yaml:"provider"`
	Model            string `yaml:"model"`
	BaseURL          string `yaml:"base_url"`
	ContextWindow    *int   `yaml:"context_window"`
	Prompt           string `yaml:"prompt"`
	Mode             string `yaml:"mode"`
	DryRun           *bool  `yaml:"dry_run"`
	Cleanup          *bool  `yaml:"cleanup"`
	Retries          *int   `yaml:"retries"`
	TimeoutSeconds   *int   `yaml:"timeout_seconds"`
	Conventional     *bool  `yaml:"conventional"`
	TicketPrefix     string `yaml:"ticket_prefix"`
	Imperative       *bool  `yaml:"imperative"`
	MaxSubjectLength *int   `yaml:"max_subject_length"`
	BodyStyle        string `yaml:"body_style"`
	DefaultBranch    string `yaml:"default_branch"`
	OutputFormat     string `yaml:"output_format"`
}

// repoConfig mirrors the untrusted repository config, which may only carry
// commit-message style preferences and output defaults.
type repoConfig struct {
	Conventional     *bool  `yaml:"conventional"`
	TicketPrefix     string `yaml:"ticket_prefix"`
	Imperative       *bool  `yaml:"imperative"`
	MaxSubjectLength *int   `yaml:"max_subject_length"`
	BodyStyle        string `yaml:"body_style"`
	DefaultBranch    string `yaml:"default_branch"`
	OutputFormat     string `yaml:"output_format"`
}

// repoAllowlist is the only set of keys an untrusted repository config may set.
// Everything else (provider, model, base_url, api_key, ...) is rejected so a
// cloned repository cannot redirect the endpoint or secrets.
var repoAllowlist = map[string]bool{
	"conventional":       true,
	"ticket_prefix":      true,
	"imperative":         true,
	"max_subject_length": true,
	"body_style":         true,
	"default_branch":     true,
	"output_format":      true,
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

// configBase returns the base directory that holds the tool's own subdir.
// COMMIT_PILOT_CONFIG_DIR defaults to ~/.config (the platform config dir).
func configBase() string {
	if env := os.Getenv(EnvConfigDir); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config")
}

// configDir is where the CLI keeps the config file plus its own working files
// (e.g. tmp summaries). It is created on first run.
func configDir() string {
	base := configBase()
	if base == "" {
		return ""
	}
	return filepath.Join(base, ToolDirName)
}

func resolveConfigFile(flagPath string) (string, string) {
	if flagPath != "" {
		return flagPath, "flag"
	}
	return filepath.Join(configDir(), DefaultConfigFile), "env/default"
}

func applyFileConfig(c *Config, fc fileConfig) {
	if fc.Provider != "" {
		c.Provider = fc.Provider
	}
	if fc.Model != "" {
		c.Model = fc.Model
	}
	if fc.BaseURL != "" {
		c.APIBase = fc.BaseURL
	}
	if fc.ContextWindow != nil {
		c.ContextWindow = *fc.ContextWindow
		c.contextSet = true
	}
	if fc.Prompt != "" {
		c.Prompt = fc.Prompt
	}
	if fc.Mode != "" {
		switch fc.Mode {
		case "single":
			c.Mode = ModeSingle
		case "auto":
			c.Mode = ModeAuto
		default:
			c.Mode = Mode(fc.Mode)
		}
	}
	if fc.DryRun != nil {
		c.DryRun = *fc.DryRun
	}
	if fc.Cleanup != nil {
		c.Cleanup = *fc.Cleanup
	}
	if fc.Retries != nil {
		c.Retries = *fc.Retries
		c.retriesSet = true
	}
	if fc.TimeoutSeconds != nil {
		c.Timeout = time.Duration(*fc.TimeoutSeconds) * time.Second
		c.timeoutSet = true
	}
	if fc.Conventional != nil {
		c.Conventional = *fc.Conventional
		c.conventionalSet = true
	}
	if fc.TicketPrefix != "" {
		c.TicketPrefix = fc.TicketPrefix
	}
	if fc.Imperative != nil {
		c.Imperative = *fc.Imperative
		c.imperativeSet = true
	}
	if fc.MaxSubjectLength != nil {
		c.MaxSubjectLength = *fc.MaxSubjectLength
	}
	if fc.BodyStyle != "" {
		c.BodyStyle = fc.BodyStyle
	}
	if fc.DefaultBranch != "" {
		c.DefaultBranch = fc.DefaultBranch
	}
	if fc.OutputFormat != "" {
		c.OutputFormat = fc.OutputFormat
	}
}

func applyRepoConfig(c *Config, rc repoConfig) {
	if rc.Conventional != nil {
		c.Conventional = *rc.Conventional
		c.conventionalSet = true
	}
	if rc.TicketPrefix != "" {
		c.TicketPrefix = rc.TicketPrefix
	}
	if rc.Imperative != nil {
		c.Imperative = *rc.Imperative
		c.imperativeSet = true
	}
	if rc.MaxSubjectLength != nil {
		c.MaxSubjectLength = *rc.MaxSubjectLength
	}
	if rc.BodyStyle != "" {
		c.BodyStyle = rc.BodyStyle
	}
	if rc.DefaultBranch != "" {
		c.DefaultBranch = rc.DefaultBranch
	}
	if rc.OutputFormat != "" {
		c.OutputFormat = rc.OutputFormat
	}
}

func applyFlags(c *Config, f RawFlags) {
	if f.DryRun {
		c.DryRun = true
	}
	if f.Cleanup {
		c.Cleanup = true
	}
	if f.Yes {
		c.Yes = true
	}
	if f.Mode == "1" {
		c.Mode = ModeSingle
	}
	if f.Staged {
		c.Scope = ScopeStaged
	} else if f.Unstaged {
		c.Scope = ScopeUnstaged
	}
	c.PlanOut = f.PlanOut
	c.Apply = f.Apply
	c.PlanLint = f.PlanLint
	c.Include = f.Include
	c.Exclude = f.Exclude
	c.IncludeSensitive = f.IncludeSensitive
	c.JSON = f.JSON
	c.Quiet = f.Quiet
}

func applyProfileDefaults(c *Config) {
	if c.Provider == "" {
		c.Provider = DefaultProviderName
	}
	if c.APIBase == "" {
		c.APIBase = KnownProviders[c.Provider]
	}
	if c.APIBase == "" {
		c.APIBase = DefaultAPIBase
	}
	if c.Model == "" {
		c.Model = ProviderDefaults[c.Provider]
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}
	if !c.contextSet {
		c.ContextWindow = defaultContextWindow
	}
	if !c.retriesSet {
		c.Retries = defaultRetries
	}
	if !c.timeoutSet {
		c.Timeout = defaultTimeout
	}
	if !c.conventionalSet {
		c.Conventional = true
	}
	if !c.imperativeSet {
		c.Imperative = true
	}
	if c.MaxSubjectLength == 0 {
		c.MaxSubjectLength = MaxSubjectLength
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Input == nil {
		c.Input = os.Stdin
	}
	if c.Output == nil {
		c.Output = os.Stdout
	}
}

func apiKeyFromEnv() (string, error) {
	if os.Getenv(LegacyEnvAPIKey) != "" {
		return "", fmt.Errorf("%s is no longer supported; rename it to %s", LegacyEnvAPIKey, EnvAPIKey)
	}
	key := os.Getenv(EnvAPIKey)
	if len(key) > maxEnvAPIKeyLen {
		key = key[:maxEnvAPIKeyLen]
	}
	return key, nil
}

func loadUserConfig(path string) (fileConfig, error) {
	var fc fileConfig
	fi, err := os.Stat(path)
	if err != nil {
		return fc, err
	}
	if fi.IsDir() {
		return fc, fmt.Errorf("config path %q is a directory, expected a file", path)
	}
	if fi.Size() > maxConfigSize {
		return fc, fmt.Errorf("config file %s too large (%d bytes, max %d)", path, fi.Size(), maxConfigSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fc, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fc, fmt.Errorf("config file %s: %v", path, err)
	}
	if _, ok := raw["api_key"]; ok {
		return fc, fmt.Errorf("config file %s: api_key is not allowed here; set the %s environment variable instead", path, EnvAPIKey)
	}

	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&fc); err != nil && !errors.Is(err, io.EOF) {
		return fc, fmt.Errorf("config file %s: %v", path, err)
	}
	return fc, nil
}

func loadRepoConfig(path string) (repoConfig, error) {
	var rc repoConfig
	fi, err := os.Stat(path)
	if err != nil {
		return rc, err
	}
	if fi.Size() > maxConfigSize {
		return rc, fmt.Errorf("repository config %s too large (%d bytes, max %d)", path, fi.Size(), maxConfigSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rc, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return rc, fmt.Errorf("repository config %s: %v", path, err)
	}
	for k := range raw {
		if !repoAllowlist[k] {
			return rc, fmt.Errorf("repository config %s: key %q is not allowed (allowlist: %s)", path, k, strings.Join(sortedKeys(repoAllowlist), ", "))
		}
	}
	if err := yaml.Unmarshal(data, &rc); err != nil {
		return rc, fmt.Errorf("repository config %s: %v", path, err)
	}
	return rc, nil
}

func discoverRepoConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, RepoConfigFile)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
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

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func isDir(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
