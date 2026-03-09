# PR70 - Replay Contract Boundary

## Metadata
- Date: 2026-03-09
- PR: 70
- Branch: `feat/pr70-replay-contract-boundary`
- Title: centralize the replay contract boundary and keep replay scoped to runtime probes
- Commit(s):
  - `refactor(inspector): centralize replay contract boundary`
  - `docs: record replay contract boundary`

## Problem
- Replay eligibility was duplicated across `inspector/src/workbench.ts` and `inspector/src/replay.ts`, which meant the product boundary lived in two separate code paths instead of one explicit contract.
- The status tracker still described replay scope as an unresolved question even though the safest current product decision is to keep replay constrained until another family earns its own contract.

## Approach (with file+line citations)
- Change 1:
  - Why: add one inspector-side replay contract module so Replay history and Workbench replay selection derive eligibility from the same runtime-probe family boundary.
  - Where: `inspector/src/replay-contract.ts:1-107`
  - Where: `inspector/src/replay.ts:1-39`
  - Where: `inspector/src/workbench.ts:178-198`
  - Where: `inspector/src/workbench.ts:352-390`
- Change 2:
  - Why: make the narrow replay scope visible in the Replay and Workbench copy so the UI does not imply that arbitrary message families are replay-safe.
  - Where: `inspector/src/tabs/replay.tsx:1-82`
  - Where: `inspector/src/tabs/workbench.tsx:217-233`
- Change 3:
  - Why: pin the replay-family boundary with dedicated tests for accepted and rejected history entries.
  - Where: `inspector/tests/replay-contract.test.ts:1-133`
  - Where: `inspector/tests/replay.test.ts:1-76`
  - Where: `inspector/tests/workbench.test.ts:153-229`
- Change 4:
  - Why: record the design decision and advance the status tracker to a concrete next milestone.
  - Where: `docs/adr/023-replay-contract-boundary.md:1-35`
  - Where: `docs/build-log/STATUS.md:68-77`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/2026-03-09-pr-070-replay-contract-boundary.md:1-66`

## Risk and Mitigation
- Risk: centralizing replay parsing could accidentally change Workbench replay selection or Replay history ordering.
- Mitigation: the PR keeps the existing runtime-probe-only contract, preserves newest-first selection, and adds dedicated contract tests alongside the existing Replay and Workbench tests.
- Risk: a documentation-only “decision” could drift from the actual implementation.
- Mitigation: the ADR is paired with a real shared module that both replay surfaces now consume.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm --dir inspector test`
  - `pnpm --dir inspector build`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - `inspector/tests/replay-contract.test.ts` verifies that only the runtime-probe family is replayable today.

## Teach-back
- Design lesson: when a product surface is intentionally narrow, encode the boundary in one shared contract instead of letting two helpers “agree by accident.”
- Testing lesson: contract tests are the right place to pin what is intentionally excluded, not just what is currently accepted.
- Team workflow lesson: once a status tracker question is answered in code, update the next target immediately so the roadmap does not keep pointing at a solved ambiguity.

## Next Step
- Add a focused Workbench chrome API activity log over existing timeline history so operators can inspect runtime probes, tabs queries, and unsupported calls in one place without widening protocol scope.
