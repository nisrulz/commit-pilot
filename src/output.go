package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

const wrapWidth = 72

func wrapText(text string, width int) []string {
	var lines []string
	runes := []rune(text)
	for len(runes) > 0 {
		if len(runes) <= width {
			lines = append(lines, string(runes))
			break
		}
		idx := width
		for idx > 0 && runes[idx] != ' ' {
			idx--
		}
		if idx == 0 {
			idx = width
		}
		lines = append(lines, string(runes[:idx]))
		runes = runes[idx:]
		if len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return lines
}

var (
	green  = color.New(color.FgGreen).SprintfFunc()
	yellow = color.New(color.FgYellow).SprintfFunc()
	cyan   = color.New(color.FgCyan).SprintfFunc()
	bold   = color.New(color.Bold).SprintfFunc()
	red    = color.New(color.FgRed).SprintfFunc()
)

func printStep(msg string) {
	fmt.Printf("  %s %s\n", green("*"), msg)
}

func printProcessing(msg string) {
	fmt.Printf("  %s %s\n", yellow(">"), msg)
}

func printCommitSection(subject, description string, filePaths []string, dryRun bool) {
	statusTag := "committed!"
	colorFn := green
	iconChar := "*"
	if dryRun {
		colorFn = yellow
		statusTag = "dry-run, skipped"
		iconChar = "!"
	}

	fmt.Printf("  %s %s\n", colorFn(iconChar), bold(subject))
	fmt.Println()

	for _, line := range strings.Split(description, "\n") {
		for _, wl := range wrapText(line, wrapWidth) {
			fmt.Printf("    %s\n", wl)
		}
	}

	fmt.Println()
	fmt.Printf("    %s %s\n", cyan(">"), cyan("files:"))
	for _, f := range filePaths {
		fmt.Printf("      %s %s\n", cyan("-"), cyan(f))
	}
	fmt.Println()
	fmt.Printf("  %s %s\n", colorFn(iconChar), colorFn(statusTag))
	fmt.Println()
}
