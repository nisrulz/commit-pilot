package lib_test

import (
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"testing"
)

func TestFilterValidFiles_allValid(t *testing.T) {
	valid := []string{"a.go", "b.go", "c.go"}
	result := lib.FilterValidFiles([]string{"a.go", "b.go"}, valid)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestFilterValidFiles_someInvalid(t *testing.T) {
	valid := []string{"a.go"}
	result := lib.FilterValidFiles([]string{"a.go", "b.go", "c.go"}, valid)
	if len(result) != 1 || result[0] != "a.go" {
		t.Fatalf("expected only a.go, got %v", result)
	}
}

func TestFilterValidFiles_empty(t *testing.T) {
	result := lib.FilterValidFiles(nil, []string{"a.go"})
	if len(result) != 0 {
		t.Fatalf("expected empty, got %v", result)
	}
}

func TestAssignBinaryFiles_noBinaries(t *testing.T) {
	groups := []lib.CommitGroup{{Files: []string{"a.go"}}}
	result := lib.AssignBinaryFiles(groups, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result))
	}
}

func TestAssignBinaryFiles_toExistingGroup(t *testing.T) {
	groups := []lib.CommitGroup{{Files: []string{"dir/a.go"}}}
	result := lib.AssignBinaryFiles(groups, []string{"dir/image.bin"})
	if len(result) != 1 || len(result[0].Files) != 2 {
		t.Fatalf("expected 2 files in group (a.go + image.bin), got %d", len(result[0].Files))
	}
}

func TestAssignBinaryFiles_newGroupForRemaining(t *testing.T) {
	groups := []lib.CommitGroup{{Files: []string{"dir/a.go"}}}
	result := lib.AssignBinaryFiles(groups, []string{"other/image.bin"})
	if len(result) != 2 {
		t.Fatalf("expected 2 groups (original + binary group), got %d", len(result))
	}
}
