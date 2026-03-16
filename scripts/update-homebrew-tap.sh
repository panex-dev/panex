#!/usr/bin/env bash
# Update the Homebrew tap formula for a given release version.
#
# Usage:
#   scripts/update-homebrew-tap.sh --version v0.1.0
#
# Requirements:
#   - gh CLI authenticated with push access to panex-dev/homebrew-tap
#   - The release must already be published on GitHub with SHA256SUMS
#
# This script:
#   1. Generates the formula using generate-homebrew-formula.sh
#   2. Clones the tap repo into a temp directory
#   3. Writes the formula and pushes to the tap
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TAP_REPO="panex-dev/homebrew-tap"

VERSION=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) shift; VERSION="${1:-}"; shift ;;
    *) echo "error: unknown option: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  echo "error: --version is required" >&2
  exit 1
fi

echo "generating formula for ${VERSION}..."
formula="$("${SCRIPT_DIR}/generate-homebrew-formula.sh" --version "$VERSION")"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "cloning ${TAP_REPO}..."
gh repo clone "$TAP_REPO" "$tmp_dir" -- --depth 1 2>/dev/null

mkdir -p "$tmp_dir/Formula"
echo "$formula" > "$tmp_dir/Formula/panex.rb"

cd "$tmp_dir"

git add Formula/panex.rb

if git diff --cached --quiet 2>/dev/null; then
  echo "formula is already up to date for ${VERSION}."
  exit 0
fi

git commit -m "panex ${VERSION}"
git push origin main

echo "homebrew tap updated to ${VERSION}."
echo ""
echo "Users can now install with:"
echo "  brew install panex-dev/tap/panex"
