package lib

import "fmt"

// Version is the commit-pilot release version shown in the startup banner.
const Version = "1.0.9"

// bannerSeparatorWidth matches the width of the banner art, so the separator
// rule that closes the header spans the full width of the word.
const bannerSeparatorWidth = 45

// PrintBanner shows the startup header: the ASCII-art word, the version, and
// the tagline, each in its own color. It is a no-op in quiet and JSON modes so
// scripted output stays clean.
func PrintBanner() {
	if quietOutput || jsonOutput {
		return
	}
	for i, line := range bannerArt {
		if i == len(bannerArt)-1 {
			fmt.Printf("%s  %s\n", cyan(line), yellow("v"+Version))
		} else {
			fmt.Println(cyan(line))
		}
	}
	fmt.Println()
	fmt.Printf(" %s\n", magenta(bannerTagline))
}
