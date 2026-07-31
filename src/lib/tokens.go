package lib

import (
	"strings"
	"unicode"
)

// EstimateTokens is a rough token estimate for arbitrary text: base 4 runes
// per token, with a small bump for code-heavy content.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	totalRunes := 0
	codeRunes := 0
	for _, r := range text {
		totalRunes++
		if !unicode.IsLetter(r) && !unicode.IsSpace(r) && !unicode.IsDigit(r) {
			codeRunes++
		}
	}

	baseTokens := totalRunes / 4
	codeRatio := float64(codeRunes) / float64(totalRunes)
	adjustedTokens := float64(baseTokens) * (1.0 + codeRatio*0.2)

	return int(adjustedTokens) + 1
}

// EstimatePromptTokens estimates the token cost of a full prompt built from a
// template, the file names, and the per-file diffs.
func EstimatePromptTokens(template string, files []FileDiff) int {
	total := EstimateTokens(template)

	fileNames := make([]string, len(files))
	for i, f := range files {
		fileNames[i] = f.Path
	}
	total += EstimateTokens(strings.Join(fileNames, ", ")) + 10

	for _, f := range files {
		total += EstimateTokens(f.Path) + EstimateTokens(f.Diff) + 10
	}

	return total
}

const (
	systemOverhead = 200
	responseTokens = 4096
	safetyMargin   = 500
	reservedTokens = systemOverhead + responseTokens + safetyMargin
)

// CanFitInContext reports whether the template plus diffs fit within the
// model's context window, after reserving tokens for the response and safety.
func CanFitInContext(template string, files []FileDiff, contextWindow int) bool {
	available := contextWindow - reservedTokens
	if available <= 0 {
		return false
	}
	estimated := EstimatePromptTokens(template, files)
	return estimated <= available
}

// AvailableDiffTokens returns how many tokens remain for diff content after
// accounting for the template and the reserved response/safety tokens.
func AvailableDiffTokens(template string, contextWindow int) int {
	budget := contextWindow - reservedTokens - EstimateTokens(template)
	if budget < 0 {
		return 0
	}
	return budget
}
