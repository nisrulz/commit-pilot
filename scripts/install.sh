#!/bin/sh
set -eu

REPO="nisrulz/commit-pilot"
BIN="commit-pilot"

arch=$(uname -m)
os=$(uname -s | tr '[:upper:]' '[:lower:]')

case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $arch"
    exit 1
    ;;
esac

case "$os" in
  darwin | linux) ;;
  *)
    echo "Unsupported OS: $os"
    exit 1
    ;;
esac

tag=$(curl -sfL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
[ -z "$tag" ] && { echo "Could not fetch latest release"; exit 1; }

# Strip leading 'v' from tag for asset names (GoReleaser default)
version=${tag#v}

archive="${BIN}_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$archive"

echo "Downloading $BIN $tag ($os/$arch)..."
curl -sfL "$url" -o "$archive"

checksums_url="https://github.com/$REPO/releases/download/$tag/checksums.txt"
expected=$(curl -sfL "$checksums_url" | grep " $archive$" | cut -d' ' -f1)
if [ -n "$expected" ]; then
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$archive" | cut -d' ' -f1)
  else
    actual=$(shasum -a 256 "$archive" | cut -d' ' -f1)
  fi
  if [ "$actual" != "$expected" ]; then
    echo "  ! Checksum mismatch — aborting"
    rm -f "$archive"
    exit 1
  fi
  echo "  ✓ Checksum verified"
fi

# Extract (binary may be in a versioned subdirectory)
tmpdir=$(mktemp -d)
tar xzf "$archive" -C "$tmpdir"
find "$tmpdir" -name "$BIN" -type f -exec mv {} . \;
rm -rf "$tmpdir" "$archive"

dst="$HOME/go/bin/$BIN"
mkdir -p "$HOME/go/bin"
mv "$BIN" "$dst"
echo "  ✓ Installed $BIN to $dst"

# Ensure go/bin is on PATH
go_bin_expanded="${HOME}/go/bin"
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$go_bin_expanded"; then
  rc_name=""
  for f in ".zshrc" ".bashrc" ".bash_profile" ".zprofile"; do
    [ -f "${HOME}/$f" ] && rc_name="$f" && break
  done
  [ -z "$rc_name" ] && rc_name=".zshrc"
  rc="${HOME}/$rc_name"
  if ! grep -qE "(export PATH=.*(go/bin|${HOME}/go/bin))" "$rc" 2>/dev/null; then
    echo "export PATH=\"\$HOME/go/bin:\$PATH\"" >> "$rc"
    echo "  ➜ Added ~/go/bin to ~/$rc_name (run: source ~/$rc_name)"
  fi
fi

echo "  ➜ Run '$BIN --dry-run' to test"
