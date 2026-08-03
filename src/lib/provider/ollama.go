package provider

// ollamaProvider is the implementation for Ollama's local OpenAI-compatible API.
type ollamaProvider struct{ openAICompat }

func init() { register(ollamaProvider{}) }

func (ollamaProvider) Name() string { return "ollama" }
