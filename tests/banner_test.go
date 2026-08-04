package lib_test

import (
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"strings"
	"testing"
)

func TestVersionConstant(t *testing.T) {
	if lib.Version != "1.0.8" {
		t.Fatalf("Version = %q, want 1.0.8", lib.Version)
	}
}

func TestPrintBannerShowsHeader(t *testing.T) {
	lib.SetOutputMode(false, false)
	out := captureStdout(t, func() { lib.PrintBanner() })
	if !strings.Contains(out, "░█▀▀░█▀█░█▄") {
		t.Fatalf("banner art line missing: %q", out)
	}
	if !strings.Contains(out, "░█▄▄░█▄█░█▀█▀█") {
		t.Fatalf("banner art line missing: %q", out)
	}
	if !strings.Contains(out, "AUTONOMOUS AI-POWERED COMMIT AGENT") {
		t.Fatalf("banner missing tagline: %q", out)
	}
	if !strings.Contains(out, "v1.0.8") {
		t.Fatalf("banner missing version: %q", out)
	}
}

func TestPrintBannerHonorsQuietMode(t *testing.T) {
	lib.SetOutputMode(true, false)
	defer lib.SetOutputMode(false, false)
	out := captureStdout(t, func() { lib.PrintBanner() })
	if out != "" {
		t.Fatalf("expected no banner output in quiet mode, got %q", out)
	}
}

func TestPrintBannerHonorsJSONMode(t *testing.T) {
	lib.SetOutputMode(false, true)
	defer lib.SetOutputMode(false, false)
	out := captureStdout(t, func() { lib.PrintBanner() })
	if out != "" {
		t.Fatalf("expected no banner output in JSON mode, got %q", out)
	}
}

func TestPrintSeparator(t *testing.T) {
	lib.SetOutputMode(false, false)
	out := captureStdout(t, func() { lib.PrintSeparator() })
	if !strings.Contains(out, strings.Repeat("=", 45)) {
		t.Fatalf("separator missing: %q", out)
	}
}

func TestPrintSeparatorHonorsQuietMode(t *testing.T) {
	lib.SetOutputMode(true, false)
	defer lib.SetOutputMode(false, false)
	out := captureStdout(t, func() { lib.PrintSeparator() })
	if out != "" {
		t.Fatalf("expected no separator output in quiet mode, got %q", out)
	}
}
