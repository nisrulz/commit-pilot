package lib_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	lib "github.com/nisrulz/commit-pilot/src/lib"
	"github.com/nisrulz/commit-pilot/src/lib/provider"
)

func TestProviderDispatch(t *testing.T) {
	cases := map[string]string{
		"":              "openai_compat",
		"unknown":       "openai_compat",
		"openai_compat": "openai_compat",
		"ollama":        "ollama",
	}
	for name, want := range cases {
		if got := provider.New(name).Name(); got != want {
			t.Fatalf("New(%q).Name() = %q, want %q", name, got, want)
		}
	}
}

func TestProviderKnownAndNames(t *testing.T) {
	for _, name := range []string{"openai_compat", "ollama"} {
		if !provider.Known(name) {
			t.Fatalf("Known(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "openai", "openai-", "unslth", "lmstudio1", "unsloth", "custom"} {
		if provider.Known(name) {
			t.Fatalf("Known(%q) = true, want false", name)
		}
	}
	names := provider.Names()
	if len(names) != 2 {
		t.Fatalf("Names() = %v, want 2 providers", names)
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("Names() not sorted: %v", names)
		}
	}
}

// probeTrackedBody records whether Read was called so tests can assert that the
// probe drained the response before closing it.
type probeTrackedBody struct {
	read bool
}

func (b *probeTrackedBody) Read(p []byte) (int, error) {
	b.read = true
	return 0, io.EOF
}

func (b *probeTrackedBody) Close() error { return nil }

// probeFakeDoer returns a canned response with a tracked body.
type probeFakeDoer struct {
	body *probeTrackedBody
}

func (d *probeFakeDoer) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: d.body, Header: http.Header{}}, nil
}

func TestProviderProbeDrainsResponseBody(t *testing.T) {
	body := &probeTrackedBody{}
	client := &probeFakeDoer{body: body}
	if err := provider.New("").Probe(context.Background(), "http://localhost:1234/v1", "", client); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !body.read {
		t.Fatal("Probe must drain the response body so the connection can be reused")
	}
}

func TestOpenAICompatProvidersProbeModels(t *testing.T) {
	for _, name := range []string{"openai_compat", "ollama"} {
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

	for _, name := range []string{"", "openai_compat"} {
		if err := provider.New(name).Probe(context.Background(), url, "", nil); err == nil {
			t.Fatalf("Probe against closed server (%s) should fail", name)
		}
	}
}

func TestOpenAICompatProviderListModelsSendsKeyAndParses(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"unsloth/LFM2.5-8B-A1B-GGUF"}]}`))
	}))
	defer server.Close()

	models, err := provider.New("openai_compat").ListModels(context.Background(), server.URL+"/v1", "sk-test", nil)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("ListModels hit %q, want /v1/models", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("auth header = %q, want Bearer key", gotAuth)
	}
	if len(models) != 1 || models[0] != "unsloth/LFM2.5-8B-A1B-GGUF" {
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

	models, err := lib.ListProviderModels(lib.Config{APIBase: server.URL + "/v1", Provider: "openai_compat"})
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
