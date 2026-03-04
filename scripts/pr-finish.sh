#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: scripts/pr-finish.sh <branch-name> [worktree-path]" >&2
  exit 1
fi

branch_name="$1"
default_path="/tmp/panex-${branch_name//\//-}"
worktree_path="${2:-$default_path}"

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

git fetch origin main

if ! git branch --format='%(refname:short)' --merged origin/main | grep -Fxq "$branch_name"; then
  echo "branch is not merged into origin/main: $branch_name" >&2
  exit 1
fi

current_branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$current_branch" == "$branch_name" ]]; then
  git checkout main
fi

if git show-ref --verify --quiet "refs/heads/$branch_name"; then
  git branch -d "$branch_name"
fi

if [[ -d "$worktree_path" ]]; then
  git worktree remove "$worktree_path"
fi

git worktree prune

echo "Cleaned up branch '$branch_name'."
echo "You are ready to start the next PR from origin/main."
