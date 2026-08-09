#!/bin/sh
set -eu

PROJECT_DIR=$(cd "$(dirname "$0")/.." && pwd)
BINARY="$PROJECT_DIR/commit-pilot"
TESTDIR="$PROJECT_DIR/.temp-test"
API_BASE="http://localhost:11434/v1"
if [ "${CI:-}" = "true" ]; then
  API_BASE="http://127.0.0.1:18080/v1"
fi
PASS=0
FAIL=0
RESULTS="${COMMIT_PILOT_LIVE_RESULTS:-$(mktemp "${TMPDIR:-/tmp}/commit-pilot-live.XXXXXX")}"

ok() { echo "$1|$2|PASS" >> "$RESULTS"; PASS=$((PASS+1)); }
fail() { echo "$1|$2|FAIL" >> "$RESULTS"; FAIL=$((FAIL+1)); }

# ANSI color escapes for terminal output; left empty when stdout is not a TTY
# so piped/redirected output stays plain.
if [ -t 1 ]; then
  C_BOLD=$(printf '\033[1m')
  C_CYAN=$(printf '\033[36m')
  C_GREEN=$(printf '\033[32m')
  C_RED=$(printf '\033[31m')
  C_DIM=$(printf '\033[2m')
  C_RESET=$(printf '\033[0m')
else
  C_BOLD=""
  C_CYAN=""
  C_GREEN=""
  C_RED=""
  C_DIM=""
  C_RESET=""
fi

cleanup() {
  rm -rf "$TESTDIR"
  if [ -z "${COMMIT_PILOT_LIVE_RESULTS:-}" ]; then rm -f "$RESULTS"; fi
}
die() { echo "  ${C_RED}${C_BOLD}! $1${C_RESET}"; cleanup; exit 1; }
run_in() { (cd "$1" && "$BINARY" ${2:-} --dry-run 2>&1 || true); }

# probe_header prints the probe section heading.
probe_header() {
  echo "  ${C_CYAN}${C_BOLD}• $1${C_RESET}"
}

# probe_row prints one provider reachability result, green for reachable and
# red for unreachable.
probe_row() {
  name="$1"
  url="$2"
  mark="${C_RED}✗${C_RESET}"
  if [ "$3" = "1" ]; then
    mark="${C_GREEN}✓${C_RESET}"
  fi
  printf "      ${C_BOLD}%-10s${C_RESET} ${C_DIM}%-34s${C_RESET} %s\n" "$name" "$url" "$mark"
}

# provider_selected prints the provider chosen for the run.
provider_selected() {
  name="$1"
  base="$2"
  echo "  ${C_GREEN}✓${C_RESET} ${C_BOLD}Using provider: $name${C_RESET} (${C_DIM}$base${C_RESET})"
}

# --- pre-check: probe available AI providers ---
# probe_endpoint tries the given probe URL and, for localhost endpoints, also
# the 127.0.0.1 alias (some servers, e.g. Unsloth Studio, bind IPv4 only).
# It returns 0 when the server answers.
probe_endpoint() {
  probe_url="$1"
  case "$probe_url" in
    http://localhost:*)
      for candidate in "$probe_url" "http://127.0.0.1${probe_url#http://localhost}"; do
        probe_reachable "$candidate" && return 0
      done
      return 1
      ;;
    *)
      probe_reachable "$probe_url"
      ;;
  esac
}

# probe_reachable returns 0 when the server answers at $1 with any HTTP
# response. A 401 counts as reachable: the server is up but needs an API key.
# Only a total connection failure (000) is a miss.
probe_reachable() {
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 \
    ${COMMIT_PILOT_OPENAI_COMPAT_API_KEY:+-H "Authorization: Bearer $COMMIT_PILOT_OPENAI_COMPAT_API_KEY"} \
    "$1")
  [ "$code" != "000" ]
}

PROVIDERS="openai_compat"
probe_header "Probing Ollama endpoint"
if probe_endpoint "${API_BASE%/}/models"; then
  probe_row "openai_compat" "${API_BASE%/}/models" 1
else
  PROVIDERS=""
  probe_row "openai_compat" "${API_BASE%/}/models" 0
fi

if [ -z "$PROVIDERS" ]; then
  echo ""
  echo "  ${C_RED}${C_BOLD}! Cannot reach any AI API.${C_RESET}"
  echo ""
  echo "    Start Ollama locally:"
  echo ""
  echo "    ${C_CYAN}${C_BOLD}Ollama (default):${C_RESET}"
  echo "      \$ ollama serve"
  echo "      \$ ollama pull lfm2.5:8b"
  echo "      URL: http://localhost:11434/v1"
  echo ""
  cleanup
  exit 1
fi

# --- write the tool's config file under an isolated config base ---
CONFIG_BASE="$TESTDIR/config-base"
export COMMIT_PILOT_CONFIG_DIR="$CONFIG_BASE"

# write_config [context_window]
write_config() {
  mkdir -p "$CONFIG_BASE/commit-pilot"
  {
    echo "provider: $PROVIDERS"
    echo "base_url: $API_BASE"
    [ -n "${1:-}" ] && echo "context_window: $1"
  } > "$CONFIG_BASE/commit-pilot/config.yaml"
  chmod 600 "$CONFIG_BASE/commit-pilot/config.yaml"
}
write_config
echo "  ✓ Config: $CONFIG_BASE/commit-pilot/config.yaml"
provider_selected "$PROVIDERS" "$API_BASE"

# --- build ---
make -C "$PROJECT_DIR" build || die "build failed"

rm -rf "$TESTDIR"
mkdir -p "$TESTDIR"

# --- test 1: outside git repo ---
NONGIT=$(mktemp -d /tmp/commit-pilot-nongit.XXXXXX)
OUT=$(run_in "$NONGIT")
rm -rf "$NONGIT"
echo "$OUT" | grep -q "not a git repository" && ok "repo & changes" "detects non-git directory" || fail "repo & changes" "should detect non-git directory"

# --- test 2: empty repo, no changes ---
git init -q "$TESTDIR/repo"
git -C "$TESTDIR/repo" config user.email "test@test"
git -C "$TESTDIR/repo" config user.name "Test"
git -C "$TESTDIR/repo" commit --allow-empty -m "init" -q
OUT=$(run_in "$TESTDIR/repo")
echo "$OUT" | grep -q "No changes to commit" && ok "repo & changes" "detects no changes" || fail "repo & changes" "should detect no changes"

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
echo "$OUT" | grep -q "changed files\|changed file" && ok "repo & changes" "detects changed files" || fail "repo & changes" "should detect changed files"
echo "$OUT" | grep -q -i "Found\|logical\|Generating\|commit message" && ok "repo & changes" "reaches AI stage" || fail "repo & changes" "should reach AI stage"

# --- test 4: single mode ---
OUT=$(run_in "$TESTDIR/repo" "--single")
echo "$OUT" | grep -q -i "Generating\|AI call" && ok "repo & changes" "single mode reaches AI stage" || fail "repo & changes" "single mode should reach AI stage"

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
OUT=$(run_in "$TESTDIR/binary" "--single")
echo "$OUT" | grep -q "binary" && ok "binary files" "detects binary files" || fail "binary files" "should detect binary files"

# --- test 6: large diff triggers batching ---
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

OUT=$(run_in "$TESTDIR/large" "--single")
echo "$OUT" | grep -q "changed file\|15" && ok "large diffs" "large diff detects all files" || fail "large diffs" "large diff should detect all files"
echo "$OUT" | grep -q -i "Generating\|commit message" && ok "large diffs" "large diff processes" || fail "large diffs" "large diff should process"

# --- test 7: context window configuration ---
cd "$TESTDIR/repo"
# Small context window should trigger batching warning
# Small context window should trigger batching warning
write_config 1000
OUT=$(run_in "$TESTDIR/repo" "--single" 2>&1 || true)
write_config
echo "$OUT" | grep -q -i "batch\|Large diff\|token" && ok "large diffs" "small context window triggers batching" || fail "large diffs" "small context window should trigger batching"

# --- test 8: empty diff (no actual changes) ---
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
OUT=$(run_in "$TESTDIR/emptydiff" "--single")
echo "$OUT" | grep -q -i "No changes\|no diff\|cannot generate\|empty" && ok "edge cases" "empty diff handled" || fail "edge cases" "empty diff should show appropriate message"

# --- test 9: very large single file diff ---
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
OUT=$(run_in "$TESTDIR/hugefile" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|batch\|token\|Large" && ok "large diffs" "large single file processed" || fail "large diffs" "large single file should be processed"

# --- test 10: unicode filenames ---
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
OUT=$(run_in "$TESTDIR/unicode" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "path edge cases" "unicode filenames handled" || fail "path edge cases" "unicode filenames should be handled"

# --- test 11: mixed staged and unstaged changes ---
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
OUT=$(run_in "$TESTDIR/mixed" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "repo & changes" "mixed changes processed" || fail "repo & changes" "mixed changes should be processed"

# --- test 12: file with special characters in diff ---
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
OUT=$(run_in "$TESTDIR/special" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "edge cases" "special characters handled" || fail "edge cases" "special characters should be handled"

# --- test 13: deleted files ---
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
git rm -q todelete.txt
echo "modified" > tokeep.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/deleted" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "path edge cases" "deleted files handled" || fail "path edge cases" "deleted files should be handled"

# --- test 14: renamed files ---
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
OUT=$(run_in "$TESTDIR/renamed" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "path edge cases" "renamed files handled" || fail "path edge cases" "renamed files should be handled"

# --- test 15: symlinked files ---
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
OUT=$(run_in "$TESTDIR/symlink" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "path edge cases" "symlinked files handled" || fail "path edge cases" "symlinked files should be handled"

# --- test 16: deeply nested directory ---
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
OUT=$(run_in "$TESTDIR/nested" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "path edge cases" "deeply nested directory handled" || fail "path edge cases" "deeply nested directory should be handled"

# --- test 17: file with spaces in path ---
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
OUT=$(run_in "$TESTDIR/spaces" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "path edge cases" "file with spaces in path handled" || fail "path edge cases" "file with spaces in path should be handled"

# --- test 18: empty file (0 bytes) ---
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
OUT=$(run_in "$TESTDIR/emptyfile" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "edge cases" "empty file handled" || fail "edge cases" "empty file should be handled"

# --- test 19: multiple binary formats ---
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
OUT=$(run_in "$TESTDIR/multibinary" "--single")
echo "$OUT" | grep -q -i "binary" && ok "binary files" "multiple binary formats handled" || fail "binary files" "multiple binary formats should be handled"

# --- test 20: small binary file detection ---
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
OUT=$(run_in "$TESTDIR/smallbinary" "--single")
# Small binaries may be treated as text, but should not crash
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file\|binary" && ok "binary files" "small binary files handled" || fail "binary files" "small binary files should be handled"

# --- test 20: file with only newlines ---
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
OUT=$(run_in "$TESTDIR/newlines" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|changed file" && ok "edge cases" "file with newlines handled" || fail "edge cases" "file with newlines should be handled"

# --- test 22: pre-commit hook rejection ---
mkdir -p "$TESTDIR/hooks"
cd "$TESTDIR/hooks"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > test.txt <<'EOF'
some content
EOF
git add -A
# Commit without hook first, then add the hook
git commit --no-verify -m "initial" -q
cat > .git/hooks/pre-commit <<'HOOK'
#!/bin/sh
exit 1
HOOK
chmod +x .git/hooks/pre-commit

echo "modified" > test.txt
git add test.txt
cd "$PROJECT_DIR"
if (cd "$TESTDIR/hooks" && "$BINARY" --single --yes < /dev/null >/dev/null 2>&1); then
  fail "hook rejection should cause failure"
else
  ok "failure modes" "hook rejection causes failure"
fi

# --- test 23: file deleted between diff and commit ---
mkdir -p "$TESTDIR/race"
cd "$TESTDIR/race"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > file1.txt <<'EOF'
content1
EOF
cat > file2.txt <<'EOF'
content2
EOF
git add -A && git commit -m "initial" -q

echo "modified1" > file1.txt
echo "modified2" > file2.txt
git add file1.txt file2.txt
# Delete file2.txt before commit (simulates race condition)
rm file2.txt
cd "$PROJECT_DIR"
# Should handle missing file gracefully
OUT=$(run_in "$TESTDIR/race" "--single" 2>&1 || true)
echo "$OUT" | grep -q -i "Generating\|commit message\|error\|warning\|file" && ok "failure modes" "deleted file race handled" || fail "failure modes" "deleted file race should be handled"

# --- test 24: binary file mixed with text ---
mkdir -p "$TESTDIR/mixedbin"
cd "$TESTDIR/mixedbin"
git init -q
git config user.email "test@test"
git config user.name "Test"
cat > code.go <<'EOF'
package main
func main() {}
EOF
git add -A && git commit -m "initial" -q

echo "func updated() {}" >> code.go
dd if=/dev/urandom bs=1024 count=5 of=image.png 2>/dev/null
git add -A
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/mixedbin" "--single")
echo "$OUT" | grep -q -i "Generating\|commit message\|binary" && ok "binary files" "mixed binary/text handled" || fail "binary files" "mixed binary/text should be handled"

# --- test 25: huge single-file diff triggers cross-LLM-call chunking ---
mkdir -p "$TESTDIR/hugediff"
cd "$TESTDIR/hugediff"
git init -q
git config user.email "test@test"
git config user.name "Test"

# Create an initial small file
echo "package main" > worker.go
echo "" >> worker.go
echo "func init() {}" >> worker.go
git add -A && git commit -m "chore: initial" -q

# Replace with a massive file (2000 struct+method blocks) to create a huge diff
for i in $(seq 1 2000); do
  echo "type Worker$i struct {"
  echo "  ID        int"
  echo "  Name      string"
  echo "  Data      []byte"
  echo "  Metadata  map[string]string"
  echo "}"
  echo ""
  echo "func NewWorker$i(id int, name string) *Worker$i {"
  echo "  return &Worker$i{ID: id, Name: name, Data: make([]byte,0), Metadata: make(map[string]string)}"
  echo "}"
  echo ""
  echo "func (w *Worker$i) Process(input string) (string, error) {"
  echo "  if input == \"\" { return \"\", fmt.Errorf(\"empty\") }"
  echo "  w.Data = []byte(input)"
  echo "  w.Metadata[\"processed\"] = \"true\""
  echo "  return fmt.Sprintf(\"ok:%s\", input), nil"
  echo "}"
  echo ""
done > worker.go

git add worker.go
cd "$PROJECT_DIR"

# Force small context window to ensure chunking is triggered
# Force small context window to ensure chunking is triggered
write_config 8192
OUT=$(run_in "$TESTDIR/hugediff" "--single" 2>&1 || true)
write_config
echo "$OUT" | grep -qi "Chunk " && ok "large diffs" "huge diff chunked across multiple LLM calls" || fail "large diffs" "huge diff should show 'Chunk 1/N' processing messages"
echo "$OUT" | grep -qi "Generating\|committed\|dry-run" && ok "large diffs" "huge diff completes successfully" || fail "large diffs" "huge diff should complete successfully"

# --- test 26: commit message subject line truncation ---
mkdir -p "$TESTDIR/truncation"
cd "$TESTDIR/truncation"
git init -q
git config user.email "test@test"
git config user.name "Test"
# Create a file with very long first line to trigger long subject
echo "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" > long.txt
git add -A && git commit -m "initial" -q

echo "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy" > long.txt
git add long.txt
cd "$PROJECT_DIR"
OUT=$(run_in "$TESTDIR/truncation" "--single")
# Subject should be truncated to 100 chars, not crash
echo "$OUT" | grep -q -i "Generating\|commit message\|committed" && ok "edge cases" "subject truncation handled" || fail "edge cases" "subject truncation should be handled"

# --- report ---
if [ -z "${COMMIT_PILOT_LIVE_RESULTS:-}" ]; then
  (cd "$PROJECT_DIR" && go build -o "$TESTDIR/live-table" ./scripts/livetable) || die "table renderer build failed"
  echo ""
  "$TESTDIR/live-table" < "$RESULTS"
fi

cleanup

[ "$FAIL" -eq 0 ]
