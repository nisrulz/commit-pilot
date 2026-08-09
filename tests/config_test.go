package lib_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lib "github.com/nisrulz/commit-pilot/src/lib"
)

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// isolatedHome points $HOME and the working directory at fresh temp dirs so
// tests never pick up a stray real config or repo config.
func isolatedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv(lib.EnvConfigDir, "")
	t.Setenv(lib.EnvAPIKey, "")
	t.Chdir(dir)
	return dir
}

func defaultPath(home string) string {
	return filepath.Join(home, ".config", "commit-pilot", "config.yaml")
}

// --- precedence: flag > env dir > default path ---

func TestPrecedenceFlagOverEnvOverDefault(t *testing.T) {
	home := isolatedHome(t)

	def := defaultPath(home)
	writeConfigFile(t, def, "provider: openai_compat\nmodel: m-default\ncontext_window: 50000\n")
	envDir := filepath.Join(home, "env")
	writeConfigFile(t, filepath.Join(envDir, "commit-pilot", "config.yaml"), "provider: openai_compat\nmodel: m-env\ncontext_window: 50000\n")
	flagPath := filepath.Join(home, "flag.yaml")
	writeConfigFile(t, flagPath, "provider: openai_compat\nmodel: m-flag\nbase_url: http://127.0.0.1:9999/v1\ncontext_window: 50000\n")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("default path resolve: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Model != "m-default" || cfg.ConfigPath != def {
		t.Fatalf("default path not used: %+v", cfg)
	}

	t.Setenv(lib.EnvConfigDir, envDir)
	cfg, err = lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("env resolve: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Model != "m-env" {
		t.Fatalf("env dir not used: %+v", cfg)
	}

	cfg, err = lib.ResolveConfig(lib.RawFlags{ConfigPath: flagPath})
	if err != nil {
		t.Fatalf("flag resolve: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Model != "m-flag" {
		t.Fatalf("flag path should win over env: %+v", cfg)
	}
}

func TestPrecedenceFlagOverConfigOverProfileDefaults(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "model: cfg-model\ncontext_window: 5000\nmode: auto\ndry_run: true\ncleanup: false\n")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.Model != "cfg-model" {
		t.Fatalf("config value should win over profile default, got %q", cfg.Model)
	}
	if cfg.ContextWindow != 5000 {
		t.Fatalf("config context_window should win, got %d", cfg.ContextWindow)
	}
	if cfg.Provider != "openai_compat" || cfg.APIBase != lib.KnownProviders["openai_compat"] {
		t.Fatalf("missing provider should fall back to profile defaults: %+v", cfg)
	}
	if cfg.Mode != lib.ModeAuto || !cfg.DryRun || cfg.Cleanup {
		t.Fatalf("config booleans/mode applied wrong: %+v", cfg)
	}

	cfg, err = lib.ResolveConfig(lib.RawFlags{Mode: "1", Cleanup: true})
	if err != nil {
		t.Fatalf("resolve with flags: %v", err)
	}
	if cfg.Mode != lib.ModeSingle {
		t.Fatalf("flag mode should win over config, got %q", cfg.Mode)
	}
	if cfg.DryRun != true || cfg.Cleanup != true {
		t.Fatalf("flags should not unset config, got %+v", cfg)
	}
}

// --- missing-file semantics per source ---

func TestMissingFileSemantics(t *testing.T) {
	home := isolatedHome(t)

	// explicit --config pointing at a missing file -> hard error
	_, err := lib.ResolveConfig(lib.RawFlags{ConfigPath: filepath.Join(home, "nope.yaml")})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected hard error for missing --config, got %v", err)
	}

	// env dir pointing at a missing directory -> no user config, run with defaults
	t.Setenv(lib.EnvConfigDir, filepath.Join(home, "nope"))
	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("missing env config dir should be ignored: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Model != lib.DefaultModel {
		t.Fatalf("expected defaults with missing env config dir, got %+v", cfg)
	}

	// default path missing -> no user config, run with defaults
	t.Setenv(lib.EnvConfigDir, "")
	cfg, err = lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("missing default config should be ignored: %v", err)
	}
	if cfg.Provider != "openai_compat" || cfg.Model != lib.DefaultModel {
		t.Fatalf("expected defaults with missing default config, got %+v", cfg)
	}
	if cfg.ContextWindow <= 0 {
		t.Fatalf("expected a usable context window, got %d", cfg.ContextWindow)
	}

	// env var pointing at a file (not a directory) -> error
	someFile := filepath.Join(home, "plainfile")
	if err := os.WriteFile(someFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(lib.EnvConfigDir, someFile)
	if _, err = lib.ResolveConfig(lib.RawFlags{}); err == nil {
		t.Fatal("expected error when env config dir is actually a file")
	}

	// --config pointing at a directory -> error
	dir := filepath.Join(home, "adir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err = lib.ResolveConfig(lib.RawFlags{ConfigPath: dir}); err == nil {
		t.Fatal("expected error when --config path is a directory")
	}
}

// --- repo config allowlist ---

func TestRepoConfigAllowlist(t *testing.T) {
	home := isolatedHome(t)

	writeConfigFile(t, filepath.Join(home, ".commit-pilot.yaml"),
		"default_branch: main\noutput_format: json\n")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("repo config with allowlisted keys should load: %v", err)
	}
	if cfg.DefaultBranch != "main" || cfg.OutputFormat != "json" {
		t.Fatalf("repo allowlist values not applied: %+v", cfg)
	}
}

func TestRepoConfigRejectsDisallowedKeys(t *testing.T) {
	home := isolatedHome(t)

	for _, content := range []string{
		"api_key: sk-leak\n",
		"provider: openai\n",
		"base_url: https://evil.example\n",
		"model: some-model\n",
	} {
		writeConfigFile(t, filepath.Join(home, ".commit-pilot.yaml"), content)
		if _, err := lib.ResolveConfig(lib.RawFlags{}); err == nil {
			t.Fatalf("repo config with %q should be rejected", strings.TrimSpace(content))
		}
	}
}

func TestRepoConfigDiscoveredFromParentDir(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, filepath.Join(home, ".commit-pilot.yaml"), "default_branch: trunk\n")
	sub := filepath.Join(home, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve from subdir: %v", err)
	}
	if cfg.DefaultBranch != "trunk" {
		t.Fatalf("repo config in parent should be discovered, got %+v", cfg)
	}
}

func TestUserConfigOverridesRepoConfig(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, filepath.Join(home, ".commit-pilot.yaml"), "default_branch: repo-branch\n")
	writeConfigFile(t, defaultPath(home), "default_branch: user-branch\ncontext_window: 50000\n")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.DefaultBranch != "user-branch" {
		t.Fatalf("trusted user config should override untrusted repo config, got %q", cfg.DefaultBranch)
	}
}

// --- corrupt / oversized config ---

func TestCorruptConfig(t *testing.T) {
	home := isolatedHome(t)

	for name, content := range map[string]string{
		"invalid yaml":    "provider: [unclosed\n",
		"unknown key":     "endpoint: http://x\n",
		"type mismatch":   "context_window: notanumber\n",
		"bad boolean":     "dry_run: maybe\n",
		"api_key present": "api_key: sk-leak\n",
	} {
		writeConfigFile(t, defaultPath(home), content)
		if _, err := lib.ResolveConfig(lib.RawFlags{}); err == nil {
			t.Fatalf("corrupt config (%s) should error", name)
		}
	}

	// api_key rejection message should point at the env var
	writeConfigFile(t, defaultPath(home), "api_key: sk-leak\n")
	_, err := lib.ResolveConfig(lib.RawFlags{})
	if err == nil || !strings.Contains(err.Error(), lib.EnvAPIKey) {
		t.Fatalf("api_key rejection should mention %s, got %v", lib.EnvAPIKey, err)
	}
}

func TestOversizedConfig(t *testing.T) {
	home := isolatedHome(t)
	big := strings.Repeat("a", 1<<20+1)
	writeConfigFile(t, defaultPath(home), "model: "+big+"\n")
	if _, err := lib.ResolveConfig(lib.RawFlags{}); err == nil {
		t.Fatal("oversized user config should error")
	}

	writeConfigFile(t, filepath.Join(home, ".commit-pilot.yaml"), "output_format: "+big+"\n")
	writeConfigFile(t, defaultPath(home), "context_window: 50000\n")
	if _, err := lib.ResolveConfig(lib.RawFlags{}); err == nil {
		t.Fatal("oversized repo config should error")
	}
}

// --- api_key: from env only, never from file or CLI ---

func TestAPIKeyFromEnvNotFile(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "provider: openai_compat\ncontext_window: 50000\n")
	t.Setenv(lib.EnvAPIKey, "sk-env-123")

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.APIKey != "sk-env-123" {
		t.Fatalf("api key should come from env, got %q", cfg.APIKey)
	}

	// no env -> empty key
	t.Setenv(lib.EnvAPIKey, "")
	cfg, err = lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve without key: %v", err)
	}
	if cfg.APIKey != "" {
		t.Fatalf("expected empty api key without env, got %q", cfg.APIKey)
	}
}

func TestLegacyAPIKeyEnvRequiresRename(t *testing.T) {
	home := isolatedHome(t)
	writeConfigFile(t, defaultPath(home), "provider: openai_compat\n")
	t.Setenv(lib.LegacyEnvAPIKey, "sk-old")

	if _, err := lib.ResolveConfig(lib.RawFlags{}); err == nil || !strings.Contains(err.Error(), lib.EnvAPIKey) {
		t.Fatalf("expected legacy API key migration error, got %v", err)
	}
}

// --- per-session scoping ---

func TestPerSessionScoping(t *testing.T) {
	home := isolatedHome(t)

	dirA := filepath.Join(home, "a")
	dirB := filepath.Join(home, "b")
	writeConfigFile(t, filepath.Join(dirA, "commit-pilot", "config.yaml"), "provider: openai_compat\nmodel: model-a\ncontext_window: 50000\n")
	writeConfigFile(t, filepath.Join(dirB, "commit-pilot", "config.yaml"), "provider: openai_compat\nmodel: model-b\ncontext_window: 50000\n")

	// inline prefix / per-run env is read fresh on each invocation
	t.Setenv(lib.EnvConfigDir, dirA)
	cfgA, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve a: %v", err)
	}
	t.Setenv(lib.EnvConfigDir, dirB)
	cfgB, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve b: %v", err)
	}
	if cfgA.Model == cfgB.Model {
		t.Fatalf("each invocation must re-read the env var (session scope)")
	}

	// per-invocation --config beats an exported env var
	t.Setenv(lib.EnvConfigDir, dirA)
	cfgF, err := lib.ResolveConfig(lib.RawFlags{ConfigPath: filepath.Join(dirB, "commit-pilot", "config.yaml")})
	if err != nil {
		t.Fatalf("resolve flag: %v", err)
	}
	if cfgF.Model != "model-b" {
		t.Fatalf("--config should win for this invocation only, got %+v", cfgF)
	}
}

// --- validation before any work ---

func TestValidation(t *testing.T) {
	home := isolatedHome(t)

	base := "provider: openai_compat\ncontext_window: 50000\n"
	for name, extra := range map[string]string{
		"unknown provider":     "provider: gpt5\n",
		"http non-loopback":    "base_url: http://api.example.com/v1\n",
		"embedded credentials": "base_url: http://user:pass@localhost:1234/v1\n",
		"bad scheme":           "base_url: ftp://localhost/v1\n",
		"missing host":         "base_url: https://\n",
		"context too small":    "context_window: 100\n",
		"context too large":    "context_window: 99999999999\n",
		"bad mode":             "mode: banana\n",
		"bad output format":    "output_format: xml\n",
	} {
		writeConfigFile(t, defaultPath(home), base+"\n"+extra)
		if _, err := lib.ResolveConfig(lib.RawFlags{}); err == nil {
			t.Fatalf("%s should be a validation error", name)
		}
	}

	// valid loopback http and remote https both pass
	writeConfigFile(t, defaultPath(home), "provider: openai_compat\nbase_url: https://api.openai.com/v1\ncontext_window: 50000\n")
	if _, err := lib.ResolveConfig(lib.RawFlags{}); err != nil {
		t.Fatalf("valid https remote should pass: %v", err)
	}
	writeConfigFile(t, defaultPath(home), "provider: openai_compat\nbase_url: http://127.0.0.1:11434/v1\ncontext_window: 50000\n")
	if _, err := lib.ResolveConfig(lib.RawFlags{}); err != nil {
		t.Fatalf("valid loopback http should pass: %v", err)
	}
}

// --- defaults ---

func TestDefaultsWithNoConfig(t *testing.T) {
	isolatedHome(t)

	cfg, err := lib.ResolveConfig(lib.RawFlags{})
	if err != nil {
		t.Fatalf("resolve with no config: %v", err)
	}
	if cfg.Provider != "openai_compat" {
		t.Fatalf("default provider should be openai_compat, got %q", cfg.Provider)
	}
	if cfg.Model != lib.DefaultModel {
		t.Fatalf("default model mismatch, got %q", cfg.Model)
	}
	if cfg.APIBase != lib.DefaultAPIBase {
		t.Fatalf("default base url mismatch, got %q", cfg.APIBase)
	}
	if cfg.ContextWindow <= 0 {
		t.Fatalf("default context window should be positive, got %d", cfg.ContextWindow)
	}
}

// --- config dir relocates CLI working files ---

func TestConfigDirRelocatesWorkingFiles(t *testing.T) {
	home := isolatedHome(t)

	defPath := lib.SummariesPath()
	if !strings.Contains(defPath, filepath.Join(".config", "commit-pilot", "tmp")) {
		t.Fatalf("default summaries should live under the config dir, got %q", defPath)
	}

	relocated := filepath.Join(home, "elsewhere")
	t.Setenv(lib.EnvConfigDir, relocated)
	if p := lib.SummariesPath(); !strings.Contains(p, filepath.Join(relocated, "commit-pilot", "tmp")) {
		t.Fatalf("summaries should follow the config dir, got %q", p)
	}
}

func TestParseConfigArgs(t *testing.T) {
	flags, _ := lib.ParseArgs([]string{"--config", "/tmp/x.yaml", "--dry-run"})
	if flags.ConfigPath != "/tmp/x.yaml" || !flags.DryRun {
		t.Fatalf("--config flag parse failed: %+v", flags)
	}
	flags, _ = lib.ParseArgs([]string{"--config=/tmp/x.yaml"})
	if flags.ConfigPath != "/tmp/x.yaml" {
		t.Fatalf("--config= parse failed: %+v", flags)
	}
	flags, _ = lib.ParseArgs([]string{"--config"})
	if flags.Error == "" {
		t.Fatal("--config without a value should error")
	}
	flags, _ = lib.ParseArgs([]string{"--config="})
	if flags.Error == "" {
		t.Fatal("--config= without a value should error")
	}
	// the old `config ...` subcommand is rejected, never run as the pipeline
	flags, _ = lib.ParseArgs([]string{"config", "show"})
	if flags.Error == "" || !strings.Contains(flags.Error, "config") {
		t.Fatalf("stray `config` should error, got %q", flags.Error)
	}
}
