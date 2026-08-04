#!/usr/bin/env bash
set -euo pipefail

# release.sh cuts a new release:
#   1. bumps the version shown in the startup banner (and the test that pins it),
#   2. commits the bump,
#   3. tags it as v<version>,
#   4. pushes the commit and the tag to the origin remote.
#
# Pushing the tag triggers the GitHub Actions release workflow
# (.github/workflows/release.yml), which builds binaries and opens a release.
#
# Usage: scripts/release.sh <X.Y.Z>
#   e.g. scripts/release.sh 1.1.0

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
banner_file="$repo_root/src/lib/banner.go"
banner_test="$repo_root/tests/banner_test.go"

new_version="${1:-}"

# Require an explicit, valid semantic version.
if ! printf '%s' "$new_version" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Usage: scripts/release.sh <X.Y.Z>" >&2
  echo "  e.g. scripts/release.sh 1.1.0" >&2
  exit 1
fi

# Read the version currently in the banner.
old_version="$(sed -n 's/^const Version = "\([^"]*\)"/\1/p' "$banner_file")"
if [ -z "$old_version" ]; then
  echo "! Could not find 'const Version' in $banner_file" >&2
  exit 1
fi

if [ "$new_version" = "$old_version" ]; then
  echo "! Version $new_version is already in the banner" >&2
  exit 1
fi

tag="v$new_version"
if git rev-parse "$tag" >/dev/null 2>&1; then
  echo "! Tag $tag already exists" >&2
  exit 1
fi

# Require a clean tree so the bump commit contains only the version bump.
if [ -n "$(git status --porcelain)" ]; then
  echo "! Working tree has uncommitted changes; commit or stash them first" >&2
  exit 1
fi

echo "Bumping version $old_version -> $new_version"

# Escape dots so the old version matches literally.
escaped_old="$(printf '%s' "$old_version" | sed 's/\./\\./g')"

# Update the version constant in the banner.
sed -i.bak "s/^const Version = \"$escaped_old\"/const Version = \"$new_version\"/" "$banner_file"
rm -f "$banner_file.bak"

# Update the test that pins the banner version (the constant, the "want"
# message, and the "v"-prefixed banner output check).
sed -i.bak "s/$escaped_old/$new_version/g" "$banner_test"
rm -f "$banner_test.bak"

# Verify the bump is consistent before committing.
(cd "$repo_root" && go test ./tests/ -run 'Banner|Version' -count=1)

git add "$banner_file" "$banner_test"
git commit -m "chore: bump version to $new_version"
git tag -a "$tag" -m "Release $tag"

git push origin HEAD
git push origin "$tag"

echo
echo "✓ Released $tag"
echo "  Branch and tag pushed to origin; the release workflow is now running."
