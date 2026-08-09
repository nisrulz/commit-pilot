package provider

// openaiCompatProvider is the implementation for any OpenAI-compatible
// endpoint. It is used for hosted APIs and local servers such as LM Studio or
// Unsloth Studio; users pair it with a base_url in the config file.
type openaiCompatProvider struct{ openAICompat }

func init() { register(openaiCompatProvider{}) }

func (openaiCompatProvider) Name() string { return "openai_compat" }
