package provider

// openAIProvider is the implementation for OpenAI-compatible hosted endpoints.
type openAIProvider struct{ openAICompat }

func init() { register(openAIProvider{}) }

func (openAIProvider) Name() string { return "openai" }
