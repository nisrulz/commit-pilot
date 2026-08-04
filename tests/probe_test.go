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

func TestAnnounceProviderIdentifiesProviderAtConfiguredBase(t *testing.T) {
	defer quiet()()
	// A customized endpoint (127.0.0.1 vs localhost) with the default provider
	// must be identified by probing: Unsloth answers /health, so it wins over
	// the /models-only fallback even though it was not configured by name.
	cfg := lib.Config{
		Provider:        "lmstudio",
		APIBase:         "http://127.0.0.1:8888/v1",
		Model:           "unsloth/gemma-4-E4B-it-qat-GGUF",
		APIBaseExplicit: true,
		HTTPClient: urlAwareDoer{reachable: map[string]bool{
			"http://127.0.0.1:8888/health":    true,
			"http://127.0.0.1:8888/v1/models": true,
		}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != "unsloth" || got.APIBase != "http://127.0.0.1:8888/v1" {
		t.Fatalf("expected unsloth identified at configured base, got %s @ %s", got.Provider, got.APIBase)
	}
}

func TestAnnounceProviderFallsBackToDefaultProviderAtUnknownBase(t *testing.T) {
	defer quiet()()
	// A custom endpoint with no /health route falls back to the /models-style
	// providers, landing on the default name.
	cfg := lib.Config{
		Provider:        "lmstudio",
		APIBase:         "http://localhost:9999/v1",
		Model:           "test-model",
		APIBaseExplicit: true,
		HTTPClient: urlAwareDoer{reachable: map[string]bool{
			"http://localhost:9999/v1/models": true,
		}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != "lmstudio" || got.APIBase != "http://localhost:9999/v1" {
		t.Fatalf("expected fallback provider at unknown base, got %s @ %s", got.Provider, got.APIBase)
	}
}

func TestAnnounceProviderAutoDetectsReachableLocalProvider(t *testing.T) {
	defer quiet()()
	// Only Unsloth's health route answers, so it must be picked over the
	// earlier lmstudio and ollama candidates.
	cfg := lib.Config{
		Model:      "gemma-4-e2b-it-qat",
		HTTPClient: urlAwareDoer{reachable: map[string]bool{"http://localhost:8888/health": true}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != "unsloth" || got.APIBase != "http://localhost:8888/v1" {
		t.Fatalf("expected unsloth to be detected, got %s @ %s", got.Provider, got.APIBase)
	}
	if got.Model != "gemma-4-e2b-it-qat" {
		t.Fatalf("configured model must be kept, got %q", got.Model)
	}
}

func TestAnnounceProviderKeepsConfigWhenNothingReachable(t *testing.T) {
	defer quiet()()
	cfg := lib.Config{
		Provider:   "lmstudio",
		APIBase:    "http://localhost:1234/v1",
		Model:      "test-model",
		HTTPClient: urlAwareDoer{reachable: map[string]bool{}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != cfg.Provider || got.APIBase != cfg.APIBase || got.Model != cfg.Model {
		t.Fatalf("config must be preserved when nothing is reachable, got %s @ %s model %s", got.Provider, got.APIBase, got.Model)
	}
}

func TestAnnounceProviderPrefersModelMatchingProvider(t *testing.T) {
	defer quiet()()
	// The configured model is Unsloth's default, so Unsloth must win even when
	// LM Studio is also reachable and would otherwise come first.
	cfg := lib.Config{
		Model: "unsloth/gemma-4-E4B-it-qat-GGUF",
		HTTPClient: urlAwareDoer{reachable: map[string]bool{
			"http://localhost:1234/v1/models": true,
			"http://localhost:8888/health":    true,
		}},
	}
	got := lib.AnnounceProvider(cfg)
	if got.Provider != "unsloth" || got.APIBase != "http://localhost:8888/v1" {
		t.Fatalf("expected model-matching unsloth to be selected, got %s @ %s", got.Provider, got.APIBase)
	}
}

func TestAnnounceProviderPrintsProbeTable(t *testing.T) {
	lib.SetOutputMode(false, false)
	defer lib.SetOutputMode(false, false)
	cfg := lib.Config{
		Model:      "gemma-4-e2b-it-qat",
		HTTPClient: urlAwareDoer{reachable: map[string]bool{"http://localhost:8888/health": true}},
	}
	out := captureStdout(t, func() { lib.AnnounceProvider(cfg) })
	for _, want := range []string{
		"Probing available AI providers",
		"lmstudio",
		"http://localhost:1234/v1/models",
		"unsloth",
		"http://localhost:8888/health",
		"Using provider: unsloth (http://localhost:8888/v1)",
		"-> Model: gemma-4-e2b-it-qat",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("probe table missing %q:\n%s", want, out)
		}
	}
}
