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

# --- report ---
echo "  ─────────────────────────────"
echo "  Results: $PASS passed, $FAIL failed"

cleanup

[ "$FAIL" -eq 0 ]
