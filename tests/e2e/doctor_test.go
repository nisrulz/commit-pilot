package e2e_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEndToEndListModelsAndDoctor(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	mock := newMockOpenAI(t, mockRespond)

	stdout, _, err := runCLI(t, bin, repo, nil, "", "--list-models", "--json", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("list-models failed: %v", err)
	}
	var models struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal([]byte(stdout), &models); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(models.Models) != 1 || models.Models[0] != "test-model" {
		t.Fatalf("unexpected models: %v", models.Models)
	}

	stdout, _, err = runCLI(t, bin, repo, nil, "", "--doctor", "--json", "--config", mockConfig(t, mock))
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	var doctor struct {
		Status            string `json:"status"`
		GitRepository     bool   `json:"git_repository"`
		ProviderReachable bool   `json:"provider_reachable"`
		ModelAvailable    bool   `json:"model_available"`
	}
	if err := json.Unmarshal([]byte(stdout), &doctor); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if doctor.Status != "completed" || !doctor.GitRepository || !doctor.ProviderReachable || !doctor.ModelAvailable {
		t.Fatalf("unexpected doctor result: %+v", doctor)
	}
}

func TestEndToEndProviderFailureJSONError(t *testing.T) {
	bin := cliBinary(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	runGit(t, repo, "add", "-A")

	// Point the binary at a dead endpoint to force a connection error.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	stdout, stderr, err := runCLI(t, bin, repo, nil, "", "--json", "--single", "--config",
		writeConfig(t, "provider: openai_compat\nmodel: test-model\nbase_url: "+deadURL+"\n"))
	if err == nil {
		t.Fatal("expected failure against dead provider")
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON error output, got: %q\nstderr: %s", stdout, stderr)
	}
	if result["status"] != "error" {
		t.Fatalf("status = %v, want error", result["status"])
	}
}
