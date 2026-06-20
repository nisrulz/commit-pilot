package main

import (
	"strings"
	"unicode"
)

// estimateTokens provides a rough estimate of token count.
// Uses a simple heuristic: ~4 characters per token for English text,
// with adjustments for code/diff content which tends to have more tokens.
func estimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	runes := []rune(text)
	totalRunes := len(runes)

	// Base estimate: ~4 characters per token
	baseTokens := totalRunes / 4

	// Adjust for code-like content (more tokens due to special chars)
	codeRunes := 0
	for _, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsSpace(r) && !unicode.IsDigit(r) {
			codeRunes++
		}
	}

	// Code content tends to have ~20% more tokens due to special characters
	codeRatio := float64(codeRunes) / float64(totalRunes)
	adjustedTokens := float64(baseTokens) * (1.0 + codeRatio*0.2)

	return int(adjustedTokens) + 1 // Add 1 to avoid rounding to 0
}

// estimatePromptTokens estimates the total tokens for a prompt with files and diffs
func estimatePromptTokens(template string, files []FileDiff) int {
	// Start with template tokens
	total := estimateTokens(template)

	// Add file list tokens (JSON array)
	fileNames := make([]string, len(files))
	for i, f := range files {
		fileNames[i] = f.Path
	}
	total += estimateTokens(strings.Join(fileNames, ", ")) + 10 // +10 for JSON brackets

	// Add diff tokens
	for _, f := range files {
		// "File: <path>\n" + diff + "\n---\n"
		total += estimateTokens(f.Path) + estimateTokens(f.Diff) + 10
	}

	return total
}

// canFitInContext checks if a prompt with given files fits within the context window
// Reserves tokens for: system prompt overhead (~200), response (~4096), and safety margin (~500)
func canFitInContext(template string, files []FileDiff, contextWindow int) bool {
	const (
		systemOverhead = 200  // Tokens for system instructions
		responseTokens = 4096 // Reserved for response
		safetyMargin   = 500  // General safety margin
	)

	reserved := systemOverhead + responseTokens + safetyMargin
	available := contextWindow - reserved

	if available <= 0 {
		return false
	}

	estimated := estimatePromptTokens(template, files)
	return estimated <= available
}

// splitFilesIntoBatches splits files into batches that fit within the context window
func splitFilesIntoBatches(template string, files []FileDiff, contextWindow int) [][]FileDiff {
	if len(files) == 0 {
		return nil
	}

	// Check if all files fit
	if canFitInContext(template, files, contextWindow) {
		return [][]FileDiff{files}
	}

	// Binary search for maximum batch size
	low, high := 1, len(files)
	bestSize := 1

	for low <= high {
		mid := (low + high) / 2
		if canFitInContext(template, files[:mid], contextWindow) {
			bestSize = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	// Split into batches
	var batches [][]FileDiff
	for i := 0; i < len(files); i += bestSize {
		end := i + bestSize
		if end > len(files) {
			end = len(files)
		}
		batches = append(batches, files[i:end])
	}

	return batches
}
