#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: scripts/pr-start.sh <branch-name> [worktree-path]" >&2
  exit 1
fi

branch_name="$1"
default_path="/tmp/panex-${branch_name//\//-}"
worktree_path="${2:-$default_path}"

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

git fetch origin

if git show-ref --verify --quiet "refs/heads/$branch_name"; then
  echo "local branch already exists: $branch_name" >&2
  exit 1
fi

if git ls-remote --exit-code --heads origin "$branch_name" >/dev/null 2>&1; then
  echo "remote branch already exists on origin: $branch_name" >&2
  exit 1
fi

if [[ -e "$worktree_path" ]]; then
  echo "worktree path already exists: $worktree_path" >&2
  exit 1
fi

git worktree add -b "$branch_name" "$worktree_path" origin/main

cat <<EOF
Created branch '$branch_name' from origin/main in a dedicated worktree.
Worktree path: $worktree_path

Next:
  cd $worktree_path
  git status
EOF
