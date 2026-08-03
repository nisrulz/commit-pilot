package lib_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lib "github.com/nisrulz/commit-pilot/src/lib"
	"github.com/nisrulz/commit-pilot/src/lib/provider"
)

func TestProviderDispatch(t *testing.T) {
	cases := map[string]string{
		"":         "openai",
		"unknown":  "openai",
		"openai":   "openai",
		"ollama":   "ollama",
		"lmstudio": "lmstudio",
		"unsloth":  "unsloth",
	}
	for name, want := range cases {
		if got := provider.New(name).Name(); got != want {
			t.Fatalf("New(%q).Name() = %q, want %q", name, got, want)
		}
	}
}

func TestOpenAICompatProvidersProbeModels(t *testing.T) {
	for _, name := range []string{"openai", "ollama", "lmstudio"} {
		var gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		base := server.URL + "/v1"

		if err := provider.New(name).Probe(context.Background(), base, "", nil); err != nil {
			server.Close()
			t.Fatalf("%s Probe: %v", name, err)
		}
		if gotPath != "/v1/models" {
			server.Close()
			t.Fatalf("%s Probe hit %q, want /v1/models", name, gotPath)
		}
		server.Close()
	}
}

func TestOpenAIProviderProbeTreatsAuthAsReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
	}))
	defer server.Close()

	if err := provider.New("").Probe(context.Background(), server.URL, "", nil); err != nil {
		t.Fatalf("Probe on 401 should report reachable: %v", err)
	}
}

func TestProviderProbeFailsWhenUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	for _, name := range []string{"", "unsloth"} {
		if err := provider.New(name).Probe(context.Background(), url, "", nil); err == nil {
			t.Fatalf("Probe against closed server (%s) should fail", name)
		}
	}
}

func TestUnslothProviderProbeUsesHealthAtRoot(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// The API base carries /v1; the health probe must land on the server root.
	if err := provider.New("unsloth").Probe(context.Background(), server.URL+"/v1", "", nil); err != nil {
		t.Fatalf("Probe failed: %v", err)
	}
	if gotPath != "/health" {
		t.Fatalf("Probe hit %q, want /health", gotPath)
	}
}

func TestUnslothProviderListModelsSendsKeyAndParses(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"unsloth/gemma-4-E4B-it-qat-GGUF"}]}`))
	}))
	defer server.Close()

	models, err := provider.New("unsloth").ListModels(context.Background(), server.URL+"/v1", "sk-unsloth-test", nil)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("ListModels hit %q, want /v1/models", gotPath)
	}
	if gotAuth != "Bearer sk-unsloth-test" {
		t.Fatalf("auth header = %q, want Bearer key", gotAuth)
	}
	if len(models) != 1 || models[0] != "unsloth/gemma-4-E4B-it-qat-GGUF" {
		t.Fatalf("models = %v", models)
	}
}

func TestListProviderModelsDispatchesByProvider(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"},{"id":"m2"}]}`))
	}))
	defer server.Close()

	models, err := lib.ListProviderModels(lib.Config{APIBase: server.URL + "/v1", Provider: "unsloth"})
	if err != nil {
		t.Fatalf("ListProviderModels: %v", err)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("hit %q, want /v1/models", gotPath)
	}
	if strings.Join(models, ",") != "m1,m2" {
		t.Fatalf("models = %v", models)
	}
}
