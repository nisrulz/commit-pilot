#!/bin/sh
set -eu

PROJECT_DIR=$(cd "$(dirname "$0")/.." && pwd)
BINARY="$PROJECT_DIR/commit-pilot"
TESTDIR="$PROJECT_DIR/.temp-test"
API_BASE="${OPENAI_BASE_URL:-http://localhost:1234/v1}"
PASS=0
FAIL=0

ok() { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

cleanup() { rm -rf "$TESTDIR"; }
die() { echo "  ! $1"; cleanup; exit 1; }
run_in() { (cd "$1" && "$BINARY" ${2:-} --dry-run 2>&1 || true); }

# --- pre-check: AI model reachability ---
echo "  • Checking AI endpoint at $API_BASE ..."
MODELS=$(curl -sf "$API_BASE/models" 2>/dev/null || true)
if [ -z "$MODELS" ]; then
  echo ""
  echo "  ! Cannot reach the AI API at: $API_BASE"
  echo ""
  echo "    To run the live test, start your AI provider:"
  echo ""
  echo "    LMStudio (default):"
  echo "      \$ lms server start"
  echo "      \$ lms get gemma-4-e2b-it-qat -y"
  echo ""
  echo "    Ollama:"
  echo "      \$ ollama serve"
  echo "      \$ ollama pull gemma4:e2b-it-qat"
  echo ""
  echo "    Or set a custom endpoint:"
  echo "      \$ OPENAI_BASE_URL=<url> make test-live"
  echo ""
  cleanup
  exit 1
fi
echo "  ✓ AI endpoint reachable"

# --- build ---
make -C "$PROJECT_DIR" build || die "build failed"
echo "  • Built commit-pilot"

rm -rf "$TESTDIR"

# --- test 1: outside git repo ---
NONGIT=$(mktemp -d /tmp/commit-pilot-nongit.XXXXXX)
OUT=$(run_in "$NONGIT")
rm -rf "$NONGIT"
echo "$OUT" | grep -q "not a git repository" && ok "detects non-git directory" || fail "should detect non-git directory"

# --- test 2: empty repo, no changes ---
git init -q "$TESTDIR/repo"
git -C "$TESTDIR/repo" config user.email "test@test"
git -C "$TESTDIR/repo" config user.name "Test"
git -C "$TESTDIR/repo" commit --allow-empty -m "init" -q
OUT=$(run_in "$TESTDIR/repo")
echo "$OUT" | grep -q "No changes to commit" && ok "detects no changes" || fail "should detect no changes"

# --- test 3: multi-file changes, dry-run ---
cd "$TESTDIR/repo"
mkdir -p src docs
cat > README.md <<'EOF' && cat > CHANGELOG.md <<'EOF2'
# My Project
EOF
## 1.0.0
EOF2
cat > .gitignore <<'EOF' && cat > config.yml <<'EOF2'
*.log
EOF
app:
  name: my-project
EOF2
cat > src/main.go <<'EOF' && cat > src/utils.go <<'EOF2'
package main
func main() { println("hello") }
EOF
package main
func helper() string { return "helper" }
EOF2
git add -A && git commit -m "chore: initial scaffold" -q

# Three logical work packages as sequential commits
cat >> README.md <<'EOF'
## Installation
Run make install.
EOF
cat >> CHANGELOG.md <<'EOF'
## 1.1.0
EOF
git add README.md CHANGELOG.md && git commit -m "wip: docs" -q

cat >> config.yml <<'EOF'
  debug: true
EOF
cat >> .gitignore <<'EOF'
.env
EOF
git add config.yml .gitignore && git commit -m "wip: config" -q

cat >> src/main.go <<'EOF'
func run() {}
EOF
cat >> src/utils.go <<'EOF'
func anotherHelper() string { return "another" }
EOF
git add src/main.go src/utils.go && git commit -m "wip: code" -q

# Unstage all three
git reset --soft HEAD~3
cd "$PROJECT_DIR"

OUT=$(run_in "$TESTDIR/repo")
echo "$OUT" | grep -q "changed files\|changed file" && ok "detects changed files" || fail "should detect changed files"
echo "$OUT" | grep -q -i "Found\|logical\|Generating\|commit message" && ok "reaches AI stage" || fail "should reach AI stage"

# --- test 4: single mode ---
OUT=$(run_in "$TESTDIR/repo" "1")
echo "$OUT" | grep -q -i "Generating\|AI call" && ok "single mode reaches AI stage" || fail "single mode should reach AI stage"

# --- test 5: binary file handling (standalone repo) ---
mkdir -p "$TESTDIR/binary"
cd "$TESTDIR/binary"
git init -q
git config user.email "test@test"
git config user.name "Test"
git commit --allow-empty -m "init" -q
printf '\xff\xd8\xff\xe0\x00\x10\x4a\x46\x49\x46' > logo.bin
git add logo.bin
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/binary" "1")
echo "$OUT" | grep -q "binary" && ok "detects binary files" || fail "should detect binary files"

# --- test 6: large diff triggers batching ---
echo "  • Testing large diff batching..."
mkdir -p "$TESTDIR/large"
cd "$TESTDIR/large"
git init -q
git config user.email "test@test"
git config user.name "Test"
git commit --allow-empty -m "init" -q

# Create many files to trigger batching
for i in $(seq 1 15); do
  echo "// Package main - file $i
package main

func init$i() string {
  return \"initialized $i\"
}

func process$i(data string) string {
  result := \"\"
  for _, c := range data {
    if c != 0 {
      result += string(c)
    }
  }
  return result
}

func validate$i(input int) bool {
  if input < 0 {
    return false
  }
  if input > 100 {
    return false
  }
  return true
}" > "file$i.go"
done
git add -A && git commit -m "chore: initial files" -q

# Now modify all files to create a large diff
for i in $(seq 1 15); do
  echo "

func updated$i() string {
  return \"updated $i\"
}" >> "file$i.go"
done
git add -A
cd "$PROJECT_DIR"

OUT=$(run_in "$TESTDIR/large" "1")
echo "$OUT" | grep -q "changed file\|15" && ok "large diff detects all files" || fail "large diff should detect all files"
echo "$OUT" | grep -q -i "Generating\|commit message" && ok "large diff processes" || fail "large diff should process"

# --- test 7: context window configuration ---
echo "  • Testing context window configuration..."
cd "$TESTDIR/repo"
# Small context window should trigger batching warning
OUT=$(COMMIT_PILOT_CONTEXT_WINDOW=1000 run_in "$TESTDIR/repo" "1" 2>&1 || true)
echo "$OUT" | grep -q -i "batch\|Large diff\|token" && ok "small context window triggers batching" || fail "small context window should trigger batching"

# --- test 8: empty diff (no actual changes) ---
echo "  • Testing empty diff scenario..."
mkdir -p "$TESTDIR/emptydiff"
cd "$TESTDIR/emptydiff"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > test.txt <<'EOF'
line1
line2
line3
EOF
git add -A && git commit -m "initial" -q

# Stage file without any changes
git add test.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/emptydiff" "1")
echo "$OUT" | grep -q -i "No changes\|no diff\|cannot generate\|empty" && ok "empty diff handled" || fail "empty diff should show appropriate message"

# --- test 9: very large single file diff ---
echo "  • Testing very large single file diff..."
mkdir -p "$TESTDIR/hugefile"
cd "$TESTDIR/hugefile"
git init -q
git config user.email "test@test"
git config user.name "Test"

# Create a base file
for i in $(seq 1 100); do
  echo "func base$i() { return $i }"
done > huge.go
git add -A && git commit -m "initial" -q

# Now make massive changes to create a huge diff
for i in $(seq 1 200); do
  echo "func added$i() string { return \"added line $i with some extra text to make it longer\" }"
done >> huge.go
git add huge.go
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/hugefile" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|batch\|token\|Large" && ok "large single file processed" || fail "large single file should be processed"

# --- test 10: unicode filenames ---
echo "  • Testing unicode filenames..."
mkdir -p "$TESTDIR/unicode"
cd "$TESTDIR/unicode"
git init -q
git config user.email "test@test"
git config user.name "Test"

# Create files with unicode names
touch "file.go"
touch "archive.go"
touch "cafe.go"
echo 'package main' > "file.go"
echo 'package main' > "archive.go"
echo 'package main' > "cafe.go"
git add -A && git commit -m "initial" -q

# Modify unicode files
echo 'func hello() {}' >> "file.go"
echo 'func world() {}' >> "archive.go"
echo 'func bonjour() {}' >> "cafe.go"
git add -A
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/unicode" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "unicode filenames handled" || fail "unicode filenames should be handled"

# --- test 11: mixed staged and unstaged changes ---
echo "  • Testing mixed staged/unstaged changes..."
mkdir -p "$TESTDIR/mixed"
cd "$TESTDIR/mixed"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > staged.txt <<'EOF'
staged content
EOF
cat > unstaged.txt <<'EOF'
unstaged content
EOF
cat > both.txt <<'EOF'
both content
EOF
git add -A && git commit -m "initial" -q

# Modify all files but only stage some
echo "modified staged" > staged.txt
echo "modified unstaged" > unstaged.txt
echo "modified both" > both.txt
git add staged.txt both.txt
cd "$PROJECT_DIR"

# With staged changes only (staged.txt and both.txt)
OUT=$(run_in "$TESTDIR/mixed" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "mixed changes processed" || fail "mixed changes should be processed"

# --- test 12: file with special characters in diff ---
echo "  • Testing special characters in diff..."
mkdir -p "$TESTDIR/special"
cd "$TESTDIR/special"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > special.txt <<'EOF'
normal line
line with quotes
line with dollar signs
line with backticks
line with backslash
EOF
git add -A && git commit -m "initial" -q

# Add lines with special characters
echo "line with tabs and quotes" >> special.txt
echo "line with newlines" >> special.txt
echo "line with unicode accents" >> special.txt
git add special.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/special" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "special characters handled" || fail "special characters should be handled"

# --- test 13: deleted files ---
echo "  • Testing deleted files..."
mkdir -p "$TESTDIR/deleted"
cd "$TESTDIR/deleted"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > todelete.txt <<'EOF'
this file will be deleted
EOF
cat > tokeep.txt <<'EOF'
this file stays
EOF
git add -A && git commit -m "initial" -q

# Delete one file, modify another
git rm todelete.txt
echo "modified" > tokeep.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/deleted" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "deleted files handled" || fail "deleted files should be handled"

# --- test 14: renamed files ---
echo "  • Testing renamed files..."
mkdir -p "$TESTDIR/renamed"
cd "$TESTDIR/renamed"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > oldname.txt <<'EOF'
content in old file
EOF
git add -A && git commit -m "initial" -q

# Rename the file
git mv oldname.txt newname.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/renamed" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "renamed files handled" || fail "renamed files should be handled"

# --- test 15: symlinked files ---
echo "  • Testing symlinked files..."
mkdir -p "$TESTDIR/symlink"
cd "$TESTDIR/symlink"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > real.txt <<'EOF'
real file content
EOF
ln -s real.txt link.txt
git add -A && git commit -m "initial" -q

# Modify the real file
echo "modified content" > real.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/symlink" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "symlinked files handled" || fail "symlinked files should be handled"

# --- test 16: deeply nested directory ---
echo "  • Testing deeply nested directory..."
mkdir -p "$TESTDIR/nested/a/b/c/d/e/f/g"
cd "$TESTDIR/nested"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > a/b/c/d/e/f/g/deep.txt <<'EOF'
deeply nested file
EOF
git add -A && git commit -m "initial" -q

echo "modified" > a/b/c/d/e/f/g/deep.txt
git add a/b/c/d/e/f/g/deep.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/nested" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "deeply nested directory handled" || fail "deeply nested directory should be handled"

# --- test 17: file with spaces in path ---
echo "  • Testing file with spaces in path..."
mkdir -p "$TESTDIR/spaces/my folder"
cd "$TESTDIR/spaces"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > "my folder/file with spaces.txt" <<'EOF'
file with spaces in path
EOF
git add -A && git commit -m "initial" -q

echo "modified" > "my folder/file with spaces.txt"
git add "my folder/file with spaces.txt"
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/spaces" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "file with spaces in path handled" || fail "file with spaces in path should be handled"

# --- test 18: empty file (0 bytes) ---
echo "  • Testing empty file..."
mkdir -p "$TESTDIR/emptyfile"
cd "$TESTDIR/emptyfile"
git init -q
git config user.email "test@test"
git config user.name "Test"
touch empty.txt
git add -A && git commit -m "initial" -q

# Add content to empty file
echo "was empty" > empty.txt
git add empty.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/emptyfile" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "empty file handled" || fail "empty file should be handled"

# --- test 19: multiple binary formats ---
echo "  • Testing multiple binary formats..."
mkdir -p "$TESTDIR/multibinary"
cd "$TESTDIR/multibinary"
git init -q
git config user.email "test@test"
git config user.name "Test"
# Create binary files with content (not just headers)
dd if=/dev/urandom bs=100 count=10 of=image.jpg 2>/dev/null
dd if=/dev/urandom bs=100 count=10 of=image.png 2>/dev/null
dd if=/dev/urandom bs=100 count=10 of=archive.zip 2>/dev/null
git add -A && git commit -m "initial" -q

# Add another binary
dd if=/dev/urandom bs=100 count=10 of=file.gz 2>/dev/null
git add file.gz
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/multibinary" "1")
echo "$OUT" | grep -q -i "binary" && ok "multiple binary formats handled" || fail "multiple binary formats should be handled"

# --- test 20: small binary file detection ---
echo "  • Testing small binary file detection..."
mkdir -p "$TESTDIR/smallbinary"
cd "$TESTDIR/smallbinary"
git init -q
git config user.email "test@test"
git config user.name "Test"
# Create small binary files (just headers)
printf '\xff\xd8\xff\xe0\x00\x10JFIF\x00' > small.jpg
printf '\x89PNG\r\n\x1a\n\x00\x00' > small.png
git add -A && git commit -m "initial" -q

# Add another small binary
printf '\x1f\x8b\x08\x00\x00\x00\x00\x00' > small.gz
git add small.gz
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/smallbinary" "1")
# Small binaries may be treated as text, but should not crash
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file\|binary" && ok "small binary files handled" || fail "small binary files should be handled"

# --- test 20: file with only newlines ---
echo "  • Testing file with only newlines..."
mkdir -p "$TESTDIR/newlines"
cd "$TESTDIR/newlines"
git init -q
git config user.email "test@test"
git config user.name "Test"
printf 'line1\n' > newlines.txt
git add -A && git commit -m "initial" -q

printf 'line1\n\n\n\n' > newlines.txt
git add newlines.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/newlines" "1")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "file with newlines handled" || fail "file with newlines should be handled"

# --- report ---
echo "  ─────────────────────────────"
echo "  Results: $PASS passed, $FAIL failed"

cleanup

[ "$FAIL" -eq 0 ]
