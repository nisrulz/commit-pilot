package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	url := strings.TrimRight(cfg.APIBase, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return false, fmt.Errorf("could not reach %s", cfg.APIBase)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Errorf("returned status %d", resp.StatusCode)
	}

	var payload struct {
		Data   []struct{ ID string } `json:"data"`
		Models []struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxResponseSize)).Decode(&payload); err != nil {
		return false, fmt.Errorf("could not read model list")
	}
	for _, model := range payload.Data {
		if model.ID == cfg.Model {
			return true, nil
		}
	}
	for _, model := range payload.Models {
		if model.ID == cfg.Model || model.Key == cfg.Model {
			return true, nil
		}
	}
	return false, nil
}
