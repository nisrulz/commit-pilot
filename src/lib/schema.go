package lib

import "strings"

// JSONSchema is the strict response schema matched to a built-in prompt.
type JSONSchema struct {
	Name string
	Doc  map[string]any
}

// jsonSchemaForPrompt returns the strict JSON schema matching the response
// shape the prompt asks for, or nil when the prompt maps to no known shape
// (custom prompts fall back to json_object mode).
func jsonSchemaForPrompt(prompt string) *JSONSchema {
	switch {
	case strings.Contains(prompt, "git commit planner"):
		return &JSONSchema{Name: "commit_plan", Doc: planSchema()}
	case strings.Contains(prompt, "git commit organizer"):
		return &JSONSchema{Name: "commit_group", Doc: commitGroupSchema()}
	case strings.Contains(prompt, "git commit message generator"):
		return &JSONSchema{Name: "commit_message", Doc: commitMessageSchema()}
	case strings.Contains(prompt, "code change summarizer"):
		return &JSONSchema{Name: "file_summary", Doc: fileSummarySchema()}
	default:
		return nil
	}
}

// objectSchema builds a strict JSON Schema object with exactly the listed
// properties, all required and no extra keys allowed.
func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             required,
	}
}

func stringProperty() map[string]any {
	return map[string]any{"type": "string"}
}

func stringArrayProperty() map[string]any {
	return map[string]any{"type": "array", "items": stringProperty()}
}

func commitMessageSchema() map[string]any {
	return objectSchema(map[string]any{
		"subject":     stringProperty(),
		"description": stringProperty(),
	}, []string{"subject", "description"})
}

func commitGroupSchema() map[string]any {
	return objectSchema(map[string]any{
		"subject":     stringProperty(),
		"description": stringProperty(),
		"files":       stringArrayProperty(),
	}, []string{"subject", "description", "files"})
}

func planSchema() map[string]any {
	return map[string]any{
		"type":  "array",
		"items": commitGroupSchema(),
	}
}

func fileSummarySchema() map[string]any {
	return objectSchema(map[string]any{
		"summary": stringProperty(),
		"changes": stringArrayProperty(),
	}, []string{"summary", "changes"})
}
