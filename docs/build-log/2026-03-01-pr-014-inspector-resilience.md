# PR14 - Inspector Resilience and Runtime Hardening

## Metadata
- Date: 2026-03-01
- PR: 14
- Branch: `feat/inspector-resilience`
- Title: add inspector reconnect, memoized filtering, and error boundary
- Commit(s): pending

## Problem
- Inspector disconnected permanently when daemon transport dropped.
- Timeline filtering re-ran multiple times per render path.
- Initial history query (`200`) did not match local buffer retention (`500`).
- Render errors had no containment boundary.

## Approach (with file+line citations)
- Added reconnect delay utility with bounded exponential policy:
  - `inspector/src/reconnect.ts:1-7`
  - tests in `inspector/tests/reconnect.test.ts:1-22`
- Hardened inspector runtime:
  - switched filtered timeline derivation to `createMemo` in `inspector/src/main.tsx:50-55`
  - added reconnect lifecycle + backoff scheduling in `inspector/src/main.tsx:85-191`
  - aligned query limit with local buffer constant in `inspector/src/main.tsx:132-139`
  - added `ErrorBoundary` fallback shell in `inspector/src/main.tsx:398-417`
- Updated inspector docs to reflect reconnect behavior:
  - `inspector/README.md:15-22`
- Captured architecture decision:
  - `docs/adr/013-inspector-resilience-baseline.md:1-23`

## Risk and Mitigation
- Risk: reconnect loops can become noisy after prolonged outages.
- Mitigation: reconnect backoff is bounded and monotonic (`inspector/src/reconnect.ts:1-7`).
- Risk: fallback rendering could hide actionable failure context.
- Mitigation: boundary surfaces the error string and exposes explicit retry (`inspector/src/main.tsx:398-404`).

## Verification
- Commands:
  - `cd inspector && pnpm run check`
  - `cd inspector && pnpm run test`
  - `cd inspector && pnpm run build`
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Expected:
  - reconnect tests pass
  - inspector bundle remains green
  - full repository quality gates remain green

## Teach-back (engineering lessons)
- Real-time tooling should treat transport drops as a normal state transition, not an exceptional dead-end.
- Derived UI projections should be memoized once they become shared by multiple render paths.
- “Limit constants” must be reused across query and storage surfaces to avoid invisible drift.

## Next Step
- Introduce connection telemetry (last reconnect delay, reconnect count) in UI for better debugging feedback.
