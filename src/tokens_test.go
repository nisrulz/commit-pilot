package main

import (
	"strings"
	"testing"
)

func generateHugeDiff(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		if i%3 == 0 {
			b.WriteString("+func newFunction() { return 42; } // added at line ")
		} else if i%3 == 1 {
			b.WriteString("-func oldFunction() { return 7; } // removed at line ")
		} else {
			b.WriteString(" // unchanged context line for padding ")
		}
		b.WriteString(string(rune('A' + (i % 26))))
		for j := 0; j < 20; j++ {
			b.WriteString(" x")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestSplitFileIntoChunks_smallDiff(t *testing.T) {
	fd := FileDiff{Path: "main.go", Diff: "line1\nline2\nline3"}
	chunks := splitFileIntoChunks(fd, 100000)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Path != "main.go" {
		t.Fatalf("expected path main.go, got %s", chunks[0].Path)
	}
}

func TestSplitFileIntoChunks_hugeDiff(t *testing.T) {
	diff := generateHugeDiff(5000)
	fd := FileDiff{Path: "huge.go", Diff: diff}
	diffBudget := 5000
	chunks := splitFileIntoChunks(fd, diffBudget)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for huge diff (5000 lines, budget=%d), got %d", diffBudget, len(chunks))
	}
	for i, c := range chunks {
		if c.Path != "huge.go" {
			t.Fatalf("chunk %d: expected path huge.go, got %s", i, c.Path)
		}
		tok := estimateTokens(c.Diff)
		if tok > diffBudget {
			t.Fatalf("chunk %d: %d tokens exceeds budget %d", i, tok, diffBudget)
		}
	}
	totalChunkTokens := 0
	for _, c := range chunks {
		totalChunkTokens += estimateTokens(c.Diff)
	}
	totalOrigTokens := estimateTokens(diff)
	if totalChunkTokens > totalOrigTokens*2 {
		t.Fatalf("chunking overhead too high: original=%d, sum(chunks)=%d", totalOrigTokens, totalChunkTokens)
	}
}

func TestSplitFileIntoChunks_exactBoundary(t *testing.T) {
	diff := "line1\nline2\nline3\n"
	fd := FileDiff{Path: "exact.go", Diff: diff}
	budget := estimateTokens(diff) + 100
	chunks := splitFileIntoChunks(fd, budget)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk when diff fits exactly, got %d", len(chunks))
	}
}

func TestSplitFileIntoChunks_zeroBudget(t *testing.T) {
	fd := FileDiff{Path: "zero.go", Diff: "some content"}
	chunks := splitFileIntoChunks(fd, 0)
	if chunks != nil {
		t.Fatalf("expected nil for zero budget, got %v", chunks)
	}
}

func TestSplitFileIntoChunks_negativeBudget(t *testing.T) {
	fd := FileDiff{Path: "neg.go", Diff: "some content"}
	chunks := splitFileIntoChunks(fd, -1)
	if chunks != nil {
		t.Fatalf("expected nil for negative budget, got %v", chunks)
	}
}

func TestSplitFilesIntoBatches_fitsInContext(t *testing.T) {
	files := []FileDiff{
		{Path: "a.go", Diff: "small change"},
	}
	batches := splitFilesIntoBatches("template {files} {diff}", files, 100000)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 1 {
		t.Fatalf("expected 1 file in batch, got %d", len(batches[0]))
	}
}

func TestSplitFilesIntoBatches_triggersChunkingForHugeFile(t *testing.T) {
	diff := generateHugeDiff(500)
	files := []FileDiff{
		{Path: "huge.go", Diff: diff},
	}
	tmpl := "template {files} {diff}"
	contextWindow := 8192
	batches := splitFilesIntoBatches(tmpl, files, contextWindow)
	if len(batches) == 0 {
		t.Fatal("expected at least one batch")
	}
	totalChunks := 0
	for _, b := range batches {
		totalChunks += len(b)
	}
	if totalChunks < 2 {
		t.Fatalf("expected huge file to be chunked into multiple pieces, got %d total across %d batches",
			totalChunks, len(batches))
	}
	for _, b := range batches {
		if !canFitInContext(tmpl, b, contextWindow) {
			t.Fatalf("batch with %d files and %d total lines does not fit in context window %d",
				len(b), totalDiffLines(b), contextWindow)
		}
	}
}

func TestSplitFilesIntoBatches_chunkedPathPreserved(t *testing.T) {
	diff := generateHugeDiff(500)
	files := []FileDiff{
		{Path: "huge.go", Diff: diff},
	}
	tmpl := "template {files} {diff}"
	contextWindow := 8192
	batches := splitFilesIntoBatches(tmpl, files, contextWindow)
	for _, b := range batches {
		for _, f := range b {
			if f.Path != "huge.go" {
				t.Fatalf("expected all chunks to have path huge.go, got %s", f.Path)
			}
		}
	}
}

func TestSplitFilesIntoBatches_mixedFileSizes(t *testing.T) {
	small := FileDiff{Path: "small.go", Diff: "tiny change"}
	diff := generateHugeDiff(500)
	huge := FileDiff{Path: "huge.go", Diff: diff}
	files := []FileDiff{small, huge}
	tmpl := "template {files} {diff}"
	contextWindow := 8192
	batches := splitFilesIntoBatches(tmpl, files, contextWindow)
	if len(batches) == 0 {
		t.Fatal("expected at least one batch")
	}
	smallFound := false
	hugeFound := false
	for _, b := range batches {
		for _, f := range b {
			if f.Path == "small.go" {
				smallFound = true
			}
			if f.Path == "huge.go" {
				hugeFound = true
			}
		}
	}
	if !smallFound {
		t.Fatal("small.go should appear in batches")
	}
	if !hugeFound {
		t.Fatal("huge.go should appear in batches (chunked)")
	}
}

func totalDiffLines(batch []FileDiff) int {
	count := 0
	for _, f := range batch {
		count += len(strings.Split(f.Diff, "\n"))
	}
	return count
}

func TestIsChunkedBatch(t *testing.T) {
	tests := []struct {
		name  string
		batch []FileDiff
		want  bool
	}{
		{
			name:  "single file not chunked",
			batch: []FileDiff{{Path: "a.go", Diff: "diff1"}},
			want:  false,
		},
		{
			name: "empty not chunked",
			batch: []FileDiff{},
			want:  false,
		},
		{
			name: "same file multiple chunks",
			batch: []FileDiff{
				{Path: "big.go", Diff: "chunk1"},
				{Path: "big.go", Diff: "chunk2"},
				{Path: "big.go", Diff: "chunk3"},
			},
			want: true,
		},
		{
			name: "different files not chunked",
			batch: []FileDiff{
				{Path: "a.go", Diff: "diff1"},
				{Path: "b.go", Diff: "diff2"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isChunkedBatch(tt.batch)
			if got != tt.want {
				t.Fatalf("isChunkedBatch(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGroupChunkedBatches_normalBatches(t *testing.T) {
	batches := [][]FileDiff{
		{{Path: "a.go", Diff: "diff1"}},
		{{Path: "b.go", Diff: "diff2"}},
	}
	got := groupChunkedBatches(batches)
	if len(got) != 2 {
		t.Fatalf("expected 2 groups for non-chunked batches, got %d", len(got))
	}
}

func TestGroupChunkedBatches_consecutiveChunks(t *testing.T) {
	batches := [][]FileDiff{
		{{Path: "big.go", Diff: "chunk1"}},
		{{Path: "big.go", Diff: "chunk2"}},
		{{Path: "big.go", Diff: "chunk3"}},
	}
	got := groupChunkedBatches(batches)
	if len(got) != 1 {
		t.Fatalf("expected 1 group for consecutive chunks of same file, got %d", len(got))
	}
	if len(got[0]) != 3 {
		t.Fatalf("expected 3 items in group, got %d", len(got[0]))
	}
}

func TestGroupChunkedBatches_interleavedChunks(t *testing.T) {
	batches := [][]FileDiff{
		{{Path: "big.go", Diff: "chunk1"}},
		{{Path: "small.go", Diff: "diff"}},
		{{Path: "big.go", Diff: "chunk2"}},
	}
	got := groupChunkedBatches(batches)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups for interleaved chunks, got %d", len(got))
	}
}

func TestGroupChunkedBatches_empty(t *testing.T) {
	var batches [][]FileDiff
	got := groupChunkedBatches(batches)
	if len(got) != 0 {
		t.Fatalf("expected empty for empty input, got %d", len(got))
	}
}

func TestGroupChunkedBatches_singleBatch(t *testing.T) {
	batches := [][]FileDiff{
		{{Path: "a.go", Diff: "diff"}},
	}
	got := groupChunkedBatches(batches)
	if len(got) != 1 {
		t.Fatalf("expected 1 group for single batch, got %d", len(got))
	}
}

func TestEstimateTokens_basic(t *testing.T) {
	text := "hello world"
	tok := estimateTokens(text)
	if tok <= 0 {
		t.Fatalf("expected positive token count, got %d", tok)
	}
}

func TestEstimateTokens_codeHeavy(t *testing.T) {
	code := "func (x *Struct) Method(a, b, c int) (result *Thing, err error) { return nil, nil }"
	plain := "the quick brown fox jumps over the lazy dog"
	codeTokens := estimateTokens(code)
	plainTokens := estimateTokens(plain)
	if codeTokens < plainTokens {
		t.Fatalf("expected code-heavy text (%d tokens) to have >= plain text (%d tokens)",
			codeTokens, plainTokens)
	}
}

func TestEstimateTokens_empty(t *testing.T) {
	if tok := estimateTokens(""); tok != 0 {
		t.Fatalf("expected 0 for empty string, got %d", tok)
	}
}

func TestEstimatePromptTokens(t *testing.T) {
	files := []FileDiff{
		{Path: "a.go", Diff: "+func foo()\n"},
		{Path: "b.go", Diff: "-func bar()\n"},
	}
	tok := estimatePromptTokens("template {files} {diff}", files)
	if tok <= 0 {
		t.Fatalf("expected positive token count, got %d", tok)
	}
}

func TestCanFitInContext(t *testing.T) {
	files := []FileDiff{
		{Path: "a.go", Diff: "small change"},
	}
	if !canFitInContext("template", files, 10000) {
		t.Fatal("small change should fit in large context")
	}
	if canFitInContext("template", files, 1) {
		t.Fatal("small change should not fit in tiny context")
	}
}

func TestCanFitInContext_hugeDiff(t *testing.T) {
	diff := generateHugeDiff(5000)
	files := []FileDiff{
		{Path: "huge.go", Diff: diff},
	}
	if canFitInContext("template {files} {diff}", files, 2048) {
		t.Fatal("huge diff should NOT fit in small context")
	}
}

func TestAvailableDiffTokens(t *testing.T) {
	budget := availableDiffTokens("template", 10000)
	if budget <= 0 {
		t.Fatalf("expected positive budget for large context, got %d", budget)
	}
	budget = availableDiffTokens("template", 1)
	if budget != 0 {
		t.Fatalf("expected 0 budget for tiny context, got %d", budget)
	}
}

func TestTruncateDiff_smallDiff(t *testing.T) {
	diff := "line1\nline2\nline3\n"
	result := truncateDiff(diff)
	if result != diff {
		t.Fatalf("expected unchanged diff, got truncated version")
	}
}

func TestTruncateDiff_largeDiff(t *testing.T) {
	diff := generateHugeDiff(200)
	result := truncateDiff(diff)
	if result == diff {
		t.Fatal("expected large diff to be truncated")
	}
	if !strings.Contains(result, "[...") {
		t.Fatal("expected truncation marker in result")
	}
}

func TestMergeCommitGroups_single(t *testing.T) {
	groups := []CommitGroup{
		{Subject: "fix: bug", Description: "Fixed a bug", Files: []string{"a.go"}},
	}
	merged := mergeCommitGroups(groups)
	if merged.Subject != "fix: bug" {
		t.Fatalf("expected subject 'fix: bug', got '%s'", merged.Subject)
	}
}

func TestMergeCommitGroups_multiple(t *testing.T) {
	groups := []CommitGroup{
		{Subject: "feat: add X", Description: "Added X"},
		{Subject: "fix: Y", Description: "Fixed Y"},
	}
	merged := mergeCommitGroups(groups)
	if merged.Subject != "feat: add X" {
		t.Fatalf("expected first subject 'feat: add X', got '%s'", merged.Subject)
	}
	if merged.Description != "Added X\n\nFixed Y" {
		t.Fatalf("expected combined descriptions, got '%s'", merged.Description)
	}
}

func TestMergeCommitGroups_empty(t *testing.T) {
	merged := mergeCommitGroups(nil)
	if merged.Subject != "" || merged.Description != "" {
		t.Fatal("expected empty CommitGroup for nil input")
	}
}

func TestMergeCommitGroups_defaultSubject(t *testing.T) {
	groups := []CommitGroup{
		{Subject: "", Description: "Work"},
		{Subject: "", Description: "More work"},
	}
	merged := mergeCommitGroups(groups)
	if merged.Subject != "chore: update" {
		t.Fatalf("expected default subject 'chore: update', got '%s'", merged.Subject)
	}
}

func TestChunkThenGroupPipeline_hugeSingleFile(t *testing.T) {
	diff := generateHugeDiff(500)
	files := []FileDiff{
		{Path: "gigantic.go", Diff: diff},
	}
	tmpl := "template {files} {diff}"
	contextWindow := 8192
	batches := splitFilesIntoBatches(tmpl, files, contextWindow)
	if len(batches) == 0 {
		t.Fatal("expected batches from huge file")
	}
	grouped := groupChunkedBatches(batches)
	totalChunks := 0
	for _, g := range grouped {
		totalChunks += len(g)
	}
	if totalChunks < 2 {
		t.Fatalf("expected huge file to produce multiple chunks, got %d total", totalChunks)
	}
	for _, g := range grouped {
		if isChunkedBatch(g) {
			for _, c := range g {
				if c.Path != "gigantic.go" {
					t.Fatalf("chunked batch should all have same path, got %s", c.Path)
				}
			}
		}
	}
}

func TestGroupFromAIChunked_empty(t *testing.T) {
	g, err := groupFromAIChunked("template", Config{}, nil, 4096)
	if err != nil {
		t.Fatalf("unexpected error for empty chunks: %v", err)
	}
	if g.Subject != "" {
		t.Fatalf("expected empty subject for no chunks, got '%s'", g.Subject)
	}
}

func TestGroupFromAIChunked_emptyList(t *testing.T) {
	g, err := groupFromAIChunked("template", Config{}, []FileDiff{}, 4096)
	if err != nil {
		t.Fatalf("unexpected error for empty chunks: %v", err)
	}
	if g.Subject != "" {
		t.Fatalf("expected empty subject for empty list, got '%s'", g.Subject)
	}
}

func TestEstimateTokens_unicode(t *testing.T) {
	text := "hello 世界 👋 foo"
	tok := estimateTokens(text)
	if tok <= 0 {
		t.Fatalf("expected positive token count for unicode text, got %d", tok)
	}
}

func TestEstimatePromptTokens_preservesOrder(t *testing.T) {
	diff := generateHugeDiff(500)
	files := []FileDiff{
		{Path: "alpha.go", Diff: diff},
		{Path: "beta.go", Diff: diff},
	}
	tmpl := "template {files} {diff}"
	batches := splitFilesIntoBatches(tmpl, files, 8192)
	seen := make(map[string]bool)
	for _, b := range batches {
		for _, f := range b {
			seen[f.Path] = true
		}
	}
	if !seen["alpha.go"] {
		t.Fatal("alpha.go should appear in some chunk")
	}
	if !seen["beta.go"] {
		t.Fatal("beta.go should appear in some chunk")
	}
}

func BenchmarkEstimateTokens(b *testing.B) {
	text := generateHugeDiff(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		estimateTokens(text)
	}
}

func BenchmarkSplitFileIntoChunks(b *testing.B) {
	diff := generateHugeDiff(5000)
	fd := FileDiff{Path: "big.go", Diff: diff}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		splitFileIntoChunks(fd, 5000)
	}
}

func BenchmarkSplitFilesIntoBatches(b *testing.B) {
	files := []FileDiff{
		{Path: "a.go", Diff: generateHugeDiff(500)},
		{Path: "b.go", Diff: generateHugeDiff(500)},
	}
	tmpl := "template {files} {diff}"
	contextWindow := 8192
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		splitFilesIntoBatches(tmpl, files, contextWindow)
	}
}

func BenchmarkGroupChunkedBatches(b *testing.B) {
	batches := [][]FileDiff{
		{{Path: "big.go", Diff: "chunk1"}},
		{{Path: "big.go", Diff: "chunk2"}},
		{{Path: "big.go", Diff: "chunk3"}},
		{{Path: "big.go", Diff: "chunk4"}},
		{{Path: "big.go", Diff: "chunk5"}},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		groupChunkedBatches(batches)
	}
}
