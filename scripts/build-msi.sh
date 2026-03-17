#!/usr/bin/env bash
# Build a Windows MSI installer for Panex using WiX v4.
#
# Usage:
#   scripts/build-msi.sh --version v0.1.0 --arch x64 --out-dir dist/release
#
# Requirements:
#   - WiX v4 CLI (`wix` command) — install with: dotnet tool install --global wix
#   - Go toolchain (for cross-compiling the Windows binary)
#
# This script is designed to run on Windows CI (GitHub Actions windows-latest).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION=""
ARCH=""
OUT_DIR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) shift; VERSION="${1:-}"; shift ;;
    --arch)    shift; ARCH="${1:-}"; shift ;;
    --out-dir) shift; OUT_DIR="${1:-}"; shift ;;
    *) echo "error: unknown option: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$VERSION" ]]; then echo "error: --version is required" >&2; exit 1; fi
if [[ -z "$ARCH" ]]; then echo "error: --arch is required (x64 or arm64)" >&2; exit 1; fi
if [[ -z "$OUT_DIR" ]]; then echo "error: --out-dir is required" >&2; exit 1; fi

# Map winget/MSI arch names to Go arch names.
case "$ARCH" in
  x64)   GOARCH="amd64" ;;
  arm64) GOARCH="arm64" ;;
  *)     echo "error: unsupported arch: $ARCH (use x64 or arm64)" >&2; exit 1 ;;
esac

MSI_VERSION="${VERSION#v}"
MSI_NAME="panex_${MSI_VERSION}_${ARCH}.msi"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "building panex.exe for windows/${GOARCH}..."
CGO_ENABLED=0 GOOS=windows GOARCH="$GOARCH" \
  go build -trimpath -buildvcs=false \
  -ldflags "-buildid= -X main.version=${VERSION}" \
  -o "${tmp_dir}/panex.exe" \
  ./cmd/panex/...

mkdir -p "$OUT_DIR"

echo "building MSI (${ARCH})..."
wix build \
  "${REPO_ROOT}/packaging/windows/panex.wxs" \
  -o "${OUT_DIR}/${MSI_NAME}" \
  -d "Version=${MSI_VERSION}" \
  -d "BinaryPath=${tmp_dir}/panex.exe" \
  -arch "$ARCH"

echo "wrote ${OUT_DIR}/${MSI_NAME}"
