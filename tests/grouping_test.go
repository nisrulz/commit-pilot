package lib_test

import (
	lib "github.com/nisrulz/commit-pilot/src/lib"
	"strings"
	"testing"
)

func TestFileCategory(t *testing.T) {
	tests := []struct {
		path string
		cat  string
	}{
		{"README.md", "docs"},
		{"readme", "docs"},
		{"docs/guide.md", "docs"},
		{"config.json", "config"},
		{".gitignore", "config"},
		{"scripts/deploy.sh", "scripts"},
		{"src/main.go", "code"},
		{"Makefile", "code"},
	}
	for _, tt := range tests {
		got := lib.FileCategory(tt.path)
		if got != tt.cat {
			t.Errorf("lib.FileCategory(%q) = %q, want %q", tt.path, got, tt.cat)
		}
	}
}

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

func TestLimitCommitScope_underLimit(t *testing.T) {
	files := []string{"a.go", "b.go"}
	result := lib.LimitCommitScope(files)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestLimitCommitScope_overLimit(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go", "d.go", "e.go"}
	result := lib.LimitCommitScope(files)
	if len(result) > 3 {
		t.Fatalf("expected at most 3 files, got %d", len(result))
	}
}

func TestLimitCommitScopeTo_zeroDisablesLimit(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go", "d.go"}
	result := lib.LimitCommitScopeTo(files, 0)
	if len(result) != len(files) {
		t.Fatalf("expected all files, got %v", result)
	}
}

func TestSplitCommitGroupsPreservesEveryFile(t *testing.T) {
	groups := lib.SplitCommitGroups([]lib.CommitGroup{{Subject: "feat: add files", Files: []string{"a.go", "b.go", "c.go", "d.go", "e.go"}}}, 2)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	var files []string
	for _, group := range groups {
		if len(group.Files) > 2 {
			t.Fatalf("group exceeds cap: %v", group.Files)
		}
		files = append(files, group.Files...)
	}
	if strings.Join(files, ",") != "a.go,b.go,c.go,d.go,e.go" {
		t.Fatalf("files were lost or reordered: %v", files)
	}
}

func TestSubjectsRelated_same(t *testing.T) {
	if !lib.SubjectsRelated("feat: add login", "feat: add login") {
		t.Fatal("identical subjects should be related")
	}
}

func TestSubjectsRelated_prefix(t *testing.T) {
	if !lib.SubjectsRelated("feat: add login page", "feat: add login") {
		t.Fatal("prefix match should be related")
	}
	if !lib.SubjectsRelated("feat: add login", "feat: add login page") {
		t.Fatal("reverse prefix match should be related")
	}
}

func TestSubjectsRelated_unrelated(t *testing.T) {
	if lib.SubjectsRelated("feat: add login", "fix: fix crash") {
		t.Fatal("unrelated subjects should not be related")
	}
}

func TestSubjectsRelated_empty(t *testing.T) {
	if lib.SubjectsRelated("", "feat: add login") {
		t.Fatal("empty subject should not be related")
	}
	if lib.SubjectsRelated("feat: add login", "") {
		t.Fatal("empty subject should not be related")
	}
}

func TestMergeGroups_single(t *testing.T) {
	groups := []lib.CommitGroup{
		{Subject: "feat: a", Files: []string{"a.go"}},
	}
	result := lib.MergeGroups(groups)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
}

func TestMergeGroups_mergeRelated(t *testing.T) {
	groups := []lib.CommitGroup{
		{Subject: "feat: add login", Files: []string{"login.go"}},
		{Subject: "feat: add login page", Files: []string{"page.go"}},
	}
	result := lib.MergeGroups(groups)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged group, got %d", len(result))
	}
}

func TestMergeGroups_noMerge(t *testing.T) {
	groups := []lib.CommitGroup{
		{Subject: "feat: add login", Files: []string{"login.go"}},
		{Subject: "fix: fix crash", Files: []string{"fix.go"}},
	}
	result := lib.MergeGroups(groups)
	if len(result) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result))
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
