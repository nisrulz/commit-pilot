package lib

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

//go:embed prompt.txt
var promptText string

var SectionRE = regexp.MustCompile(`(?m)^=== (\w+) ===\s*$`)

func LoadPrompt(mode Mode, resolved string) string {
	if resolved != "" {
		return resolved
	}
	return SectionFor(mode)
}

func LoadSection(name string) string {
	return SectionByName(name)
}

func SectionFor(mode Mode) string {
	needed := "groups"
	if mode == ModeSingle {
		needed = "single"
	}
	return SectionByName(needed)
}

func SectionByName(name string) string {
	headers := SectionRE.FindAllStringSubmatch(promptText, -1)
	parts := SectionRE.Split(promptText, -1)

	for i, h := range headers {
		if h[1] == name && i+1 < len(parts) {
			return strings.TrimSpace(parts[i+1])
		}
	}

	if len(parts) > 0 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}

func FormatPrompt(tmpl string, fileList []string, diff string) string {
	fileJSON, err := json.Marshal(fileList)
	if err != nil {
		fileJSON = []byte("[]")
	}
	r := strings.NewReplacer(
		"{files}", string(fileJSON),
		"{diff}", SanitizeDiff(diff),
	)
	return r.Replace(tmpl)
}

func ApplyMessagePreferences(tmpl string, cfg Config) string {
	var rules []string
	if cfg.Conventional {
		rules = append(rules, "Use conventional commit format.")
	} else {
		rules = append(rules, "Do not require a conventional-commit prefix.")
	}
	if cfg.TicketPrefix != "" {
		rules = append(rules, "Start the subject with ticket prefix "+cfg.TicketPrefix+".")
	}
	if cfg.Imperative {
		rules = append(rules, "Use imperative tone in the subject.")
	}
	if cfg.MaxSubjectLength > 0 {
		rules = append(rules, fmt.Sprintf("Keep the subject at most %d characters.", cfg.MaxSubjectLength))
	}
	if cfg.BodyStyle != "" {
		rules = append(rules, "Use "+cfg.BodyStyle+" body style.")
	}
	return tmpl + "\n\nMessage preferences:\n- " + strings.Join(rules, "\n- ")
}

func SanitizeDiff(diff string) string {
	return strings.Map(func(r rune) rune {
		if r == 0 || (r < 0x20 && r != '\n' && r != '\t') {
			return -1
		}
		return r
	}, diff)
}

func FormatDiffSection(files []FileDiff) string {
	if len(files) == 0 {
		return ""
	}
	var parts []string
	for _, f := range files {
		parts = append(parts, fmt.Sprintf("File: %s\n%s", f.Path, f.Diff))
	}
	return strings.Join(parts, "\n---\n")
}
