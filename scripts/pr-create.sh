#!/usr/bin/env bash
# Safe wrapper around `gh pr create` that enforces main as the base branch.
#
# Usage: scripts/pr-create.sh [gh pr create flags...]
#
# This script prevents the mistake of creating PRs that target feature branches
# instead of main. All additional arguments are forwarded to `gh pr create`.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

current_branch="$(git rev-parse --abbrev-ref HEAD)"

if [[ "$current_branch" == "main" ]]; then
  echo "error: you are on main — switch to a feature branch first." >&2
  exit 1
fi

# Ensure the branch is rebased on origin/main before creating the PR.
./scripts/pr-ensure-rebased.sh

# Enforce --base main. If the caller already passed --base, reject it to avoid
# confusion — the base is always main.
for arg in "$@"; do
  if [[ "$arg" == "--base" || "$arg" == -b ]]; then
    echo "error: do not pass --base; PRs always target main." >&2
    exit 1
  fi
done

echo "Creating PR: $current_branch → main"
gh pr create --base main "$@"
