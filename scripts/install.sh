#!/bin/sh
# Panex installer — downloads the latest (or pinned) release and places the
# binary on PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/panex-dev/panex/main/scripts/install.sh | sh
#   curl -fsSL ... | sh -s -- --version v0.2.0
#   curl -fsSL ... | sh -s -- --install-dir ~/.local/bin
#   curl -fsSL ... | sh -s -- --method apt
#
# The script:
#   1. Detects OS and architecture.
#   2. Downloads the matching release archive from GitHub.
#   3. Verifies the SHA256 checksum.
#   4. Extracts the binary to the install directory.
#   5. Prints a success summary with next steps.

set -eu

REPO="panex-dev/panex"
BINARY="panex"
DEFAULT_INSTALL_DIR="/usr/local/bin"

# --- helpers -----------------------------------------------------------------

log()   { printf '%s\n' "$*"; }
warn()  { printf '%s\n' "$*" >&2; }
fatal() { warn "error: $*"; exit 1; }

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fatal "required command not found: $1"
  fi
}

# --- argument parsing --------------------------------------------------------

VERSION=""
INSTALL_DIR=""
METHOD=""

while [ $# -gt 0 ]; do
  case "$1" in
    --version)    shift; VERSION="${1:-}"; shift ;;
    --install-dir) shift; INSTALL_DIR="${1:-}"; shift ;;
    --method)     shift; METHOD="${1:-}"; shift ;;
    --help|-h)
      log "Usage: install.sh [--version VERSION] [--install-dir DIR] [--method METHOD]"
      log ""
      log "Options:"
      log "  --version      Pin a specific release (e.g. v0.2.0). Default: latest."
      log "  --install-dir  Directory to install into. Default: /usr/local/bin"
      log "                 (falls back to ~/.local/bin without sudo)."
      log "  --method       Install method: 'apt' to set up the APT repository"
      log "                 (Debian/Ubuntu only). Default: direct binary download."
      exit 0
      ;;
    *) fatal "unknown option: $1" ;;
  esac
done

# --- platform detection ------------------------------------------------------

detect_os() {
  os="$(uname -s)"
  case "$os" in
    Linux)  echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)      fatal "unsupported operating system: $os" ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)       echo "amd64" ;;
    aarch64|arm64)      echo "arm64" ;;
    *)                  fatal "unsupported architecture: $arch" ;;
  esac
}

# --- version resolution ------------------------------------------------------

resolve_version() {
  if [ -n "$VERSION" ]; then
    echo "$VERSION"
    return
  fi

  need_cmd curl

  latest="$(curl -fsSL -o /dev/null -w '%{redirect_url}' \
    "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"

  if [ -z "$latest" ]; then
    fatal "could not determine latest release version (is github.com reachable?)"
  fi

  # The redirect URL ends with /tag/vX.Y.Z — extract the tag.
  echo "${latest##*/}"
}

# --- download and verify -----------------------------------------------------

download_and_verify() {
  version="$1"
  os="$2"
  arch="$3"
  dest_dir="$4"

  archive_name="${BINARY}_${version}_${os}_${arch}.tar.gz"
  checksum_name="${BINARY}_${version}_SHA256SUMS"
  base_url="https://github.com/${REPO}/releases/download/${version}"

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  log "downloading ${archive_name}..."
  curl -fsSL -o "${tmp_dir}/${archive_name}" "${base_url}/${archive_name}" \
    || fatal "download failed — does release ${version} exist for ${os}/${arch}?"

  log "downloading checksum manifest..."
  curl -fsSL -o "${tmp_dir}/${checksum_name}" "${base_url}/${checksum_name}" \
    || fatal "checksum manifest download failed"

  log "verifying SHA256 checksum..."
  expected="$(grep "  ${archive_name}$" "${tmp_dir}/${checksum_name}" | cut -d' ' -f1)"
  if [ -z "$expected" ]; then
    fatal "archive ${archive_name} not found in checksum manifest"
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "${tmp_dir}/${archive_name}" | cut -d' ' -f1)"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "${tmp_dir}/${archive_name}" | cut -d' ' -f1)"
  else
    fatal "no sha256sum or shasum found — cannot verify checksum"
  fi

  if [ "$expected" != "$actual" ]; then
    fatal "checksum mismatch: expected ${expected}, got ${actual}"
  fi
  log "checksum verified."

  log "extracting ${BINARY}..."
  tar -xzf "${tmp_dir}/${archive_name}" -C "${tmp_dir}"

  # The archive contains a directory named panex_<version>_<os>_<arch>/panex.
  extracted="${tmp_dir}/${BINARY}_${version}_${os}_${arch}/${BINARY}"
  if [ ! -f "$extracted" ]; then
    fatal "binary not found in archive at expected path"
  fi

  install_binary "$extracted" "$dest_dir"
}

# --- installation ------------------------------------------------------------

install_binary() {
  src="$1"
  dest_dir="$2"

  mkdir -p "$dest_dir" 2>/dev/null || true

  if [ -w "$dest_dir" ]; then
    cp "$src" "${dest_dir}/${BINARY}"
    chmod 755 "${dest_dir}/${BINARY}"
  else
    log "installing to ${dest_dir} requires elevated privileges..."
    sudo cp "$src" "${dest_dir}/${BINARY}"
    sudo chmod 755 "${dest_dir}/${BINARY}"
  fi
}

# --- path check --------------------------------------------------------------

check_path() {
  install_dir="$1"
  case ":${PATH}:" in
    *":${install_dir}:"*) return 0 ;;
    *) return 1 ;;
  esac
}

# --- apt installation --------------------------------------------------------

install_via_apt() {
  need_cmd curl
  need_cmd sudo

  os="$(detect_os)"
  if [ "$os" != "linux" ]; then
    fatal "--method apt is only supported on Linux (Debian/Ubuntu)"
  fi

  if ! command -v apt-get >/dev/null 2>&1; then
    fatal "--method apt requires apt-get (Debian/Ubuntu)"
  fi

  log ""
  log "panex installer (apt)"
  log ""

  log "adding GPG key..."
  sudo mkdir -p /etc/apt/keyrings
  curl -fsSL https://panex-dev.github.io/apt/gpg.key \
    | sudo gpg --dearmor -o /etc/apt/keyrings/panex.gpg

  log "adding APT repository..."
  echo "deb [signed-by=/etc/apt/keyrings/panex.gpg] https://panex-dev.github.io/apt stable main" \
    | sudo tee /etc/apt/sources.list.d/panex.list >/dev/null

  log "installing panex..."
  sudo apt-get update -qq
  if [ -n "$VERSION" ]; then
    deb_version="${VERSION#v}"
    sudo apt-get install -y "panex=${deb_version}"
  else
    sudo apt-get install -y panex
  fi

  installed_version="$(panex version 2>/dev/null || echo "unknown")"
  log ""
  log "panex installed successfully via apt."
  log "  version: ${installed_version}"
  log ""
  log "Future updates: sudo apt update && sudo apt upgrade panex"
  log ""
}

# --- main --------------------------------------------------------------------

main() {
  if [ "$METHOD" = "apt" ]; then
    install_via_apt
    return
  elif [ -n "$METHOD" ]; then
    fatal "unknown method: $METHOD (supported: apt)"
  fi

  need_cmd curl
  need_cmd tar
  need_cmd mktemp
  need_cmd uname

  os="$(detect_os)"
  arch="$(detect_arch)"
  version="$(resolve_version)"

  log ""
  log "panex installer"
  log "  os:      ${os}"
  log "  arch:    ${arch}"
  log "  version: ${version}"
  log ""

  # Determine install directory.
  if [ -n "$INSTALL_DIR" ]; then
    dest="$INSTALL_DIR"
  elif [ -w "$DEFAULT_INSTALL_DIR" ] || command -v sudo >/dev/null 2>&1; then
    dest="$DEFAULT_INSTALL_DIR"
  else
    dest="${HOME}/.local/bin"
  fi

  download_and_verify "$version" "$os" "$arch" "$dest"

  installed_path="${dest}/${BINARY}"
  installed_version="$("$installed_path" version 2>/dev/null || echo "unknown")"

  log ""
  log "panex installed successfully."
  log "  version:  ${installed_version}"
  log "  location: ${installed_path}"

  if ! check_path "$dest"; then
    log ""
    log "NOTE: ${dest} is not in your PATH."
    log "Add it by running:"
    log ""
    log "  export PATH=\"${dest}:\$PATH\""
    log ""
    log "To make this permanent, add the line above to your shell profile"
    log "(~/.bashrc, ~/.zshrc, or ~/.profile)."
  fi

  log ""
  log "Next steps:"
  log "  panex init        # scaffold a starter extension"
  log "  panex dev         # start the development runtime"
  log ""
}

main
