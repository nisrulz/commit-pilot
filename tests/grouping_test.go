package lib_test

import (
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"testing"
)

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
	if len(result) != 2 || len(result[0].Files) != 1 || len(result[1].Files) != 1 {
		t.Fatalf("binary file should remain in its own group: %#v", result)
	}
}

func TestAssignBinaryFiles_newGroupForRemaining(t *testing.T) {
	groups := []lib.CommitGroup{{Files: []string{"dir/a.go"}}}
	result := lib.AssignBinaryFiles(groups, []string{"other/image.bin"})
	if len(result) != 2 {
		t.Fatalf("expected 2 groups (original + binary group), got %d", len(result))
	}
}
