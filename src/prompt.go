package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

//go:embed prompt.txt
var promptText string

var sectionRE = regexp.MustCompile(`(?m)^=== (\w+) ===\s*$`)

func loadPrompt(mode Mode, resolved string) string {
	if resolved != "" {
		return resolved
	}
	return sectionFor(mode)
}

func sectionFor(mode Mode) string {
	headers := sectionRE.FindAllStringSubmatch(promptText, -1)
	parts := sectionRE.Split(promptText, -1)

	needed := "groups"
	if mode == ModeSingle {
		needed = "single"
	}

	for i, h := range headers {
		if h[1] == needed && i+1 < len(parts) {
			return strings.TrimSpace(parts[i+1])
		}
	}

	if len(parts) > 0 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}

func formatPrompt(tmpl string, fileList []string, diff string) string {
	fileJSON, err := json.Marshal(fileList)
	if err != nil {
		fileJSON = []byte("[]")
	}
	r := strings.NewReplacer(
		"{files}", string(fileJSON),
		"{diff}", sanitizeDiff(diff),
	)
	return r.Replace(tmpl)
}

func sanitizeDiff(diff string) string {
	return strings.Map(func(r rune) rune {
		if r == 0 || (r < 0x20 && r != '\n' && r != '\t') {
			return -1
		}
		return r
	}, diff)
}

func formatDiffSection(files []FileDiff) string {
	if len(files) == 0 {
		return ""
	}
	var parts []string
	for _, f := range files {
		parts = append(parts, fmt.Sprintf("File: %s\n%s", f.Path, f.Diff))
	}
	return strings.Join(parts, "\n---\n")
}
