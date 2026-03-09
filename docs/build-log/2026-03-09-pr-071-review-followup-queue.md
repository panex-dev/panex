# PR71 - Review Follow-Up Queue

## Metadata
- Date: 2026-03-09
- PR: 71
- Branch: `docs/pr71-roadmap-followup-queue`
- Title: fold preserved code-review follow-ups into the roadmap queue
- Commit(s):
  - `docs(build-log): queue follow-ups from preserved review`

## Problem
- A pre-sync local `audit.md` backup contained a broad codebase review with a mix of already-resolved findings, stale audit items, and still-relevant follow-up work.
- Leaving those items only in the backup would hide useful next-step candidates from the project’s actual execution tracker, while restoring the whole file would reintroduce obsolete findings and duplicate the active audit tracker.

## Approach (with file+line citations)
- Change 1:
  - Why: keep the current next milestone unchanged while adding a short ordered queue of still-actionable follow-ons from the preserved review.
  - Where: `docs/build-log/STATUS.md:76-87`
- Change 2:
  - Why: mirror that queue in the build-log README so the public “current build check” reflects the same execution order as the status tracker.
  - Where: `docs/build-log/README.md:44-52`
- Change 3:
  - Why: record why the preserved review was not restored verbatim and which items were promoted into the active roadmap queue.
  - Where: `docs/build-log/2026-03-09-pr-071-review-followup-queue.md:1-43`

## Risk and Mitigation
- Risk: the queue could become a second roadmap and compete with the explicit `Next` milestone.
- Mitigation: the entry keeps one concrete `Next` item and places the preserved review items in a separate “queued follow-ons” list behind it.
- Risk: promoting items from an informal review could reintroduce work that is already resolved.
- Mitigation: only unresolved items that still match the current codebase were carried forward; resolved audit items were intentionally excluded.

## Verification
- Commands run:
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Compared `/tmp/panex-audit.md.pre-sync-backup-20260309-1136` against `audit.md` and promoted only still-relevant unresolved items into the roadmap queue.

## Teach-back
- Design lesson: review notes are useful input, but the execution tracker should keep only still-relevant next steps, not the whole narrative around them.
- Testing lesson: even docs-only sequencing changes should still pass the normal repo gates so the tracker itself stays trustworthy.
- Workflow lesson: when rescuing local notes after a sync, promote the actionable remainder into the canonical queue instead of restoring a parallel tracker.

## Next Step
- Implement the Workbench chrome API activity log, then burn down the queued follow-ons in order of runtime safety, operator clarity, and release readiness.
