#!/usr/bin/env bash
# Update the APT repository for a given release version.
#
# Usage:
#   scripts/update-apt-repo.sh --version v0.1.0
#
# Requirements:
#   - gh CLI authenticated with push access to panex-dev/apt
#   - GPG signing key already imported
#   - dpkg-scanpackages, apt-ftparchive, gzip
#
# This script:
#   1. Downloads .deb assets from the GitHub release
#   2. Clones the apt repo into a temp directory
#   3. Regenerates Packages indexes, Release metadata, and GPG signatures
#   4. Pushes the updated repo
set -euo pipefail

APT_REPO="panex-dev/apt"

log() { echo "[apt-repo] $*"; }

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

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "error: version must match v<semver> (got: $VERSION)" >&2
  exit 1
fi

# Strip leading "v" for Debian version convention.
DEB_VERSION="${VERSION#v}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

# --- Download .deb assets ---
asset_dir="$tmp_dir/assets"
mkdir -p "$asset_dir"
log "downloading assets for ${VERSION}"
gh release download "$VERSION" --repo panex-dev/panex --pattern '*.deb' --dir "$asset_dir"

amd64_deb="panex_${DEB_VERSION}_amd64.deb"
arm64_deb="panex_${DEB_VERSION}_arm64.deb"

if [[ ! -f "$asset_dir/$amd64_deb" ]]; then
  echo "error: missing expected asset: $amd64_deb" >&2
  exit 1
fi
log "verified: $amd64_deb"

if [[ ! -f "$asset_dir/$arm64_deb" ]]; then
  echo "error: missing expected asset: $arm64_deb" >&2
  exit 1
fi
log "verified: $arm64_deb"

# --- Clone apt repo ---
repo_dir="$tmp_dir/repo"
log "cloning ${APT_REPO}"
gh repo clone "$APT_REPO" "$repo_dir" -- --depth 1 2>/dev/null

# --- Populate pool ---
pool_dir="$repo_dir/pool/main/p/panex"
mkdir -p "$pool_dir"
cp "$asset_dir"/*.deb "$pool_dir/"

# --- Generate per-arch Packages indexes ---
mkdir -p "$repo_dir/dists/stable/main/binary-amd64"
mkdir -p "$repo_dir/dists/stable/main/binary-arm64"

cd "$repo_dir"

log "generating binary-amd64/Packages.gz"
dpkg-scanpackages --arch amd64 pool > dists/stable/main/binary-amd64/Packages
gzip -9fk dists/stable/main/binary-amd64/Packages

log "generating binary-arm64/Packages.gz"
dpkg-scanpackages --arch arm64 pool > dists/stable/main/binary-arm64/Packages
gzip -9fk dists/stable/main/binary-arm64/Packages

# --- Generate Release metadata ---
log "generating Release via apt-ftparchive"
apt-ftparchive release \
  -o APT::FTPArchive::Release::Origin="Panex" \
  -o APT::FTPArchive::Release::Label="Panex" \
  -o APT::FTPArchive::Release::Suite="stable" \
  -o APT::FTPArchive::Release::Codename="stable" \
  -o APT::FTPArchive::Release::Architectures="amd64 arm64" \
  -o APT::FTPArchive::Release::Components="main" \
  -o APT::FTPArchive::Release::Description="Panex APT repository" \
  dists/stable/ > dists/stable/Release

# --- Sign ---
log "signing Release -> Release.gpg + InRelease"
gpg --batch --yes --armor --detach-sign -o dists/stable/Release.gpg dists/stable/Release
gpg --batch --yes --armor --clearsign -o dists/stable/InRelease dists/stable/Release

# --- Export public key and fingerprint ---
gpg --armor --export > gpg.key
gpg --fingerprint --with-colons | awk -F: '/^fpr:/{print $10; exit}' > FINGERPRINT.txt

# --- GitHub Pages marker ---
touch .nojekyll

# --- Commit and push ---
git config user.name "panex-bot"
git config user.email "bot@panex.dev"

git add -A

if git diff --cached --quiet 2>/dev/null; then
  log "no changes, skipping push"
  exit 0
fi

# TODO: add retention/cleanup logic for old versions in pool/
git commit -m "release: update apt repo for ${VERSION}"
git push origin main

log "apt repo updated to ${VERSION}"
echo ""
echo "Users can now install with:"
echo "  deb [signed-by=/etc/apt/keyrings/panex.gpg] https://panex-dev.github.io/apt stable main"
