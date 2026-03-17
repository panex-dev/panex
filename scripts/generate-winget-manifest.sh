#!/usr/bin/env bash
# Generate a winget manifest for submission to microsoft/winget-pkgs.
#
# Usage:
#   scripts/generate-winget-manifest.sh --version v0.1.0 --out-dir /tmp/winget
#
# Requirements:
#   - MSI installers must already be published on the GitHub release.
#   - gh CLI authenticated.
#
# Produces three files matching the winget manifest schema v1.6.0.
# Submit them as a PR to microsoft/winget-pkgs under:
#   manifests/p/PanexDev/Panex/<version>/
set -euo pipefail

REPO="panex-dev/panex"
PACKAGE_ID="PanexDev.Panex"

VERSION=""
OUT_DIR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) shift; VERSION="${1:-}"; shift ;;
    --out-dir) shift; OUT_DIR="${1:-}"; shift ;;
    *) echo "error: unknown option: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$VERSION" ]]; then echo "error: --version is required" >&2; exit 1; fi
if [[ -z "$OUT_DIR" ]]; then echo "error: --out-dir is required" >&2; exit 1; fi

MANIFEST_VERSION="${VERSION#v}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

# Compute SHA256 of MSI assets directly from the release.
sha_of_msi() {
  local arch="$1"
  local name="panex_${MANIFEST_VERSION}_${arch}.msi"
  local hash
  hash="$(gh release download "$VERSION" --repo "$REPO" --pattern "$name" -O - 2>/dev/null | sha256sum | cut -d' ' -f1)" \
    || { echo "error: could not download $name from release $VERSION" >&2; exit 1; }
  echo "$hash"
}

echo "downloading MSI assets to compute checksums..."
SHA_X64="$(sha_of_msi x64)"
SHA_ARM64="$(sha_of_msi arm64)"

echo "  x64:   ${SHA_X64}"
echo "  arm64: ${SHA_ARM64}"

mkdir -p "$OUT_DIR"

# Version manifest.
cat > "$OUT_DIR/${PACKAGE_ID}.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.1.6.0.schema.json
PackageIdentifier: ${PACKAGE_ID}
PackageVersion: ${MANIFEST_VERSION}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
YAML

# Installer manifest.
cat > "$OUT_DIR/${PACKAGE_ID}.installer.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.1.6.0.schema.json
PackageIdentifier: ${PACKAGE_ID}
PackageVersion: ${MANIFEST_VERSION}
InstallerType: msi
Scope: machine
InstallModes:
  - silent
  - silentWithProgress
UpgradeBehavior: install
Installers:
  - Architecture: x64
    InstallerUrl: ${BASE_URL}/panex_${MANIFEST_VERSION}_x64.msi
    InstallerSha256: ${SHA_X64}
  - Architecture: arm64
    InstallerUrl: ${BASE_URL}/panex_${MANIFEST_VERSION}_arm64.msi
    InstallerSha256: ${SHA_ARM64}
ManifestType: installer
ManifestVersion: 1.6.0
YAML

# Default locale manifest.
cat > "$OUT_DIR/${PACKAGE_ID}.locale.en-US.yaml" <<YAML
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.1.6.0.schema.json
PackageIdentifier: ${PACKAGE_ID}
PackageVersion: ${MANIFEST_VERSION}
PackageLocale: en-US
Publisher: Panex
PublisherUrl: https://github.com/panex-dev
PackageName: Panex
PackageUrl: https://github.com/${REPO}
License: MIT
LicenseUrl: https://github.com/${REPO}/blob/main/LICENSE
ShortDescription: Development runtime for Chrome extensions
Description: |-
  Panex is a development runtime for Chrome extensions that lets you save,
  inspect, and replay extension behavior across contexts. It provides live
  reload, a visual inspector, and a Chrome API simulator.
Tags:
  - chrome-extension
  - developer-tools
  - browser
  - javascript
  - typescript
ManifestType: defaultLocale
ManifestVersion: 1.6.0
YAML

echo ""
echo "wrote ${OUT_DIR}/${PACKAGE_ID}.yaml"
echo "wrote ${OUT_DIR}/${PACKAGE_ID}.installer.yaml"
echo "wrote ${OUT_DIR}/${PACKAGE_ID}.locale.en-US.yaml"
echo ""
echo "Submit these files as a PR to microsoft/winget-pkgs under:"
echo "  manifests/p/PanexDev/Panex/${MANIFEST_VERSION}/"
