#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

git config core.hooksPath .githooks
echo "Configured core.hooksPath=.githooks"
echo "Pre-push checks will now enforce branch base hygiene."
