package lib

import (
	"context"
	"fmt"
	"os"

	"github.com/nisrulz/commit-pilot/src/lib/provider"
)

// runListModels prints the models available from the configured provider.
func runListModels(cfg Config) {
	models, err := ListProviderModels(cfg)
	if err != nil {
		Die("list models: %v", err)
	}
	if cfg.JSON {
		PrintJSON(map[string]any{"models": models})
	} else {
		for _, model := range models {
			fmt.Println(sanitizeLine(model, 1024))
		}
	}
}

// runDoctorCheck verifies the git repository and provider setup, reporting a
// machine-readable result in JSON mode and exiting non-zero on any failure.
func runDoctorCheck(cfg Config) {
	if cfg.JSON {
		_, gitErr := GitRun("rev-parse", "--show-toplevel")
		reachable := provider.New(cfg.Provider).Probe(runContext(cfg), cfg.APIBase, cfg.APIKey, cfg.HTTPClient) == nil
		var found bool
		var modelErr error
		if reachable {
			found, modelErr = CheckProvider(cfg)
		}
		status := "completed"
		if gitErr != nil || !reachable || modelErr != nil || !found {
			status = "error"
		}
		PrintJSON(map[string]any{"status": status, "git_repository": gitErr == nil, "model": cfg.Model, "provider_reachable": reachable, "model_available": found})
		if gitErr != nil || !reachable || modelErr != nil || !found {
			os.Exit(1)
		}
		return
	}
	if !RunDoctor(cfg) {
		os.Exit(1)
	}
}

func RunDoctor(cfg Config) bool {
	fmt.Println("  Commit Pilot doctor")
	ok := true

	if _, err := GitRun("rev-parse", "--show-toplevel"); err != nil {
		fmt.Printf("  %s Git repository: %v\n", red("!"), err)
		ok = false
	} else {
		fmt.Printf("  %s Git repository\n", green("✔"))
	}

	keyStatus := "not set"
	if cfg.APIKey != "" {
		keyStatus = "set"
	}
	fmt.Printf("  %s Model: %s\n", green("✔"), sanitizeLine(cfg.Model, 1024))
	fmt.Printf("    API base: %s\n", sanitizeLine(cfg.APIBase, 2048))
	fmt.Printf("    API key: %s\n", keyStatus)
	fmt.Printf("    Config: %s/config.env\n", sanitizePath(ConfigDir()))

	if err := provider.New(cfg.Provider).Probe(runContext(cfg), cfg.APIBase, cfg.APIKey, cfg.HTTPClient); err != nil {
		fmt.Printf("  %s Provider: %v\n", red("!"), err)
		return false
	}
	fmt.Printf("  %s Provider is reachable\n", green("✔"))

	found, err := CheckProvider(cfg)
	if err != nil {
		fmt.Printf("  %s Provider: %v\n", red("!"), err)
		return false
	}
	if !found {
		fmt.Printf("  %s Model %q is not available from the provider\n", red("!"), sanitizeLine(cfg.Model, 1024))
		return false
	}
	return ok
}

func CheckProvider(cfg Config) (bool, error) {
	models, err := ListProviderModels(cfg)
	if err != nil {
		return false, err
	}
	for _, model := range models {
		if model == cfg.Model {
			return true, nil
		}
	}
	return false, nil
}

// ListProviderModels returns the model IDs exposed by the provider configured in
// cfg.Provider, delegating to the provider's own listing implementation.
func ListProviderModels(cfg Config) ([]string, error) {
	return provider.New(cfg.Provider).ListModels(runContext(cfg), cfg.APIBase, cfg.APIKey, cfg.HTTPClient)
}

// runContext returns the configured context, falling back to a background
// context for runs that do not set one.
func runContext(cfg Config) context.Context {
	if cfg.Context != nil {
		return cfg.Context
	}
	return context.Background()
}
