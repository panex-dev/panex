# Agent Build Protocol

This file is the mandatory operating contract for any coding agent working in this repository, including Codex, Claude Code, and similar CLI or app-based agents.

If this file conflicts with convenience, speed, or token-saving behavior, this file wins.

## Non-Negotiables

1. Start every implementation PR from the latest `origin/main` in its own dedicated worktree.
2. Never build on top of another open PR branch.
3. Never claim a check passed unless the agent actually ran it or observed the passing CI result.
4. Never bypass, weaken, skip, mute, or delete tests just to get green.
5. If a test is wrong or flaky, fix the test correctly and explain why.
6. Merge only after every required check is green.
7. Keep PRs narrow, reviewable, and tied to one milestone.
8. If a merged change breaks downstream behavior, prefer revert-first over speculative fix-forward unless the user explicitly directs otherwise.

## Required Sequence For Every PR

1. Sync first.
   - Run `git fetch origin`.
   - Confirm the next milestone from `docs/build-log/STATUS.md`, the active roadmap, or explicit user direction.
   - If the repo is on `main`, leave unrelated local files alone.
2. Start cleanly.
   - Create a dedicated worktree from latest `origin/main` with `./scripts/pr-start.sh <branch-name>`.
   - Work only inside that PR worktree.
   - Do not reuse an old feature branch.
3. Understand before editing.
   - Read the relevant code, tests, docs, and prior build-log entries.
   - Identify the smallest correct slice that advances the milestone.
   - Prefer fixing the real cause over adding special cases.
4. Implement with discipline.
   - Preserve existing product boundaries unless the PR explicitly changes them.
   - Keep build/test behavior honest; no fake stubs, no hidden fallbacks, no “temporary” bypasses.
   - Add or update negative-path tests in the same PR when behavior changes.
   - Keep commit history reviewable: prefer one clean commit for a small PR, or a few atomic commits when they materially improve review clarity.
5. Verify locally.
   - Run the relevant package checks and the root quality gates.
   - Minimum default expectation unless the change is truly narrower:
     - `make fmt`
     - `make check`
     - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
     - `GOCACHE=/tmp/go-build make test`
     - `GOCACHE=/tmp/go-build make build`
   - If sandbox limits block a real check, rerun the same check with the required permissions. Do not downgrade the check.
   - Treat test isolation as part of correctness: a result that depends on order, leaked state, or previous runs is not a valid pass.
6. Verify branch hygiene.
   - Run `./scripts/pr-ensure-rebased.sh` before push.
   - If it fails, rebase on latest `origin/main` and rerun verification as needed.
7. Prepare the PR properly.
   - Fill the full `.github/PULL_REQUEST_TEMPLATE.md`.
   - The required sections are: `Problem`, `Approach`, `Risk and mitigation`, `Verification`, `Teach-back`, `Branch base guard`, and `Protocol parity impact` when protocol-related files or behavior are affected.
   - Do not satisfy template requirements with filler text or empty compliance boilerplate.
   - Include exact file and line citations in the `Approach` section.
   - Record exact commands used in `Verification`.
   - Update `docs/build-log/STATUS.md` and add a build-log entry for product/code increments.
   - Process-only PRs may skip the build log only if the change is genuinely process-only and that fact is explicit.
8. Push and watch CI.
   - Push only after local verification is complete.
   - Watch the actual GitHub checks.
   - If CI fails, inspect logs, fix the root cause, rerun the relevant local checks, push again, and rewatch CI.
9. Merge only when clean.
   - Merge only if PR state is mergeable and every required check is passing.
   - Never merge on red, partial, stale, or assumed green.
10. Handle post-merge breakage explicitly.
   - If a merged change causes regressions on `main`, default to revert-first to restore a known-good state.
   - Only choose fix-forward when the user explicitly wants it or when the fix is already proven, minimal, and lower risk than reverting.
   - Do not stack an unverified emergency fix on top of a broken mainline and call that closure.

## Must Not Do

- Do not stack branches.
- Do not merge with failing or pending required checks.
- Do not mark work “done” when only local assumptions exist.
- Do not hide failing tests behind retries, sleeps, conditionals, `skip`, or reduced coverage.
- Do not rewrite unrelated files to make a diff look cleaner.
- Do not delete user changes or untracked files unless explicitly asked.
- Do not broaden scope just because a refactor is tempting.

## Test Truth Standard

- A passing result must be real.
- If a command fails because of environment restrictions, say so precisely and rerun it correctly if permissions allow.
- If a test is invalid, fix the test and preserve the product guarantee it was supposed to protect.
- If the result depends on execution order, cached artifacts, leaked ports, or leftover state, the test is not passing correctly.
- “Passes locally except CI” is not done.
- “CI is probably fine” is not done.

## PR Quality Standard

Every PR must make review easy:

- clear problem statement
- smallest viable change
- exact citations
- explicit risks
- real verification
- one short teach-back with reusable engineering lessons for future reviewers and future agents reading the build log

If the PR cannot be explained clearly, the design is probably not ready.

## Commit Hygiene Standard

- Prefer one clean commit for a small, single-purpose PR.
- Use multiple commits only when each commit is atomic and reviewable on its own.
- Do not leave behind noisy checkpoint, WIP, or “fix ci” commit chains if they can be cleanly consolidated before merge.
- Do not rewrite published history unless the user explicitly asks for it.

## Long-Term Design Standard

- Centralize only the stable layer.
- Keep package-specific runtime/build behavior local until repeated patterns are proven, not guessed.
- Prefer explicit contracts over hidden coupling.
- Make the next milestone easier, not merely the current diff smaller.

## Merge Standard

Before merging, the agent must be able to state all of the following truthfully:

- The branch started from latest `origin/main`.
- The PR is not stacked on another open PR.
- Local verification was run at the appropriate depth.
- CI passed on the final pushed commit.
- The PR template is fully filled.
- The change does not rely on bypassed tests or weakened checks.

If any item above is false, do not merge.
