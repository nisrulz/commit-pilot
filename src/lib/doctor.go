package lib

import (
	"fmt"
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

	if err := CheckProvider(cfg); err != nil {
		fmt.Printf("  %s Provider: %v\n", red("!"), err)
		return false
	}
	fmt.Printf("  %s Provider is reachable\n", green("✔"))
	return ok
}

func CheckProvider(cfg Config) error {
	url := strings.TrimRight(cfg.APIBase, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s", cfg.APIBase)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("returned status %d", resp.StatusCode)
	}
	return nil
}
