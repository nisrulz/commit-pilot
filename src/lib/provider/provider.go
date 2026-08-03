// Package provider holds the pluggable model-serving backends that commit-pilot
// talks to. Each provider lives in its own file and registers itself on init,
// so the rest of the tool stays provider-agnostic.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Doer abstracts HTTP calls so tests can inject a fake client.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// MaxResponseSize caps how much of a provider response is read.
const MaxResponseSize = 1 << 20

// Provider is a pluggable model-serving backend. Each implementation knows how
// to verify its own health and list the models it exposes.
type Provider interface {
	// Name is the provider identifier used in OPENAI_PROVIDER and the
	// KnownProviders map.
	Name() string

	// Probe verifies the provider at base is running. It must not depend on a
	// valid API key, so an unauthenticated check can still report reachability.
	Probe(ctx context.Context, base, key string, client Doer) error

	// ListModels returns the model IDs the provider currently exposes.
	ListModels(ctx context.Context, base, key string, client Doer) ([]string, error)
}

// registeredProviders holds the provider implementations, each registering
// itself in its own file. Adding a new provider is a matter of adding a file.
var registeredProviders = map[string]Provider{}

// register adds a provider to the registry by its Name.
func register(p Provider) {
	registeredProviders[p.Name()] = p
}

// New returns the registered implementation for name, falling back to the
// OpenAI-compatible provider for unknown or empty names.
func New(name string) Provider {
	if p, ok := registeredProviders[name]; ok {
		return p
	}
	return openAIProvider{}
}

// Known reports whether name identifies a registered provider. The empty name
// is not known; callers treat it as "use the default".
func Known(name string) bool {
	_, ok := registeredProviders[name]
	return ok
}

// Names returns the registered provider names in sorted order, for error
// messages and help text.
func Names() []string {
	names := make([]string, 0, len(registeredProviders))
	for name := range registeredProviders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// openAICompat provides the standard OpenAI-compatible behavior: reachability
// is probed against /models and model IDs are listed from it. Providers that
// behave exactly like OpenAI's API embed this and only add a name.
type openAICompat struct{}

func (openAICompat) Probe(ctx context.Context, base, key string, client Doer) error {
	if err := ValidateURL(base); err != nil {
		return err
	}
	return probeReachable(ctx, base, key, client, "/models")
}

func (openAICompat) ListModels(ctx context.Context, base, key string, client Doer) ([]string, error) {
	return listProviderModels(ctx, base, key, client, "/models")
}

// probeReachable performs a GET against base+path and returns an error only when
// the server cannot be reached. Any HTTP response, including an auth error,
// means the server is up.
func probeReachable(ctx context.Context, base, key string, client Doer, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := doer(client).Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s", base)
	}
	// Drain before closing so the connection is returned to the pool instead of
	// being discarded.
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// listProviderModels fetches base+path and parses the model IDs from either the
// OpenAI "data[].id" shape or the llama.cpp "models[].key" shape.
func listProviderModels(ctx context.Context, base, key string, client Doer, path string) ([]string, error) {
	if err := ValidateURL(base); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := doer(client).Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s", base)
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

// doer returns the injected client, falling back to a client with a 10 second
// timeout that never follows redirects.
func doer(client Doer) Doer {
	if client != nil {
		return client
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ValidateURL rejects malformed endpoints and plain HTTP outside the local
// machine so repository data and API keys are never sent in clear text.
func ValidateURL(apiBase string) error {
	u, err := url.Parse(apiBase)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid provider URL %q", apiBase)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("provider URL must not contain credentials, a query, or a fragment")
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return fmt.Errorf("refusing plain HTTP provider %s; use HTTPS or a loopback address", u.Host)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
