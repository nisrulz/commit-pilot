package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

func estimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	runes := []rune(text)
	totalRunes := len(runes)
	baseTokens := totalRunes / 4

	codeRunes := 0
	for _, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsSpace(r) && !unicode.IsDigit(r) {
			codeRunes++
		}
	}

	codeRatio := float64(codeRunes) / float64(totalRunes)
	adjustedTokens := float64(baseTokens) * (1.0 + codeRatio*0.2)

	return int(adjustedTokens) + 1
}

func estimatePromptTokens(template string, files []FileDiff) int {
	total := estimateTokens(template)

	fileNames := make([]string, len(files))
	for i, f := range files {
		fileNames[i] = f.Path
	}
	total += estimateTokens(strings.Join(fileNames, ", ")) + 10

	for _, f := range files {
		total += estimateTokens(f.Path) + estimateTokens(f.Diff) + 10
	}

	return total
}

const (
	systemOverhead = 200
	responseTokens = 4096
	safetyMargin   = 500
	reservedTokens = systemOverhead + responseTokens + safetyMargin
)

func canFitInContext(template string, files []FileDiff, contextWindow int) bool {
	available := contextWindow - reservedTokens
	if available <= 0 {
		return false
	}
	estimated := estimatePromptTokens(template, files)
	return estimated <= available
}

func availableDiffTokens(template string, contextWindow int) int {
	budget := contextWindow - reservedTokens - estimateTokens(template)
	if budget < 0 {
		return 0
	}
	return budget
}

func splitFileIntoChunks(fd FileDiff, diffBudget int) []FileDiff {
	if diffBudget <= 0 {
		return nil
	}

	const fileHeaderOverhead = 60
	perChunkBudget := diffBudget - fileHeaderOverhead
	if perChunkBudget <= 0 {
		return nil
	}

	lines := strings.Split(fd.Diff, "\n")
	var chunks []FileDiff
	var cur []string
	tok := 0

	flush := func() {
		if len(cur) > 0 {
			chunks = append(chunks, FileDiff{Path: fd.Path, Diff: strings.Join(cur, "\n")})
			cur = nil
			tok = 0
		}
	}

	for _, line := range lines {
		lt := estimateTokens(line) + 1
		if tok+lt > perChunkBudget && len(cur) > 0 {
			flush()
		}
		cur = append(cur, line)
		tok += lt
	}
	flush()
	return chunks
}

func splitFilesIntoBatches(template string, files []FileDiff, contextWindow int) [][]FileDiff {
	if len(files) == 0 {
		return nil
	}

	if canFitInContext(template, files, contextWindow) {
		return [][]FileDiff{files}
	}

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

	if !canFitInContext(template, files[:1], contextWindow) {
		diffBudget := availableDiffTokens(template, contextWindow)
		if diffBudget <= 0 {
			fmt.Fprintf(os.Stderr, "  %s Context window too small for any diff content\n", yellow("!"))
			return [][]FileDiff{files}
		}

		var out [][]FileDiff
		for _, f := range files {
			chunks := splitFileIntoChunks(f, diffBudget)
			if len(chunks) > 1 {
				for _, c := range chunks {
					out = append(out, []FileDiff{c})
				}
			} else if len(chunks) == 1 {
				out = append(out, chunks)
			} else {
				fmt.Fprintf(os.Stderr, "  %s File %s too large to chunk, sending truncated diff\n",
					yellow("!"), f.Path)
				truncated := truncateDiff(f.Diff)
				out = append(out, []FileDiff{{Path: f.Path, Diff: truncated}})
			}
		}
		return out
	}

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

const truncateKeepLines = 50

func truncateDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	if len(lines) <= truncateKeepLines*2+5 {
		return diff
	}

	head := strings.Join(lines[:truncateKeepLines], "\n")
	tail := strings.Join(lines[len(lines)-truncateKeepLines:], "\n")
	marker := fmt.Sprintf("\n[... %d lines truncated ...]\n", len(lines)-truncateKeepLines*2)
	return head + marker + tail
}
