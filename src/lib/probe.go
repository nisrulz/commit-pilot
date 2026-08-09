package lib

import (
	"strings"
	"time"

	"github.com/nisrulz/commit-pilot/src/lib/provider"
)

// probeTimeout bounds how long each reachability probe may wait for a response
// so a dead endpoint does not stall the run.
const probeTimeout = 3 * time.Second

// probeCandidate is one provider endpoint that the detection step checks.
type probeCandidate struct {
	name     string
	base     string
	probeURL string
}

// modelsURL returns the URL used to check reachability for display.
func modelsURL(base string) string {
	return strings.TrimRight(base, "/") + "/models"
}

// probeCandidatesFor builds the ordered list of endpoints to probe. A provider
// chosen by name is probed exactly as configured. A customized endpoint with no
// named provider is probed as an OpenAI-compatible server. Otherwise the
// default local provider (Ollama) is probed at its default base.
func probeCandidatesFor(cfg Config) []probeCandidate {
	if cfg.ProviderExplicit {
		return []probeCandidate{{cfg.Provider, cfg.APIBase, modelsURL(cfg.APIBase)}}
	}
	if cfg.APIBaseExplicit {
		return []probeCandidate{{"openai_compat", cfg.APIBase, modelsURL(cfg.APIBase)}}
	}
	return []probeCandidate{{DefaultProviderName, KnownProviders[DefaultProviderName], modelsURL(KnownProviders[DefaultProviderName])}}
}

// probeOne reports whether the provider at base answers a reachability probe,
// using a short timeout so a dead endpoint fails quickly.
func probeOne(name, base string, cfg Config) error {
	client := cfg.HTTPClient
	if client == nil {
		client = newProviderHTTPClient(probeTimeout)
	}
	return provider.New(name).Probe(runContext(cfg), base, cfg.APIKey, client)
}

// AnnounceProvider picks the provider for the run and reports it in a colored
// probe table. When no provider was configured explicitly, it probes the
// default local provider (Ollama) and selects it when reachable, updating cfg's
// provider and API base. The configured model is left untouched and shown on
// the selection line. It returns cfg unchanged when nothing is reachable.
func AnnounceProvider(cfg Config) Config {
	PrintProbeHeader("Probing available AI providers")
	for _, candidate := range probeCandidatesFor(cfg) {
		if err := probeOne(candidate.name, candidate.base, cfg); err != nil {
			PrintProbeResult(candidate.name, candidate.probeURL, false)
			continue
		}
		PrintProbeResult(candidate.name, candidate.probeURL, true)
		cfg.Provider = candidate.name
		cfg.APIBase = candidate.base
		PrintProviderSelected(cfg.Provider, cfg.APIBase, cfg.Model)
		return cfg
	}
	return cfg
}
