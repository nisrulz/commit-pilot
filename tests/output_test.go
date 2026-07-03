package lib_test

import (
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"strings"
	"testing"
)

func TestWrapText_short(t *testing.T) {
	lines := lib.WrapText("hello world", 72)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestWrapText_long(t *testing.T) {
	text := "hello world " + strings.Repeat("a", 200)
	lines := lib.WrapText(text, 20)
	if len(lines) < 2 {
		t.Fatal("expected multiple wrapped lines")
	}
}

func TestWrapText_empty(t *testing.T) {
	lines := lib.WrapText("", 72)
	if len(lines) != 0 {
		t.Fatalf("expected empty slice, got %d lines", len(lines))
	}
}

func TestWrapText_exactWidth(t *testing.T) {
	text := "1234567890"
	lines := lib.WrapText(text, 10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestWrapText_longWord(t *testing.T) {
	text := "abcdefghijklmnopqrstuvwxyz"
	lines := lib.WrapText(text, 10)
	if len(lines) < 2 {
		t.Fatal("expected long word to be broken")
	}
}

func TestWrapText_singleChar(t *testing.T) {
	lines := lib.WrapText("a", 72)
	if len(lines) != 1 || lines[0] != "a" {
		t.Fatalf("expected ['a'], got %v", lines)
	}
}
