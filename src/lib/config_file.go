package lib

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

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
			return rc, fmt.Errorf("repository config %s: key %q is not allowed (allowlist: %s)", path, k, strings.Join(slices.Sorted(maps.Keys(repoAllowlist)), ", "))
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

// resolveConfigFile returns the user config path and how it was chosen.
func resolveConfigFile(flagPath string) (string, string) {
	if flagPath != "" {
		return flagPath, "flag"
	}
	return filepath.Join(configDir(), DefaultConfigFile), "env/default"
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
