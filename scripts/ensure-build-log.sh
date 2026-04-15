#!/usr/bin/env bash
# ensure-build-log.sh — Verify that every commit being pushed has a
# corresponding build-log entry or is explicitly exempt. Runs as part
# of the pre-push hook.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

build_log_dir="docs/build-log"
status_file="$build_log_dir/STATUS.md"

# Collect commits being pushed that are not yet on the remote.
# pre-push receives lines on stdin: <local ref> <local sha> <remote ref> <remote sha>
while read -r _ local_sha _ remote_sha; do
    # Skip delete pushes
    if [ "$local_sha" = "0000000000000000000000000000000000000000" ]; then
        continue
    fi

    # Determine range: if remote_sha is zero, this is a new branch — check all commits
    if [ "$remote_sha" = "0000000000000000000000000000000000000000" ]; then
        range="$local_sha"
    else
        range="$remote_sha..$local_sha"
    fi

    # Check that at least one build-log file was added or modified in this push range
    build_log_files=$(git diff --name-only "$range" -- "$build_log_dir" 2>/dev/null || true)

    if [ -z "$build_log_files" ]; then
        echo ""
        echo "ERROR: No build-log entry found in push range."
        echo ""
        echo "Every push must include a build-log entry in $build_log_dir/."
        echo "Create a build log documenting what changed and why, then try again."
        echo ""
        echo "To bypass in exceptional cases: git push --no-verify"
        exit 1
    fi
done

exit 0
