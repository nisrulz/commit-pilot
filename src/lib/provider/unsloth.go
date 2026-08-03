package provider

import (
	"context"
	"strings"
)

// unslothProvider is the implementation for Unsloth Studio, whose OpenAI API is
// key-protected (401 without a key). Reachability is probed against the UI
// health route at the server root, which responds without authentication, so an
// unauthenticated check can still detect a running server.
type unslothProvider struct{}

func init() { register(unslothProvider{}) }

func (unslothProvider) Name() string { return "unsloth" }

// Probe checks the UI health route. The /v1 prefix is stripped first because
// Unsloth serves /health from the server root, not under the API path.
func (unslothProvider) Probe(ctx context.Context, base, key string, client Doer) error {
	if err := ValidateURL(base); err != nil {
		return err
	}
	root := strings.TrimSuffix(strings.TrimRight(base, "/"), "/v1")
	return probeReachable(ctx, root, key, client, "/health")
}

// ListModels uses the standard OpenAI model endpoint, sending the API key when
// one is configured.
func (unslothProvider) ListModels(ctx context.Context, base, key string, client Doer) ([]string, error) {
	return listProviderModels(ctx, base, key, client, "/models")
}
