package lib

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileSummary struct {
	File    string   `json:"file"`
	Summary string   `json:"summary"`
	Changes []string `json:"changes"`
}

func SummariesPath() string {
	date := time.Now().Format("2006-01-02")

	name := "unknown"
	if out, err := GitRun("rev-parse", "--show-toplevel"); err == nil {
		name = filepath.Base(strings.TrimSpace(out))
	}

	id := make([]byte, 4)
	if _, err := rand.Read(id); err != nil {
		id = []byte{0, 0, 0, 1}
	}

	dir := filepath.Join(configDir(), "tmp")
	os.MkdirAll(dir, 0700)

	return filepath.Join(dir, fmt.Sprintf("git_diff_summaries_%s_%s_%s.json", date, name, hex.EncodeToString(id)))
}

func SummarizeChanges(cfg Config, tmpl string, files []FileDiff, dst string) (string, error) {
	if cfg.ContextWindow <= 0 {
		cfg.ContextWindow = defaultContextWindow
	}
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	defer cancel()
	cfg.Context = ctx

	type jobResult struct {
		index   int
		summary FileSummary
	}
	jobs := make(chan int, len(files))
	results := make(chan jobResult, len(files))
	for i := range files {
		jobs <- i
	}
	close(jobs)

	// A dead provider fails the run; any other per-file failure degrades to a
	// local summary so the rest of the change set still reaches the plan.
	var (
		fatalOnce sync.Once
		fatalErr  error
	)
	workers := min(4, len(files))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				fd := files[i]
				PrintProcessing(fmt.Sprintf("Summarizing %s (%d/%d)...", fd.Path, i+1, len(files)))
				summary, err := summarizeFile(cfg, tmpl, fd)
				if err != nil {
					if IsUnreachable(err) {
						fatalOnce.Do(func() {
							fatalErr = err
							cancel()
						})
						continue
					}
					Warningf("could not summarize %s: %v", sanitizePath(fd.Path), err)
					summary = LocalSummary(fd)
				}
				results <- jobResult{index: i, summary: summary}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	summaries := make([]FileSummary, len(files))
	for result := range results {
		summaries[result.index] = result.summary
	}
	if fatalErr != nil {
		return "", fatalErr
	}

	out, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal summaries: %w", err)
	}
	if err := os.WriteFile(dst, out, 0600); err != nil {
		Warningf("warning: could not write summaries: %v", err)
	}
	return string(out), nil
}

func summarizeFile(cfg Config, tmpl string, file FileDiff) (FileSummary, error) {
	batches := SplitFilesIntoBatches(tmpl, []FileDiff{file}, cfg.ContextWindow)
	merged := FileSummary{File: file.Path}
	var summaries []string
	for i, batch := range batches {
		if len(batches) > 1 {
			PrintProcessing(fmt.Sprintf("Chunk %d/%d of %s", i+1, len(batches), file.Path))
		}
		prompt := FormatPrompt(tmpl, []string{file.Path}, batch[0].Diff)
		result, err := callWithTruncationRetry(prompt, cfg, DefaultMaxTokens)
		if err != nil {
			return FileSummary{}, fmt.Errorf("summarize %s: %w", file.Path, err)
		}
		summary := ParseSummary(result, file.Path)
		if summary.Summary != "" {
			summaries = append(summaries, summary.Summary)
		}
		merged.Changes = append(merged.Changes, summary.Changes...)
	}
	merged.Summary = sanitizeText(strings.Join(summaries, "\n"), MaxSummaryLen)
	return merged, nil
}

// LocalSummary describes a file from its diff alone, with no model call. It is
// the last-resort summary when the provider fails for that file, so the run can
// still produce commits from the change set.
func LocalSummary(file FileDiff) FileSummary {
	added, removed := diffLineCounts(file.Diff)
	summary := fmt.Sprintf("%s: %d lines added, %d lines removed", file.Path, added, removed)
	return FileSummary{
		File:    file.Path,
		Summary: summary,
		Changes: []string{summary},
	}
}

// diffLineCounts counts added and removed lines in a unified diff. It ignores
// the "+++"/"---" file headers so they are not counted as changes.
func diffLineCounts(diff string) (added, removed int) {
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			removed++
		}
	}
	return added, removed
}

func ParseSummary(text, file string) FileSummary {
	raw, err := ExtractJSON(text)
	if err != nil {
		return FallbackSummary(text, file)
	}

	var s FileSummary
	if err := json.Unmarshal(raw, &s); err != nil {
		return FallbackSummary(text, file)
	}

	s.File = file
	s.Summary = sanitizeText(s.Summary, MaxSummaryLen)
	for i := range s.Changes {
		s.Changes[i] = sanitizeText(s.Changes[i], MaxSummaryLen)
	}
	return s
}

const (
	MaxDumpLen    = 300
	MaxSummaryLen = 500
)

func FallbackSummary(text, file string) FileSummary {
	dump := truncateSanitizedText(text, MaxDumpLen)
	// First line only in the error message, full text goes into the summary
	first := strings.SplitN(dump, "\n", 2)[0]
	if len([]rune(first)) > 200 {
		first = string([]rune(first)[:200]) + "..."
	}
	Warningf("could not parse summary for %s", sanitizePath(file))
	fmt.Fprintf(os.Stderr, "    response: %s\n", first)

	summary := truncateSanitizedText(text, MaxSummaryLen)
	return FileSummary{File: file, Summary: summary}
}

func truncateSanitizedText(text string, maxRunes int) string {
	clean := []rune(sanitizeText(text, 0))
	if len(clean) <= maxRunes {
		return string(clean)
	}
	return string(clean[:maxRunes]) + "..."
}
