#!/bin/sh
set -eu

GO_BIN="~/go/bin"
GO_BIN_EXPANDED="${HOME}/go/bin"

if echo "$PATH" | tr ':' '\n' | grep -qx "$GO_BIN_EXPANDED"; then
  echo "  ✓ $GO_BIN is in your PATH"
  exit 0
fi

# Find which shell config file to use
rc_name=""
for f in ".zshrc" ".bashrc" ".bash_profile" ".zprofile"; do
  [ -f "${HOME}/$f" ] && rc_name="$f" && break
done
[ -z "$rc_name" ] && rc_name=".zshrc"
rc="${HOME}/$rc_name"
rc_display="~/$rc_name"

# Check if it's already in the config file (match expanded or $HOME form)
if grep -qE "(export PATH=.*(go/bin|${HOME}/go/bin))" "$rc" 2>/dev/null; then
  echo "  ✓ $GO_BIN is in your $rc_display"
  echo "  ➜ Run: source $rc_display"
  exit 0
fi

echo "export PATH=\"\$HOME/go/bin:\$PATH\"" >> "$rc"
echo "  ✓ Added $GO_BIN to $rc_display"
echo "  ➜ Run: source $rc_display"
