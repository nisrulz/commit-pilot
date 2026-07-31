package lib

import (
	"fmt"
	"os"
	"strings"
)

// SplitFilesIntoBatches divides files into groups that each fit the model's
// context window. When even a single file is too large, its diff is split into
// chunks that each fit within the available token budget.
func SplitFilesIntoBatches(template string, files []FileDiff, contextWindow int) [][]FileDiff {
	if len(files) == 0 {
		return nil
	}

	if CanFitInContext(template, files, contextWindow) {
		return [][]FileDiff{files}
	}

	// Binary-search the largest batch size that still fits the context window.
	low, high := 1, len(files)
	bestSize := 1

	for low <= high {
		mid := (low + high) / 2
		if CanFitInContext(template, files[:mid], contextWindow) {
			bestSize = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	if !CanFitInContext(template, files[:1], contextWindow) {
		diffBudget := AvailableDiffTokens(template, contextWindow)
		if diffBudget <= 0 {
			fmt.Fprintf(os.Stderr, "  %s Context window too small for any diff content\n", yellow("!"))
			return [][]FileDiff{files}
		}

		var out [][]FileDiff
		for _, f := range files {
			chunks := SplitFileIntoChunks(f, diffBudget)
			if len(chunks) > 1 {
				for _, c := range chunks {
					out = append(out, []FileDiff{c})
				}
			} else if len(chunks) == 1 {
				out = append(out, chunks)
			} else {
				fmt.Fprintf(os.Stderr, "  %s File %s too large to chunk, sending truncated diff\n",
					yellow("!"), f.Path)
				truncated := TruncateDiff(f.Diff)
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
		if tok+lt > perChunkBudget && len(cur) > 0 {
			flush()
		}
		cur = append(cur, line)
		tok += lt
	}
	flush()
	return chunks
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
