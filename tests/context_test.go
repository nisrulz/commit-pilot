package lib_test

import (
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"testing"
)

func TestParseEstimatedMemory_giB(t *testing.T) {
	output := "Estimated Total Memory: 12 GiB"
	v := lib.ParseEstimatedMemory(output)
	expected := int64(12) * (1 << 30)
	if v != expected {
		t.Fatalf("expected %d, got %d", expected, v)
	}
}

func TestParseEstimatedMemory_miB(t *testing.T) {
	output := "Estimated Total Memory: 2048 MiB"
	v := lib.ParseEstimatedMemory(output)
	expected := int64(2048 * (1 << 20))
	if v != expected {
		t.Fatalf("expected %d, got %d", expected, v)
	}
}

func TestParseEstimatedMemory_giBWithComma(t *testing.T) {
	output := "Estimated Total Memory: 3,456 MiB"
	v := lib.ParseEstimatedMemory(output)
	expected := int64(3456 * (1 << 20))
	if v != expected {
		t.Fatalf("expected %d, got %d", expected, v)
	}
}

func TestParseEstimatedMemory_noMatch(t *testing.T) {
	output := "some random output"
	v := lib.ParseEstimatedMemory(output)
	if v != -1 {
		t.Fatalf("expected -1 for no match, got %d", v)
	}
}

func TestParseEstimatedMemory_empty(t *testing.T) {
	v := lib.ParseEstimatedMemory("")
	if v != -1 {
		t.Fatalf("expected -1 for empty input, got %d", v)
	}
}

func TestParseEstimatedMemory_unknownUnit(t *testing.T) {
	output := "Estimated Total Memory: 100 KB"
	v := lib.ParseEstimatedMemory(output)
	if v != -1 {
		t.Fatalf("expected -1 for unknown unit, got %d", v)
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"0", 0},
		{"1", 1},
		{"1.5", 1.5},
		{"100.25", 100.25},
		{"abc", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := lib.ParseFloat(tt.input)
		if got != tt.want {
			t.Fatalf("lib.ParseFloat(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestModelInfo_serialization(t *testing.T) {
	mi := &lib.ModelInfo{Key: "test-model", MaxContextLength: 131072}
	if mi.Key != "test-model" {
		t.Fatalf("expected key 'test-model', got '%s'", mi.Key)
	}
	if mi.MaxContextLength != 131072 {
		t.Fatalf("expected maxContextLength 131072, got %d", mi.MaxContextLength)
	}
}
