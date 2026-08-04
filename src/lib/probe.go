package lib

import (
	"strings"
	"time"

	"github.com/nisrulz/commit-pilot/src/lib/provider"
)

// probeTimeout bounds how long each reachability probe may wait for a response
// so a dead endpoint does not stall the run.
const probeTimeout = 3 * time.Second

// localProbeOrder is the order in which the bundled local providers are probed
// when no provider was explicitly configured.
var localProbeOrder = []string{"lmstudio", "ollama", "unsloth"}

// probeCandidate is one provider endpoint that the detection step checks.
type probeCandidate struct {
	name     string
	base     string
	probeURL string
}

// probeURLFor returns the URL used to check reachability for display. Unsloth
// is probed at its UI health route because its OpenAI API is key-protected.
func probeURLFor(name, base string) string {
	if name == "unsloth" {
		return strings.TrimSuffix(strings.TrimRight(base, "/"), "/v1") + "/health"
	}
	return strings.TrimRight(base, "/") + "/models"
}

// providerForModel returns the bundled provider whose default model equals the
// configured model, if any.
func providerForModel(model string) string {
	for _, name := range localProbeOrder {
		if ProviderDefaults[name] == model {
			return name
		}
	}
	return ""
}

// prepend moves name to the front of names, dropping it from its old position.
func prepend(name string, names []string) []string {
	rest := make([]string, 0, len(names))
	for _, n := range names {
		if n != name {
			rest = append(rest, n)
		}
	}
	return append([]string{name}, rest...)
}

// probeCandidatesFor builds the ordered list of endpoints to probe. A provider
// chosen by name is probed exactly as configured. A customized endpoint with no
// named provider is probed with each bundled provider to identify what runs
// there, checking Unsloth first because its /health route is the only one that
// tells it apart from a plain /models server. Otherwise the bundled local
// providers are probed at their own defaults, preferring the one whose model
// matches the configured model.
func probeCandidatesFor(cfg Config) []probeCandidate {
	if cfg.ProviderExplicit {
		return []probeCandidate{{cfg.Provider, cfg.APIBase, probeURLFor(cfg.Provider, cfg.APIBase)}}
	}
	if cfg.APIBaseExplicit {
		base := cfg.APIBase
		return []probeCandidate{
			{"unsloth", base, probeURLFor("unsloth", base)},
			{"lmstudio", base, probeURLFor("lmstudio", base)},
			{"ollama", base, probeURLFor("ollama", base)},
		}
	}
	names := localProbeOrder
	if preferred := providerForModel(cfg.Model); preferred != "" {
		names = prepend(preferred, names)
	}
	candidates := make([]probeCandidate, 0, len(names))
	for _, name := range names {
		base := KnownProviders[name]
		candidates = append(candidates, probeCandidate{name, base, probeURLFor(name, base)})
	}
	return candidates
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
// bundled local servers and selects the first reachable one, updating cfg's
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
