package lib_test

import (
	"errors"
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// urlAwareDoer lets tests decide per-URL which probes reach a live provider.
type urlAwareDoer struct {
	reachable map[string]bool
}

func (d urlAwareDoer) Do(req *http.Request) (*http.Response, error) {
	if d.reachable[req.URL.String()] {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	}
	return nil, errors.New("unreachable")
}

func quiet() func() {
	lib.SetOutputMode(true, false)
	return func() { lib.SetOutputMode(false, false) }
}

func TestAnnounceProviderNamedProviderRespected(t *testing.T) {
	defer quiet()()
	// A provider chosen by name is probed exactly as configured and kept.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	cfg := lib.Config{
		Provider:         "ollama",
		APIBase:          server.URL,
		Model:            "test-model",
		ProviderExplicit: true,
		HTTPClient:       server.Client(),
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != "ollama" || got.APIBase != server.URL || got.Model != "test-model" {
		t.Fatalf("named provider must be preserved, got %s @ %s model %s", got.Provider, got.APIBase, got.Model)
	}
}

func TestAnnounceProviderIdentifiesOpenAICompatAtConfiguredBase(t *testing.T) {
	defer quiet()()
	// A customized endpoint with no named provider is probed as an
	// OpenAI-compatible server.
	cfg := lib.Config{
		APIBase:         "http://127.0.0.1:8888/v1",
		Model:           "test-model",
		APIBaseExplicit: true,
		HTTPClient: urlAwareDoer{reachable: map[string]bool{
			"http://127.0.0.1:8888/v1/models": true,
		}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != "openai_compat" || got.APIBase != "http://127.0.0.1:8888/v1" {
		t.Fatalf("expected openai_compat at configured base, got %s @ %s", got.Provider, got.APIBase)
	}
}

func TestAnnounceProviderAutoDetectsReachableOllama(t *testing.T) {
	defer quiet()()
	// With no explicit provider, the default Ollama endpoint is probed.
	cfg := lib.Config{
		Model:      "lfm2.5:8b",
		HTTPClient: urlAwareDoer{reachable: map[string]bool{"http://localhost:11434/v1/models": true}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != "ollama" || got.APIBase != "http://localhost:11434/v1" {
		t.Fatalf("expected ollama to be detected, got %s @ %s", got.Provider, got.APIBase)
	}
	if got.Model != "lfm2.5:8b" {
		t.Fatalf("configured model must be kept, got %q", got.Model)
	}
}

func TestAnnounceProviderKeepsConfigWhenNothingReachable(t *testing.T) {
	defer quiet()()
	cfg := lib.Config{
		Provider:   "ollama",
		APIBase:    "http://localhost:11434/v1",
		Model:      "test-model",
		HTTPClient: urlAwareDoer{reachable: map[string]bool{}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != cfg.Provider || got.APIBase != cfg.APIBase || got.Model != cfg.Model {
		t.Fatalf("config must be preserved when nothing is reachable, got %s @ %s model %s", got.Provider, got.APIBase, got.Model)
	}
}

func TestAnnounceProviderPrintsProbeTable(t *testing.T) {
	lib.SetOutputMode(false, false)
	defer lib.SetOutputMode(false, false)
	cfg := lib.Config{
		Model:      "lfm2.5:8b",
		HTTPClient: urlAwareDoer{reachable: map[string]bool{"http://localhost:11434/v1/models": true}},
	}
	out := captureStdout(t, func() { lib.AnnounceProvider(cfg) })
	for _, want := range []string{
		"Probing available AI providers",
		"ollama",
		"http://localhost:11434/v1/models",
		"Using provider: ollama (http://localhost:11434/v1)",
		"-> Model: lfm2.5:8b",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("probe table missing %q:\n%s", want, out)
		}
	}
}
