# PR72 - Workbench Chrome API Activity Log

## Metadata
- Date: 2026-03-09
- PR: 72
- Branch: `feat/pr72-workbench-chrome-api-activity-log`
- Title: add a focused Workbench chrome API activity log over existing timeline history
- Commit(s):
  - `feat(inspector): add chrome API activity log to Workbench`
  - `docs: record Workbench activity log boundary`

## Problem
- Workbench already exposes runtime-probe and storage tools, but operators still have to read raw timeline traffic to understand what simulator-backed calls actually happened.
- The roadmap still described chrome API visibility as missing, which made the next Workbench increment ambiguous even though the safest product move is a read-only inspector-side summary over existing history.

## Approach (with file+line citations)
- Change 1:
  - Why: derive one focused activity summary from existing `chrome.api.call`, `chrome.api.result`, and `chrome.api.event` timeline entries, including paired call/result latency and unsupported-call classification, without changing protocol or daemon behavior.
  - Where: `inspector/src/activity-log.ts:1-163`
  - Where: `inspector/tests/activity-log.test.ts:1-115`
- Change 2:
  - Why: expose the derived activity list in Workbench so operators can inspect recent runtime probes, tabs queries, failures, and observed runtime events in one place.
  - Where: `inspector/src/tabs/workbench.tsx:1-259`
  - Where: `inspector/src/styles.css:348-395`
- Change 3:
  - Why: record the product boundary explicitly and advance the roadmap to the next runtime-hardening slice instead of silently letting the activity log become a generic timeline replacement.
  - Where: `docs/adr/024-workbench-chrome-api-activity-log-boundary.md:1-34`
  - Where: `docs/build-log/STATUS.md:72-82`
  - Where: `docs/build-log/README.md:42-48`
  - Where: `docs/build-log/2026-03-09-pr-072-workbench-chrome-api-activity-log.md:1-41`

## Risk and Mitigation
- Risk: pairing `chrome.api.call` and `chrome.api.result` incorrectly could mislabel failures or show wrong latency.
- Mitigation: the activity helper derives pairings strictly by `call_id`, classifies unsupported calls from daemon error text, and adds dedicated tests for success, failure, unsupported, pending, and unrelated traffic.
- Risk: the Workbench could start drifting toward a second full timeline surface.
- Mitigation: the ADR and UI copy keep the log scoped to simulator-backed `chrome.api.*` traffic only, and the implementation remains a pure inspector-side summary over existing entries.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm --dir inspector check`
  - `pnpm --dir inspector test`
  - `pnpm --dir inspector build`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - `inspector/tests/activity-log.test.ts` verifies paired success results, unsupported calls, pending calls, standalone events, and unrelated timeline filtering.

## Teach-back
- Design lesson: when a product slice adds visibility, prefer deriving from stable existing history before inventing new transport or storage contracts.
- Testing lesson: summary helpers that pair two message families need negative-path tests for unsupported, failed, and unmatched entries, not just the happy path.
- Team workflow lesson: advancing the roadmap in the same PR keeps the next hardening slice explicit and prevents a read-only UI increment from quietly turning into scope creep.

## Next Step
- Enforce agent-side `hello.ack` completion before the dev-agent accepts live commands so extension behavior depends on a completed handshake instead of optimistic startup.
