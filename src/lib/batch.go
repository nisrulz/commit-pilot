package lib

import (
	"fmt"
	"strings"
)

// SplitFilesIntoBatches divides files into groups that each fit the model's
// context window. When even a single file is too large, its diff is split into
// chunks that each fit within the available token budget.
func SplitFilesIntoBatches(template string, files []FileDiff, contextWindow int) [][]FileDiff {
	if len(files) == 0 {
		return nil
	}

	diffBudget := AvailableDiffTokens(template, contextWindow)
	if diffBudget <= 0 {
		Warning("Context window too small for any diff content")
		return [][]FileDiff{files}
	}

	var batches [][]FileDiff
	var current []FileDiff
	flush := func() {
		if len(current) > 0 {
			batches = append(batches, current)
			current = nil
		}
	}

	for _, file := range files {
		if !CanFitInContext(template, []FileDiff{file}, contextWindow) {
			flush()
			chunks := SplitFileIntoChunks(file, diffBudget)
			if len(chunks) == 0 {
				chunks = []FileDiff{{Path: file.Path, Diff: TruncateDiff(file.Diff)}}
			}
			for _, chunk := range chunks {
				batches = append(batches, []FileDiff{chunk})
			}
			continue
		}

		candidate := append(append([]FileDiff(nil), current...), file)
		if len(current) > 0 && !CanFitInContext(template, candidate, contextWindow) {
			flush()
		}
		current = append(current, file)
	}
	flush()
	return batches
}

// SplitFileIntoChunks splits one file's diff into line-aligned chunks that each
// fit within the available token budget.
func SplitFileIntoChunks(fd FileDiff, diffBudget int) []FileDiff {
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
		lt := EstimateTokens(line) + 1
		if lt > perChunkBudget {
			flush()
			for _, part := range splitByTokenBudget(line, perChunkBudget-1) {
				cur = []string{part}
				tok = EstimateTokens(part) + 1
				flush()
			}
			continue
		}
		if tok+lt > perChunkBudget && len(cur) > 0 {
			flush()
		}
		cur = append(cur, line)
		tok += lt
	}
	flush()
	return chunks
}

func splitByTokenBudget(text string, budget int) []string {
	if budget <= 0 {
		return nil
	}
	runes := []rune(text)
	var parts []string
	for start := 0; start < len(runes); {
		low, high, best := 1, len(runes)-start, 1
		for low <= high {
			mid := (low + high) / 2
			if EstimateTokens(string(runes[start:start+mid])) <= budget {
				best = mid
				low = mid + 1
			} else {
				high = mid - 1
			}
		}
		parts = append(parts, string(runes[start:start+best]))
		start += best
	}
	return parts
}

// TruncateKeepLines is the number of head/tail lines kept when truncating.
const TruncateKeepLines = 50

// TruncateDiff shortens an oversized diff, keeping its head and tail with a
// marker for the truncated middle.
func TruncateDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	if len(lines) <= TruncateKeepLines*2+5 {
		return diff
	}

	head := strings.Join(lines[:TruncateKeepLines], "\n")
	tail := strings.Join(lines[len(lines)-TruncateKeepLines:], "\n")
	marker := fmt.Sprintf("\n[... %d lines truncated ...]\n", len(lines)-TruncateKeepLines*2)
	return head + marker + tail
}
