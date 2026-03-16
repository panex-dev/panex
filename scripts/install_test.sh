#!/bin/sh
# Test suite for install.sh — runs the script in dry-run scenarios to verify
# argument parsing, platform detection, and error handling.
#
# Usage: sh scripts/install_test.sh
#
# Each test function prints PASS or FAIL. The script exits non-zero if any
# test fails.

set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/install.sh"
FAILURES=0
TESTS=0

pass() { TESTS=$((TESTS + 1)); printf "  PASS: %s\n" "$1"; }
fail() { TESTS=$((TESTS + 1)); FAILURES=$((FAILURES + 1)); printf "  FAIL: %s\n" "$1"; }

# --- tests -------------------------------------------------------------------

test_help_flag() {
  output="$(sh "$INSTALL_SCRIPT" --help 2>&1)" || true
  if echo "$output" | grep -q "Usage:"; then
    pass "--help prints usage"
  else
    fail "--help prints usage (got: $output)"
  fi
}

test_help_short_flag() {
  output="$(sh "$INSTALL_SCRIPT" -h 2>&1)" || true
  if echo "$output" | grep -q "Usage:"; then
    pass "-h prints usage"
  else
    fail "-h prints usage (got: $output)"
  fi
}

test_unknown_option_fails() {
  output="$(sh "$INSTALL_SCRIPT" --bogus 2>&1)" && status=0 || status=$?
  if [ "$status" -ne 0 ] && echo "$output" | grep -q "unknown option"; then
    pass "unknown option exits non-zero"
  else
    fail "unknown option exits non-zero (status=$status, got: $output)"
  fi
}

test_version_flag_accepted() {
  # The script will fail at download (no server), but should get past arg parsing.
  output="$(sh "$INSTALL_SCRIPT" --version v99.0.0 2>&1)" && status=0 || status=$?
  if echo "$output" | grep -q "v99.0.0"; then
    pass "--version flag is parsed and displayed"
  else
    fail "--version flag is parsed and displayed (got: $output)"
  fi
}

test_install_dir_flag_accepted() {
  tmp="$(mktemp -d)"
  output="$(sh "$INSTALL_SCRIPT" --version v99.0.0 --install-dir "$tmp" 2>&1)" && status=0 || status=$?
  rm -rf "$tmp"
  if echo "$output" | grep -q "v99.0.0"; then
    pass "--install-dir flag is parsed"
  else
    fail "--install-dir flag is parsed (got: $output)"
  fi
}

test_detects_current_os() {
  # The script should print the detected OS before failing on download.
  output="$(sh "$INSTALL_SCRIPT" --version v99.0.0 2>&1)" && status=0 || status=$?
  expected_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$expected_os" in
    linux|darwin) ;;
    *) expected_os="unsupported" ;;
  esac
  if echo "$output" | grep -q "os:.*${expected_os}"; then
    pass "detects current OS ($expected_os)"
  else
    fail "detects current OS (expected $expected_os in: $output)"
  fi
}

test_detects_current_arch() {
  output="$(sh "$INSTALL_SCRIPT" --version v99.0.0 2>&1)" && status=0 || status=$?
  machine="$(uname -m)"
  case "$machine" in
    x86_64|amd64)  expected_arch="amd64" ;;
    aarch64|arm64) expected_arch="arm64" ;;
    *) expected_arch="unsupported" ;;
  esac
  if echo "$output" | grep -q "arch:.*${expected_arch}"; then
    pass "detects current architecture ($expected_arch)"
  else
    fail "detects current architecture (expected $expected_arch in: $output)"
  fi
}

test_download_failure_message() {
  # With a fake version, curl should fail and the script should report it.
  output="$(sh "$INSTALL_SCRIPT" --version v0.0.0-nonexistent 2>&1)" && status=0 || status=$?
  if [ "$status" -ne 0 ] && echo "$output" | grep -q "download failed"; then
    pass "reports download failure cleanly"
  else
    fail "reports download failure cleanly (status=$status, got: $output)"
  fi
}

# --- runner ------------------------------------------------------------------

main() {
  printf "install.sh tests\n\n"

  test_help_flag
  test_help_short_flag
  test_unknown_option_fails
  test_version_flag_accepted
  test_install_dir_flag_accepted
  test_detects_current_os
  test_detects_current_arch
  test_download_failure_message

  printf "\n%d tests, %d failures\n" "$TESTS" "$FAILURES"
  if [ "$FAILURES" -gt 0 ]; then
    exit 1
  fi
}

main
