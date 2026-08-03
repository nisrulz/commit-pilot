package provider

// lmstudioProvider is the implementation for LM Studio's local
// OpenAI-compatible API.
type lmstudioProvider struct{ openAICompat }

func init() { register(lmstudioProvider{}) }

func (lmstudioProvider) Name() string { return "lmstudio" }
