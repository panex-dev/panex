#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

git fetch origin main

if git merge-base --is-ancestor origin/main HEAD; then
  echo "Branch includes latest origin/main commit."
  exit 0
fi

echo "Branch is not rebased onto latest origin/main." >&2
echo "Fix with: git fetch origin && git rebase origin/main" >&2
exit 1
