package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

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
	fmt.Printf("  %s Model: %s\n", green("✔"), cfg.Model)
	fmt.Printf("    API base: %s\n", cfg.APIBase)
	fmt.Printf("    API key: %s\n", keyStatus)
	fmt.Printf("    Config: %s/config.env\n", ConfigDir())

	found, err := CheckProvider(cfg)
	if err != nil {
		fmt.Printf("  %s Provider: %v\n", red("!"), err)
		return false
	}
	if !found {
		fmt.Printf("  %s Model %q is not available from the provider\n", red("!"), cfg.Model)
		return false
	}
	fmt.Printf("  %s Provider is reachable\n", green("✔"))
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

func ListProviderModels(cfg Config) ([]string, error) {
	url := strings.TrimRight(cfg.APIBase, "/") + "/models"
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s", cfg.APIBase)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("returned status %d", resp.StatusCode)
	}

	var payload struct {
		Data   []struct{ ID string } `json:"data"`
		Models []struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxResponseSize)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("could not read model list")
	}
	seen := make(map[string]bool)
	var models []string
	for _, model := range payload.Data {
		if model.ID != "" && !seen[model.ID] {
			models = append(models, model.ID)
			seen[model.ID] = true
		}
	}
	for _, model := range payload.Models {
		for _, name := range []string{model.ID, model.Key} {
			if name != "" && !seen[name] {
				models = append(models, name)
				seen[name] = true
			}
		}
	}
	sort.Strings(models)
	return models, nil
}
