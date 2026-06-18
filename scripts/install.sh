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

archive="${BIN}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$archive"

echo "Downloading $BIN $tag ($os/$arch)..."
curl -sfL "$url" -o "$archive"

checksums_url="https://github.com/$REPO/releases/download/$tag/checksums.txt"
expected=$(curl -sfL "$checksums_url" | grep " $archive$" | cut -d' ' -f1)
if [ -n "$expected" ]; then
  actual=$(sha256sum "$archive" | cut -d' ' -f1)
  if [ "$actual" != "$expected" ]; then
    echo "  ! Checksum mismatch — aborting"
    rm -f "$archive"
    exit 1
  fi
  echo "  ✓ Checksum verified"
fi

tar xzf "$archive"

dst="/usr/local/bin/$BIN"
if [ -w /usr/local/bin ]; then
  mv "$BIN" "$dst"
else
  echo "  Installing to $dst (requires sudo)..."
  sudo mv "$BIN" "$dst"
fi
echo "  ✓ Installed $BIN to $dst"
echo "  ➜ Run '$BIN --dry-run' to test"
