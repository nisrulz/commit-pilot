package provider

// customProvider is the implementation for user-configured OpenAI-compatible
// endpoints that do not match one of the named backends. It behaves exactly
// like the OpenAI provider and always pairs with an explicit OPENAI_BASE_URL.
type customProvider struct{ openAICompat }

func init() { register(customProvider{}) }

func (customProvider) Name() string { return "custom" }
